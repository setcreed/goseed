package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/setcreed/goseed/internal/generator"
	"github.com/setcreed/goseed/internal/templates"
)

const resourceMatrixSpec = `version: 1
fields:
  - name: title
    type: string
    required: true
  - name: enabled
    type: bool
  - name: amount
    type: int64
`

func TestResourceGenerationMatrix(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	base, err := os.MkdirTemp(filepath.Join(root, ".tmp"), "d6-resource-matrix-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(base); err != nil {
			t.Error(err)
		}
	})
	for _, transport := range []string{"http", "grpc", "dual", "gateway"} {
		for _, database := range []string{"sqlite", "mysql", "postgres"} {
			for _, redis := range []bool{false, true} {
				for _, authz := range []string{"none", "casbin"} {
					name := database + "/" + transport + "/redis=" + map[bool]string{false: "off", true: "on"}[redis] + "/authz=" + authz
					t.Run(name, func(t *testing.T) {
						directory, err := os.MkdirTemp(base, "project-")
						if err != nil {
							t.Fatal(err)
						}
						t.Cleanup(func() {
							if err := os.RemoveAll(directory); err != nil {
								t.Error(err)
							}
						})
						options := templates.Options{Database: database, Redis: redis, Authz: authz, Transport: transport}
						project, err := generator.Prepare(directory, templates.Project{Name: "demo", Module: "example.com/demo"}, options)
						if err != nil {
							t.Fatal(err)
						}
						if err := project.Write(); err != nil {
							t.Fatal(err)
						}
						if transport != "http" {
							linkResourceMatrixTools(t, root, directory, transport)
						}
						specPath := filepath.Join(directory, "specs", "orders.yaml")
						if err := os.MkdirAll(filepath.Dir(specPath), 0750); err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(specPath, []byte(resourceMatrixSpec), 0600); err != nil {
							t.Fatal(err)
						}
						plan, err := generator.PrepareResource(directory, "orders", generator.ResourceOptions{Spec: "specs/orders.yaml", Migration: true})
						if err != nil {
							t.Fatal(err)
						}
						if err := plan.Write(); err != nil {
							t.Fatal(err)
						}
						if _, err := generator.LoadProject(directory); err != nil {
							t.Fatal(err)
						}
						verifyReadonly(t, directory)
						run(t, directory, "go", "test", "-mod=readonly", "-run", "^$", "./...")
					})
				}
			}
		}
	}
}

func linkResourceMatrixTools(t *testing.T, repository, project, transport string) {
	t.Helper()
	paths := []string{
		".tools/protoc/bin/protoc",
		".tools/bin/protoc-gen-go",
		".tools/bin/protoc-gen-go-grpc",
		".tools/protoc/include/google/protobuf/empty.proto",
		".tools/protoc/include/google/protobuf/timestamp.proto",
	}
	if transport == "gateway" {
		paths = append(paths, ".tools/bin/protoc-gen-grpc-gateway", ".tools/bin/protoc-gen-openapiv2")
	}
	for _, path := range paths {
		source := filepath.Join(repository, filepath.FromSlash(path))
		destination := filepath.Join(project, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(destination), 0750); err != nil {
			t.Fatal(err)
		}
		if err := os.Link(source, destination); err != nil {
			t.Fatalf("link pinned protocol tool %s: %v", path, err)
		}
	}
}
