package integration

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/setcreed/goseed/internal/templates"
)

func TestCasbinProjects(t *testing.T) {
	for _, backend := range []string{"sqlite", "mysql", "postgres", "none"} {
		for _, redis := range []bool{false, true} {
			label := backend
			if redis {
				label += "-redis"
			}
			t.Run(label, func(t *testing.T) {
				directory := renderProjectWith(t, templates.Options{Database: backend, Redis: redis, Authz: "casbin"})
				verifyReadonly(t, directory)
				run(t, directory, "go", "test", "-mod=readonly", "./...")
				run(t, directory, "go", "vet", "-mod=readonly", "./...")
				run(t, directory, "go", "test", "-mod=readonly", "-race", "./...")
				root, err := filepath.Abs("../..")
				if err != nil {
					t.Fatal(err)
				}
				run(t, directory, filepath.Join(root, ".tools/bin/golangci-lint"), "run", "./...")
				runWith(t, directory, []string{"CGO_ENABLED=0", "GOOS=linux", "GOARCH=amd64"}, "go", "build", "-mod=readonly", "-o", "bin/apiserver-linux", "./cmd/apiserver")
				if backend == "sqlite" || backend == "none" {
					binary := filepath.Join(directory, "bin/apiserver")
					run(t, directory, "go", "build", "-mod=readonly", "-o", binary, "./cmd/apiserver")
					var env []string
					if redis {
						env = []string{"APP_REDIS_ADDRESS=127.0.0.1:1", "APP_REDIS_PASSWORD=unit-placeholder", "APP_REDIS_TLS=false"}
					}
					verifyCasbinHTTP(t, directory, binary, backend, env)
				}
			})
		}
	}
}

func installCasbinPolicy(t *testing.T, directory, binary, backend string, env []string, policy string) {
	t.Helper()
	path := filepath.Join(directory, "configs/policy.csv")
	if err := os.WriteFile(path, []byte(policy), 0600); err != nil {
		t.Fatal(err)
	}
	if backend != "none" {
		output := runWith(t, directory, env, binary, "policy", "replace", "--file", path)
		if !strings.Contains(string(output), "restart running instances") {
			t.Fatal("policy command omitted reload instruction")
		}
	}
}

func casbinPolicy(backend string) string {
	text := "p, role:reader, example, read\ng, user:alice, role:reader\ng, user:bob, role:reader\n"
	if backend != "none" {
		for _, action := range []string{"create", "update", "delete"} {
			text += "p, role:reader, example, " + action + "\n"
		}
	}
	return text
}

func verifyCasbinHTTP(t *testing.T, directory, binary, backend string, env []string) {
	t.Helper()
	runWith(t, directory, env, binary, "--help")
	runWith(t, directory, env, binary, "dev", "init")
	if backend != "none" {
		for range 2 {
			runWith(t, directory, env, binary, "migrate", "up")
		}
		if output := runWith(t, directory, env, binary, "migrate", "status"); !strings.Contains(string(output), "schema version: 2") {
			t.Fatal("policy migration version")
		}
	}
	token := func(subject string) string {
		return strings.TrimSpace(string(runWith(t, directory, env, binary, "dev", "token", "--subject", subject)))
	}
	alice, bob, role := token("alice"), token("bob"), token("role:reader")
	client := &http.Client{Timeout: 3 * time.Second}
	defer client.CloseIdleConnections()
	process := startServiceWith(t, directory, binary, client, "http", env)
	request(t, client, process.url, "GET", "/healthz", "", "", 200)
	request(t, client, process.url, "GET", "/v1/examples", "", "", 401)
	request(t, client, process.url, "GET", "/v1/examples", "", alice, 403)
	process.stop(t)
	policy := "p, role:reader, example, read\ng, user:bob, role:reader\n"
	if backend == "none" {
		policy += "g, user:alice, role:reader\n"
	} else {
		policy += "g, user:alice, role:writer\n"
		for _, operation := range []string{"read", "create", "update", "delete"} {
			policy += "p, role:writer, example, " + operation + "\n"
		}
	}
	installCasbinPolicy(t, directory, binary, backend, env, policy)
	process = startServiceWith(t, directory, binary, client, "http", env)
	path := "/v1/examples?name=authz-private"
	if backend != "none" {
		data, _ := request(t, client, process.url, "POST", "/v1/examples", `{"name":"authz-private"}`, alice, 201)
		item := resource(t, data)
		path = "/v1/examples/" + strconv.FormatUint(item.ID, 10)
		request(t, client, process.url, "GET", path, "", bob, 404)
		request(t, client, process.url, "PATCH", path, `{"enabled":false}`, bob, 403)
		request(t, client, process.url, "POST", "/v1/examples", `{"name":"not-created"}`, bob, 403)
		request(t, client, process.url, "PATCH", path, `{"enabled":false}`, alice, 200)
	}
	for range 2 {
		request(t, client, process.url, "GET", path, "", alice, 200)
	}
	request(t, client, process.url, "GET", path, "", role, 403)
	process.stop(t)
	invalid := filepath.Join(directory, "configs/invalid-policy.csv")
	if err := os.WriteFile(invalid, []byte("p, role:reader, example, *\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if backend != "none" {
		rejectCommand(t, directory, binary, env, "policy", "replace", "--file", invalid)
		rejectCommand(t, directory, binary, env, "policy", "replace")
	} else {
		rejectCommand(t, directory, binary, env, "policy", "replace", "--file", invalid)
		rejectCommand(t, directory, binary, append(append([]string{}, env...), "APP_AUTHZ_POLICY_FILE="+invalid), "serve")
		rejectCommand(t, directory, binary, append(append([]string{}, env...), "APP_AUTHZ_POLICY_FILE="+invalid+"missing"), "serve")
	}
	unchanged := startServiceWith(t, directory, binary, client, "http", env)
	request(t, client, unchanged.url, "GET", path, "", alice, 200)
	unchanged.stop(t)
	installCasbinPolicy(t, directory, binary, backend, env, "")
	revoked := startServiceWith(t, directory, binary, client, "http", env)
	request(t, client, revoked.url, "GET", path, "", alice, 403)
	revoked.stop(t)
	for _, process := range []*serviceProcess{process, unchanged, revoked} {
		data, err := os.ReadFile(process.log)
		if err != nil {
			t.Fatal(err)
		}
		for _, secret := range []string{alice, bob, role, "authz-private", "user:alice", "role:reader"} {
			if strings.Contains(string(data), secret) {
				t.Fatal("authorization log disclosed private input")
			}
		}
	}
}

func TestExternalCasbinProjects(t *testing.T) {
	if os.Getenv("GOSEED_TEMPLATE_DB") != "1" {
		t.Skip("run make template-db for isolated real Casbin SQL verification")
	}
	for _, backend := range []string{"mysql", "postgres"} {
		t.Run(backend, func(t *testing.T) {
			directory := renderProjectWith(t, templates.Options{Database: backend, Authz: "casbin"})
			verifyReadonly(t, directory)
			port, password, ca := os.Getenv("GOSEED_TEST_"+strings.ToUpper(backend)+"_PORT"), os.Getenv("GOSEED_TEST_PASSWORD"), os.Getenv("GOSEED_TEST_CA_FILE")
			if port == "" || password == "" || ca == "" {
				t.Fatal("explicit SQL TLS fixture required")
			}
			user := "root"
			if backend == "postgres" {
				user = "goseed_test"
			}
			fixtures := []string{"GOSEED_TEST_EXTERNAL=1", "GOSEED_TEST_DATABASE_HOST=127.0.0.1", "GOSEED_TEST_DATABASE_PORT=" + port, "GOSEED_TEST_DATABASE_USER=" + user, "GOSEED_TEST_DATABASE_PASSWORD=" + password}
			output := runWith(t, directory, fixtures, "go", "test", "-mod=readonly", "-race", "-count=1", "-v", "./...")
			if strings.Contains(string(output), "--- SKIP:") {
				t.Fatal("real policy SQL test skipped")
			}
			binary := filepath.Join(directory, "bin/apiserver")
			run(t, directory, "go", "build", "-mod=readonly", "-o", binary, "./cmd/apiserver")
			env := []string{"APP_DATABASE_HOST=127.0.0.1", "APP_DATABASE_PORT=" + port, "APP_DATABASE_USER=" + user, "APP_DATABASE_PASSWORD=" + password, "APP_DATABASE_NAME=goseed_test", "APP_DATABASE_TLS=true", "APP_DATABASE_CA_FILE=" + ca}
			verifyCasbinHTTP(t, directory, binary, backend, env)
		})
	}
}

func TestCasbinFileContainer(t *testing.T) {
	if os.Getenv("GOSEED_TEST_DOCKER") != "1" {
		t.Skip("run make template-container for actual Casbin policy container verification")
	}
	directory := renderProjectWith(t, templates.Options{Database: "none", Authz: "casbin"})
	verifyReadonly(t, directory)
	binary := filepath.Join(directory, "bin/apiserver")
	run(t, directory, "go", "build", "-mod=readonly", "-o", binary, "./cmd/apiserver")
	run(t, directory, binary, "dev", "init")
	verifyNoneContainer(t, directory, binary)
}
