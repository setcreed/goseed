package integration

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/setcreed/goseed/internal/templates"
)

func TestDualProjects(t *testing.T) {
	for _, backend := range []string{"sqlite", "mysql", "postgres", "none"} {
		for _, redis := range []bool{false, true} {
			for _, authorization := range []string{"none", "casbin"} {
				label := backend + "-" + authorization + map[bool]string{false: "", true: "-redis"}[redis]
				t.Run(label, func(t *testing.T) {
					directory := renderProjectWith(t, templates.Options{Database: backend, Redis: redis, Authz: authorization, Transport: "dual"})
					verifyReadonly(t, directory)
					run(t, directory, "go", "test", "-mod=readonly", "-run", "^$", "./...")
					runWith(t, directory, []string{"CGO_ENABLED=0", "GOOS=linux", "GOARCH=amd64"}, "go", "build", "-mod=readonly", "-o", "bin/apiserver-linux", "./cmd/apiserver")
					runWith(t, directory, []string{"CGO_ENABLED=0", "GOOS=linux", "GOARCH=amd64"}, "go", "build", "-mod=readonly", "-o", "bin/exampleclient-linux", "./cmd/exampleclient")
					if (backend == "sqlite" || backend == "none") && ((!redis && authorization == "none") || (redis && authorization == "casbin")) {
						run(t, directory, "go", "test", "-mod=readonly", "-race", "./...")
						run(t, directory, "go", "vet", "-mod=readonly", "./...")
						root, err := filepath.Abs("../..")
						if err != nil {
							t.Fatal(err)
						}
						run(t, directory, filepath.Join(root, ".tools/bin/golangci-lint"), "run", "./...")
						verifyGRPCProto(t, directory, root)
						binary, client := buildDual(t, directory)
						var env []string
						if redis {
							env = []string{"APP_REDIS_ADDRESS=127.0.0.1:1", "APP_REDIS_PASSWORD=placeholder", "APP_REDIS_TLS=false"}
						}
						verifyDualProcess(t, directory, binary, client, backend, authorization, env, true)
					}
				})
			}
		}
	}
}

func buildDual(t *testing.T, directory string) (string, string) {
	t.Helper()
	binary, client := filepath.Join(directory, "bin/apiserver"), filepath.Join(directory, "bin/exampleclient")
	run(t, directory, "go", "build", "-mod=readonly", "-o", binary, "./cmd/apiserver")
	run(t, directory, "go", "build", "-mod=readonly", "-o", client, "./cmd/exampleclient")
	return binary, client
}

func startDual(t *testing.T, directory, binary, client string, httpClient *http.Client, scheme string, env, options []string) (*serviceProcess, string) {
	t.Helper()
	process := startServiceWith(t, directory, binary, httpClient, scheme, append(append([]string{}, env...), "APP_GRPC_ADDRESS=127.0.0.1:0"))
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(process.log)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(string(data), "\n") {
			var entry struct {
				Message string `json:"msg"`
				Address string `json:"address"`
			}
			if json.Unmarshal([]byte(line), &entry) == nil && entry.Message == "gRPC server started" {
				status := rpcCall(t, directory, client, entry.Address, "", env, options, []string{"--action", "health"}, "")
				if !strings.Contains(string(status), "SERVING") || strings.Contains(string(status), "NOT_SERVING") {
					t.Fatal("dual RPC is not ready")
				}
				return process, entry.Address
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("dual RPC startup timed out")
	return nil, ""
}

func verifyDualProcess(t *testing.T, directory, binary, client, backend, authorization string, env []string, tlsProof bool) {
	t.Helper()
	runWith(t, directory, env, binary, "--help")
	run(t, directory, client, "--help")
	runWith(t, directory, env, binary, "dev", "init")
	if backend != "none" {
		runWith(t, directory, env, binary, "migrate", "up")
	}
	if authorization == "casbin" {
		installCasbinPolicy(t, directory, binary, backend, env, casbinPolicy(backend))
	}
	alice := strings.TrimSpace(string(runWith(t, directory, env, binary, "dev", "token", "--subject", "alice")))
	bob := strings.TrimSpace(string(runWith(t, directory, env, binary, "dev", "token", "--subject", "bob")))
	role := strings.TrimSpace(string(runWith(t, directory, env, binary, "dev", "token", "--subject", "role:reader")))
	httpClient := &http.Client{Timeout: 3 * time.Second}
	defer httpClient.CloseIdleConnections()
	options := []string{"--tls=false"}
	process, address := startDual(t, directory, binary, client, httpClient, "http", env, options)
	operation := []string{"--action", "list"}
	if backend == "none" {
		operation = []string{"--action", "greet"}
	}
	request(t, httpClient, process.url, "GET", "/v1/examples", "", "", 401)
	rpcCall(t, directory, client, address, "", env, options, operation, "Unauthenticated")
	if authorization == "casbin" {
		request(t, httpClient, process.url, "GET", "/v1/examples", "", role, 403)
		rpcCall(t, directory, client, address, role, env, options, operation, "PermissionDenied")
	}
	var item struct {
		ID          uint64 `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Enabled     bool   `json:"enabled"`
	}
	if backend == "none" {
		httpData, _ := request(t, httpClient, process.url, "GET", "/v1/examples?name=dual-private", "", alice, 200)
		rpcData := rpcCall(t, directory, client, address, alice, env, options, []string{"--action", "greet", "--name", "dual-private"}, "")
		var a, b map[string]string
		if json.Unmarshal(httpData, &a) != nil || json.Unmarshal(rpcData, &b) != nil || a["message"] != b["message"] || a["subject"] != b["subject"] {
			t.Fatal("greeting semantics differ")
		}
		request(t, httpClient, process.url, "GET", "/v1/examples?name=", "", alice, 400)
		rpcCall(t, directory, client, address, alice, env, options, []string{"--action", "greet", "--name", " "}, "InvalidArgument")
	} else {
		data, _ := request(t, httpClient, process.url, "POST", "/v1/examples", `{"name":"dual-private","description":"initial"}`, alice, 201)
		if json.Unmarshal(data, &item) != nil || item.ID == 0 || !item.Enabled {
			t.Fatal("HTTP create result")
		}
		id := strconv.FormatUint(item.ID, 10)
		operation = []string{"--action", "get", "--id", id}
		rpcCall(t, directory, client, address, alice, env, options, operation, "")
		request(t, httpClient, process.url, "POST", "/v1/examples", `{"name":"dual-private"}`, alice, 409)
		rpcCall(t, directory, client, address, alice, env, options, []string{"--action", "create", "--name", "dual-private"}, "AlreadyExists")
		request(t, httpClient, process.url, "GET", "/v1/examples/"+id, "", bob, 404)
		rpcCall(t, directory, client, address, bob, env, options, operation, "NotFound")
		rpcCall(t, directory, client, address, alice, env, options, []string{"--action", "update", "--id", id, "--enabled=false", "--description", ""}, "")
		data, _ = request(t, httpClient, process.url, "GET", "/v1/examples/"+id, "", alice, 200)
		if json.Unmarshal(data, &item) != nil || item.Enabled || item.Description != "" {
			t.Fatal("RPC zero update not visible to HTTP")
		}
		request(t, httpClient, process.url, "PATCH", "/v1/examples/"+id, `{"enabled":true,"description":"shared-update"}`, alice, 200)
		data = rpcCall(t, directory, client, address, alice, env, options, operation, "")
		var result struct {
			Enabled     bool   `json:"enabled"`
			Description string `json:"description"`
		}
		if json.Unmarshal(data, &result) != nil || !result.Enabled || result.Description != "shared-update" {
			t.Fatal("HTTP update not visible to RPC")
		}
		request(t, httpClient, process.url, "GET", "/v1/examples?page=0", "", alice, 400)
		rpcCall(t, directory, client, address, alice, env, options, []string{"--action", "list", "--page", "0"}, "InvalidArgument")
	}
	process.stop(t)
	restart, restartAddress := startDual(t, directory, binary, client, httpClient, "http", env, options)
	rpcCall(t, directory, client, restartAddress, alice, env, options, operation, "")
	if backend != "none" {
		id := strconv.FormatUint(item.ID, 10)
		request(t, httpClient, restart.url, "GET", "/v1/examples/"+id, "", alice, 200)
		rpcCall(t, directory, client, restartAddress, alice, env, options, []string{"--action", "delete", "--id", id}, "")
		request(t, httpClient, restart.url, "GET", "/v1/examples/"+id, "", alice, 404)
	}
	restart.stop(t)
	for _, logPath := range []string{process.log, restart.log} {
		data, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatal(err)
		}
		for _, private := range []string{alice, bob, role, "dual-private", "shared-update"} {
			if strings.Contains(string(data), private) {
				t.Fatal("dual log disclosed input")
			}
		}
		for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
			if !json.Valid([]byte(line)) {
				t.Fatal("dual log bypassed JSON")
			}
		}
	}
	rejectCommand(t, directory, binary, env, "dev", "init", "--grpc-address", "127.0.0.1:0")
	rejectCommand(t, directory, binary, append(append([]string{}, env...), "APP_GRPC_TLS_CERT=missing", "APP_GRPC_TLS_KEY=missing"), "serve")
	if tlsProof {
		cert, key := grpcCertificate(t, directory)
		secure := append(append([]string{}, env...), "APP_HTTP_TLS_CERT="+cert, "APP_HTTP_TLS_KEY="+key, "APP_GRPC_TLS_CERT="+cert, "APP_GRPC_TLS_KEY="+key)
		if len(env) == 0 {
			secure = append(secure, "APP_ENV=production")
		}
		pem, err := os.ReadFile(cert)
		if err != nil {
			t.Fatal(err)
		}
		roots := x509.NewCertPool()
		if !roots.AppendCertsFromPEM(pem) {
			t.Fatal("test CA")
		}
		transport := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}}
		defer transport.CloseIdleConnections()
		trusted := &http.Client{Timeout: 3 * time.Second, Transport: transport}
		options = []string{"--ca", cert}
		secureProcess, secureAddress := startDual(t, directory, binary, client, trusted, "https", secure, options)
		operation = []string{"--action", "list"}
		if backend == "none" {
			operation = []string{"--action", "greet"}
		}
		request(t, trusted, secureProcess.url, "GET", "/v1/examples", "", alice, 200)
		rpcCall(t, directory, client, secureAddress, alice, secure, options, operation, "")
		rpcCall(t, directory, client, secureAddress, alice, secure, nil, operation, "Unavailable")
		rpcCall(t, directory, client, secureAddress, alice, secure, []string{"--ca", cert, "--server-name", "wrong.example"}, operation, "Unavailable")
		secureProcess.stop(t)
	}
}

func TestExternalDualProjects(t *testing.T) {
	if os.Getenv("GOSEED_TEMPLATE_DB") != "1" {
		t.Skip("run make template-db for real dual SQL verification")
	}
	for _, backend := range []string{"mysql", "postgres"} {
		t.Run(backend, func(t *testing.T) {
			directory := renderProjectWith(t, templates.Options{Database: backend, Authz: "casbin", Transport: "dual"})
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
				t.Fatal("real SQL dual verification skipped a test")
			}
			binary, client := buildDual(t, directory)
			env := []string{"APP_DATABASE_HOST=127.0.0.1", "APP_DATABASE_PORT=" + port, "APP_DATABASE_USER=" + user, "APP_DATABASE_PASSWORD=" + password, "APP_DATABASE_NAME=goseed_test", "APP_DATABASE_TLS=true", "APP_DATABASE_CA_FILE=" + ca}
			verifyDualProcess(t, directory, binary, client, backend, "casbin", env, false)
		})
	}
}

func TestRealDualProjects(t *testing.T) {
	if os.Getenv("GOSEED_TEMPLATE_REDIS") != "1" {
		t.Skip("run make template-redis for isolated dual Redis verification")
	}
	address, password, ca := os.Getenv("GOSEED_TEST_REDIS_ADDRESS"), os.Getenv("GOSEED_TEST_REDIS_PASSWORD"), os.Getenv("GOSEED_TEST_REDIS_CA_FILE")
	if address == "" || password == "" || ca == "" {
		t.Fatal("explicit Redis TLS fixture required")
	}
	for _, backend := range []string{"sqlite", "none"} {
		t.Run(backend, func(t *testing.T) {
			directory := renderProjectWith(t, templates.Options{Database: backend, Transport: "dual", Redis: true, Authz: "casbin"})
			verifyReadonly(t, directory)
			fixtures := []string{"GOSEED_TEST_REDIS=1", "GOSEED_TEST_REDIS_ADDRESS=" + address, "GOSEED_TEST_REDIS_PASSWORD=" + password, "GOSEED_TEST_REDIS_CA_FILE=" + ca}
			output := runWith(t, directory, fixtures, "go", "test", "-mod=readonly", "-race", "-count=1", "-v", "./...")
			if strings.Contains(string(output), "--- SKIP:") {
				t.Fatal("real Redis dual verification skipped a test")
			}
			binary, client := buildDual(t, directory)
			env := []string{"APP_REDIS_ADDRESS=" + address, "APP_REDIS_PASSWORD=" + password, "APP_REDIS_TLS=true", "APP_REDIS_CA_FILE=" + ca, "APP_REDIS_KEY_PREFIX=goseed_dual_" + backend}
			verifyDualProcess(t, directory, binary, client, backend, "casbin", env, false)
			token := strings.TrimSpace(string(runWith(t, directory, env, binary, "dev", "token", "--subject", "alice")))
			httpClient := &http.Client{Timeout: 3 * time.Second}
			defer httpClient.CloseIdleConnections()
			options := []string{"--tls=false"}
			process, rpcAddress := startDual(t, directory, binary, client, httpClient, "http", env, options)
			path, operation := "/v1/examples?name=redis-dual-private", []string{"--action", "greet", "--name", "redis-dual-private"}
			if backend != "none" {
				data, _ := request(t, httpClient, process.url, "POST", "/v1/examples", `{"name":"redis-dual-private"}`, token, 201)
				var item struct {
					ID uint64 `json:"id"`
				}
				if json.Unmarshal(data, &item) != nil || item.ID == 0 {
					t.Fatal("dual Redis item")
				}
				id := strconv.FormatUint(item.ID, 10)
				path, operation = "/v1/examples/"+id, []string{"--action", "get", "--id", id}
			}
			for range 2 {
				request(t, httpClient, process.url, "GET", path, "", token, 200)
				rpcCall(t, directory, client, rpcAddress, token, env, options, operation, "")
			}
			process.stop(t)
			// Cache remains warm while the offline policy is revoked.
			installCasbinPolicy(t, directory, binary, backend, env, "")
			revoked, revokedAddress := startDual(t, directory, binary, client, httpClient, "http", env, options)
			request(t, httpClient, revoked.url, "GET", path, "", token, 403)
			rpcCall(t, directory, client, revokedAddress, token, env, options, operation, "PermissionDenied")
			revoked.stop(t)
			installCasbinPolicy(t, directory, binary, backend, env, casbinPolicy(backend))
			unavailable := append(append([]string{}, env...), "APP_REDIS_ADDRESS=127.0.0.1:1")
			fallback, fallbackAddress := startDual(t, directory, binary, client, httpClient, "http", unavailable, options)
			request(t, httpClient, fallback.url, "GET", path, "", token, 200)
			rpcCall(t, directory, client, fallbackAddress, token, unavailable, options, operation, "")
			fallback.stop(t)
			rejectCommand(t, directory, binary, append(append([]string{}, unavailable...), "APP_REDIS_REQUIRED=true"), "serve")
			requiredEnv := append(append([]string{}, env...), "APP_REDIS_REQUIRED=true")
			required, requiredAddress := startDual(t, directory, binary, client, httpClient, "http", requiredEnv, options)
			optional, optionalAddress := startDual(t, directory, binary, client, httpClient, "http", env, options)
			signalRedisFault(t, true)
			t.Cleanup(func() { signalRedisFault(t, false) })
			checkRedisReadiness(t, httpClient, required.url, 503)
			check := rpcCall(t, directory, client, requiredAddress, "", requiredEnv, options, []string{"--action", "health"}, "")
			if !strings.Contains(string(check), "NOT_SERVING") {
				t.Fatal("dual RPC readiness ignored required Redis")
			}
			request(t, httpClient, required.url, "GET", path, "", token, 503)
			rpcCall(t, directory, client, requiredAddress, token, requiredEnv, options, operation, "Unavailable")
			request(t, httpClient, optional.url, "GET", path, "", token, 200)
			rpcCall(t, directory, client, optionalAddress, token, env, options, operation, "")
			signalRedisFault(t, false)
			// Retries are disabled per command. After a Redis restart, stale pool
			// connections can fail their first Ping before being replaced.
			deadline := time.Now().Add(5 * time.Second)
			for {
				response, err := httpClient.Get(required.url + "/readyz")
				if err != nil {
					t.Fatal(err)
				}
				_ = response.Body.Close()
				check = rpcCall(t, directory, client, requiredAddress, "", requiredEnv, options, []string{"--action", "health"}, "")
				if response.StatusCode == 200 && strings.Contains(string(check), "SERVING") && !strings.Contains(string(check), "NOT_SERVING") {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("both protocols must recover after Redis reconnects")
				}
				time.Sleep(50 * time.Millisecond)
			}
			request(t, httpClient, required.url, "GET", path, "", token, 200)
			rpcCall(t, directory, client, requiredAddress, token, requiredEnv, options, operation, "")
			required.stop(t)
			optional.stop(t)
			for _, logPath := range []string{process.log, revoked.log, fallback.log, required.log, optional.log} {
				data, err := os.ReadFile(logPath)
				if err != nil {
					t.Fatal(err)
				}
				for _, secret := range []string{password, token, "redis-dual-private"} {
					if strings.Contains(string(data), secret) {
						t.Fatal("dual Redis log disclosed input")
					}
				}
				for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
					if !json.Valid([]byte(line)) {
						t.Fatal("dual Redis log bypassed JSON")
					}
				}
			}
		})
	}
}
