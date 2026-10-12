package templates

import (
	"bytes"
	"strings"
	"testing"
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
