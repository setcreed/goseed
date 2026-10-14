package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/setcreed/goseed/internal/templates"
)

func TestGeneratedMakefileRespectsGoCaches(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	var base []string
	for _, variable := range os.Environ() {
		key, _, _ := strings.Cut(variable, "=")
		switch key {
		case "GOCACHE", "GOMODCACHE", "GOLANGCI_LINT_CACHE", "GOENV", "GOTOOLCHAIN", "GOWORK":
			continue
		}
		base = append(base, variable)
	}
	base = append(base, "GOTOOLCHAIN=local", "GOWORK=off")
	for _, database := range []string{"sqlite", "none"} {
		t.Run(database, func(t *testing.T) {
			directory := renderProjectWith(t, templates.Options{Database: database})
			probe := "include Makefile\ngoseed-cache-probe:\n\t@$(GO) env GOCACHE GOMODCACHE\n\t@printf '%s\\n' \"$$GOLANGCI_LINT_CACHE\"\n"
			if err := os.WriteFile(filepath.Join(directory, "cache-probe.mk"), []byte(probe), 0600); err != nil {
				t.Fatal(err)
			}
			goenv := filepath.Join(directory, "goenv")
			cache := filepath.Join(root, ".tmp", "go-cache")
			modules := filepath.Join(root, ".tmp", "mod-cache")
			if err := os.WriteFile(goenv, []byte("GOCACHE="+cache+"\nGOMODCACHE="+modules+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			for _, test := range []struct {
				name, lintCache string
				env             []string
				build           bool
			}{
				{name: "defaults", env: []string{"GOENV=off"}},
				{name: "environment", lintCache: filepath.Join(directory, "custom-lint-cache"), env: []string{
					"GOENV=off", "GOCACHE=" + cache, "GOMODCACHE=" + modules,
					"GOLANGCI_LINT_CACHE=" + filepath.Join(directory, "custom-lint-cache"),
				}},
				{name: "go env configuration", env: []string{"GOENV=" + goenv}, build: true},
			} {
				t.Run(test.name, func(t *testing.T) {
					env := append(append([]string{}, base...), test.env...)
					invoke := func(args ...string) string {
						t.Helper()
						command := exec.Command(args[0], args[1:]...)
						command.Dir, command.Env = directory, env
						output, err := command.CombinedOutput()
						if err != nil {
							t.Fatalf("%v: %v\n%s", args, err, output)
						}
						return string(output)
					}
					want := invoke("go", "env", "GOCACHE", "GOMODCACHE") + test.lintCache + "\n"
					if got := invoke("make", "--no-print-directory", "--silent", "-f", "cache-probe.mk", "goseed-cache-probe"); got != want {
						t.Fatalf("Makefile changed cache settings:\ngot  %q\nwant %q", got, want)
					}
					if test.build {
						invoke("make", "--no-print-directory", "build")
					}
					if _, err := os.Stat(filepath.Join(directory, ".tmp")); !os.IsNotExist(err) {
						t.Fatalf("Makefile created project-local .tmp: %v", err)
					}
				})
			}
		})
	}
}
