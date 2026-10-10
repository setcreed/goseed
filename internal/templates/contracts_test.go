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
	for _, backend := range []string{"sqlite", "mysql", "postgres", "none"} {
		t.Run(backend, func(t *testing.T) { checkDeliveredContracts(t, backend) })
	}
}

func checkDeliveredContracts(t *testing.T, backend string) {
	t.Helper()
	plan, err := Render(Project{Name: "demo", Module: "example.com/demo"}, Options{Database: backend})
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
		Go         string            `json:"go"`
		Runtime    map[string]string `json:"runtime"`
		Builder    string            `json:"service_builder_image"`
		Connectors map[string]string `json:"sql_connectors"`
	}
	if err := json.Unmarshal(data, &lock); err != nil {
		t.Fatal(err)
	}
	for name, version := range lock.Connectors {
		lock.Runtime[name] = version
	}
	manifest := string(files["go.mod"])
	if backend == "none" {
		for filename, content := range files {
			for _, prefix := range []string{"migrations/", "internal/pkg/database/", "internal/apiserver/model/", "internal/apiserver/store/", "internal/testutil/"} {
				if strings.HasPrefix(filename, prefix) {
					t.Fatalf("database file delivered in none project: %s", filename)
				}
			}
			if filename == "go.mod" || filename == "go.sum" || strings.HasSuffix(filename, ".go") {
				for _, dependency := range []string{"gorm.io/", "modernc.org/", "github.com/libtnb/sqlite", "github.com/pressly/goose", "github.com/go-sql-driver/mysql", "github.com/jackc/pgx"} {
					if strings.Contains(string(content), dependency) {
						t.Fatalf("database dependency leaked in %s: %s", filename, dependency)
					}
				}
			}
		}
		for filename, forbidden := range map[string]string{"configs/app.example.yaml": "database:", "build/docker/Dockerfile": "/app/data", "Makefile": "migrate", "cmd/apiserver/app/app.go": "migrate"} {
			if strings.Contains(string(files[filename]), forbidden) {
				t.Fatalf("database entry leaked in %s", filename)
			}
		}
	}
	for driver, selected := range map[string]bool{"github.com/libtnb/sqlite": backend == "sqlite", "modernc.org/sqlite": backend == "sqlite", "gorm.io/driver/mysql": backend == "mysql", "github.com/go-sql-driver/mysql": backend == "mysql", "gorm.io/driver/postgres": backend == "postgres", "github.com/jackc/pgx/v5": backend == "postgres"} {
		if strings.Contains(manifest, driver+" ") != selected {
			t.Fatalf("incorrect driver composition for %s: %s", backend, driver)
		}
	}
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
	expectedRoutes := []string{"/healthz", "/readyz", "/v1/examples"}
	if backend == "none" {
		if len(routes) != 3 || len(routes["/v1/examples"].(map[string]any)) != 1 {
			t.Fatal("none document includes unsupported routes or methods")
		}
	} else {
		expectedRoutes = append(expectedRoutes, "/v1/examples/{id}")
	}
	for _, route := range expectedRoutes {
		if _, ok := routes[route]; !ok {
			t.Fatalf("missing documented route %s", route)
		}
	}
}
