package templates

import (
	"bytes"
	"strings"
	"testing"

	"github.com/setcreed/goseed/internal/openapi"
)

func TestResourceCoreRendering(t *testing.T) {
	project := Project{Name: "shop", Module: "example.com/shop"}
	for _, fields := range [][]ResourceField{
		{{SchemaField: SchemaField{Name: "select", Type: "string", MaxLength: 3}, Required: true}, {SchemaField: SchemaField{Name: "enabled", Type: "bool"}}, {SchemaField: SchemaField{Name: "count", Type: "int64"}}},
		{{SchemaField: SchemaField{Name: "flag", Type: "bool"}}},
		{{SchemaField: SchemaField{Name: "number", Type: "int64"}, Required: true}},
	} {
		for _, database := range []string{"sqlite", "mysql", "postgres"} {
			for _, redis := range []bool{false, true} {
				files, err := RenderResourceCore(project, Options{Database: database, Redis: redis}, "orders", fields)
				if err != nil || len(files) != 6 {
					t.Fatal("core source contract", database, redis, err)
				}
				for _, file := range files {
					if bytes.Contains(file.Content, []byte("[[")) || strings.HasPrefix(file.Path, "api/") {
						t.Fatal("unrendered or protocol-specific core file", file.Path)
					}
				}
			}
		}
	}
	for _, name := range []string{"i_d", "table_name"} {
		if _, err := RenderResourceCore(project, Options{}, "orders", []ResourceField{{SchemaField: SchemaField{Name: name, Type: "bool"}}}); err == nil {
			t.Fatal("reserved Go name accepted", name)
		}
	}
}

func TestResourceHTTPRendering(t *testing.T) {
	project := Project{Name: "shop", Module: "example.com/shop"}
	fields := []ResourceField{{SchemaField: SchemaField{Name: "display_name", Type: "string", MaxLength: 30}, Required: true}, {SchemaField: SchemaField{Name: "enabled", Type: "bool"}}}
	for _, transport := range []string{"http", "dual"} {
		files, err := RenderResourceHTTP(project, Options{Database: "sqlite", Transport: transport, Authz: "casbin", Redis: true}, "orders", fields)
		if err != nil || len(files) != 5 {
			t.Fatalf("HTTP render %s: %d files, %v", transport, len(files), err)
		}
		byPath := map[string][]byte{}
		for _, file := range files {
			byPath[file.Path] = file.Content
		}
		for _, path := range []string{"internal/apiserver/handler/http/orders/handler.go", "internal/apiserver/bootstrap/module_orders.go", "docs/orders.md", "api/openapi/orders.yaml"} {
			if len(byPath[path]) == 0 {
				t.Fatalf("missing %s", path)
			}
		}
		handler := string(byPath["internal/apiserver/handler/http/orders/handler.go"])
		for _, part := range []string{"biz.UpdateInput", "duplicate or invalid JSON field", "unknown JSON field", "null fields are not supported", "page_size", "http.StatusCreated"} {
			if !strings.Contains(handler, part) {
				t.Fatalf("handler missing %q", part)
			}
		}
		factory := string(byPath["internal/apiserver/bootstrap/module_orders.go"])
		for _, part := range []string{"CheckSchema(dependencies.Context)", "ordersPermissions().HTTP", "business.New(repository, dependency)"} {
			if !strings.Contains(factory, part) {
				t.Fatalf("factory missing %q", part)
			}
		}
		openAPI := string(byPath["api/openapi/orders.yaml"])
		for _, part := range []string{"/v1/orders:", "/v1/orders/{id}:", "additionalProperties: false", "required: [id, display_name, enabled, created_at, updated_at]", "required: [items, total, page, page_size]"} {
			if !strings.Contains(openAPI, part) {
				t.Fatalf("OpenAPI missing %q", part)
			}
		}
		guide := string(byPath["docs/orders.md"])
		for _, part := range []string{`${TOKEN:?set TOKEN first}`, `${API_URL:=http://127.0.0.1:8080}`, `curl --fail --silent --show-error`, `"Authorization: Bearer ${TOKEN}"`, `"display_name":"xxxxxxxx"`} {
			if !strings.Contains(guide, part) {
				t.Fatalf("resource guide missing %q", part)
			}
		}
		merged, err := openapi.Merge([]openapi.Source{{Name: "example", YAML: []byte("swagger: \"2.0\"\npaths:\n  /examples:\n    get:\n      operationId: exampleGet\n")}, {Name: "orders", YAML: byPath["api/openapi/orders.yaml"]}})
		if err != nil || len(merged.JSON) == 0 {
			t.Fatalf("merge source: %v", err)
		}
		if _, err := openapi.Merge([]openapi.Source{{Name: "example", YAML: []byte("swagger: \"2.0\"\npaths:\n  /v1/orders:\n    get:\n      operationId: old\n")}, {Name: "orders", YAML: byPath["api/openapi/orders.yaml"]}}); err == nil {
			t.Fatal("path collision accepted")
		}
	}
	if _, err := RenderResourceHTTP(project, Options{Database: "sqlite", Transport: "grpc"}, "orders", fields); err == nil {
		t.Fatal("HTTP rendering accepted gRPC-only project")
	}
}
