package generator

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/setcreed/goseed/internal/templates"
)

func TestPrepareAndWriteBusinessModuleAcrossTransports(t *testing.T) {
	for _, transport := range []string{"http", "grpc", "dual", "gateway"} {
		for _, database := range []string{"sqlite", "none"} {
			t.Run(transport+"/"+database, func(t *testing.T) {
				root := generatedProject(t, templates.Options{Transport: transport, Database: database})
				plan, err := PrepareModule(root, "orders")
				if err != nil {
					t.Fatal(err)
				}
				before, err := os.ReadFile(filepath.Join(root, "goseed.yaml"))
				if err != nil {
					t.Fatal(err)
				}
				var report bytes.Buffer
				if err := plan.Report(&report, true); err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(report.String(), "internal/apiserver/bootstrap/modules_gen.go") || !strings.Contains(report.String(), "Dry run") {
					t.Fatalf("module dry-run report is incomplete: %s", report.String())
				}
				after, err := os.ReadFile(filepath.Join(root, "goseed.yaml"))
				if err != nil || !bytes.Equal(before, after) {
					t.Fatal("preparing or reporting the plan changed metadata")
				}
				if err := plan.Write(); err != nil {
					t.Fatal(err)
				}
				state, err := LoadProject(root)
				if err != nil {
					t.Fatal(err)
				}
				if len(state.Metadata.Modules) != 2 || state.Metadata.Modules[1].Name != "orders" || state.Metadata.Modules[1].Type != "module" {
					t.Fatalf("new module was not registered: %+v", state.Metadata.Modules)
				}
				if transport == "grpc" {
					if len(state.Sources) != 0 {
						t.Fatalf("gRPC-only project gained HTTP OpenAPI sources: %+v", state.Sources)
					}
				} else if len(state.Sources) != 2 || state.OpenAPIIndexStale {
					t.Fatalf("new module OpenAPI was not merged: sources=%d stale=%t", len(state.Sources), state.OpenAPIIndexStale)
				}
				if _, err := os.Stat(filepath.Join(root, "internal/apiserver/biz/orders/biz.go")); err != nil {
					t.Fatal(err)
				}
				_, storeErr := os.Stat(filepath.Join(root, "internal/apiserver/store/orders/store.go"))
				if database == "none" && !os.IsNotExist(storeErr) {
					t.Fatal("no-database module unexpectedly generated a Store")
				}
				if database != "none" && storeErr != nil {
					t.Fatalf("database module Store was not generated: %v", storeErr)
				}
			})
		}
	}
}

func TestModulePlanRejectsConflictsAndStaleInputs(t *testing.T) {
	t.Run("existing module output", func(t *testing.T) {
		root := generatedProject(t, templates.Options{})
		path := filepath.Join(root, "internal/apiserver/biz/orders/biz.go")
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("keep"), 0644); err != nil {
			t.Fatal(err)
		}
		plan, err := PrepareModule(root, "orders")
		if err != nil || len(plan.Conflicts) != 1 || !strings.Contains(plan.Conflicts[0], "already exists") {
			t.Fatalf("existing business file was not reported as a conflict: conflicts=%v err=%v", plan.Conflicts, err)
		}
		if err := plan.Write(); err == nil || !strings.Contains(err.Error(), "generation conflicts") {
			t.Fatalf("conflicted module plan was written: %v", err)
		}
	})
	t.Run("stale metadata", func(t *testing.T) {
		root := generatedProject(t, templates.Options{})
		plan, err := PrepareModule(root, "orders")
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, "goseed.yaml")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		data = bytes.Replace(data, []byte("name: shop"), []byte("name: other"), 1)
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatal(err)
		}
		if err := plan.Write(); err == nil || !strings.Contains(err.Error(), "changed after") {
			t.Fatalf("stale module plan was not rejected: %v", err)
		}
		if _, err := os.Stat(filepath.Join(root, "internal/apiserver/biz/orders/biz.go")); !os.IsNotExist(err) {
			t.Fatal("stale module plan wrote new business files")
		}
	})
}

func TestModulePlanRollsBackNewFilesAndUpdatedIndexesOnFailure(t *testing.T) {
	root := generatedProject(t, templates.Options{})
	plan, err := PrepareModule(root, "orders")
	if err != nil {
		t.Fatal(err)
	}
	originals := make(map[string][]byte, len(plan.updates))
	for _, update := range plan.updates {
		originals[update.Path] = bytes.Clone(update.Original)
	}
	writes := 0
	err = plan.writeWith(func(path string, data []byte, mode os.FileMode) error {
		writes++
		if writes == 2 {
			return errors.New("injected module index failure")
		}
		return writeAtomic(path, data, mode)
	})
	if err == nil || !strings.Contains(err.Error(), "injected module index failure") {
		t.Fatalf("injected update failure was not returned: %v", err)
	}
	for relative, expected := range originals {
		actual, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
		if readErr != nil || !bytes.Equal(actual, expected) {
			t.Errorf("updated file %s was not rolled back: %v", relative, readErr)
		}
	}
	for _, file := range plan.creates {
		if _, statErr := os.Lstat(filepath.Join(root, filepath.FromSlash(file.Path))); !os.IsNotExist(statErr) {
			t.Errorf("new file %s survived rollback: %v", file.Path, statErr)
		}
	}
}

func TestModuleAssemblyPreservesDeveloperOwnedFiles(t *testing.T) {
	for _, transport := range []string{"http", "grpc", "dual", "gateway"} {
		t.Run(transport, func(t *testing.T) {
			root := generatedProject(t, templates.Options{Transport: transport})
			plan, err := PrepareModule(root, "orders")
			if err != nil {
				t.Fatal(err)
			}
			if err := plan.Write(); err != nil {
				t.Fatal(err)
			}
			paths := []string{"internal/apiserver/bootstrap/providers.go", "internal/apiserver/bootstrap/module_orders.go"}
			originals := make(map[string][]byte)
			for _, path := range paths {
				filename := filepath.Join(root, filepath.FromSlash(path))
				data, err := os.ReadFile(filename)
				if err != nil {
					t.Fatal(err)
				}
				data = append(data, []byte("\n// Developer-owned customization must survive module generation.\n")...)
				if err := os.WriteFile(filename, data, 0644); err != nil {
					t.Fatal(err)
				}
				originals[path] = data
			}
			second, err := PrepareModule(root, "type")
			if err != nil {
				t.Fatal(err)
			}
			if err := second.Write(); err != nil {
				t.Fatal(err)
			}
			for path, expected := range originals {
				actual, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
				if err != nil || !bytes.Equal(actual, expected) {
					t.Fatalf("developer-owned %s was changed: %v", path, err)
				}
			}
		})
	}
}

func TestModuleAssemblyRejectsIncompatibleAndChangedInputs(t *testing.T) {
	for _, test := range []struct {
		name string
		path string
		old  string
		new  string
	}{
		{"old registry", "modules_gen.go", "const moduleAssemblyVersion = 1", "const legacyAssemblyVersion = 1"},
		{"modified registry", "modules_gen.go", "Name: \"example\"", "Name: \"modified\""},
		{"missing factory entry", "providers.go", "newModuleServices(", "legacyModuleServices("},
		{"old registration call", "providers.go", "httpModules(examples, modules)", "httpModules(examples)"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := generatedProject(t, templates.Options{})
			path := filepath.Join(root, "internal/apiserver/bootstrap", test.path)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(data, []byte(test.old)) {
				t.Fatalf("test mutation %q missing", test.old)
			}
			if err := os.WriteFile(path, bytes.Replace(data, []byte(test.old), []byte(test.new), 1), 0644); err != nil {
				t.Fatal(err)
			}
			if _, err := PrepareModule(root, "orders"); err == nil {
				t.Fatal("incompatible or modified assembly was accepted")
			}
			if _, err := os.Stat(filepath.Join(root, "internal/apiserver/biz/orders/biz.go")); !os.IsNotExist(err) {
				t.Fatal("failed preflight wrote module files")
			}
		})
	}
	for _, target := range []string{"providers.go", "module_orders.go"} {
		t.Run("stale "+target, func(t *testing.T) {
			root := generatedProject(t, templates.Options{})
			first, err := PrepareModule(root, "orders")
			if err != nil {
				t.Fatal(err)
			}
			if err := first.Write(); err != nil {
				t.Fatal(err)
			}
			plan, err := PrepareModule(root, "products")
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, "internal/apiserver/bootstrap", target)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, append(data, []byte("\n// changed after plan\n")...), 0644); err != nil {
				t.Fatal(err)
			}
			if err := plan.Write(); err == nil || !strings.Contains(err.Error(), "changed after") {
				t.Fatalf("stale assembly input accepted: %v", err)
			}
			if _, err := os.Stat(filepath.Join(root, "internal/apiserver/biz/products/biz.go")); !os.IsNotExist(err) {
				t.Fatal("stale plan wrote module files")
			}
		})
	}
}

func TestModuleAssemblyRefusesFactoryConflictsAndRepeatedConstruction(t *testing.T) {
	t.Run("factory conflict", func(t *testing.T) {
		root := generatedProject(t, templates.Options{})
		path := filepath.Join(root, "internal/apiserver/bootstrap/module_orders.go")
		original := []byte("// developer-owned factory\n")
		if err := os.WriteFile(path, original, 0644); err != nil {
			t.Fatal(err)
		}
		plan, err := PrepareModule(root, "orders")
		if err != nil || len(plan.Conflicts) != 1 {
			t.Fatalf("factory conflict not reported: %v %v", plan.Conflicts, err)
		}
		if err := plan.Write(); err == nil {
			t.Fatal("conflicted factory was overwritten")
		}
		actual, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(actual, original) {
			t.Fatal("factory was modified")
		}
	})
	t.Run("repeated construction", func(t *testing.T) {
		root := generatedProject(t, templates.Options{})
		path := filepath.Join(root, "internal/apiserver/bootstrap/providers.go")
		original, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(original, []byte("\nfunc repeated(){_,_ = newModuleServices(moduleDependencies{})}\n")...), 0644); err != nil {
			t.Fatal(err)
		}
		if _, err := PrepareModule(root, "orders"); err == nil || !strings.Contains(err.Error(), "repeated") {
			t.Fatalf("repeated module construction accepted: %v", err)
		}
	})
}
