package generator

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/setcreed/goseed/internal/templates"
)

const resourcePlanSpec = "version: 1\nfields:\n  - name: title\n    type: string\n    required: true\n  - name: enabled\n    type: bool\n  - name: count\n    type: int64\n"

func resourcePlanProject(t *testing.T, options templates.Options, tools bool) string {
	t.Helper()
	base, err := filepath.Abs("../../.tmp")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(base, 0755); err != nil {
		t.Fatal(err)
	}
	root, err := os.MkdirTemp(base, "d6-resource-plan-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(root); err != nil {
			t.Errorf("clean resource plan fixture: %v", err)
		}
	})
	plan, err := Prepare(root, templates.Project{Name: "shop", Module: "example.com/shop"}, options)
	if err != nil {
		t.Fatal(err)
	}
	if err := plan.Write(); err != nil {
		t.Fatal(err)
	}
	writeResourceInput(t, root, "specs/orders.yaml", resourcePlanSpec)
	if tools {
		for _, tool := range requiredResourceTools(options.Transport) {
			linkResourceTool(t, root, tool.Path)
		}
		if options.Transport != "http" {
			for _, name := range []string{"empty", "timestamp"} {
				linkResourceTool(t, root, ".tools/protoc/include/google/protobuf/"+name+".proto")
			}
		}
	}
	return root
}

func linkResourceTool(t *testing.T, root, path string) {
	t.Helper()
	source, err := filepath.Abs(filepath.Join("../..", filepath.FromSlash(path)))
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
		t.Fatal(err)
	}
	// Fixtures only read these links; no installation or binary copies per case.
	if err := os.Link(source, destination); err != nil {
		t.Fatalf("prepare pinned tool fixture; run make protocol-tools first: %v", err)
	}
}

func writeResourceInput(t *testing.T, root, path, content string) {
	t.Helper()
	filename := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func resourceTree(t *testing.T, root string) map[string]string {
	t.Helper()
	tree := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		value := info.Mode().String()
		if info.Mode().IsRegular() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			value += fmt.Sprintf(" %x", sha256.Sum256(data))
		} else if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			value += " " + link
		}
		tree[filepath.ToSlash(relative)] = value
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

func TestResourceTargetPlanAcrossSQLCombinations(t *testing.T) {
	for _, transport := range []string{"http", "grpc", "dual", "gateway"} {
		for _, database := range []string{"sqlite", "mysql", "postgres"} {
			for _, redis := range []bool{false, true} {
				for _, authz := range []string{"none", "casbin"} {
					t.Run(fmt.Sprintf("%s/%s/redis=%t/%s", transport, database, redis, authz), func(t *testing.T) {
						root := resourcePlanProject(t, templates.Options{Transport: transport, Database: database, Redis: redis, Authz: authz}, true)
						before := resourceTree(t, root)
						plan, err := PrepareResource(root, "orders", ResourceOptions{Spec: "specs/orders.yaml", Migration: true})
						if err != nil {
							t.Fatal(err)
						}
						if len(plan.Conflicts()) != 0 {
							t.Fatalf("unexpected conflicts: %v", plan.Conflicts())
						}
						var report bytes.Buffer
						if err := plan.Report(&report); err != nil {
							t.Fatal(err)
						}
						if !strings.Contains(report.String(), "Target preview only") || !strings.Contains(report.String(), "No files written") {
							t.Fatal("preview advertises unimplemented resource code")
						}
						migration := "migrations/00002_orders.sql"
						if authz == "casbin" {
							migration = "migrations/00003_orders.sql"
						}
						if !strings.Contains(report.String(), migration) {
							t.Fatalf("next migration missing: %s", report.String())
						}
						if strings.Contains(report.String(), "api/openapi/") == (transport == "grpc") {
							t.Fatal("OpenAPI trimming is incorrect")
						}
						if strings.Contains(report.String(), "handler/http/orders") != (transport == "http" || transport == "dual") {
							t.Fatal("HTTP handler trimming is incorrect")
						}
						if strings.Contains(report.String(), "handler/grpc/orders") != (transport != "http") {
							t.Fatal("RPC handler trimming is incorrect")
						}
						if strings.Contains(report.String(), "orders.pb.gw.go") != (transport == "gateway") {
							t.Fatal("Gateway trimming is incorrect")
						}
						if err := plan.Revalidate(); err != nil {
							t.Fatal(err)
						}
						if !reflect.DeepEqual(before, resourceTree(t, root)) {
							t.Fatal("target planning changed the project tree")
						}
					})
				}
			}
		}
	}
}

func TestResourcePlanCopiesAndReports(t *testing.T) {
	root := resourcePlanProject(t, templates.Options{Transport: "http"}, false)
	plan, err := PrepareResource(root, "orders", ResourceOptions{Spec: "specs/orders.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := PrepareResource(root, "orders", ResourceOptions{Spec: "specs/orders.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	var a, b bytes.Buffer
	if err := plan.Report(&a); err != nil {
		t.Fatal(err)
	}
	if err := other.Report(&b); err != nil {
		t.Fatal(err)
	}
	if a.String() != b.String() || strings.Contains(a.String(), "migrations/") {
		t.Fatal("nondeterministic preview or unrequested migration")
	}
	plan.Targets()[0].Path = "changed"
	plan.Specification().Fields[0].Type = "changed"
	if plan.Targets()[0].Path == "changed" || plan.Specification().Fields[0].Type != "string" {
		t.Fatal("resource plan exposes mutable slices")
	}
	if err := plan.Report(resourceShortWriter{}); err != io.ErrShortWrite {
		t.Fatalf("short report write ignored: %v", err)
	}
	if err := (ResourcePlan{}).Revalidate(); err == nil {
		t.Fatal("unprepared resource plan accepted")
	}
}

type resourceShortWriter struct{}

func (resourceShortWriter) Write(data []byte) (int, error) { return len(data) - 1, nil }

func TestResourcePlanRejectsInvalidInputs(t *testing.T) {
	cases := []struct{ name, spec, body, want string }{
		{"Orders", "specs/orders.yaml", resourcePlanSpec, "invalid or reserved"},
		{"example", "specs/orders.yaml", resourcePlanSpec, "invalid or reserved"},
		{"examples", "specs/orders.yaml", resourcePlanSpec, "resource table"},
		{"orders", "../orders.yaml", resourcePlanSpec, "project-relative"},
		{"orders", "specs/../orders.yaml", resourcePlanSpec, "project-relative"},
		{"orders", "/specs/orders.yaml", resourcePlanSpec, "project-relative"},
		{"orders", "specs/orders.yml", resourcePlanSpec, "project-relative"},
		{"orders", "specs/orders.yaml", "version: 2\nfields: []\n", "resource spec"},
		{"orders", "specs/orders.yaml", "version: 1\nfields:\n - name: table_name\n   type: string\n", "TableName"},
	}
	for _, test := range cases {
		t.Run(test.name+"/"+test.want+"/"+test.spec, func(t *testing.T) {
			root := resourcePlanProject(t, templates.Options{Transport: "http"}, false)
			writeResourceInput(t, root, "specs/orders.yaml", test.body)
			before := resourceTree(t, root)
			_, err := PrepareResource(root, test.name, ResourceOptions{Spec: test.spec})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("unexpected error: %v; want %s", err, test.want)
			}
			if !reflect.DeepEqual(before, resourceTree(t, root)) {
				t.Fatal("invalid input modified the tree")
			}
		})
	}
}

func TestResourcePlanRejectsNoDatabaseCombinations(t *testing.T) {
	for _, transport := range []string{"http", "grpc", "dual", "gateway"} {
		for _, redis := range []bool{false, true} {
			for _, authz := range []string{"none", "casbin"} {
				t.Run(fmt.Sprintf("%s/%t/%s", transport, redis, authz), func(t *testing.T) {
					root := resourcePlanProject(t, templates.Options{Transport: transport, Database: "none", Redis: redis, Authz: authz}, false)
					before := resourceTree(t, root)
					if _, err := PrepareResource(root, "orders", ResourceOptions{Spec: "specs/orders.yaml"}); err == nil || !strings.Contains(err.Error(), "use gen module") {
						t.Fatalf("no-database resource accepted: %v", err)
					}
					if !reflect.DeepEqual(before, resourceTree(t, root)) {
						t.Fatal("no-database rejection changed project tree")
					}
				})
			}
		}
	}
}

func TestResourcePlanConflictsAndChanges(t *testing.T) {
	t.Run("existing target", func(t *testing.T) {
		root := resourcePlanProject(t, templates.Options{Transport: "http"}, false)
		writeResourceInput(t, root, "internal/apiserver/model/orders.go", "keep\n")
		plan, err := PrepareResource(root, "orders", ResourceOptions{Spec: "specs/orders.yaml"})
		if err != nil || len(plan.Conflicts()) != 1 {
			t.Fatalf("target conflict missing: %v, %v", plan.Conflicts(), err)
		}
		plan.Conflicts()[0] = "changed"
		if !strings.Contains(plan.Conflicts()[0], "orders.go") || plan.Revalidate() == nil {
			t.Fatal("conflict was mutable or ignored")
		}
	})
	for _, path := range []string{"specs/orders.yaml", "goseed.yaml", "internal/apiserver/bootstrap/providers.go", "api/openapi/example.yaml", "api/openapi/swagger.json", "migrations/00001_examples.sql", "migrations/migrations.go"} {
		t.Run("change/"+path, func(t *testing.T) {
			root := resourcePlanProject(t, templates.Options{Transport: "http"}, false)
			plan, err := PrepareResource(root, "orders", ResourceOptions{Spec: "specs/orders.yaml", Migration: true})
			if err != nil {
				t.Fatal(err)
			}
			filename := filepath.Join(root, filepath.FromSlash(path))
			data, err := os.ReadFile(filename)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filename, append(data, '\n'), 0644); err != nil {
				t.Fatal(err)
			}
			if err := plan.Revalidate(); err == nil || !strings.Contains(err.Error(), "changed after") {
				t.Fatalf("stale input accepted: %v", err)
			}
		})
	}
	for _, path := range []string{"migrations/00002_manual.sql", "internal/apiserver/biz/orders/biz.go"} {
		t.Run("add/"+path, func(t *testing.T) {
			root := resourcePlanProject(t, templates.Options{Transport: "http"}, false)
			plan, err := PrepareResource(root, "orders", ResourceOptions{Spec: "specs/orders.yaml", Migration: true})
			if err != nil {
				t.Fatal(err)
			}
			writeResourceInput(t, root, path, "keep\n")
			if err := plan.Revalidate(); err == nil {
				t.Fatal("new migration or target did not invalidate the plan")
			}
		})
	}
	t.Run("shared namespace and factory", func(t *testing.T) {
		root := resourcePlanProject(t, templates.Options{Transport: "http"}, false)
		module, err := PrepareModule(root, "custom")
		if err != nil {
			t.Fatal(err)
		}
		if err := module.Write(); err != nil {
			t.Fatal(err)
		}
		if _, err := PrepareResource(root, "custom", ResourceOptions{Spec: "specs/orders.yaml"}); err == nil || !strings.Contains(err.Error(), "already registered") {
			t.Fatalf("ordinary module silently converted to resource: %v", err)
		}
		plan, err := PrepareResource(root, "orders", ResourceOptions{Spec: "specs/orders.yaml"})
		if err != nil {
			t.Fatal(err)
		}
		writeResourceInput(t, root, "internal/apiserver/bootstrap/module_custom.go", "changed\n")
		if err := plan.Revalidate(); err == nil || !strings.Contains(err.Error(), "module_custom.go") {
			t.Fatalf("factory change ignored: %v", err)
		}
	})
}

func TestResourcePlanUnsafePathsAndProjection(t *testing.T) {
	for _, path := range []string{"specs", "internal/apiserver/biz/orders", "migrations", "api/openapi"} {
		t.Run(path, func(t *testing.T) {
			root := resourcePlanProject(t, templates.Options{Transport: "http"}, false)
			linked := filepath.Join(root, filepath.FromSlash(path))
			backup := filepath.Join(root, "linked-backup")
			if _, err := os.Stat(linked); err == nil {
				if err := os.Rename(linked, backup); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.MkdirAll(filepath.Dir(linked), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(backup, 0755); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Symlink(backup, linked); err != nil {
				t.Fatal(err)
			}
			if _, err := PrepareResource(root, "orders", ResourceOptions{Spec: "specs/orders.yaml"}); err == nil || !strings.Contains(err.Error(), "unsafe") {
				t.Fatalf("symlink parent accepted: %v", err)
			}
		})
	}
	for _, replacement := range []string{"const moduleAssemblyVersion = 0", "const injected = true\nconst moduleAssemblyVersion = 2"} {
		t.Run(replacement, func(t *testing.T) {
			root := resourcePlanProject(t, templates.Options{Transport: "http"}, false)
			path := filepath.Join(root, "internal/apiserver/bootstrap/modules_gen.go")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			data = bytes.Replace(data, []byte("const moduleAssemblyVersion = 2"), []byte(replacement), 1)
			if err := os.WriteFile(path, data, 0644); err != nil {
				t.Fatal(err)
			}
			if _, err := PrepareResource(root, "orders", ResourceOptions{Spec: "specs/orders.yaml"}); err == nil {
				t.Fatal("old or modified registration accepted")
			}
		})
	}
}

func TestResourceMigrationTargetVersions(t *testing.T) {
	for _, filename := range []string{"00001_duplicate.sql", "00000_invalid.sql", "bad.sql", "00002_invalid.SQL", "9223372036854775808_overflow.sql", "9223372036854775807_final.sql"} {
		t.Run(filename, func(t *testing.T) {
			root := resourcePlanProject(t, templates.Options{Transport: "http"}, false)
			writeResourceInput(t, root, "migrations/"+filename, "-- draft\n")
			if _, err := PrepareResource(root, "orders", ResourceOptions{Spec: "specs/orders.yaml", Migration: true}); err == nil {
				t.Fatal("invalid or exhausted migration set accepted")
			}
		})
	}
	t.Run("nonconsecutive version", func(t *testing.T) {
		root := resourcePlanProject(t, templates.Options{Transport: "http"}, false)
		writeResourceInput(t, root, "migrations/00100_ManualChange.sql", "-- draft\n")
		plan, err := PrepareResource(root, "orders", ResourceOptions{Spec: "specs/orders.yaml", Migration: true})
		if err != nil {
			t.Fatal(err)
		}
		var report bytes.Buffer
		if err := plan.Report(&report); err != nil || !strings.Contains(report.String(), "00101_orders.sql") {
			t.Fatalf("migration must follow maximum version: %v %s", err, report.String())
		}
	})
}

func TestResourceProtocolTools(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		root := resourcePlanProject(t, templates.Options{Transport: "grpc"}, false)
		if _, err := PrepareResource(root, "orders", ResourceOptions{Spec: "specs/orders.yaml"}); err == nil || !strings.Contains(err.Error(), "protocol tool protoc is unavailable") {
			t.Fatalf("missing tool accepted: %v", err)
		}
	})
	t.Run("wrong protoc version", func(t *testing.T) {
		root := resourcePlanProject(t, templates.Options{Transport: "grpc"}, false)
		path := ".tools/protoc/bin/protoc"
		writeResourceInput(t, root, path, "#!/bin/sh\nprintf 'libprotoc 0.0\\n'\n")
		if err := os.Chmod(filepath.Join(root, filepath.FromSlash(path)), 0755); err != nil {
			t.Fatal(err)
		}
		if _, err := PrepareResource(root, "orders", ResourceOptions{Spec: "specs/orders.yaml"}); err == nil || !strings.Contains(err.Error(), "version mismatch") {
			t.Fatalf("wrong protoc accepted: %v", err)
		}
	})
	t.Run("wrong plugin identity", func(t *testing.T) {
		root := resourcePlanProject(t, templates.Options{Transport: "grpc"}, false)
		linkResourceTool(t, root, ".tools/protoc/bin/protoc")
		path := ".tools/bin/protoc-gen-go"
		writeResourceInput(t, root, path, "#!/bin/sh\nexit 0\n")
		if err := os.Chmod(filepath.Join(root, filepath.FromSlash(path)), 0755); err != nil {
			t.Fatal(err)
		}
		if _, err := PrepareResource(root, "orders", ResourceOptions{Spec: "specs/orders.yaml"}); err == nil || !strings.Contains(err.Error(), "build identity") {
			t.Fatalf("unidentified plugin accepted: %v", err)
		}
	})
	t.Run("changed tool", func(t *testing.T) {
		root := resourcePlanProject(t, templates.Options{Transport: "grpc"}, true)
		plan, err := PrepareResource(root, "orders", ResourceOptions{Spec: "specs/orders.yaml"})
		if err != nil {
			t.Fatal(err)
		}
		plan.Tools()[0].Version = "changed"
		if plan.Tools()[0].Version != "36.2" {
			t.Fatal("tool requirements are mutable")
		}
		// Unlink the fixture before replacing it; never modify the shared binary.
		path := ".tools/bin/protoc-gen-go"
		if err := os.Remove(filepath.Join(root, filepath.FromSlash(path))); err != nil {
			t.Fatal(err)
		}
		writeResourceInput(t, root, path, "changed\n")
		if err := plan.Revalidate(); err == nil || !strings.Contains(err.Error(), "changed after") {
			t.Fatalf("changed tool accepted: %v", err)
		}
	})
	t.Run("versions match lock", func(t *testing.T) {
		data, err := os.ReadFile("../../dependencies.lock.json")
		if err != nil {
			t.Fatal(err)
		}
		var lock map[string]json.RawMessage
		if err := json.Unmarshal(data, &lock); err != nil {
			t.Fatal(err)
		}
		var tools map[string]string
		var protoc map[string]json.RawMessage
		if err := json.Unmarshal(lock["tools"], &tools); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(lock["protoc"], &protoc); err != nil {
			t.Fatal(err)
		}
		var protocVersion string
		if err := json.Unmarshal(protoc["version"], &protocVersion); err != nil {
			t.Fatal(err)
		}
		for _, tool := range requiredResourceTools("gateway") {
			version := protocVersion
			if tool.Module != "" {
				path := tool.Module + "/cmd/" + tool.Name
				if tool.Name == "protoc-gen-go-grpc" {
					path = tool.Module
				}
				if tool.Name == "protoc-gen-grpc-gateway" || tool.Name == "protoc-gen-openapiv2" {
					path = tool.Module + "/" + tool.Name
				}
				version = tools[path]
			}
			if version != tool.Version {
				t.Fatalf("tool %s differs from lock: %s != %s", tool.Name, tool.Version, version)
			}
		}
	})
}
