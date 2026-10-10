package templates

import (
	"encoding/json"
	"os"
	"path"
	"regexp"
	"strings"
	"testing"
)

func TestDeliveredContracts(t *testing.T) {
	plan, err := Default(Project{Name: "demo", Module: "example.com/demo"})
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for _, file := range plan {
		files[file.Path] = file.Content
	}
	data, err := os.ReadFile("../../dependencies.lock.json")
	if err != nil {
		t.Fatal(err)
	}
	var lock struct {
		Go      string            `json:"go"`
		Runtime map[string]string `json:"runtime"`
		Builder string            `json:"service_builder_image"`
	}
	if err := json.Unmarshal(data, &lock); err != nil {
		t.Fatal(err)
	}
	manifest := string(files["go.mod"])
	if !strings.Contains(manifest, "go "+lock.Go+"\n") {
		t.Fatal("template toolchain differs from lock")
	}
	for _, line := range strings.Split(manifest, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.HasPrefix(fields[1], "v") {
			if lock.Runtime[fields[0]] != fields[1] {
				t.Fatalf("direct dependency differs from audited lock: %s", fields[0])
			}
		}
	}
	if !strings.Contains(string(files["build/docker/Dockerfile"]), "FROM "+lock.Builder+" AS builder") {
		t.Fatal("Docker builder differs from lock")
	}
	links := regexp.MustCompile(`\[[^\]]*\]\(([^)]+)\)`)
	for filename, content := range files {
		if !strings.HasSuffix(filename, ".md") {
			continue
		}
		for _, link := range links.FindAllSubmatch(content, -1) {
			target := string(link[1])
			if strings.Contains(target, "://") {
				continue
			}
			target, _, _ = strings.Cut(target, "#")
			if _, ok := files[path.Clean(path.Join(path.Dir(filename), target))]; !ok {
				t.Fatalf("%s links to missing file %s", filename, target)
			}
		}
	}
	var swagger map[string]any
	if err := json.Unmarshal(files["api/openapi/swagger.json"], &swagger); err != nil {
		t.Fatal(err)
	}
	if swagger["swagger"] != "2.0" {
		t.Fatal("API document must be Swagger 2.0")
	}
	var checkRefs func(any)
	checkRefs = func(value any) {
		switch value := value.(type) {
		case map[string]any:
			if raw, ok := value["$ref"].(string); ok {
				if !strings.HasPrefix(raw, "#/") {
					t.Fatal("unexpected external Swagger reference")
				}
				current := any(swagger)
				for _, segment := range strings.Split(strings.TrimPrefix(raw, "#/"), "/") {
					object, ok := current.(map[string]any)
					if !ok {
						t.Fatalf("invalid Swagger reference %s", raw)
					}
					current, ok = object[segment]
					if !ok {
						t.Fatalf("missing Swagger reference %s", raw)
					}
				}
			}
			for _, child := range value {
				checkRefs(child)
			}
		case []any:
			for _, child := range value {
				checkRefs(child)
			}
		}
	}
	checkRefs(swagger)
	routes, ok := swagger["paths"].(map[string]any)
	if !ok {
		t.Fatal("Swagger paths missing")
	}
	for _, route := range []string{"/healthz", "/readyz", "/v1/examples", "/v1/examples/{id}"} {
		if _, ok := routes[route]; !ok {
			t.Fatalf("missing documented route %s", route)
		}
	}
}
