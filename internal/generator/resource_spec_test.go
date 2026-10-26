package generator

import (
	"strings"
	"testing"
)

func TestParseResourceSpecAppliesDefaultsAndPreservesFieldOrder(t *testing.T) {
	spec, err := ParseResourceSpec([]byte("version: 1\nfields:\n  - name: display_name\n    type: string\n    required: true\n    max_length: 120\n  - name: enabled\n    type: bool\n  - name: retry_count\n    type: int64\n"))
	if err != nil {
		t.Fatal(err)
	}
	if spec.Version != 1 || len(spec.Fields) != 3 {
		t.Fatalf("unexpected spec: %+v", spec)
	}
	if spec.Fields[0] != (ResourceField{Name: "display_name", GoName: "DisplayName", Type: "string", Required: true, MaxLength: 120}) {
		t.Fatalf("unexpected first field: %+v", spec.Fields[0])
	}
	if spec.Fields[1] != (ResourceField{Name: "enabled", GoName: "Enabled", Type: "bool"}) {
		t.Fatalf("unexpected bool field defaults: %+v", spec.Fields[1])
	}
	if spec.Fields[2].Name != "retry_count" || spec.Fields[2].Type != "int64" {
		t.Fatalf("field order or type changed: %+v", spec.Fields[2])
	}
}

func TestParseResourceSpecFieldSegmentStartingWithDigit(t *testing.T) {
	spec, err := ParseResourceSpec([]byte("version: 1\nfields:\n  - name: foo_1\n    type: string\n"))
	if err != nil {
		t.Fatal(err)
	}
	if spec.Fields[0].GoName != "Foo1" {
		t.Fatalf("business Go name: got %q, want Foo1", spec.Fields[0].GoName)
	}
}

func TestParseResourceSpecRejectsInvalidDocuments(t *testing.T) {
	valid := "version: 1\nfields:\n  - name: title\n    type: string\n"
	cases := map[string]string{
		"invalid UTF-8":            string([]byte{0xff}),
		"multiple documents":       valid + "---\nversion: 1\nfields:\n  - name: other\n    type: bool\n",
		"duplicate root key":       "version: 1\nversion: 1\nfields:\n  - name: title\n    type: string\n",
		"duplicate field key":      "version: 1\nfields:\n  - name: title\n    name: other\n    type: string\n",
		"unknown root key":         valid + "extra: true\n",
		"unknown field key":        strings.Replace(valid, "type: string", "type: string\n    default: hi", 1),
		"wrong version":            strings.Replace(valid, "version: 1", "version: 2", 1),
		"string version":           strings.Replace(valid, "version: 1", "version: \"1\"", 1),
		"empty fields":             "version: 1\nfields: []\n",
		"invalid field name":       strings.Replace(valid, "title", "Title", 1),
		"reserved field":           strings.Replace(valid, "title", "owner", 1),
		"unsupported type":         strings.Replace(valid, "string", "float", 1),
		"required string":          strings.Replace(valid, "type: string", "type: string\n    required: yes", 1),
		"null field":               strings.Replace(valid, "type: string", "type: string\n    required: null", 1),
		"max length on bool":       "version: 1\nfields:\n  - name: enabled\n    type: bool\n    max_length: 10\n",
		"fractional max length":    "version: 1\nfields:\n  - name: title\n    type: string\n    max_length: 1.5\n",
		"max length out of bounds": "version: 1\nfields:\n  - name: title\n    type: string\n    max_length: 4097\n",
		"anchor":                   "version: 1\nfields:\n  - name: &name title\n    type: string\n",
		"alias":                    "version: 1\nfields:\n  - name: title\n    type: *name\n",
		"merge key":                "version: 1\nfields:\n  - <<: {name: title}\n    type: string\n",
		"custom tag":               "version: 1\nfields:\n  - name: !custom title\n    type: string\n",
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseResourceSpec([]byte(input)); err == nil {
				t.Fatal("invalid resource spec was accepted")
			}
		})
	}
}

func TestParseResourceSpecRejectsDuplicateFieldsAndInvalidBounds(t *testing.T) {
	cases := []string{
		"version: 1\nfields:\n  - name: title\n    type: string\n  - name: title\n    type: bool\n",
		"version: 1\nfields:\n  - name: title\n    type: string\n    max_length: 0\n",
		"version: 1\nfields:\n  - name: title\n    type: int64\n    max_length: 1\n",
	}
	for _, input := range cases {
		if _, err := ParseResourceSpec([]byte(input)); err == nil {
			t.Fatalf("invalid spec was accepted: %s", input)
		}
	}
	fields := strings.Repeat("  - name: f\n    type: bool\n", 33)
	if _, err := ParseResourceSpec([]byte("version: 1\nfields:\n" + fields)); err == nil {
		t.Fatal("more than 32 fields were accepted")
	}
}
