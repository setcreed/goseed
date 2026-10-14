package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultCommandGeneratesProjectInCurrentDirectory(t *testing.T) {
	dir := projectDir(t)
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".DS_Store"), []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	var output bytes.Buffer
	cmd := NewCommand("v0.0.0-test")
	cmd.SetOut(&output)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"go.mod", "goseed.yaml", "cmd/apiserver/main.go", "migrations/00001_examples.sql"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("generated file %s: %v", name, err)
		}
	}
	if !strings.Contains(output.String(), "Generated shop") {
		t.Fatalf("unexpected output: %s", output.String())
	}
	if data, err := os.ReadFile(filepath.Join(dir, ".DS_Store")); err != nil || string(data) != "preserve" {
		t.Fatalf("system file changed: %q, %v", data, err)
	}
}

func TestDryRunReportsPlanWithoutWriting(t *testing.T) {
	dir := projectDir(t)
	t.Chdir(dir)

	var output bytes.Buffer
	cmd := NewCommand("test")
	cmd.SetArgs([]string{"--name", "shopapi", "--module", "github.com/acme/shop", "--db", "postgres", "--redis", "--authz", "casbin", "--transport", "dual", "--deployment", "single-host", "--dry-run"})
	cmd.SetOut(&output)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"Project: shopapi", "Module: github.com/acme/shop", "Database: postgres", "Authorization: casbin", "Redis: true", "Transport: dual", "Deployment: single-host", "goseed.yaml", "deploy/nginx/http.conf", "Dry run: no files written."} {
		if !strings.Contains(output.String(), expected) {
			t.Errorf("plan does not contain %q:\n%s", expected, output.String())
		}
	}
	for _, dependency := range []string{"github.com/casbin/casbin/v3", "github.com/redis/go-redis/v9", "google.golang.org/grpc"} {
		if !strings.Contains(output.String(), dependency) {
			t.Errorf("dry-run output does not list dependency %s", dependency)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("dry-run created %d entries", len(entries))
	}
}

func TestSingleHostDeploymentGenerationIsExplicit(t *testing.T) {
	dir := projectDir(t)
	t.Chdir(dir)
	cmd := NewCommand("test")
	cmd.SetArgs([]string{"--deployment=single-host"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"deploy/systemd/apiserver.service", "deploy/nginx/http.conf", "deploy/app.env.example", "deploy/README.md"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("generated deployment file %s: %v", name, err)
		}
	}
	metadata, err := os.ReadFile(filepath.Join(dir, "goseed.yaml"))
	if err != nil || !strings.Contains(string(metadata), "deployment: single-host\n") {
		t.Fatalf("deployment choice missing from metadata: %q, %v", metadata, err)
	}
}

func TestGenModuleDryRunAndWrite(t *testing.T) {
	dir := projectDir(t)
	t.Chdir(dir)
	create := NewCommand("test")
	if err := create.Execute(); err != nil {
		t.Fatal(err)
	}
	var dryOutput bytes.Buffer
	dry := NewCommand("test")
	dry.SetArgs([]string{"gen", "module", "orders", "--dry-run"})
	dry.SetOut(&dryOutput)
	if err := dry.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(dryOutput.String(), "internal/apiserver/bootstrap/modules_gen.go") || !strings.Contains(dryOutput.String(), "Dry run") {
		t.Fatalf("unexpected module dry-run output: %s", dryOutput.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "internal/apiserver/biz/orders/biz.go")); !os.IsNotExist(err) {
		t.Fatal("module dry-run wrote business files")
	}
	var output bytes.Buffer
	command := NewCommand("test")
	command.SetArgs([]string{"gen", "module", "orders"})
	command.SetOut(&output)
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Generated module orders") {
		t.Fatalf("unexpected module output: %s", output.String())
	}
	for _, path := range []string{"internal/apiserver/biz/orders/biz.go", "internal/apiserver/handler/http/orders/handler.go", "internal/apiserver/store/orders/store.go", "api/openapi/orders.yaml"} {
		if _, err := os.Stat(filepath.Join(dir, path)); err != nil {
			t.Errorf("generated module file %s: %v", path, err)
		}
	}
}

func TestGenResourceDryRunAndWrite(t *testing.T) {
	dir := projectDir(t)
	t.Chdir(dir)
	if err := NewCommand("test").Execute(); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "specs"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "specs/product.yaml"), []byte("version: 1\nfields:\n  - name: title\n    type: string\n    required: true\n  - name: enabled\n    type: bool\n"), 0644); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(dir, "goseed.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var dryOutput bytes.Buffer
	dry := NewCommand("test")
	dry.SetArgs([]string{"gen", "resource", "product", "--spec", "specs/product.yaml", "--migration", "--dry-run"})
	dry.SetOut(&dryOutput)
	if err := dry.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"internal/apiserver/biz/product/biz.go", "migrations/00002_product.sql", "goseed.yaml", "Dry run"} {
		if !strings.Contains(dryOutput.String(), expected) {
			t.Errorf("resource dry-run missing %q:\n%s", expected, dryOutput.String())
		}
	}
	after, err := os.ReadFile(filepath.Join(dir, "goseed.yaml"))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("resource dry-run changed project metadata")
	}
	var output bytes.Buffer
	command := NewCommand("test")
	command.SetArgs([]string{"gen", "resource", "product", "--spec", "specs/product.yaml", "--migration"})
	command.SetOut(&output)
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Generated resource product") {
		t.Fatalf("unexpected resource output: %s", output.String())
	}
	for _, path := range []string{"internal/apiserver/biz/product/biz.go", "internal/apiserver/handler/http/product/handler.go", "internal/apiserver/bootstrap/module_product.go", "api/openapi/product.yaml", "migrations/00002_product.sql"} {
		if _, err := os.Stat(filepath.Join(dir, path)); err != nil {
			t.Errorf("missing CLI resource output %s: %v", path, err)
		}
	}
}

func TestGenResourceProtocolCLI(t *testing.T) {
	repository, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, transport := range []string{"grpc", "dual", "gateway"} {
		t.Run(transport, func(t *testing.T) {
			dir := projectDir(t)
			t.Chdir(dir)
			create := NewCommand("test")
			create.SetArgs([]string{"--transport", transport})
			if err := create.Execute(); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Join(dir, "specs"), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "specs/orders.yaml"), []byte("version: 1\nfields:\n  - name: title\n    type: string\n    required: true\n"), 0644); err != nil {
				t.Fatal(err)
			}
			toolPaths := []string{".tools/protoc/bin/protoc", ".tools/bin/protoc-gen-go", ".tools/bin/protoc-gen-go-grpc"}
			if transport == "gateway" {
				toolPaths = append(toolPaths, ".tools/bin/protoc-gen-grpc-gateway", ".tools/bin/protoc-gen-openapiv2")
			}
			for _, path := range append(toolPaths, ".tools/protoc/include/google/protobuf/empty.proto", ".tools/protoc/include/google/protobuf/timestamp.proto") {
				source := filepath.Join(repository, filepath.FromSlash(path))
				destination := filepath.Join(dir, filepath.FromSlash(path))
				if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.Link(source, destination); err != nil {
					t.Fatalf("link pinned protocol fixture %s: %v", path, err)
				}
			}
			command := NewCommand("test")
			command.SetArgs([]string{"gen", "resource", "orders", "--spec", "specs/orders.yaml"})
			if err := command.Execute(); err != nil {
				t.Fatal(err)
			}
			for _, path := range []string{"internal/apiserver/handler/grpc/orders/handler.go", "pkg/api/orders/v1/orders_grpc.pb.go", "internal/apiserver/bootstrap/module_orders.go"} {
				if _, err := os.Stat(filepath.Join(dir, path)); err != nil {
					t.Errorf("missing %s: %v", path, err)
				}
			}
			if transport == "gateway" {
				for _, path := range []string{"pkg/api/orders/v1/orders.pb.gw.go", "api/openapi/orders.yaml", "api/openapi/swagger.json"} {
					if _, err := os.Stat(filepath.Join(dir, path)); err != nil {
						t.Errorf("missing Gateway output %s: %v", path, err)
					}
				}
			}
		})
	}
}

func TestGenModuleDryRunReportsOutputConflicts(t *testing.T) {
	dir := projectDir(t)
	t.Chdir(dir)
	create := NewCommand("test")
	if err := create.Execute(); err != nil {
		t.Fatal(err)
	}
	conflict := filepath.Join(dir, "internal/apiserver/biz/orders/biz.go")
	if err := os.MkdirAll(filepath.Dir(conflict), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(conflict, []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	command := NewCommand("test")
	command.SetArgs([]string{"gen", "module", "orders", "--dry-run"})
	command.SetOut(&output)
	if err := command.Execute(); err == nil {
		t.Fatal("conflicted module generation succeeded")
	}
	if !strings.Contains(output.String(), "! output already exists internal/apiserver/biz/orders/biz.go") {
		t.Fatalf("dry-run omitted the output conflict: %s", output.String())
	}
	if data, err := os.ReadFile(conflict); err != nil || string(data) != "keep" {
		t.Fatal("dry-run changed the conflicting file")
	}
}

func TestInvalidParametersAndExistingDirectoryDoNotWrite(t *testing.T) {
	tests := []struct {
		name string
		args []string
		file bool
	}{
		{name: "invalid name", args: []string{"--name", "Bad-Name"}},
		{name: "invalid database", args: []string{"--db", "oracle"}},
		{name: "invalid authorization", args: []string{"--authz", "unknown"}},
		{name: "invalid transport", args: []string{"--transport", "unknown"}},
		{name: "invalid deployment", args: []string{"--deployment", "kubernetes"}},
		{name: "invalid module", args: []string{"--module", "../outside"}},
		{name: "positional argument", args: []string{"shop"}},
		{name: "non-empty directory", args: []string{"--dry-run"}, file: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := projectDir(t)
			if tt.file {
				if err := os.WriteFile(filepath.Join(dir, "keep.txt"), []byte("keep"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			t.Chdir(dir)
			cmd := NewCommand("test")
			cmd.SetArgs(tt.args)
			var output bytes.Buffer
			cmd.SetOut(&output)
			err := cmd.Execute()
			if err == nil {
				t.Fatal("expected command failure")
			}
			entries, readErr := os.ReadDir(dir)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if tt.file {
				if len(entries) != 1 || entries[0].Name() != "keep.txt" {
					t.Fatalf("existing directory changed: %v", entries)
				}
			} else if len(entries) != 0 {
				t.Fatalf("invalid command created files: %v", entries)
			}
		})
	}
}

func TestHelpAndVersionRemainAvailable(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{args: []string{"--help"}, want: "Generate a standalone Go business project"},
		{args: []string{"--version"}, want: "v0.0.0-test"},
	}
	for _, tt := range tests {
		var output bytes.Buffer
		cmd := NewCommand("v0.0.0-test")
		cmd.SetArgs(tt.args)
		cmd.SetOut(&output)
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(output.String(), tt.want) {
			t.Fatalf("output %q does not contain %q", output.String(), tt.want)
		}
	}
}

func projectDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "shop")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	return dir
}
