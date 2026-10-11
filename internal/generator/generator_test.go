package generator

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/setcreed/goseed/internal/templates"
)

func TestPrepareAddsVersionedGenerationMetadata(t *testing.T) {
	root := t.TempDir()
	plan, err := Prepare(root, templates.Project{Name: "shop", Module: "github.com/acme/shop"}, templates.Options{Database: "postgres", Redis: true, Authz: "casbin", Transport: "gateway"})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Conflicts) != 0 {
		t.Fatalf("unexpected conflicts: %v", plan.Conflicts)
	}
	metadata := string(fileContent(t, plan.Files, "goseed.yaml"))
	for _, expected := range []string{"version: 1", "template_version: 1", "name: shop", "module: github.com/acme/shop", "database: postgres", "authentication: jwt", "authorization: casbin", "redis: true", "transport: gateway", "  - example"} {
		if !strings.Contains(metadata, expected) {
			t.Errorf("metadata missing %q:\n%s", expected, metadata)
		}
	}
}

func TestWriteCreatesProjectAndNeverOverwrites(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Thumbs.db"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	project := templates.Project{Name: "shop", Module: "example.com/shop"}
	plan, err := Prepare(root, project, templates.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err := plan.Write(); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(root, "goseed.yaml")); err != nil || !strings.Contains(string(data), "module: example.com/shop") {
		t.Fatalf("metadata was not written: %q, %v", data, err)
	}
	if data, err := os.ReadFile(filepath.Join(root, "Thumbs.db")); err != nil || string(data) != "keep" {
		t.Fatalf("system file changed: %q, %v", data, err)
	}
	second, err := Prepare(root, project, templates.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Conflicts) == 0 {
		t.Fatal("existing generated project was not reported as a conflict")
	}
	if err := second.Write(); err == nil {
		t.Fatal("existing project was overwritten")
	}
}

func TestReportListsConflictsWithoutWriting(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	plan, err := Prepare(root, templates.Project{Name: "shop", Module: "example.com/shop"}, templates.Options{})
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := plan.Report(&output, true); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "target directory already contains notes.txt") || !strings.Contains(output.String(), "Dry run: no files written.") {
		t.Fatalf("report does not explain conflict and dry-run:\n%s", output.String())
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "notes.txt" {
		t.Fatalf("report changed target directory: %v", entries)
	}
}

func TestSymlinkEntryBlocksGenerationWithoutTouchingTarget(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "linked.txt")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	plan, err := Prepare(root, templates.Project{Name: "shop", Module: "example.com/shop"}, templates.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Conflicts) == 0 {
		t.Fatal("symlink entry did not block generation")
	}
	if err := plan.Write(); err == nil {
		t.Fatal("generation followed or ignored the symlink conflict")
	}
	if data, err := os.ReadFile(outside); err != nil || string(data) != "preserve" {
		t.Fatalf("symlink target changed: %q, %v", data, err)
	}
}

func TestInspectRejectsUnsafeAndDuplicatePaths(t *testing.T) {
	root := t.TempDir()
	files := []templates.File{{Path: "../escape", Content: []byte("x")}, {Path: "same.txt"}, {Path: "same.txt"}}
	conflicts, err := inspect(root, files)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(conflicts, "\n")
	for _, expected := range []string{"unsafe output path ../escape", "duplicate output path same.txt"} {
		if !strings.Contains(joined, expected) {
			t.Errorf("conflicts %q missing %q", joined, expected)
		}
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(root), "escape")); !os.IsNotExist(err) {
		t.Fatalf("unsafe path was written: %v", err)
	}
}

func fileContent(t *testing.T, files []templates.File, name string) []byte {
	t.Helper()
	for _, file := range files {
		if file.Path == name {
			return file.Content
		}
	}
	t.Fatalf("missing planned file %s", name)
	return nil
}
