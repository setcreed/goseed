package generator

import (
	"reflect"
	"testing"

	"github.com/setcreed/goseed/internal/templates"
)

func TestIncrementalGenerationPreservesProjectOptions(t *testing.T) {
	for _, transport := range []string{"http", "grpc", "dual", "gateway"} {
		for _, kind := range []string{"module", "resource"} {
			t.Run(transport+"/"+kind, func(t *testing.T) {
				options := templates.Options{Database: "sqlite", Transport: transport, Redis: true, Authz: "casbin", Metrics: true, Tracing: true, Deployment: "single-host"}
				root := resourcePlanProject(t, options, true)
				before, err := LoadProject(root)
				if err != nil {
					t.Fatal(err)
				}
				if kind == "module" {
					plan, err := PrepareModule(root, "orders")
					if err != nil {
						t.Fatal(err)
					}
					if err := plan.Write(); err != nil {
						t.Fatal(err)
					}
				} else {
					plan, err := PrepareResource(root, "orders", ResourceOptions{Spec: "specs/orders.yaml"})
					if err != nil {
						t.Fatal(err)
					}
					if err := plan.Write(); err != nil {
						t.Fatal(err)
					}
				}
				after, err := LoadProject(root)
				if err != nil {
					t.Fatal(err)
				}
				before.Metadata.Modules, after.Metadata.Modules = nil, nil
				if !reflect.DeepEqual(before.Metadata, after.Metadata) {
					t.Fatalf("project options changed:\nbefore: %+v\nafter: %+v", before.Metadata, after.Metadata)
				}
				if _, err := PrepareModule(root, "next"); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestMetadataYAMLNamesRoundTrip(t *testing.T) {
	for _, name := range []string{"null", "true", "false", "yes", "no"} {
		t.Run(name, func(t *testing.T) {
			data, err := renderMetadata(templates.Project{Name: name, Module: "example.com/" + name}, templates.Options{})
			if err != nil {
				t.Fatal(err)
			}
			metadata, err := parseProjectMetadata(data)
			if err != nil {
				t.Fatal(err)
			}
			if metadata.Name != name {
				t.Fatalf("name changed: %q", metadata.Name)
			}
		})
	}
}

func TestIncrementalGenerationWithYAMLNames(t *testing.T) {
	for _, resource := range []bool{false, true} {
		name := "module"
		if resource {
			name = "resource"
		}
		t.Run(name, func(t *testing.T) {
			root := resourcePlanProject(t, templates.Options{Transport: "http"}, false)
			if resource {
				plan, err := PrepareResource(root, "null", ResourceOptions{Spec: "specs/orders.yaml"})
				if err != nil {
					t.Fatal(err)
				}
				if err := plan.Write(); err != nil {
					t.Fatal(err)
				}
			} else {
				plan, err := PrepareModule(root, "null")
				if err != nil {
					t.Fatal(err)
				}
				if err := plan.Write(); err != nil {
					t.Fatal(err)
				}
			}
			state, err := LoadProject(root)
			if err != nil {
				t.Fatal(err)
			}
			if state.Metadata.Modules[1].Name != "null" {
				t.Fatal("incremental name changed")
			}
			if _, err := PrepareModule(root, "next"); err != nil {
				t.Fatal(err)
			}
		})
	}
}
