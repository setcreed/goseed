package integration

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/setcreed/goseed/internal/templates"
)

func renderProject(t *testing.T) string {
	return renderProjectWith(t, templates.Options{})
}

func renderProjectWith(t *testing.T, options templates.Options) string {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(root, ".tmp", "d2")
	if err := os.MkdirAll(base, 0750); err != nil {
		t.Fatal(err)
	}
	directory, err := os.MkdirTemp(base, "render-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(directory); err != nil {
			t.Error(err)
		}
	})
	files, err := templates.Render(templates.Project{Name: "demo", Module: "example.com/demo"}, options)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		filename := filepath.Join(directory, filepath.FromSlash(file.Path))
		if err := os.MkdirAll(filepath.Dir(filename), 0750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filename, file.Content, 0644); err != nil {
			t.Fatal(err)
		}
	}
	return directory
}

func run(t *testing.T, directory string, arguments ...string) []byte {
	return runWith(t, directory, nil, arguments...)
}

func runWith(t *testing.T, directory string, overrides []string, arguments ...string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, arguments[0], arguments[1:]...)
	command.Dir = directory
	command.Env = append(environment(t), overrides...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %v\n%s", arguments, err, output)
	}
	return output
}

func environment(t *testing.T) []string {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	var variables []string
	for _, variable := range os.Environ() {
		key, _, _ := strings.Cut(variable, "=")
		if !strings.HasPrefix(key, "APP_") {
			variables = append(variables, variable)
		}
	}
	return append(variables, "GOTOOLCHAIN=go1.26.4", "GOWORK=off", "GOCACHE="+filepath.Join(root, ".tmp/go-cache"), "GOMODCACHE="+filepath.Join(root, ".tmp/mod-cache"), "GOLANGCI_LINT_CACHE="+filepath.Join(root, ".tmp/lint-cache"))
}

func TestDefaultProject(t *testing.T) {
	directory := renderProject(t)
	manifests := map[string][]byte{}
	for _, name := range []string{"go.mod", "go.sum"} {
		data, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil {
			t.Fatal(err)
		}
		manifests[name] = data
	}
	t.Cleanup(func() {
		for name, original := range manifests {
			current, err := os.ReadFile(filepath.Join(directory, name))
			if err != nil || string(original) != string(current) {
				t.Errorf("generated %s changed during readonly verification", name)
			}
		}
	})
	run(t, directory, "go", "test", "-mod=readonly", "./...")
	run(t, directory, "go", "vet", "-mod=readonly", "./...")
	run(t, directory, "go", "test", "-mod=readonly", "-race", "./...")
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	run(t, directory, filepath.Join(root, ".tools/bin/golangci-lint"), "run", "./...")
	binary := filepath.Join(directory, "bin/apiserver")
	run(t, directory, "go", "build", "-mod=readonly", "-o", binary, "./cmd/apiserver")
	run(t, directory, binary, "dev", "init")
	configuration := filepath.Join(directory, "configs/app.yaml")
	before, err := os.ReadFile(configuration)
	if err != nil {
		t.Fatal(err)
	}
	run(t, directory, binary, "dev", "init")
	after, err := os.ReadFile(configuration)
	if err != nil || string(before) != string(after) {
		t.Fatal("development initialization replaced configuration")
	}
	if output := run(t, directory, binary, "migrate", "status"); !strings.Contains(string(output), "schema version: 0") {
		t.Fatal("initial migration status")
	}
	for range 2 {
		run(t, directory, binary, "migrate", "up")
	}
	if output := run(t, directory, binary, "migrate", "status"); !strings.Contains(string(output), "schema version: 1") {
		t.Fatal("applied migration status")
	}
	verifyHTTP(t, directory, binary)
	verifyStartupFailures(t, directory, binary)
	verifyTLS(t, directory, binary)
	if os.Getenv("GOSEED_TEST_DOCKER") == "1" {
		verifyContainer(t, directory, binary)
	}
}
