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

func verifyReadonly(t *testing.T, directory string) {
	t.Helper()
	for _, name := range []string{"go.mod", "go.sum"} {
		original, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			current, err := os.ReadFile(filepath.Join(directory, name))
			if err != nil || string(original) != string(current) {
				t.Errorf("readonly checks changed %s", name)
			}
		})
	}
}

func TestSQLProjects(t *testing.T) {
	for _, backend := range []string{"mysql", "postgres"} {
		t.Run(backend, func(t *testing.T) {
			directory := renderProjectWith(t, templates.Options{Database: backend})
			verifyReadonly(t, directory)
			run(t, directory, "go", "test", "-mod=readonly", "./...")
			run(t, directory, "go", "vet", "-mod=readonly", "./...")
			run(t, directory, "go", "test", "-mod=readonly", "-race", "./...")
			root, err := filepath.Abs("../..")
			if err != nil {
				t.Fatal(err)
			}
			run(t, directory, filepath.Join(root, ".tools/bin/golangci-lint"), "run", "./...")
			runWith(t, directory, []string{"CGO_ENABLED=0", "GOOS=linux", "GOARCH=amd64"}, "go", "build", "-mod=readonly", "-o", "bin/apiserver", "./cmd/apiserver")
		})
	}
}

func TestExternalSQLProjects(t *testing.T) {
	if os.Getenv("GOSEED_TEMPLATE_DB") != "1" {
		t.Skip("run make template-db for isolated real SQL template verification")
	}
	for _, backend := range []string{"mysql", "postgres"} {
		t.Run(backend, func(t *testing.T) {
			directory := renderProjectWith(t, templates.Options{Database: backend})
			verifyReadonly(t, directory)
			port := os.Getenv("GOSEED_TEST_" + strings.ToUpper(backend) + "_PORT")
			password := os.Getenv("GOSEED_TEST_PASSWORD")
			caFile := os.Getenv("GOSEED_TEST_CA_FILE")
			if port == "" || password == "" || caFile == "" {
				t.Fatal("dedicated database ports, credentials and test CA required")
			}
			user := "root"
			if backend == "postgres" {
				user = "goseed_test"
			}
			fixtureEnv := []string{"GOSEED_TEST_EXTERNAL=1", "GOSEED_TEST_DATABASE_HOST=127.0.0.1", "GOSEED_TEST_DATABASE_PORT=" + port, "GOSEED_TEST_DATABASE_USER=" + user, "GOSEED_TEST_DATABASE_PASSWORD=" + password}
			output := runWith(t, directory, fixtureEnv, "go", "test", "-mod=readonly", "-race", "-count=1", "-v", "./...")
			if strings.Contains(string(output), "--- SKIP:") {
				t.Fatal("real database verification skipped a test")
			}
			binary := filepath.Join(directory, "bin/apiserver")
			run(t, directory, "go", "build", "-mod=readonly", "-o", binary, "./cmd/apiserver")
			env := []string{"APP_DATABASE_HOST=127.0.0.1", "APP_DATABASE_PORT=" + port, "APP_DATABASE_USER=" + user, "APP_DATABASE_PASSWORD=" + password, "APP_DATABASE_NAME=goseed_test", "APP_DATABASE_TLS=true", "APP_DATABASE_CA_FILE=" + caFile}
			runWith(t, directory, env, binary, "dev", "init")
			if output := runWith(t, directory, env, binary, "migrate", "status"); !strings.Contains(string(output), "schema version: 0") {
				t.Fatal("initial schema status")
			}
			for range 2 {
				runWith(t, directory, env, binary, "migrate", "up")
			}
			verifyHTTPWith(t, directory, binary, env)
			for _, invalid := range [][]string{
				{"APP_DATABASE_PASSWORD=invalid-private-password"},
				{"APP_DATABASE_CA_FILE=" + filepath.Join(directory, "missing-private-ca.pem")},
				{"APP_DATABASE_CA_FILE="},       // Test CA is not part of the system trust store.
				{"APP_DATABASE_HOST=localhost"}, // Certificate has only the loopback IP SAN.
				{"APP_ENV=production", "APP_LOG_FORMAT=json", "APP_DATABASE_TLS=false", "APP_DATABASE_CA_FILE="},
			} {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				command := exec.CommandContext(ctx, binary, "migrate", "status")
				command.Dir = directory
				command.Env = append(append(environment(t), env...), invalid...)
				output, err := command.CombinedOutput()
				cancel()
				if err == nil || ctx.Err() == context.DeadlineExceeded {
					t.Fatal("invalid SQL security settings accepted or hung")
				}
				for _, secret := range []string{password, "invalid-private-password", "missing-private-ca"} {
					if strings.Contains(string(output), secret) {
						t.Fatal("SQL connection failure disclosed private input")
					}
				}
			}
		})
	}
}
