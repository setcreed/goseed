package integration

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
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
	if os.Getenv("GOSEED_TEST_DOCKER") != "1" {
		t.Skip("run make template-container for production gRPC TLS container verification")
	}
	directory := renderProjectWith(t, templates.Options{Database: "none", Transport: "grpc", Authz: "casbin"})
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
	operation := []string{"--action", "greet", "--name", "container-rpc-private"}
	address := start(false)
	ready(address)
	rpcCall(t, directory, client, address, token, nil, options, operation, "PermissionDenied")
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
	rpcCall(t, directory, client, address, role, nil, options, operation, "PermissionDenied")
	rpcCall(t, directory, client, address, token, nil, options, operation, "")
	docker("stop", "--time", "3", name)
	docker("start", name)
	address = docker("port", name, "9090/tcp")
	ready(address)
	rpcCall(t, directory, client, address, token, nil, options, operation, "")
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
