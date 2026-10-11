package templates

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"testing/fstest"
)

func TestGatewayPrunesHTTPAndMapsOnlySelectedBusiness(t *testing.T) {
	for _, backend := range []string{"sqlite", "mysql", "postgres", "none"} {
		for _, redis := range []bool{false, true} {
			for _, authorization := range []string{"none", "casbin"} {
				files, err := Render(Project{Name: "demo", Module: "example.com/demo"}, Options{Database: backend, Redis: redis, Authz: authorization, Transport: "gateway"})
				if err != nil {
					t.Fatal(err)
				}
				plan := map[string][]byte{}
				for _, file := range files {
					plan[file.Path] = file.Content
					if strings.HasPrefix(file.Path, "internal/apiserver/handler/http/") || strings.HasPrefix(file.Path, "internal/pkg/middleware/") || strings.HasPrefix(file.Path, "internal/pkg/dual/") || file.Path == "internal/apiserver/router.go" {
						t.Fatal("duplicate HTTP business", file.Path)
					}
				}
				if bytes.Contains(plan["go.mod"], []byte("gin-gonic")) || !bytes.Contains(plan["go.mod"], []byte("grpc-gateway/v2 v2.31.0")) {
					t.Fatal("Gateway dependency pruning")
				}
				for _, path := range []string{"pkg/api/example/v1/example.pb.gw.go", "api/proto/example/v1/http.yaml", "internal/pkg/gateway/server.go", "docs/gateway.md"} {
					if len(plan[path]) == 0 {
						t.Fatal("missing Gateway file", path)
					}
				}
				if !bytes.Contains(plan["internal/apiserver/bootstrap/modules_gen.go"], []byte("gatewayModules")) || bytes.Contains(plan["internal/pkg/gateway/handler.go"], []byte("/v1/examples")) {
					t.Fatal("Gateway route registration is not module-driven")
				}
				var document struct {
					Swagger string                    `json:"swagger"`
					Paths   map[string]map[string]any `json:"paths"`
				}
				if json.Unmarshal(plan["api/openapi/swagger.json"], &document) != nil || document.Swagger != "2.0" {
					t.Fatal("invalid generated Swagger")
				}
				if backend == "none" {
					if len(document.Paths) != 1 || document.Paths["/v1/examples"]["get"] == nil {
						t.Fatal("no database HTTP mappings")
					}
				} else if len(document.Paths) != 2 || document.Paths["/v1/examples"]["post"] == nil || document.Paths["/v1/examples/{id}"]["patch"] == nil {
					t.Fatal("CRUD HTTP mappings")
				}
			}
		}
	}
}

func TestDefaultPlan(t *testing.T) {
	first, err := Default(Project{Name: "demo", Module: "example.com/demo"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := Default(Project{Name: "demo", Module: "example.com/demo"})
	if err != nil {
		t.Fatal(err)
	}
	if len(first) == 0 || len(first) != len(second) {
		t.Fatal("empty or inconsistent file plan")
	}
	for i, file := range first {
		if file.Path != second[i].Path || !bytes.Equal(file.Content, second[i].Content) {
			t.Fatal("non-deterministic rendering")
		}
		if bytes.Contains(file.Content, []byte("[[.")) {
			t.Fatalf("unexpanded variable: %s", file.Path)
		}
	}
	var swagger, registry []byte
	for _, file := range first {
		switch file.Path {
		case "api/openapi/swagger.json":
			swagger = file.Content
		case "internal/apiserver/bootstrap/modules_gen.go":
			registry = file.Content
		}
	}
	digest := sha256.Sum256(swagger)
	if len(swagger) == 0 || !bytes.Contains(registry, []byte("const openAPISHA256 = \""+hex.EncodeToString(digest[:])+"\"")) {
		t.Fatal("default module registration index does not contain the rendered OpenAPI digest")
	}
}

func TestVariantPlansAndInvalidOptions(t *testing.T) {
	project := Project{Name: "demo", Module: "example.com/demo"}
	for _, backend := range []string{"mysql", "postgres", "none"} {
		first, err := Render(project, Options{Database: backend})
		if err != nil {
			t.Fatal(err)
		}
		second, err := Render(project, Options{Database: backend})
		if err != nil || len(first) != len(second) {
			t.Fatal("inconsistent variant plan")
		}
		for i, file := range first {
			if file.Path != second[i].Path || !bytes.Equal(file.Content, second[i].Content) {
				t.Fatal("non-deterministic variant plan")
			}
			if i > 0 && first[i-1].Path >= file.Path {
				t.Fatal("plan order or duplicate path")
			}
			if file.Path == "configs/app.example.yaml" && bytes.Contains(file.Content, []byte("busy_timeout")) {
				t.Fatal("SQLite configuration leaked into SQL project")
			}
		}
	}
	for _, invalid := range []string{"mongodb", "MYSQL", "../mysql", "mysql\n"} {
		if files, err := Render(project, Options{Database: invalid}); err == nil || files != nil {
			t.Fatal("invalid option returned a file plan")
		}
	}
}

func TestInvalidProject(t *testing.T) {
	for _, project := range []Project{{Name: "../demo", Module: "example.com/demo"}, {Name: "demo", Module: "example.com/../demo"}, {Name: "demo", Module: "example.com/demo\";panic(1)"}, {Name: "demo", Module: ""}} {
		if files, err := Default(project); err == nil || files != nil {
			t.Fatalf("accepted invalid project: %+v", project)
		}
	}
}

func TestProtocolPlansAndInvalidTransport(t *testing.T) {
	project := Project{Name: "short", Module: "example.com/different-name"}
	for _, transport := range []string{"grpc", "dual", "gateway"} {
		options := Options{Transport: transport, Database: "none", Redis: true, Authz: "casbin"}
		first, err := Render(project, options)
		if err != nil {
			t.Fatal(err)
		}
		if transport == "dual" {
			for _, file := range first {
				if file.Path == "internal/apiserver/bootstrap/modules_gen.go" && (!bytes.Contains(file.Content, []byte("httpModules")) || !bytes.Contains(file.Content, []byte("rpcModules"))) {
					t.Fatal("dual registration index must include HTTP and RPC modules")
				}
			}
		}
		second, err := Render(project, options)
		if err != nil || len(first) == 0 || len(first) != len(second) {
			t.Fatal("inconsistent protocol plan")
		}
		for i, file := range first {
			if file.Path != second[i].Path || !bytes.Equal(file.Content, second[i].Content) {
				t.Fatal("non-deterministic protocol plan")
			}
			if i > 0 && first[i-1].Path >= file.Path {
				t.Fatal("plan order or duplicate")
			}
		}
		if transport == "grpc" {
			for _, file := range first {
				if file.Path == "api/openapi/swagger.json" {
					t.Fatal("gRPC-only project unexpectedly contains HTTP OpenAPI")
				}
			}
		} else {
			var swagger, registry []byte
			for _, file := range first {
				switch file.Path {
				case "api/openapi/swagger.json":
					swagger = file.Content
				case "internal/apiserver/bootstrap/modules_gen.go":
					registry = file.Content
				}
			}
			digest := sha256.Sum256(swagger)
			want := []byte(`const openAPISHA256 = "` + hex.EncodeToString(digest[:]) + `"`)
			if len(swagger) == 0 || !bytes.Contains(registry, want) {
				t.Fatalf("%s registration index does not contain the rendered OpenAPI digest", transport)
			}
		}
	}
	for _, invalid := range []string{"GRPC", "unknown", "../grpc", "grpc\n"} {
		if files, err := Render(project, Options{Transport: invalid}); err == nil || files != nil {
			t.Fatal("invalid transport returned a plan")
		}
	}
}

func TestHTTPModuleRegistryIsTransportScoped(t *testing.T) {
	project := Project{Name: "short", Module: "example.com/short"}
	for _, transport := range []string{"http", "dual"} {
		files, err := Render(project, Options{Transport: transport})
		if err != nil {
			t.Fatal(err)
		}
		plan := map[string][]byte{}
		for _, file := range files {
			plan[file.Path] = file.Content
		}
		if len(plan["internal/apiserver/bootstrap/modules_gen.go"]) == 0 || !bytes.Contains(plan["internal/apiserver/router.go"], []byte("for _, module := range modules")) {
			t.Fatalf("%s does not use the generated HTTP module registry", transport)
		}
	}
	for _, transport := range []string{"grpc", "gateway"} {
		files, err := Render(project, Options{Transport: transport})
		if err != nil {
			t.Fatal(err)
		}
		plan := map[string][]byte{}
		for _, file := range files {
			plan[file.Path] = file.Content
		}
		if !bytes.Contains(plan["internal/apiserver/bootstrap/modules_gen.go"], []byte("rpcModules")) || bytes.Contains(plan["internal/apiserver/bootstrap/modules_gen.go"], []byte("httpModules")) || len(plan["internal/apiserver/router.go"]) != 0 {
			t.Fatalf("%s did not prune HTTP and retain RPC registration", transport)
		}
	}
}

func TestManifestModulesRenderIntoProtocolRegistries(t *testing.T) {
	project := Project{Name: "short", Module: "example.com/short"}
	modules := []ModuleEntry{{Name: "example"}, {Name: "orders"}}
	for _, transport := range []string{"http", "grpc", "dual", "gateway"} {
		files, err := RenderWithModules(project, Options{Transport: transport, Database: "sqlite"}, modules)
		if err != nil {
			t.Fatalf("%s render: %v", transport, err)
		}
		var registry []byte
		for _, file := range files {
			if file.Path == "internal/apiserver/bootstrap/modules_gen.go" {
				registry = file.Content
			}
		}
		expectedValues := [][]byte{[]byte("handler/"), []byte("orders")}
		if transport != "grpc" {
			expectedValues = append(expectedValues, []byte("/v1/orders"))
		}
		for _, expected := range expectedValues {
			if !bytes.Contains(registry, expected) {
				t.Fatalf("%s registry omitted %q:\n%s", transport, expected, registry)
			}
		}
		if transport == "grpc" && bytes.Contains(registry, []byte("httpModules")) {
			t.Fatal("gRPC registry contains HTTP registration")
		}
		if transport == "http" && bytes.Contains(registry, []byte("rpcModules")) {
			t.Fatal("HTTP registry contains RPC registration")
		}
	}
}

func TestBusinessModuleSkeletonUsesSafePackageIdentifiers(t *testing.T) {
	project := Project{Name: "short", Module: "example.com/short"}
	files, err := RenderBusinessModule(project, Options{Database: "postgres", Transport: "dual"}, "type")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if strings.HasSuffix(file.Path, ".go") && bytes.Contains(file.Content, []byte("package type\n")) {
			t.Fatalf("Go keyword leaked into package declaration: %s", file.Path)
		}
	}
	if len(files) != 4 {
		t.Fatalf("dual SQL module expected Biz, Store and two Handler files, got %d", len(files))
	}
}

func TestCasbinPlansAndInvalidOptions(t *testing.T) {
	project := Project{Name: "demo", Module: "example.com/demo"}
	for _, backend := range []string{"sqlite", "mysql", "postgres", "none"} {
		for _, redis := range []bool{false, true} {
			options := Options{Database: backend, Redis: redis, Authz: "casbin"}
			first, err := Render(project, options)
			if err != nil {
				t.Fatal(err)
			}
			second, err := Render(project, options)
			if err != nil || len(first) != len(second) {
				t.Fatal("inconsistent Casbin plan")
			}
			for i, file := range first {
				if file.Path != second[i].Path || !bytes.Equal(file.Content, second[i].Content) {
					t.Fatal("non-deterministic Casbin plan")
				}
				if i > 0 && first[i-1].Path >= file.Path {
					t.Fatal("plan order or duplicate")
				}
			}
		}
	}
	for _, authz := range []string{"CASBIN", "rbac", "../casbin", "casbin\n"} {
		if files, err := Render(project, Options{Authz: authz}); err == nil || files != nil {
			t.Fatal("invalid authorization produced a plan")
		}
	}
}

func TestRedisPlanDeterminism(t *testing.T) {
	project := Project{Name: "demo", Module: "example.com/demo"}
	for _, backend := range []string{"sqlite", "mysql", "postgres", "none"} {
		t.Run(backend, func(t *testing.T) {
			options := Options{Database: backend, Redis: true}
			first, err := Render(project, options)
			if err != nil {
				t.Fatal(err)
			}
			second, err := Render(project, options)
			if err != nil || len(first) == 0 || len(first) != len(second) {
				t.Fatal("inconsistent Redis plan")
			}
			for i, file := range first {
				if file.Path != second[i].Path || !bytes.Equal(file.Content, second[i].Content) {
					t.Fatal("non-deterministic Redis plan")
				}
				if i > 0 && first[i-1].Path >= file.Path {
					t.Fatal("plan order or duplicate path")
				}
			}
		})
	}
}

func TestRenderFailuresReturnNoPartialPlan(t *testing.T) {
	for _, invalid := range []string{"[[.Missing]]", "[[", "package invalid; func {"} {
		source := fstest.MapFS{"base/a.txt.tmpl": &fstest.MapFile{Data: []byte("valid")}, "base/b.go.tmpl": &fstest.MapFile{Data: []byte(invalid)}}
		if files, err := render(source, Project{}); err == nil || files != nil {
			t.Fatal("render failure returned partial plan")
		}
	}
}

func TestSystemMetadataAndDotFiles(t *testing.T) {
	source := fstest.MapFS{
		"base/.DS_Store":          &fstest.MapFile{Data: []byte("OS metadata")},
		"base/Thumbs.db":          &fstest.MapFile{Data: []byte("OS metadata")},
		"base/dot.gitignore.tmpl": &fstest.MapFile{Data: []byte("/data/\n")},
	}
	files, err := render(source, Project{})
	if err != nil || len(files) != 1 || files[0].Path != ".gitignore" {
		t.Fatalf("metadata/dot file handling: %v %+v", err, files)
	}
}
