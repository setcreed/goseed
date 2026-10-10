package integration

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

type serviceProcess struct {
	command *exec.Cmd
	done    chan struct{}
	err     error
	url     string
	log     string
}

func startService(t *testing.T, directory, binary string) *serviceProcess {
	t.Helper()
	log, err := os.CreateTemp(directory, "service-*.log")
	if err != nil {
		t.Fatal(err)
	}
	process := &serviceProcess{command: exec.Command(binary, "serve", "--address", "127.0.0.1:0"), done: make(chan struct{}), log: log.Name()}
	process.command.Dir = directory
	process.command.Env = append(environment(t), "APP_LOG_FORMAT=json")
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
				t.Error("service cleanup timed out")
			}
		}
		_ = log.Close()
	})
	deadline := time.Now().Add(10 * time.Second)
	client := &http.Client{Timeout: time.Second}
	for time.Now().Before(deadline) {
		select {
		case <-process.done:
			t.Fatalf("service exited before readiness: %v", process.err)
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
			if json.Unmarshal([]byte(line), &entry) == nil && entry.Message == "HTTP server started" {
				process.url = "http://" + entry.Address
			}
		}
		if process.url != "" {
			response, err := client.Get(process.url + "/readyz")
			if err == nil {
				_ = response.Body.Close()
				if response.StatusCode == 200 {
					return process
				}
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("service readiness timed out")
	return nil
}
func (p *serviceProcess) stop(t *testing.T) {
	t.Helper()
	if err := p.command.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	select {
	case <-p.done:
		if p.err != nil {
			t.Fatalf("service shutdown: %v", p.err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("service shutdown timed out")
	}
}
func request(t *testing.T, client *http.Client, base, method, path, body, token string, want int) ([]byte, http.Header) {
	t.Helper()
	req, err := http.NewRequest(method, base+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("X-Request-ID", "integration.request-1")
	response, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != want {
		t.Fatalf("%s %s: status %d want %d: %s", method, path, response.StatusCode, want, data)
	}
	if response.Header.Get("X-Request-ID") != "integration.request-1" {
		t.Fatal("request ID not preserved")
	}
	if want >= 400 {
		var failure struct {
			Code    string `json:"code"`
			Message string `json:"message"`
			ID      string `json:"request_id"`
		}
		if json.Unmarshal(data, &failure) != nil || failure.Code == "" || failure.Message == "" || failure.ID != "integration.request-1" {
			t.Fatal("invalid error envelope")
		}
	}
	return data, response.Header
}

type exampleResource struct {
	ID          uint64    `json:"id"`
	Owner       string    `json:"owner"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func resource(t *testing.T, data []byte) exampleResource {
	t.Helper()
	var item exampleResource
	if err := json.Unmarshal(data, &item); err != nil {
		t.Fatal(err)
	}
	return item
}
func verifyHTTP(t *testing.T, directory, binary string) {
	t.Helper()
	alice := strings.TrimSpace(string(run(t, directory, binary, "dev", "token", "--subject", "alice", "--ttl", "15m")))
	bob := strings.TrimSpace(string(run(t, directory, binary, "dev", "token", "--subject", "bob")))
	process := startService(t, directory, binary)
	client := &http.Client{Timeout: 3 * time.Second}
	defer client.CloseIdleConnections()
	request(t, client, process.url, "GET", "/healthz", "", "", 200)
	request(t, client, process.url, "GET", "/debug/pprof/", "", "", 404)
	_, headers := request(t, client, process.url, "GET", "/v1/examples", "", "", 401)
	if headers.Get("WWW-Authenticate") != "Bearer" {
		t.Fatal("missing bearer challenge")
	}
	request(t, client, process.url, "GET", "/v1/examples", "", "invalid", 401)
	data, _ := request(t, client, process.url, "POST", "/v1/examples", `{"name":"sql-private","description":"body-private"}`, alice, 201)
	first := resource(t, data)
	if first.ID == 0 || first.Owner != "alice" || !first.Enabled || first.CreatedAt.IsZero() || first.CreatedAt.Location() != time.UTC {
		t.Fatal("create defaults")
	}
	path := fmt.Sprintf("/v1/examples/%d", first.ID)
	for _, method := range []string{"GET", "PATCH", "DELETE"} {
		body := ""
		if method == "PATCH" {
			body = `{"enabled":false}`
		}
		request(t, client, process.url, method, path, body, bob, 404)
	}
	request(t, client, process.url, "POST", "/v1/examples", `{"name":"sql-private"}`, alice, 409)
	request(t, client, process.url, "POST", "/v1/examples", `{"name":"sql-private"}`, bob, 201)
	data, _ = request(t, client, process.url, "PATCH", path, `{"description":"","enabled":false}`, alice, 200)
	updated := resource(t, data)
	if updated.Enabled || updated.Description != "" || updated.Name != first.Name || updated.UpdatedAt.Before(first.UpdatedAt) {
		t.Fatal("patch zero values")
	}
	data, _ = request(t, client, process.url, "POST", "/v1/examples", `{"name":"second"}`, alice, 201)
	second := resource(t, data)
	data, _ = request(t, client, process.url, "GET", "/v1/examples?page=2&page_size=1", "", alice, 200)
	var page struct {
		Items    []exampleResource `json:"items"`
		Total    int               `json:"total"`
		Page     int               `json:"page"`
		PageSize int               `json:"page_size"`
	}
	if json.Unmarshal(data, &page) != nil || page.Total != 2 || len(page.Items) != 1 || page.Items[0].ID != second.ID || page.Page != 2 || page.PageSize != 1 {
		t.Fatal("pagination and ordering")
	}
	data, _ = request(t, client, process.url, "GET", "/v1/examples", "", bob, 200)
	if json.Unmarshal(data, &page) != nil || page.Total != 1 || len(page.Items) != 1 || page.Items[0].Owner != "bob" {
		t.Fatal("list ownership")
	}
	for _, body := range []string{`{"name":"ok","name":"other"}`, `{"name":null}`, `{"name":"ok","owner":"bob"}`, `{"name":"ok","id":1}`, `{"name":"ok","created_at":"now"}`, `{"Name":"ok"}`, `{"name":"ok"} {}`, `{"name":false}`, `{"name":" surrounding"}`, `{"name":"` + strings.Repeat("中", 81) + `"}`, `{"name":"ok","description":"` + strings.Repeat("a", 1001) + `"}`, string([]byte{'{', '"', 'n', 'a', 'm', 'e', '"', ':', '"', 0xff, '"', '}'}), `{"name":"` + strings.Repeat("a", (1<<20)+1) + `"}`} {
		request(t, client, process.url, "POST", "/v1/examples", body, alice, 400)
	}
	for _, body := range []string{`{}`, `{"description":null}`, `{"enabled":null}`, `{"enabled":"false"}`} {
		request(t, client, process.url, "PATCH", path, body, alice, 400)
	}
	for _, query := range []string{"page=0", "page=100001", "page_size=101", "page=1&page=2", "page=no", "unknown=1"} {
		request(t, client, process.url, "GET", "/v1/examples?"+query, "", alice, 400)
	}
	for _, id := range []string{"0", "-1", "9223372036854775808", "x"} {
		request(t, client, process.url, "GET", "/v1/examples/"+id, "", alice, 400)
	}
	request(t, client, process.url, "PUT", path, `{"name":"ok"}`, alice, 405)
	process.stop(t)
	logs, err := os.ReadFile(process.log)
	if err != nil {
		t.Fatal(err)
	}
	config, err := os.ReadFile(filepath.Join(directory, "configs/app.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{alice, bob, "sql-private", "body-private"} {
		if strings.Contains(string(logs), secret) {
			t.Fatal("runtime logs contain private data")
		}
	}
	for _, line := range strings.Split(string(config), "\n") {
		if strings.Contains(line, "secret:") {
			_, value, _ := strings.Cut(line, ":")
			value = strings.Trim(strings.TrimSpace(value), `"`)
			if strings.Contains(string(logs), value) {
				t.Fatal("configuration secret leaked")
			}
		}
	}
	restarted := startService(t, directory, binary)
	data, _ = request(t, client, restarted.url, "GET", path, "", alice, 200)
	if got := resource(t, data); got.ID != first.ID || got.Enabled || got.Description != "" {
		t.Fatal("restart persistence")
	}
	request(t, client, restarted.url, "DELETE", path, "", alice, 204)
	request(t, client, restarted.url, "DELETE", path, "", alice, 404)
	request(t, client, restarted.url, "GET", path, "", alice, 404)
	restarted.stop(t)
}
