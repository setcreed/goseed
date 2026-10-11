package integration

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/setcreed/goseed/internal/templates"
)

func TestGRPCContainer(t *testing.T) {
	verifyProtocolContainer(t, "grpc")
}

func TestGatewayContainer(t *testing.T) { verifyProtocolContainer(t, "gateway") }

func TestDualContainer(t *testing.T) {
	verifyProtocolContainer(t, "dual")
}

func verifyProtocolContainer(t *testing.T, protocol string) {
	t.Helper()
	if os.Getenv("GOSEED_TEST_DOCKER") != "1" {
		t.Skip("run make template-container for production gRPC TLS container verification")
	}
	directory := renderProjectWith(t, templates.Options{Database: "none", Transport: protocol, Authz: "casbin"})
	verifyReadonly(t, directory)
	binary, client := filepath.Join(directory, "bin/apiserver"), filepath.Join(directory, "bin/exampleclient")
	run(t, directory, "go", "build", "-mod=readonly", "-o", binary, "./cmd/apiserver")
	run(t, directory, "go", "build", "-mod=readonly", "-o", client, "./cmd/exampleclient")
	run(t, directory, binary, "dev", "init")
	run(t, directory, client, "--help")
	token := strings.TrimSpace(string(run(t, directory, binary, "dev", "token", "--subject", "alice")))
	role := strings.TrimSpace(string(run(t, directory, binary, "dev", "token", "--subject", "role:reader")))
	config, err := os.ReadFile(filepath.Join(directory, "configs/app.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var secret string
	for _, line := range strings.Split(string(config), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "secret:") {
			_, raw, _ := strings.Cut(line, ":")
			secret, err = strconv.Unquote(strings.TrimSpace(raw))
			if err != nil {
				t.Fatal("secret encoding")
			}
		}
	}
	if secret == "" {
		t.Fatal("missing generated secret")
	}
	cert, key := grpcCertificate(t, directory)
	pem, err := os.ReadFile(cert)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(pem) {
		t.Fatal("container test CA")
	}
	transport := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}}
	defer transport.CloseIdleConnections()
	httpClient := &http.Client{Timeout: 3 * time.Second, Transport: transport}
	// Ephemeral test credentials must be readable by the container's unrelated UID.
	for _, path := range []string{cert, key} {
		if err := os.Chmod(path, 0444); err != nil {
			t.Fatal(err)
		}
	}
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	name := "goseed-grpc-" + hex.EncodeToString(random[:])
	image := name + ":test"
	docker := func(args ...string) string {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
		defer cancel()
		command := exec.CommandContext(ctx, "docker", args...)
		command.Dir = directory
		command.Env = append(environment(t), "APP_JWT_SECRET="+secret)
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("docker %s: %v %s", args[0], err, output)
		}
		return strings.TrimSpace(string(output))
	}
	built, started := false, false
	t.Cleanup(func() {
		var commands [][]string
		if started {
			commands = append(commands, []string{"rm", "-f", name})
		}
		if built {
			commands = append(commands, []string{"image", "rm", image})
		}
		for _, args := range commands {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			output, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
			cancel()
			if err != nil {
				t.Errorf("own Docker cleanup: %v %s", err, output)
			}
		}
	})
	docker("build", "--progress=plain", "--label", "org.goseed.grpc="+name, "-f", "build/docker/Dockerfile", "-t", image, ".")
	built = true
	if docker("image", "inspect", "--format", "{{.Config.User}}", image) != "10001:10001" {
		t.Fatal("container is not non-root")
	}
	policy := filepath.Join(directory, "configs/policy.csv")
	start := func(mountPolicy bool) string {
		t.Helper()
		args := []string{"run", "-d", "--name", name, "--label", "org.goseed.grpc=" + name, "--read-only", "-e", "APP_JWT_SECRET", "-e", "APP_GRPC_TLS_CERT=/app/certs/server.crt", "-e", "APP_GRPC_TLS_KEY=/app/certs/server.key", "-v", cert + ":/app/certs/server.crt:ro", "-v", key + ":/app/certs/server.key:ro", "-p", "127.0.0.1::9090"}
		if protocol == "dual" || protocol == "gateway" {
			args = append(args, "-e", "APP_HTTP_TLS_CERT=/app/certs/server.crt", "-e", "APP_HTTP_TLS_KEY=/app/certs/server.key", "-p", "127.0.0.1::8080")
		}
		if protocol == "gateway" {
			args = append(args, "-e", "APP_GATEWAY_CA_FILE=/app/certs/server.crt")
		}
		if mountPolicy {
			args = append(args, "-v", policy+":/app/configs/policy.csv:ro")
		}
		args = append(args, image)
		docker(args...)
		started = true
		return docker("port", name, "9090/tcp")
	}
	ready := func(address string) {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
			command := exec.CommandContext(ctx, client, "--address", address, "--action", "health", "--ca", cert)
			command.Dir = directory
			command.Env = environment(t)
			data, err := command.CombinedOutput()
			cancel()
			var state struct {
				Status string `json:"status"`
			}
			if err == nil && json.Unmarshal(data, &state) == nil && state.Status == "SERVING" {
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
		t.Fatal("gRPC TLS container did not become ready")
	}
	options := []string{"--ca", cert}
	checkHTTP := func(token string, expected int) {
		t.Helper()
		if protocol == "dual" || protocol == "gateway" {
			base := "https://" + docker("port", name, "8080/tcp")
			deadline := time.Now().Add(10 * time.Second)
			for {
				response, err := httpClient.Get(base + "/readyz")
				if err == nil {
					_ = response.Body.Close()
					if response.StatusCode == 200 {
						break
					}
				}
				if time.Now().After(deadline) {
					t.Fatalf("%s HTTP TLS container did not become ready", protocol)
				}
				time.Sleep(20 * time.Millisecond)
			}
			request(t, httpClient, base, "GET", "/readyz", "", "", 200)
			if protocol == "gateway" {
				gatewayRequest(t, httpClient, base, "GET", "/v1/examples", "", token, expected)
			} else {
				request(t, httpClient, base, "GET", "/v1/examples", "", token, expected)
			}
		}
	}
	operation := []string{"--action", "greet", "--name", "container-rpc-private"}
	address := start(false)
	ready(address)
	rpcCall(t, directory, client, address, token, nil, options, operation, "PermissionDenied")
	checkHTTP(token, 403)
	docker("stop", "--time", "3", name)
	docker("rm", name)
	started = false
	if err := os.WriteFile(policy, []byte(casbinPolicy("none")), 0644); err != nil {
		t.Fatal(err)
	}
	address = start(true)
	ready(address)
	if mounts := docker("inspect", "--format", "{{range .Mounts}}{{.RW}} {{end}}", name); mounts != "false false false" {
		t.Fatal("TLS or policy mounts writable")
	}
	if docker("inspect", "--format", "{{.HostConfig.ReadonlyRootfs}}", name) != "true" {
		t.Fatal("root filesystem writable")
	}
	rpcCall(t, directory, client, address, "", nil, options, operation, "Unauthenticated")
	checkHTTP("", 401)
	rpcCall(t, directory, client, address, role, nil, options, operation, "PermissionDenied")
	checkHTTP(role, 403)
	rpcCall(t, directory, client, address, token, nil, options, operation, "")
	checkHTTP(token, 200)
	docker("stop", "--time", "3", name)
	docker("start", name)
	address = docker("port", name, "9090/tcp")
	ready(address)
	rpcCall(t, directory, client, address, token, nil, options, operation, "")
	checkHTTP(token, 200)
	docker("stop", "--time", "3", name)
	logs := docker("logs", name)
	for _, private := range []string{secret, token, role, "container-rpc-private"} {
		if strings.Contains(logs, private) {
			t.Fatal("container RPC logs disclosed input")
		}
	}
	for _, line := range strings.Split(logs, "\n") {
		if !json.Valid([]byte(line)) {
			t.Fatal("container logging bypassed JSON")
		}
	}
}
