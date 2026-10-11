package integration

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/setcreed/goseed/internal/templates"
)

func TestGRPCProjects(t *testing.T) {
	for _, backend := range []string{"sqlite", "mysql", "postgres", "none"} {
		for _, redis := range []bool{false, true} {
			for _, authorization := range []string{"none", "casbin"} {
				label := backend + "-" + authorization
				if redis {
					label += "-redis"
				}
				t.Run(label, func(t *testing.T) {
					directory := renderProjectWith(t, templates.Options{Database: backend, Redis: redis, Authz: authorization, Transport: "grpc"})
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
						binary, client := filepath.Join(directory, "bin/apiserver"), filepath.Join(directory, "bin/exampleclient")
						run(t, directory, "go", "build", "-mod=readonly", "-o", binary, "./cmd/apiserver")
						run(t, directory, "go", "build", "-mod=readonly", "-o", client, "./cmd/exampleclient")
						env := []string{}
						if redis {
							env = []string{"APP_REDIS_ADDRESS=127.0.0.1:1", "APP_REDIS_PASSWORD=placeholder", "APP_REDIS_TLS=false"}
						}
						verifyGRPCProcess(t, directory, binary, client, backend, authorization, env, true)
					}
				})
			}
		}
	}
}
func verifyGRPCProto(t *testing.T, directory, root string) {
	t.Helper()
	if err := os.Symlink(filepath.Join(root, ".tools"), filepath.Join(directory, ".tools")); err != nil {
		t.Fatal(err)
	}
	original := map[string][]byte{}
	names := []string{"example.pb.go", "example_grpc.pb.go"}
	if _, err := os.Stat(filepath.Join(directory, "pkg/api/example/v1/example.pb.gw.go")); err == nil {
		names = append(names, "example.pb.gw.go")
	}
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(directory, "pkg/api/example/v1", name))
		if err != nil {
			t.Fatal(err)
		}
		original[name] = data
	}
	var swagger []byte
	if len(names) == 3 {
		var err error
		swagger, err = os.ReadFile(filepath.Join(directory, "api/openapi/swagger.json"))
		if err != nil {
			t.Fatal(err)
		}
	}
	run(t, directory, "make", "proto-generate")
	if len(names) == 3 {
		data, err := os.ReadFile(filepath.Join(directory, "api/openapi/swagger.json"))
		if err != nil || string(data) != string(swagger) {
			t.Fatal("Swagger reproduction differs")
		}
	}
	for name, want := range original {
		got, err := os.ReadFile(filepath.Join(directory, "pkg/api/example/v1", name))
		if err != nil || string(got) != string(want) {
			t.Fatalf("protocol regeneration differs %s", name)
		}
	}
	// An independent Go module must be able to import the delivered client types.
	consumer := filepath.Join(directory, ".tmp/client-import")
	if err := os.MkdirAll(consumer, 0750); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"go.mod", "go.sum"} {
		data, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil {
			t.Fatal(err)
		}
		if name == "go.mod" {
			data = []byte(strings.Replace(string(data), "module example.com/demo", "module example.com/consumer", 1) + "\nrequire example.com/demo v0.0.0\nreplace example.com/demo => ../..\n")
		}
		if err := os.WriteFile(filepath.Join(consumer, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(consumer, "client.go"), []byte("package consumer\nimport api \"example.com/demo/pkg/api/example/v1\"\nvar Client = api.NewExampleServiceClient\n"), 0600); err != nil {
		t.Fatal(err)
	}
	run(t, consumer, "go", "test", "-mod=readonly", "./...")
}
func startGRPC(t *testing.T, directory, binary, client string, env, clientOptions []string) *serviceProcess {
	t.Helper()
	log, err := os.CreateTemp(directory, "grpc-*.log")
	if err != nil {
		t.Fatal(err)
	}
	process := &serviceProcess{command: exec.Command(binary, "serve", "--address", "127.0.0.1:0"), done: make(chan struct{}), log: log.Name()}
	process.command.Dir = directory
	process.command.Env = append(append(environment(t), "APP_LOG_FORMAT=json"), env...)
	process.command.Stdout = log
	process.command.Stderr = log
	if err := process.command.Start(); err != nil {
		_ = log.Close()
		t.Fatal(err)
	}
	go func() { process.err = process.command.Wait(); close(process.done) }()
	t.Cleanup(func() {
		select {
		case <-process.done:
		default:
			_ = process.command.Process.Kill()
			select {
			case <-process.done:
			case <-time.After(3 * time.Second):
				t.Error("gRPC cleanup timeout")
			}
		}
		_ = log.Close()
	})
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-process.done:
			t.Fatalf("gRPC startup exited %v", process.err)
		default:
		}
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
				process.url = entry.Address
			}
		}
		if process.url != "" {
			args := append([]string{client, "--address", process.url, "--action", "health"}, clientOptions...)
			data := runWith(t, directory, env, args...)
			var health struct {
				Status string `json:"status"`
			}
			if json.Unmarshal(data, &health) == nil && health.Status == "SERVING" {
				return process
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("gRPC readiness timeout")
	return nil
}
func rpcCall(t *testing.T, directory, client, address, token string, env, options, args []string, expected string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, client, append(append([]string{"--address", address}, options...), args...)...)
	command.Dir = directory
	command.Env = append(append(environment(t), env...), "APP_CLIENT_TOKEN="+token)
	output, err := command.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatal("RPC client failed to finish")
	}
	if expected == "" {
		if err != nil || !json.Valid(output) {
			t.Fatalf("RPC failed %v %s", err, output)
		}
	} else if err == nil || !strings.Contains(string(output), expected) {
		t.Fatalf("RPC status %s got %v %s", expected, err, output)
	}
	return output
}
func verifyGRPCProcess(t *testing.T, directory, binary, client, backend, authorization string, env []string, tlsProof bool) {
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
	options := []string{"--tls=false"}
	process := startGRPC(t, directory, binary, client, env, options)
	operation := []string{"--action", "list"}
	if backend == "none" {
		operation = []string{"--action", "greet", "--name", "rpc-private"}
	}
	rpcCall(t, directory, client, process.url, "", env, options, operation, "Unauthenticated")
	rpcCall(t, directory, client, process.url, alice, env, options, operation, "")
	if authorization == "casbin" {
		rpcCall(t, directory, client, process.url, role, env, options, operation, "PermissionDenied")
	}
	if backend != "none" {
		data := rpcCall(t, directory, client, process.url, alice, env, options, []string{"--action", "create", "--name", "rpc-process-private"}, "")
		var item struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(data, &item); err != nil || item.ID == "" {
			t.Fatal("ID output")
		}
		operation = []string{"--action", "get", "--id", item.ID}
		rpcCall(t, directory, client, process.url, bob, env, options, operation, "NotFound")
		rpcCall(t, directory, client, process.url, alice, env, options, []string{"--action", "update", "--id", item.ID, "--enabled=false", "--description", ""}, "")
	}
	process.stop(t)
	restart := startGRPC(t, directory, binary, client, env, options)
	rpcCall(t, directory, client, restart.url, alice, env, options, operation, "")
	restart.stop(t)
	for _, path := range []string{process.log, restart.log} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, value := range []string{alice, bob, role, "rpc-private", "rpc-process-private"} {
			if strings.Contains(string(data), value) {
				t.Fatal("RPC log disclosed input")
			}
		}
		for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
			if !json.Valid([]byte(line)) {
				t.Fatal("RPC log bypassed JSON")
			}
		}
	}
	rejectCommand(t, directory, binary, append(append([]string{}, env...), "APP_GRPC_TLS_CERT=missing", "APP_GRPC_TLS_KEY=missing"), "serve")
	rejectCommand(t, directory, binary, append(append([]string{}, env...), "APP_ENV=production"), "serve")
	if tlsProof {
		verifyGRPCTLS(t, directory, binary, client, alice, env, operation)
	}
}
func verifyGRPCTLS(t *testing.T, directory, binary, client, token string, env, operation []string) {
	t.Helper()
	cert, keyPath := grpcCertificate(t, directory)
	secure := append(append([]string{}, env...), "APP_GRPC_TLS_CERT="+cert, "APP_GRPC_TLS_KEY="+keyPath)
	if len(env) == 0 {
		secure = append(secure, "APP_ENV=production")
	}
	options := []string{"--ca", cert}
	process := startGRPC(t, directory, binary, client, secure, options)
	rpcCall(t, directory, client, process.url, token, secure, options, operation, "")
	rpcCall(t, directory, client, process.url, token, secure, nil, operation, "Unavailable")
	rpcCall(t, directory, client, process.url, token, secure, []string{"--ca", cert, "--server-name", "wrong.example"}, operation, "Unavailable")
	process.stop(t)
}

func grpcCertificate(t *testing.T, directory string) (string, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	certificate := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, certificate, certificate, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	private, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	cert, keyPath := filepath.Join(directory, "grpc-cert.pem"), filepath.Join(directory, "grpc-key.pem")
	if err := os.WriteFile(cert, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: private}), 0600); err != nil {
		t.Fatal(err)
	}
	return cert, keyPath
}

func TestExternalGRPCProjects(t *testing.T) {
	if os.Getenv("GOSEED_TEMPLATE_DB") != "1" {
		t.Skip("run make template-db for real gRPC SQL verification")
	}
	for _, backend := range []string{"mysql", "postgres"} {
		t.Run(backend, func(t *testing.T) {
			directory := renderProjectWith(t, templates.Options{Database: backend, Authz: "casbin", Transport: "grpc"})
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
				t.Fatal("real SQL RPC test skipped")
			}
			binary, client := filepath.Join(directory, "bin/apiserver"), filepath.Join(directory, "bin/exampleclient")
			run(t, directory, "go", "build", "-mod=readonly", "-o", binary, "./cmd/apiserver")
			run(t, directory, "go", "build", "-mod=readonly", "-o", client, "./cmd/exampleclient")
			env := []string{"APP_DATABASE_HOST=127.0.0.1", "APP_DATABASE_PORT=" + port, "APP_DATABASE_USER=" + user, "APP_DATABASE_PASSWORD=" + password, "APP_DATABASE_NAME=goseed_test", "APP_DATABASE_TLS=true", "APP_DATABASE_CA_FILE=" + ca}
			verifyGRPCProcess(t, directory, binary, client, backend, "casbin", env, false)
		})
	}
}

func TestRealGRPCProjects(t *testing.T) {
	if os.Getenv("GOSEED_TEMPLATE_REDIS") != "1" {
		t.Skip("run make template-redis for isolated gRPC Redis verification")
	}
	address, password, ca := os.Getenv("GOSEED_TEST_REDIS_ADDRESS"), os.Getenv("GOSEED_TEST_REDIS_PASSWORD"), os.Getenv("GOSEED_TEST_REDIS_CA_FILE")
	if address == "" || password == "" || ca == "" {
		t.Fatal("explicit Redis TLS fixture required")
	}
	for _, backend := range []string{"sqlite", "none"} {
		t.Run(backend, func(t *testing.T) {
			directory := renderProjectWith(t, templates.Options{Database: backend, Transport: "grpc", Redis: true, Authz: "casbin"})
			verifyReadonly(t, directory)
			fixtures := []string{"GOSEED_TEST_REDIS=1", "GOSEED_TEST_REDIS_ADDRESS=" + address, "GOSEED_TEST_REDIS_PASSWORD=" + password, "GOSEED_TEST_REDIS_CA_FILE=" + ca}
			output := runWith(t, directory, fixtures, "go", "test", "-mod=readonly", "-race", "-count=1", "-v", "./...")
			if strings.Contains(string(output), "--- SKIP:") {
				t.Fatal("real Redis RPC verification skipped a test")
			}
			binary, client := filepath.Join(directory, "bin/apiserver"), filepath.Join(directory, "bin/exampleclient")
			run(t, directory, "go", "build", "-mod=readonly", "-o", binary, "./cmd/apiserver")
			run(t, directory, "go", "build", "-mod=readonly", "-o", client, "./cmd/exampleclient")
			env := []string{"APP_REDIS_ADDRESS=" + address, "APP_REDIS_PASSWORD=" + password, "APP_REDIS_TLS=true", "APP_REDIS_CA_FILE=" + ca, "APP_REDIS_KEY_PREFIX=goseed_rpc_" + backend}
			verifyGRPCProcess(t, directory, binary, client, backend, "casbin", env, false)
			token := strings.TrimSpace(string(runWith(t, directory, env, binary, "dev", "token", "--subject", "alice")))
			options := []string{"--tls=false"}
			process := startGRPC(t, directory, binary, client, env, options)
			operation := []string{"--action", "greet", "--name", "redis-rpc-private"}
			var item struct {
				ID string `json:"id"`
			}
			if backend == "sqlite" {
				data := rpcCall(t, directory, client, process.url, token, env, options, []string{"--action", "create", "--name", "redis-rpc-read"}, "")
				if json.Unmarshal(data, &item) != nil || item.ID == "" {
					t.Fatal("RPC item missing")
				}
				operation = []string{"--action", "get", "--id", item.ID}
			}
			for range 2 {
				rpcCall(t, directory, client, process.url, token, env, options, operation, "")
			}
			process.stop(t)
			// Leave warm cache entries intact while revoking the user's policy.
			installCasbinPolicy(t, directory, binary, backend, env, "")
			revoked := startGRPC(t, directory, binary, client, env, options)
			rpcCall(t, directory, client, revoked.url, token, env, options, operation, "PermissionDenied")
			revoked.stop(t)
			installCasbinPolicy(t, directory, binary, backend, env, casbinPolicy(backend))
			unavailable := append(append([]string{}, env...), "APP_REDIS_ADDRESS=127.0.0.1:1")
			fallback := startGRPC(t, directory, binary, client, unavailable, options)
			rpcCall(t, directory, client, fallback.url, token, unavailable, options, operation, "")
			fallback.stop(t)
			rejectCommand(t, directory, binary, append(append([]string{}, unavailable...), "APP_REDIS_REQUIRED=true"), "serve")
			requiredEnv := append(append([]string{}, env...), "APP_REDIS_REQUIRED=true")
			required := startGRPC(t, directory, binary, client, requiredEnv, options)
			optional := startGRPC(t, directory, binary, client, env, options)
			signalRedisFault(t, true)
			t.Cleanup(func() { signalRedisFault(t, false) })
			check := rpcCall(t, directory, client, required.url, "", requiredEnv, options, []string{"--action", "health"}, "")
			if !strings.Contains(string(check), "NOT_SERVING") {
				t.Fatal("required Redis health stayed serving")
			}
			rpcCall(t, directory, client, required.url, token, requiredEnv, options, operation, "Unavailable")
			rpcCall(t, directory, client, optional.url, token, env, options, operation, "")
			if backend == "sqlite" {
				rpcCall(t, directory, client, required.url, token, requiredEnv, options, []string{"--action", "update", "--id", item.ID, "--enabled=false"}, "Unavailable")
				data := rpcCall(t, directory, client, optional.url, token, env, options, operation, "")
				var result struct {
					Enabled bool `json:"enabled"`
				}
				if json.Unmarshal(data, &result) != nil || !result.Enabled {
					t.Fatal("required Redis failure reached persistence")
				}
				rpcCall(t, directory, client, optional.url, token, env, options, []string{"--action", "update", "--id", item.ID, "--description", "updated while cache down"}, "")
			}
			signalRedisFault(t, false)
			var health struct {
				Status string `json:"status"`
			}
			deadline := time.Now().Add(5 * time.Second)
			for {
				check = rpcCall(t, directory, client, required.url, "", requiredEnv, options, []string{"--action", "health"}, "")
				if json.Unmarshal(check, &health) == nil && health.Status == "SERVING" {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("Redis health did not recover within 5s")
				}
				time.Sleep(50 * time.Millisecond)
			}
			rpcCall(t, directory, client, required.url, token, requiredEnv, options, operation, "")
			required.stop(t)
			optional.stop(t)
			for _, logPath := range []string{process.log, revoked.log, fallback.log, required.log, optional.log} {
				data, err := os.ReadFile(logPath)
				if err != nil {
					t.Fatal(err)
				}
				for _, secret := range []string{password, token, "redis-rpc-private", "redis-rpc-read"} {
					if strings.Contains(string(data), secret) {
						t.Fatal("RPC Redis log disclosed input")
					}
				}
				for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
					if !json.Valid([]byte(line)) {
						t.Fatal("RPC Redis failure bypassed JSON logging")
					}
				}
			}
		})
	}
}
