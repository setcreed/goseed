package integration

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/setcreed/goseed/internal/templates"
)

func TestRedisProjects(t *testing.T) {
	for _, backend := range []string{"sqlite", "mysql", "postgres", "none"} {
		t.Run(backend, func(t *testing.T) {
			directory := renderProjectWith(t, templates.Options{Database: backend, Redis: true})
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

func TestRealRedisProjects(t *testing.T) {
	if os.Getenv("GOSEED_TEMPLATE_REDIS") != "1" {
		t.Skip("run make template-redis for isolated real Redis verification")
	}
	address, password, ca := os.Getenv("GOSEED_TEST_REDIS_ADDRESS"), os.Getenv("GOSEED_TEST_REDIS_PASSWORD"), os.Getenv("GOSEED_TEST_REDIS_CA_FILE")
	if address == "" || password == "" || ca == "" {
		t.Fatal("explicit Redis TLS fixture required")
	}
	for _, backend := range []string{"sqlite", "none"} {
		for _, authorization := range []string{"none", "casbin"} {
			t.Run(backend+"-"+authorization, func(t *testing.T) {
				directory := renderProjectWith(t, templates.Options{Database: backend, Redis: true, Authz: authorization})
				verifyReadonly(t, directory)
				fixtures := []string{"GOSEED_TEST_REDIS=1", "GOSEED_TEST_REDIS_ADDRESS=" + address, "GOSEED_TEST_REDIS_PASSWORD=" + password, "GOSEED_TEST_REDIS_CA_FILE=" + ca}
				output := runWith(t, directory, fixtures, "go", "test", "-mod=readonly", "-race", "-count=1", "-v", "./...")
				if strings.Contains(string(output), "--- SKIP:") {
					t.Fatal("real Redis verification skipped a test")
				}
				binary := filepath.Join(directory, "bin/apiserver")
				run(t, directory, "go", "build", "-mod=readonly", "-o", binary, "./cmd/apiserver")
				env := []string{"APP_REDIS_ADDRESS=" + address, "APP_REDIS_PASSWORD=" + password, "APP_REDIS_TLS=true", "APP_REDIS_CA_FILE=" + ca, "APP_REDIS_KEY_PREFIX=goseed_http_" + backend}
				runWith(t, directory, env, binary, "dev", "init")
				if backend == "sqlite" {
					runWith(t, directory, env, binary, "migrate", "up")
					if authorization == "none" {
						verifyHTTPWith(t, directory, binary, env)
					}
				}
				if authorization == "casbin" {
					verifyCasbinHTTP(t, directory, binary, backend, env)
					installCasbinPolicy(t, directory, binary, backend, env, casbinPolicy(backend))
				}
				token := strings.TrimSpace(string(runWith(t, directory, env, binary, "dev", "token", "--subject", "alice")))
				client := &http.Client{Timeout: 3 * time.Second}
				defer client.CloseIdleConnections()
				process := startServiceWith(t, directory, binary, client, "http", env)
				path := "/v1/examples?name=redis-private"
				if backend == "sqlite" {
					data, _ := request(t, client, process.url, "POST", "/v1/examples", `{"name":"redis-read"}`, token, 201)
					item := resource(t, data)
					path = "/v1/examples/" + strconv.FormatUint(item.ID, 10)
				}
				request(t, client, process.url, "GET", path, "", token, 200)
				request(t, client, process.url, "GET", path, "", token, 200)
				process.stop(t)
				// An unreachable loopback port deterministically verifies optional and required startup behavior.
				unavailable := append(append([]string{}, env...), "APP_REDIS_ADDRESS=127.0.0.1:1")
				fallback := startServiceWith(t, directory, binary, client, "http", unavailable)
				request(t, client, fallback.url, "GET", path, "", token, 200)
				fallback.stop(t)
				rejectCommand(t, directory, binary, append(unavailable, "APP_REDIS_REQUIRED=true"), "serve")
				// Required mode starts against the real fixture, then its pool is exercised through the process fault test.
				required := startServiceWith(t, directory, binary, client, "http", append(append([]string{}, env...), "APP_REDIS_REQUIRED=true"))
				request(t, client, required.url, "GET", path, "", token, 200)
				optional := startServiceWith(t, directory, binary, client, "http", env)
				signalRedisFault(t, true)
				t.Cleanup(func() { signalRedisFault(t, false) })
				checkRedisReadiness(t, client, required.url, 503)
				request(t, client, required.url, "GET", path, "", token, 503)
				request(t, client, optional.url, "GET", "/readyz", "", "", 200)
				request(t, client, optional.url, "GET", path, "", token, 200)
				if backend == "sqlite" {
					request(t, client, required.url, "PATCH", path, `{"enabled":false}`, token, 503)
					data, _ := request(t, client, optional.url, "GET", path, "", token, 200)
					if !resource(t, data).Enabled {
						t.Fatal("required Redis failure reached database")
					}
					request(t, client, optional.url, "PATCH", path, `{"description":"updated while cache down"}`, token, 200)
				}
				signalRedisFault(t, false)
				request(t, client, required.url, "GET", "/readyz", "", "", 200)
				request(t, client, required.url, "GET", path, "", token, 200)
				request(t, client, optional.url, "GET", path, "", token, 200)
				required.stop(t)
				optional.stop(t)
				for _, logPath := range []string{process.log, fallback.log, required.log, optional.log} {
					data, err := os.ReadFile(logPath)
					if err != nil {
						t.Fatal(err)
					}
					for _, secret := range []string{password, token, "redis-private", "redis-read"} {
						if strings.Contains(string(data), secret) {
							t.Fatal("Redis HTTP log disclosed input")
						}
					}
					for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
						if !json.Valid([]byte(line)) {
							t.Fatal("Redis failure bypassed JSON logging")
						}
					}
				}
			})
		}
	}
}

func checkRedisReadiness(t *testing.T, client *http.Client, base string, want int) {
	t.Helper()
	response, err := client.Get(base + "/readyz")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != want {
		t.Fatalf("readiness: %d want %d", response.StatusCode, want)
	}
}

func signalRedisFault(t *testing.T, stop bool) {
	t.Helper()
	name := os.Getenv("GOSEED_TEST_REDIS_CONTAINER")
	if !regexp.MustCompile(`^goseed-template-redis-[a-f0-9]{12}$`).MatchString(name) {
		t.Fatal("owned Redis container name required")
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	if label := strings.TrimSpace(string(run(t, root, "docker", "inspect", "--format", `{{index .Config.Labels "org.goseed.redis"}}`, name))); label != name {
		t.Fatal("Redis fixture label does not match owner")
	}
	if stop {
		run(t, root, "docker", "stop", "--time", "1", name)
		return
	}
	run(t, root, "docker", "start", name)
	if address := strings.TrimSpace(string(run(t, root, "docker", "port", name, "6379/tcp"))); address != os.Getenv("GOSEED_TEST_REDIS_ADDRESS") {
		t.Fatal("Redis fixture port changed across restart")
	}
	run(t, root, "docker", "exec", name, "sh", "-c", `for i in $(seq 1 100); do REDISCLI_AUTH=$(cat /tmp/test-password) redis-cli --tls --cacert /testtls/ca.crt -h 127.0.0.1 ping 2>/dev/null | grep -q PONG && exit 0; sleep .05; done; exit 1`)
}
