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

func TestOpenAPIRefreshPlansAndWritesCanonicalIndex(t *testing.T) {
	root := generatedProject(t, templates.Options{})
	sourcePath := filepath.Join(root, "api/openapi/example.yaml")
	indexPath := filepath.Join(root, "api/openapi/swagger.json")
	registryPath := filepath.Join(root, "internal/apiserver/bootstrap/modules_gen.go")

	originalIndex, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	changedSource := bytes.Replace(source, []byte("shop HTTP API"), []byte("updated HTTP API"), 1)
	if bytes.Equal(source, changedSource) {
		t.Fatal("test source has no expected project title")
	}
	if err := os.WriteFile(sourcePath, changedSource, 0644); err != nil {
		t.Fatal(err)
	}

	plan, err := PrepareOpenAPIRefresh(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.updates) != 2 {
		t.Fatalf("expected aggregate and digest updates, got %d", len(plan.updates))
	}
	var report bytes.Buffer
	if err := plan.Report(&report, true); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(report.String(), "api/openapi/swagger.json") || !strings.Contains(report.String(), "Dry run") {
		t.Fatalf("refresh plan report is incomplete: %s", report.String())
	}
	currentIndex, err := os.ReadFile(indexPath)
	if err != nil || !bytes.Equal(currentIndex, originalIndex) {
		t.Fatal("preparing or reporting the plan changed the aggregate index")
	}
	if err := plan.Write(); err != nil {
		t.Fatal(err)
	}
	state, err := LoadProject(root)
	if err != nil {
		t.Fatal(err)
	}
	if state.OpenAPIIndexStale {
		t.Fatal("refreshed OpenAPI index is still stale")
	}
	registry, err := os.ReadFile(registryPath)
	if err != nil || !bytes.Contains(registry, []byte(`const openAPISHA256 = "`+state.OpenAPI.Digest+`"`)) {
		t.Fatalf("registration digest was not refreshed: %v", err)
	}
}

func TestOpenAPIRefreshRejectsChangedInputsBeforeWriting(t *testing.T) {
	root := generatedProject(t, templates.Options{})
	sourcePath := filepath.Join(root, "api/openapi/example.yaml")
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	changed := bytes.Replace(source, []byte("shop HTTP API"), []byte("updated HTTP API"), 1)
	if err := os.WriteFile(sourcePath, changed, 0644); err != nil {
		t.Fatal(err)
	}
	plan, err := PrepareOpenAPIRefresh(root)
	if err != nil {
		t.Fatal(err)
	}
	indexPath := filepath.Join(root, "api/openapi/swagger.json")
	originalIndex, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	registryPath := filepath.Join(root, "internal/apiserver/bootstrap/modules_gen.go")
	registry, err := os.ReadFile(registryPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sourcePath, append(changed, []byte("\n# newer source\n")...), 0644); err != nil {
		t.Fatal(err)
	}
	if err := plan.Write(); err == nil || !strings.Contains(err.Error(), "changed after") {
		t.Fatalf("stale plan was not rejected: %v", err)
	}
	currentIndex, _ := os.ReadFile(indexPath)
	currentRegistry, _ := os.ReadFile(registryPath)
	if !bytes.Equal(originalIndex, currentIndex) || !bytes.Equal(registry, currentRegistry) {
		t.Fatal("stale plan changed generated files")
	}
}

func TestOpenAPIRefreshRollsBackEarlierReplacementOnFailure(t *testing.T) {
	root := generatedProject(t, templates.Options{})
	sourcePath := filepath.Join(root, "api/openapi/example.yaml")
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	changed := bytes.Replace(source, []byte("shop HTTP API"), []byte("updated HTTP API"), 1)
	if err := os.WriteFile(sourcePath, changed, 0644); err != nil {
		t.Fatal(err)
	}
	plan, err := PrepareOpenAPIRefresh(root)
	if err != nil {
		t.Fatal(err)
	}
	original := make(map[string][]byte, len(plan.updates))
	for _, update := range plan.updates {
		original[update.Path] = bytes.Clone(update.Original)
	}
	writes := 0
	err = plan.writeWith(func(path string, data []byte, mode os.FileMode) error {
		writes++
		if writes == 2 {
			return errors.New("injected replacement failure")
		}
		return writeAtomic(path, data, mode)
	})
	if err == nil || !strings.Contains(err.Error(), "injected replacement failure") {
		t.Fatalf("replacement failure was not returned: %v", err)
	}
	for relative, expected := range original {
		actual, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
		if readErr != nil || !bytes.Equal(actual, expected) {
			t.Fatalf("%s was not rolled back: %v", relative, readErr)
		}
	}
}
