package integration

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func verifyNoneContainer(t *testing.T, directory, binary string) {
	t.Helper()
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	name := "goseed-none-" + hex.EncodeToString(random[:])
	image := name + ":test"
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
		t.Fatal("missing secret")
	}
	docker := func(args ...string) string {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
		defer cancel()
		command := exec.CommandContext(ctx, "docker", args...)
		command.Dir = directory
		command.Env = append(environment(t), "APP_JWT_SECRET="+secret)
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("docker %s: %v\n%s", args[0], err, output)
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
				t.Errorf("own Docker resource cleanup: %v %s", err, output)
			}
		}
	})
	docker("build", "--progress=plain", "--label", "org.goseed.none="+name, "-f", "build/docker/Dockerfile", "-t", image, ".")
	built = true
	if docker("image", "inspect", "--format", "{{.Config.User}}", image) != "10001:10001" {
		t.Fatal("non root configuration missing")
	}
	docker("run", "-d", "--name", name, "--label", "org.goseed.none="+name, "--read-only", "-e", "APP_JWT_SECRET", "-p", "127.0.0.1::8080", image)
	started = true
	client := &http.Client{Timeout: time.Second}
	defer client.CloseIdleConnections()
	ready := func() string {
		t.Helper()
		base := "http://" + docker("port", name, "8080/tcp")
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			response, err := client.Get(base + "/readyz")
			if err == nil {
				_ = response.Body.Close()
				if response.StatusCode == 200 {
					return base
				}
			}
			time.Sleep(20 * time.Millisecond)
		}
		t.Fatal("none container readiness")
		return ""
	}
	token := strings.TrimSpace(string(run(t, directory, binary, "dev", "token", "--subject", "alice")))
	base := ready()
	data, _ := request(t, client, base, "GET", "/v1/examples?name=container-private", "", token, 200)
	verifyGreeting(t, data, "hello, container-private", "alice")
	docker("stop", "--time", "3", name)
	docker("start", name)
	base = ready()
	data, _ = request(t, client, base, "GET", "/v1/examples", "", token, 200)
	verifyGreeting(t, data, "hello, world", "alice")
	docker("stop", "--time", "3", name)
	logs := docker("logs", name)
	for _, private := range []string{secret, token, "container-private"} {
		if strings.Contains(logs, private) {
			t.Fatal("container log disclosed private data")
		}
	}
	if docker("inspect", "--format", "{{len .Mounts}}", name) != "0" {
		t.Fatal("none container unexpectedly mounts storage")
	}
	t.Log("non root, read only root filesystem, JWT, no volumes and restart passed")
}
