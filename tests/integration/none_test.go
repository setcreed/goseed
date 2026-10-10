package integration

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/setcreed/goseed/internal/templates"
)

func TestNoDatabaseProject(t *testing.T) {
	directory := renderProjectWith(t, templates.Options{Database: "none"})
	verifyReadonly(t, directory)
	run(t, directory, "go", "test", "-mod=readonly", "./...")
	run(t, directory, "go", "vet", "-mod=readonly", "./...")
	run(t, directory, "go", "test", "-mod=readonly", "-race", "./...")
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	run(t, directory, filepath.Join(root, ".tools/bin/golangci-lint"), "run", "./...")
	dependencies := string(run(t, directory, "go", "list", "-mod=readonly", "-deps", "./..."))
	for _, forbidden := range []string{"gorm.io/", "github.com/pressly/goose", "github.com/libtnb/sqlite", "modernc.org/", "github.com/go-sql-driver/mysql", "github.com/jackc/pgx"} {
		if strings.Contains(dependencies, forbidden) {
			t.Fatalf("database dependency: %s", forbidden)
		}
	}
	runWith(t, directory, []string{"CGO_ENABLED=0", "GOOS=linux", "GOARCH=amd64"}, "go", "build", "-mod=readonly", "-o", "bin/apiserver-linux", "./cmd/apiserver")
	binary := filepath.Join(directory, "bin/apiserver")
	run(t, directory, "go", "build", "-mod=readonly", "-o", binary, "./cmd/apiserver")
	// Check the first launch without file writes before measuring failure exits.
	if help := string(run(t, directory, binary, "--help")); strings.Contains(help, "migrate") {
		t.Fatal("help registered migration")
	}
	rejectCommand(t, directory, binary, nil, "migrate", "up")
	if _, err := os.Stat(filepath.Join(directory, "configs/app.yaml")); !os.IsNotExist(err) {
		t.Fatal("rejected migration created config")
	}
	run(t, directory, binary, "dev", "init")
	configPath := filepath.Join(directory, "configs/app.yaml")
	before, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(before), "database:") {
		t.Fatal("init contains database config")
	}
	run(t, directory, binary, "dev", "init")
	after, err := os.ReadFile(configPath)
	if err != nil || string(before) != string(after) {
		t.Fatal("init overwrote config")
	}
	for _, input := range []string{"database:\n  path: data/app.db\n", "database:\n  max_open: 4\n", "unknown: test-secret-private\n"} {
		filename := filepath.Join(directory, "configs/invalid.yaml")
		if err := os.WriteFile(filename, []byte(input), 0600); err != nil {
			t.Fatal(err)
		}
		rejectCommand(t, directory, binary, nil, "serve", "--config", filename)
	}
	rejectCommand(t, directory, binary, nil, "serve", "--config", "missing.yaml")
	rejectCommand(t, directory, binary, nil, "serve", "--config", "configs/app.example.yaml")
	rejectCommand(t, directory, binary, []string{"APP_JWT_SECRET=test-secret-private"}, "serve")
	production := []string{"APP_ENV=production", "APP_LOG_FORMAT=json"}
	rejectCommand(t, directory, binary, production, "dev", "token", "--subject", "alice")
	rejectCommand(t, directory, binary, production, "dev", "init", "--config", "configs/production-init.yaml")
	if _, err := os.Stat(filepath.Join(directory, "configs/production-init.yaml")); !os.IsNotExist(err) {
		t.Fatal("production init wrote config")
	}
	alice := strings.TrimSpace(string(run(t, directory, binary, "dev", "token", "--subject", "alice")))
	bob := strings.TrimSpace(string(run(t, directory, binary, "dev", "token", "--subject", "bob")))
	client := &http.Client{Timeout: 3 * time.Second}
	defer client.CloseIdleConnections()
	process := startServiceWith(t, directory, binary, client, "http", nil)
	request(t, client, process.url, "GET", "/healthz", "", "", 200)
	request(t, client, process.url, "GET", "/v1/examples", "", "", 401)
	request(t, client, process.url, "GET", "/v1/examples", "", "invalid", 401)
	for _, test := range []struct{ token, name, subject string }{{alice, "world", "alice"}, {bob, strings.Repeat("中", 80), "bob"}, {alice, "query-private", "alice"}} {
		data, _ := request(t, client, process.url, "GET", "/v1/examples?name="+url.QueryEscape(test.name), "", test.token, 200)
		verifyGreeting(t, data, "hello, "+test.name, test.subject)
	}
	for _, query := range []string{"name=", "name=a&name=b", "unknown=x", "name=%ZZ", "name=%ff", "name=%20a", "name=bad%00name", "name=bad%0aname", "name=a;unknown=b", "name=" + url.QueryEscape(strings.Repeat("中", 81))} {
		request(t, client, process.url, "GET", "/v1/examples?"+query, "", alice, 400)
	}
	for _, method := range []string{"POST", "PATCH", "DELETE"} {
		request(t, client, process.url, method, "/v1/examples", "", alice, 405)
	}
	request(t, client, process.url, "GET", "/v1/examples/1", "", alice, 404)
	process.stop(t)
	logs, err := os.ReadFile(process.log)
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{alice, bob, "query-private"} {
		if strings.Contains(string(logs), private) {
			t.Fatal("runtime log disclosed private data")
		}
	}
	for _, line := range strings.Split(string(before), "\n") {
		if strings.Contains(line, "secret:") {
			_, value, _ := strings.Cut(line, ":")
			value = strings.Trim(strings.TrimSpace(value), `"`)
			if strings.Contains(string(logs), value) {
				t.Fatal("secret disclosed")
			}
		}
	}
	for _, line := range strings.Split(strings.TrimSpace(string(logs)), "\n") {
		if !json.Valid([]byte(line)) {
			t.Fatal("non JSON log")
		}
	}
	restarted := startServiceWith(t, directory, binary, client, "http", nil)
	data, _ := request(t, client, restarted.url, "GET", "/v1/examples", "", alice, 200)
	verifyGreeting(t, data, "hello, world", "alice")
	restarted.stop(t)
	verifyTLS(t, directory, binary)
	for _, name := range []string{"data", "migrations", "internal/pkg/database", "internal/apiserver/store", "internal/apiserver/model"} {
		if _, err := os.Stat(filepath.Join(directory, name)); !os.IsNotExist(err) {
			t.Fatalf("database artifact exists: %s", name)
		}
	}
	if os.Getenv("GOSEED_TEST_DOCKER") == "1" {
		verifyNoneContainer(t, directory, binary)
	}
}

func verifyGreeting(t *testing.T, data []byte, message, subject string) {
	t.Helper()
	var result struct {
		Message string `json:"message"`
		Subject string `json:"subject"`
	}
	if json.Unmarshal(data, &result) != nil || result.Message != message || result.Subject != subject {
		t.Fatal("unexpected greeting response")
	}
}
