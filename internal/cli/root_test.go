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
	cmd.SetArgs([]string{"--name", "shopapi", "--module", "github.com/acme/shop", "--db", "postgres", "--redis", "--authz", "casbin", "--transport", "dual", "--dry-run"})
	cmd.SetOut(&output)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"Project: shopapi", "Module: github.com/acme/shop", "Database: postgres", "Authorization: casbin", "Redis: true", "Transport: dual", "goseed.yaml", "Dry run: no files written."} {
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
