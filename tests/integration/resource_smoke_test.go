package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/setcreed/goseed/internal/generator"
	"github.com/setcreed/goseed/internal/templates"
)

// Compile real generated projects: formatting templates alone cannot detect
// differences between business field names and protoc's generated selectors.
func TestResourceGeneratedProjectSmoke(t *testing.T) {
	repository, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, transport := range []string{"http", "grpc", "dual", "gateway"} {
		t.Run(transport, func(t *testing.T) {
			directory, err := os.MkdirTemp(filepath.Join(repository, ".tmp"), "resource-smoke-")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := os.RemoveAll(directory); err != nil {
					t.Error(err)
				}
			})
			plan, err := generator.Prepare(directory, templates.Project{Name: "demo", Module: "example.com/demo"}, templates.Options{Database: "sqlite", Transport: transport})
			if err != nil {
				t.Fatal(err)
			}
			if err := plan.Write(); err != nil {
				t.Fatal(err)
			}
			if transport != "http" {
				linkResourceMatrixTools(t, repository, directory, transport)
			}
			spec := "version: 1\nfields:\n"
			for _, name := range []string{"reset", "string", "proto_message", "descriptor", "marshal", "unmarshal", "extension_range_array", "extension_map", "foo_1", "a1b", "get_id", "get_created_at", "foo", "get_foo", "x_foo"} {
				spec += "  - name: " + name + "\n    type: string\n    required: true\n"
			}
			if err := os.Mkdir(filepath.Join(directory, "specs"), 0750); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(directory, "specs/item.yaml"), []byte(spec), 0600); err != nil {
				t.Fatal(err)
			}
			resource, err := generator.PrepareResource(directory, "item", generator.ResourceOptions{Spec: "specs/item.yaml", Migration: true})
			if err != nil {
				t.Fatal(err)
			}
			if err := resource.Write(); err != nil {
				t.Fatal(err)
			}
			verifyReadonly(t, directory)
			run(t, directory, "go", "test", "-mod=readonly", "-run", "^$", "./...")
		})
	}
}
