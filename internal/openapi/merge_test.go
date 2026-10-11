package openapi

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

func TestMergeIsStableAndCombinesModulePaths(t *testing.T) {
	sources := []Source{
		{Name: "zeta", YAML: []byte(`swagger: "2.0"
info: {title: Shop, version: v1}
basePath: /
paths:
  /v1/items:
    post: {operationId: createItem, responses: {"200": {description: ok}}}
  /v1/zeta:
    get: {operationId: getZeta, responses: {"200": {description: ok}}}
definitions:
  Shared: {type: object}
responses:
  Failure: {description: failure}
tags:
  - {name: zeta}
`)},
		{Name: "alpha", YAML: []byte(`swagger: "2.0"
info: {title: Shop, version: v1}
basePath: /
paths:
  /v1/items:
    get: {operationId: listItems, responses: {"200": {description: ok}}}
  /v1/alpha:
    get: {operationId: getAlpha, responses: {"200": {description: ok}}}
definitions:
  Shared: {type: object}
responses:
  Failure: {description: failure}
tags:
  - {name: alpha}
`)},
	}
	first, err := Merge(sources)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Merge([]Source{sources[1], sources[0]})
	if err != nil {
		t.Fatal(err)
	}
	if string(first.JSON) != string(second.JSON) || first.Digest != second.Digest {
		t.Fatal("source order changed merged output")
	}
	if !json.Valid(first.JSON) {
		t.Fatalf("merge output is not JSON: %s", first.JSON)
	}
	var document struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
		Tags  []struct {
			Name string `json:"name"`
		} `json:"tags"`
	}
	if err := json.Unmarshal(first.JSON, &document); err != nil {
		t.Fatal(err)
	}
	if len(document.Paths) != 3 || len(document.Paths["/v1/items"]) != 2 {
		t.Fatalf("paths were not combined: %#v", document.Paths)
	}
	if len(document.Tags) != 2 || document.Tags[0].Name != "alpha" || document.Tags[1].Name != "zeta" {
		t.Fatalf("tags are not in stable name order: %#v", document.Tags)
	}
	digest := sha256.Sum256(first.JSON)
	if first.Digest != hex.EncodeToString(digest[:]) {
		t.Fatal("digest does not cover exact JSON output")
	}
}

func TestMergeRejectsConflicts(t *testing.T) {
	base := `swagger: "2.0"
info: {title: Shop, version: v1}
paths:
  /v1/items:
    get: {operationId: listItems}
definitions:
  Shared: {type: object}
`
	tests := []struct {
		name   string
		second string
		want   string
	}{
		{name: "duplicate operation", second: base, want: "duplicates operation"},
		{name: "duplicate operationId", second: "swagger: \"2.0\"\ninfo: {title: Shop, version: v1}\npaths:\n  /v1/other:\n    get: {operationId: listItems}\n", want: "operationId \"listItems\" is shared"},
		{name: "definition conflict", second: "swagger: \"2.0\"\ninfo: {title: Shop, version: v1}\npaths: {}\ndefinitions:\n  Shared: {type: string}\n", want: "conflicts on definitions"},
		{name: "top-level mismatch", second: "swagger: \"2.0\"\ninfo: {title: Other, version: v1}\npaths: {}\n", want: "conflicts on top-level field \"info\""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Merge([]Source{{Name: "first", YAML: []byte(base)}, {Name: "second", YAML: []byte(test.second)}})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("got %v, want error containing %q", err, test.want)
			}
		})
	}
}

func TestMergeRejectsInvalidSources(t *testing.T) {
	valid := "swagger: \"2.0\"\npaths: {}\n"
	tests := []struct {
		name, source, want string
	}{
		{name: "duplicate key", source: "swagger: \"2.0\"\nswagger: \"2.0\"\npaths: {}\n", want: "duplicate YAML key"},
		{name: "alias", source: "defaults: &defaults {type: object}\nswagger: \"2.0\"\npaths: {}\ndefinitions: {Thing: *defaults}\n", want: "anchors and aliases"},
		{name: "merge key", source: "swagger: \"2.0\"\npaths: {}\nx-data: {<<: {key: value}}\n", want: "merge keys"},
		{name: "multiple documents", source: valid + "---\nswagger: \"2.0\"\npaths: {}\n", want: "only one YAML document"},
		{name: "wrong version", source: "swagger: \"3.0.0\"\npaths: {}\n", want: "swagger: \"2.0\""},
		{name: "custom YAML tag", source: "swagger: \"2.0\"\npaths: {}\nx-value: !custom data\n", want: "YAML tag"},
		{name: "missing paths", source: "swagger: \"2.0\"\n", want: "paths mapping"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Merge([]Source{{Name: "valid", YAML: []byte(test.source)}})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("got %v, want error containing %q", err, test.want)
			}
		})
	}
	if _, err := Merge([]Source{{Name: "first", YAML: []byte(valid)}, {Name: "first", YAML: []byte(valid)}}); err == nil {
		t.Fatal("duplicate module name accepted")
	}
	if _, err := Merge([]Source{{Name: "bad-name", YAML: []byte(valid)}}); err == nil {
		t.Fatal("invalid module name accepted")
	}
	if _, err := Merge(nil); err == nil {
		t.Fatal("empty source list accepted")
	}
}
