package templates

import (
	"encoding/json"
	"testing"

	"github.com/setcreed/goseed/internal/openapi"
)

func TestResourceOpenAPIFieldContracts(t *testing.T) {
	files, err := RenderResourceHTTP(Project{Name: "demo", Module: "example.com/demo"}, Options{Database: "sqlite", Transport: "http"}, "item", []ResourceField{
		{SchemaField: SchemaField{Name: "amount", Type: "int64"}},
		{SchemaField: SchemaField{Name: "title", Type: "string", MaxLength: 80}},
		{SchemaField: SchemaField{Name: "enabled", Type: "bool"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var source []byte
	for _, file := range files {
		if file.Path == "api/openapi/item.yaml" {
			source = file.Content
		}
	}
	merged, err := openapi.Merge([]openapi.Source{{Name: "item", YAML: source}})
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Definitions map[string]struct{ Properties map[string]map[string]any }
	}
	if err := json.Unmarshal(merged.JSON, &document); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"CreateItemRequest", "UpdateItemRequest", "ItemItem"} {
		properties := document.Definitions[name].Properties
		if properties["amount"]["type"] != "integer" || properties["amount"]["format"] != "int64" {
			t.Errorf("%s amount does not preserve int64: %v", name, properties["amount"])
		}
		if properties["title"]["type"] != "string" || properties["title"]["maxLength"] != float64(80) {
			t.Errorf("%s title contract changed: %v", name, properties["title"])
		}
		if properties["enabled"]["type"] != "boolean" {
			t.Errorf("%s bool contract changed", name)
		}
	}
}
