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

func verifyContainer(t *testing.T, directory, binary string) {
	t.Helper()
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	name := "goseed-d2-" + hex.EncodeToString(random[:])
	volume := name + "-data"
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
				t.Fatal("development key encoding")
			}
		}
	}
	if secret == "" {
		t.Fatal("development key missing")
	}
	docker := func(success bool, args ...string) []byte {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
		defer cancel()
		command := exec.CommandContext(ctx, "docker", args...)
		command.Dir = directory
		command.Env = append(environment(t), "APP_JWT_SECRET="+secret)
		output, err := command.CombinedOutput()
		if (err == nil) != success {
			t.Fatalf("docker %s: unexpected exit %v\n%s", args[0], err, output)
		}
		return output
	}
	// All names and labels are unique to this test. Never prune Docker or touch pre-existing resources.
	t.Cleanup(func() {
		for _, args := range [][]string{{"rm", "-f", name, name + "-migration", name + "-forbidden"}, {"volume", "rm", volume}, {"image", "rm", image}} {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			command := exec.CommandContext(ctx, "docker", args...)
			_ = command.Run()
			cancel()
		}
	})
	t.Log("building the independent service container")
	docker(true, "build", "--progress=plain", "--label", "org.goseed.d2="+name, "-f", "build/docker/Dockerfile", "-t", image, ".")
	if user := strings.TrimSpace(string(docker(true, "image", "inspect", "--format", "{{.Config.User}}", image))); user != "10001:10001" {
		t.Fatal("container is not configured for non-root")
	}
	docker(true, "volume", "create", "--label", "org.goseed.d2="+name, volume)
	common := []string{"run", "--rm", "--name", name + "-migration", "--label", "org.goseed.d2=" + name, "-e", "APP_JWT_SECRET", "-v", volume + ":/app/data", image, "migrate", "up"}
	for range 2 {
		docker(true, common...)
	}
	docker(false, "run", "--rm", "--name", name+"-forbidden", "--label", "org.goseed.d2="+name, "-e", "APP_JWT_SECRET", image, "dev", "token", "--subject", "alice")
	docker(true, "run", "-d", "--name", name, "--label", "org.goseed.d2="+name, "-e", "APP_JWT_SECRET", "-v", volume+":/app/data", "-p", "127.0.0.1::8080", image)
	address := func() string { return "http://" + strings.TrimSpace(string(docker(true, "port", name, "8080/tcp"))) }
	client := &http.Client{Timeout: time.Second}
	defer client.CloseIdleConnections()
	ready := func(base string) {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			response, err := client.Get(base + "/readyz")
			if err == nil {
				_ = response.Body.Close()
				if response.StatusCode == 200 {
					return
				}
			}
			time.Sleep(20 * time.Millisecond)
		}
		t.Fatal("container did not become ready")
	}
	base := address()
	ready(base)
	token := strings.TrimSpace(string(run(t, directory, binary, "dev", "token", "--subject", "alice")))
	data, _ := request(t, client, base, "POST", "/v1/examples", `{"name":"container-private","enabled":false}`, token, 201)
	item := resource(t, data)
	if item.ID == 0 || item.Enabled {
		t.Fatal("container create")
	}
	docker(true, "stop", "--time", "3", name)
	docker(true, "start", name)
	base = address()
	ready(base)
	data, _ = request(t, client, base, "GET", "/v1/examples/"+strconv.FormatUint(item.ID, 10), "", token, 200)
	if persisted := resource(t, data); persisted.ID != item.ID || persisted.Enabled {
		t.Fatal("container persistence")
	}
	docker(true, "stop", "--time", "3", name)
	logs := string(docker(true, "logs", name))
	for _, private := range []string{secret, token, "container-private"} {
		if strings.Contains(logs, private) {
			t.Fatal("container logs leaked private data")
		}
	}
	t.Log("non-root, explicit migration, production guard, CRUD and restart persistence passed")
}
