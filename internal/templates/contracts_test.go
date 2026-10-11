package templates

import (
	"encoding/json"
	"os"
	"path"
	"regexp"
	"strings"
	"testing"
)

func TestDeliveredContracts(t *testing.T) {
	for _, backend := range []string{"sqlite", "mysql", "postgres", "none"} {
		t.Run(backend, func(t *testing.T) { checkDeliveredContracts(t, backend) })
		t.Run(backend+"-redis", func(t *testing.T) { checkDeliveredOptions(t, Options{Database: backend, Redis: true}) })
		for _, redis := range []bool{false, true} {
			label := backend + "-casbin"
			if redis {
				label += "-redis"
			}
			t.Run(label, func(t *testing.T) {
				checkDeliveredOptions(t, Options{Database: backend, Redis: redis, Authz: "casbin"})
			})
		}
	}
}

func TestGRPCContracts(t *testing.T) {
	for _, backend := range []string{"sqlite", "mysql", "postgres", "none"} {
		for _, redis := range []bool{false, true} {
			for _, authorization := range []string{"none", "casbin"} {
				t.Run(backend+"-"+authorization+map[bool]string{false: "", true: "-redis"}[redis], func(t *testing.T) {
					checkDeliveredOptions(t, Options{Database: backend, Redis: redis, Authz: authorization, Transport: "grpc"})
				})
			}
		}
	}
}

func TestGatewayContracts(t *testing.T) {
	for _, backend := range []string{"sqlite", "mysql", "postgres", "none"} {
		for _, redis := range []bool{false, true} {
			for _, authorization := range []string{"none", "casbin"} {
				t.Run(backend+"-"+authorization+map[bool]string{false: "", true: "-redis"}[redis], func(t *testing.T) {
					checkDeliveredOptions(t, Options{Database: backend, Redis: redis, Authz: authorization, Transport: "gateway"})
				})
			}
		}
	}
}

func TestDualContracts(t *testing.T) {
	for _, backend := range []string{"sqlite", "mysql", "postgres", "none"} {
		for _, redis := range []bool{false, true} {
			for _, authorization := range []string{"none", "casbin"} {
				t.Run(backend+"-"+authorization+map[bool]string{false: "", true: "-redis"}[redis], func(t *testing.T) {
					checkDeliveredOptions(t, Options{Database: backend, Redis: redis, Authz: authorization, Transport: "dual"})
				})
			}
		}
	}
}

func checkDeliveredContracts(t *testing.T, backend string) {
	checkDeliveredOptions(t, Options{Database: backend})
}

func checkDeliveredOptions(t *testing.T, options Options) {
	t.Helper()
	backend := options.Database
	plan, err := Render(Project{Name: "demo", Module: "example.com/demo"}, options)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for _, file := range plan {
		files[file.Path] = file.Content
	}
	data, err := os.ReadFile("../../dependencies.lock.json")
	if err != nil {
		t.Fatal(err)
	}
	var lock struct {
		Go         string            `json:"go"`
		Runtime    map[string]string `json:"runtime"`
		Builder    string            `json:"service_builder_image"`
		Connectors map[string]string `json:"sql_connectors"`
	}
	if err := json.Unmarshal(data, &lock); err != nil {
		t.Fatal(err)
	}
	for name, version := range lock.Connectors {
		lock.Runtime[name] = version
	}
	manifest := string(files["go.mod"])
	casbin := options.Authz == "casbin"
	if strings.Contains(manifest, "github.com/casbin/casbin/v3 ") != casbin {
		t.Fatal("incorrect Casbin dependency composition")
	}
	for _, filename := range []string{"internal/pkg/authz/policy.go", "docs/authz.md", "configs/policy.csv"} {
		if _, exists := files[filename]; exists != casbin {
			t.Fatalf("incorrect Casbin file composition: %s", filename)
		}
	}
	if _, exists := files["internal/pkg/middleware/authz.go"]; exists != (casbin && options.Transport != "grpc" && options.Transport != "gateway") {
		t.Fatal("incorrect HTTP authorization middleware composition")
	}
	for _, filename := range []string{"internal/pkg/authz/database.go", "migrations/00002_casbin.sql"} {
		if _, exists := files[filename]; exists != (casbin && backend != "none") {
			t.Fatalf("incorrect policy storage composition: %s", filename)
		}
	}
	if strings.Contains(string(files["configs/app.example.yaml"]), "authz:") != (casbin && backend == "none") {
		t.Fatal("incorrect policy file configuration composition")
	}
	if strings.Contains(manifest, "github.com/redis/go-redis/v9 ") != options.Redis {
		t.Fatal("incorrect Redis dependency composition")
	}
	for _, filename := range []string{"internal/pkg/cache/redis.go", "internal/apiserver/biz/example/cache.go", "docs/redis.md"} {
		if _, exists := files[filename]; exists != options.Redis {
			t.Fatalf("incorrect Redis file composition: %s", filename)
		}
	}
	if strings.Contains(string(files["configs/app.example.yaml"]), "redis:") != options.Redis {
		t.Fatal("incorrect Redis configuration composition")
	}
	if backend == "none" {
		for filename, content := range files {
			for _, prefix := range []string{"migrations/", "internal/pkg/database/", "internal/apiserver/model/", "internal/apiserver/store/", "internal/testutil/"} {
				if strings.HasPrefix(filename, prefix) {
					t.Fatalf("database file delivered in none project: %s", filename)
				}
			}
			if filename == "go.mod" || filename == "go.sum" || strings.HasSuffix(filename, ".go") {
				for _, dependency := range []string{"gorm.io/", "modernc.org/", "github.com/libtnb/sqlite", "github.com/pressly/goose", "github.com/go-sql-driver/mysql", "github.com/jackc/pgx"} {
					if strings.Contains(string(content), dependency) {
						t.Fatalf("database dependency leaked in %s: %s", filename, dependency)
					}
				}
			}
		}
		for filename, forbidden := range map[string]string{"configs/app.example.yaml": "database:", "build/docker/Dockerfile": "/app/data", "Makefile": "migrate", "cmd/apiserver/app/app.go": "migrate"} {
			if strings.Contains(string(files[filename]), forbidden) {
				t.Fatalf("database entry leaked in %s", filename)
			}
		}
	}
	for driver, selected := range map[string]bool{"github.com/libtnb/sqlite": backend == "sqlite", "modernc.org/sqlite": backend == "sqlite", "gorm.io/driver/mysql": backend == "mysql", "github.com/go-sql-driver/mysql": backend == "mysql", "gorm.io/driver/postgres": backend == "postgres", "github.com/jackc/pgx/v5": backend == "postgres"} {
		if strings.Contains(manifest, driver+" ") != selected {
			t.Fatalf("incorrect driver composition for %s: %s", backend, driver)
		}
	}
	if !strings.Contains(manifest, "go "+lock.Go+"\n") {
		t.Fatal("template toolchain differs from lock")
	}
	for _, line := range strings.Split(manifest, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.HasPrefix(fields[1], "v") {
			if lock.Runtime[fields[0]] != fields[1] {
				t.Fatalf("direct dependency differs from audited lock: %s", fields[0])
			}
		}
	}
	if !strings.Contains(string(files["build/docker/Dockerfile"]), "FROM "+lock.Builder+" AS builder") {
		t.Fatal("Docker builder differs from lock")
	}
	links := regexp.MustCompile(`\[[^\]]*\]\(([^)]+)\)`)
	for filename, content := range files {
		if !strings.HasSuffix(filename, ".md") {
			continue
		}
		for _, link := range links.FindAllSubmatch(content, -1) {
			target := string(link[1])
			if strings.Contains(target, "://") {
				continue
			}
			target, _, _ = strings.Cut(target, "#")
			if _, ok := files[path.Clean(path.Join(path.Dir(filename), target))]; !ok {
				t.Fatalf("%s links to missing file %s", filename, target)
			}
		}
	}
	if options.Transport == "grpc" || options.Transport == "dual" || options.Transport == "gateway" {
		for _, filename := range []string{"api/proto/example/v1/example.proto", "pkg/api/example/v1/example.pb.go", "pkg/api/example/v1/example_grpc.pb.go", "internal/pkg/rpc/server.go", "internal/pkg/rpc/interceptors.go", "cmd/exampleclient/main.go", "docs/grpc.md", "scripts/proto.py"} {
			if _, exists := files[filename]; !exists {
				t.Fatalf("missing gRPC file %s", filename)
			}
		}
		if options.Transport == "grpc" {
			for filename, content := range files {
				if strings.HasPrefix(filename, "internal/pkg/server/") || strings.HasPrefix(filename, "internal/pkg/middleware/") || strings.HasPrefix(filename, "internal/apiserver/handler/http/") || strings.HasPrefix(filename, "api/openapi/") || filename == "internal/apiserver/router.go" {
					t.Fatalf("HTTP file leaked %s", filename)
				}
				if strings.HasSuffix(filename, ".go") || filename == "go.mod" || filename == "go.sum" {
					if strings.Contains(string(content), "github.com/gin-gonic/") {
						t.Fatalf("Gin leaked %s", filename)
					}
				}
			}
			if strings.Contains(string(files["configs/app.example.yaml"]), "http:") {
				t.Fatal("HTTP config leaked")
			}
		}
		if !strings.Contains(manifest, "google.golang.org/grpc "+lock.Runtime["google.golang.org/grpc"]) {
			t.Fatal("gRPC lock differs")
		}
		if options.Transport == "grpc" {
			return
		}
		if options.Transport == "dual" {
			for _, filename := range []string{"internal/pkg/dual/server.go", "internal/apiserver/handler/http/example/handler.go", "internal/apiserver/router.go", "docs/dual.md"} {
				if _, exists := files[filename]; !exists {
					t.Fatalf("missing dual file %s", filename)
				}
			}
			if !strings.Contains(manifest, "github.com/gin-gonic/gin "+lock.Runtime["github.com/gin-gonic/gin"]) || !strings.Contains(string(files["configs/app.example.yaml"]), "http:") {
				t.Fatal("dual HTTP dependency/config missing")
			}
			dockerfile := string(files["build/docker/Dockerfile"])
			for _, required := range []string{"APP_HTTP_ADDRESS=0.0.0.0:8080", "APP_GRPC_ADDRESS=0.0.0.0:9090", "EXPOSE 8080 9090"} {
				if !strings.Contains(dockerfile, required) {
					t.Fatalf("dual Docker configuration missing %s", required)
				}
			}
		}
	}
	var swagger map[string]any
	if err := json.Unmarshal(files["api/openapi/swagger.json"], &swagger); err != nil {
		t.Fatal(err)
	}
	if swagger["swagger"] != "2.0" {
		t.Fatal("API document must be Swagger 2.0")
	}
	var checkRefs func(any)
	checkRefs = func(value any) {
		switch value := value.(type) {
		case map[string]any:
			if raw, ok := value["$ref"].(string); ok {
				if !strings.HasPrefix(raw, "#/") {
					t.Fatal("unexpected external Swagger reference")
				}
				current := any(swagger)
				for _, segment := range strings.Split(strings.TrimPrefix(raw, "#/"), "/") {
					object, ok := current.(map[string]any)
					if !ok {
						t.Fatalf("invalid Swagger reference %s", raw)
					}
					current, ok = object[segment]
					if !ok {
						t.Fatalf("missing Swagger reference %s", raw)
					}
				}
			}
			for _, child := range value {
				checkRefs(child)
			}
		case []any:
			for _, child := range value {
				checkRefs(child)
			}
		}
	}
	checkRefs(swagger)
	routes, ok := swagger["paths"].(map[string]any)
	if !ok {
		t.Fatal("Swagger paths missing")
	}
	expectedRoutes := []string{"/healthz", "/readyz", "/v1/examples"}
	if options.Transport == "gateway" {
		expectedRoutes = []string{"/v1/examples"}
	}
	if backend == "none" {
		expectedCount := 3
		if options.Transport == "gateway" {
			expectedCount = 1
		}
		if len(routes) != expectedCount || len(routes["/v1/examples"].(map[string]any)) != 1 {
			t.Fatal("none document includes unsupported routes or methods")
		}
	} else {
		expectedRoutes = append(expectedRoutes, "/v1/examples/{id}")
	}
	for _, route := range expectedRoutes {
		if _, ok := routes[route]; !ok {
			t.Fatalf("missing documented route %s", route)
		}
	}
	if casbin {
		for route, item := range routes {
			if !strings.HasPrefix(route, "/v1/") {
				continue
			}
			for method, raw := range item.(map[string]any) {
				if method == "parameters" {
					continue
				}
				if _, exists := raw.(map[string]any)["responses"].(map[string]any)["403"]; !exists {
					t.Fatalf("authorization response undocumented: %s %s", method, route)
				}
			}
		}
	}
}
