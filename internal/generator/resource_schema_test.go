package generator

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/setcreed/goseed/internal/templates"
)

func TestResourceSchemaDraft(t *testing.T) {
	for _, database := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(database, func(t *testing.T) {
			root := resourcePlanProject(t, templates.Options{Database: database}, false)
			before := resourceTree(t, root)
			plan, err := PrepareResource(root, "orders", ResourceOptions{Spec: "specs/orders.yaml", Migration: true})
			if err != nil {
				t.Fatal(err)
			}
			draft, ok := plan.MigrationDraft()
			if !ok || draft.Path != "migrations/00002_orders.sql" || !bytes.Contains(draft.Content, []byte("-- +goose Down")) {
				t.Fatal("migration draft missing", draft.Path)
			}
			original := bytes.Clone(draft.Content)
			draft.Content[0] = 'X'
			fresh, _ := plan.MigrationDraft()
			if !bytes.Equal(fresh.Content, original) {
				t.Fatal("caller changed SQL draft")
			}
			store := plan.SchemaStore()
			core := plan.CoreFiles()
			if len(core) != 6 {
				t.Fatal("business core missing")
			}
			first := bytes.Clone(core[0].Content)
			core[0].Content[0] = 'X'
			if !bytes.Equal(plan.CoreFiles()[0].Content, first) {
				t.Fatal("caller changed business sources")
			}
			httpFiles := plan.HTTPFiles()
			if len(httpFiles) != 5 {
				t.Fatalf("HTTP files missing: %d", len(httpFiles))
			}
			var handler bool
			for _, file := range httpFiles {
				if file.Path == "internal/apiserver/handler/http/orders/handler.go" {
					handler = true
				}
			}
			if !handler {
				t.Fatal("resource HTTP handler missing")
			}
			firstHTTP := bytes.Clone(httpFiles[0].Content)
			httpFiles[0].Content[0] = 'X'
			if !bytes.Equal(plan.HTTPFiles()[0].Content, firstHTTP) {
				t.Fatal("caller changed HTTP sources")
			}
			if !bytes.Contains(store.Content, []byte("CheckResourceSchema(ctx, s.db")) {
				t.Fatal("read-only schema store missing")
			}
			original = bytes.Clone(store.Content)
			store.Content[0] = 'X'
			if !bytes.Equal(plan.SchemaStore().Content, original) {
				t.Fatal("caller changed schema store")
			}
			without, err := PrepareResource(root, "orders", ResourceOptions{Spec: "specs/orders.yaml"})
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := without.MigrationDraft(); ok {
				t.Fatal("migration created without selection")
			}
			if !reflect.DeepEqual(before, resourceTree(t, root)) {
				t.Fatal("preparing schema connected or wrote files")
			}
			if _, err := PrepareResource(root, strings.Repeat("a", 64), ResourceOptions{Spec: "specs/orders.yaml"}); err == nil {
				t.Fatal("truncating resource name accepted")
			}
		})
	}
	for _, length := range []int{63, 64} {
		_, err := ParseResourceSpec([]byte("version: 1\nfields:\n  - name: " + strings.Repeat("a", length) + "\n    type: bool\n"))
		if (err == nil) != (length == 63) {
			t.Fatal("field identifier limit differs", length, err)
		}
	}
}
