package integration

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/setcreed/goseed/internal/generator"
	"github.com/setcreed/goseed/internal/templates"
)

func TestModuleAssemblyProjects(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(root, ".tmp", "d6-module-assembly")
	if err := os.Mkdir(base, 0750); err == nil {
		t.Cleanup(func() {
			if err := os.Remove(base); err != nil {
				t.Error(err)
			}
		})
	} else if !os.IsExist(err) {
		t.Fatal(err)
	}
	for _, transport := range []string{"http", "grpc", "dual", "gateway"} {
		for _, backend := range []string{"sqlite", "none", "mysql", "postgres"} {
			t.Run(transport+"/"+backend, func(t *testing.T) {
				directory, err := os.MkdirTemp(base, "project-")
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := os.RemoveAll(directory); err != nil {
						t.Error(err)
					}
				})
				options := templates.Options{Database: backend, Transport: transport}
				plan, err := generator.Prepare(directory, templates.Project{Name: "demo", Module: "example.com/demo"}, options)
				if err != nil {
					t.Fatal(err)
				}
				if err := plan.Write(); err != nil {
					t.Fatal(err)
				}
				first, err := generator.PrepareModule(directory, "orders")
				if err != nil {
					t.Fatal(err)
				}
				if err := first.Write(); err != nil {
					t.Fatal(err)
				}
				factoryPath := filepath.Join(directory, "internal/apiserver/bootstrap/module_orders.go")
				factory, err := os.ReadFile(factoryPath)
				if err != nil {
					t.Fatal(err)
				}
				factory = instrumentModuleFactory(t, factory, transport, backend)
				if err := os.WriteFile(factoryPath, factory, 0644); err != nil {
					t.Fatal(err)
				}
				providerPath := filepath.Join(directory, "internal/apiserver/bootstrap/providers.go")
				providers, err := os.ReadFile(providerPath)
				if err != nil {
					t.Fatal(err)
				}
				providers = append(providers, []byte("\n// User startup customization must be preserved.\n")...)
				if err := os.WriteFile(providerPath, providers, 0644); err != nil {
					t.Fatal(err)
				}
				second, err := generator.PrepareModule(directory, "type")
				if err != nil {
					t.Fatal(err)
				}
				if err := second.Write(); err != nil {
					t.Fatal(err)
				}
				for path, expected := range map[string][]byte{factoryPath: factory, providerPath: providers} {
					actual, err := os.ReadFile(path)
					if err != nil || !bytes.Equal(actual, expected) {
						t.Fatalf("developer assembly changed at %s: %v", path, err)
					}
				}
				proof := filepath.Join(directory, "internal/apiserver/bootstrap/module_assembly_test.go")
				if err := os.WriteFile(proof, []byte(moduleAssemblyProof(transport, backend)), 0644); err != nil {
					t.Fatal(err)
				}
				run(t, directory, "gofmt", "-w", factoryPath, proof)
				if backend == "sqlite" || backend == "none" {
					run(t, directory, "go", "test", "-mod=readonly", "-race", "./...")
				} else {
					// These cases prove SQL source compatibility and shared pointer
					// wiring, not live MySQL/PostgreSQL service behavior.
					run(t, directory, "go", "test", "-mod=readonly", "-run", "^TestGeneratedModuleAssembly$", "./...")
				}
				run(t, directory, "go", "vet", "-mod=readonly", "./...")
				run(t, directory, filepath.Join(root, ".tools/bin/golangci-lint"), "run", "./...")
			})
		}
	}
}

func TestModuleAssemblyOptionalCompilation(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	base, err := os.MkdirTemp(filepath.Join(root, ".tmp"), "d6-module-optional-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(base); err != nil {
			t.Error(err)
		}
	})
	for _, transport := range []string{"http", "grpc", "dual", "gateway"} {
		for _, backend := range []string{"sqlite", "none"} {
			t.Run(transport+"/"+backend, func(t *testing.T) {
				directory := filepath.Join(base, transport+"-"+backend)
				if err := os.Mkdir(directory, 0750); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := os.RemoveAll(directory); err != nil {
						t.Error(err)
					}
				})
				options := templates.Options{Transport: transport, Database: backend, Redis: true, Authz: "casbin"}
				project, err := generator.Prepare(directory, templates.Project{Name: "demo", Module: "example.com/demo"}, options)
				if err != nil {
					t.Fatal(err)
				}
				if err := project.Write(); err != nil {
					t.Fatal(err)
				}
				module, err := generator.PrepareModule(directory, "orders")
				if err != nil {
					t.Fatal(err)
				}
				if err := module.Write(); err != nil {
					t.Fatal(err)
				}
				run(t, directory, "go", "test", "-mod=readonly", "-run", "^$", "./...")
				run(t, directory, "go", "vet", "-mod=readonly", "./...")
				run(t, directory, filepath.Join(root, ".tools/bin/golangci-lint"), "run", "./...")
			})
		}
	}
}

func instrumentModuleFactory(t *testing.T, factory []byte, transport, backend string) []byte {
	t.Helper()
	anchor := []byte("func newOrdersModule(dependencies moduleDependencies) (ordersModule, error) {")
	if bytes.Count(factory, anchor) != 1 {
		t.Fatal("module factory has no stable typed assembly function")
	}
	factory = bytes.Replace(factory, anchor, append(bytes.Clone(anchor), []byte("\nassemblyCalls++;assemblyDependencies=dependencies;if assemblyFailure!=nil{return ordersModule{},assemblyFailure}\n")...), 1)
	if transport == "http" || transport == "dual" {
		factory = bytes.Replace(factory, []byte("map[string]string{}"), []byte("map[string]string{\"GET\":\"read\"}"), 1)
	}
	if transport != "http" {
		method := "Get"
		if backend == "none" {
			method = "Greet"
		}
		permissions := fmt.Sprintf("func()map[string]string{values:=map[string]string{\"/orders.Test/Get\":\"read\"};if assemblyDuplicate{values[%q]=\"read\"};return values}()", "/example.v1.ExampleService/"+method)
		factory = bytes.Replace(factory, []byte("rpcHandler.Permissions"), []byte(permissions), 1)
	}
	if transport == "gateway" {
		factory = bytes.Replace(factory, []byte("map[string][]string{}"), []byte("map[string][]string{\"GET\":{\"page\"}}"), 1)
		factory = bytes.Replace(factory, []byte("map[string]bool{}"), []byte("map[string]bool{\"POST\":true}"), 1)
		factory = bytes.Replace(factory, []byte("return nil"), []byte("gatewayCalls++;return nil"), 1)
	}
	factory = append(factory, []byte("\nvar assemblyCalls int\nvar assemblyDependencies moduleDependencies\nvar assemblyFailure error\n")...)
	if transport == "gateway" {
		factory = append(factory, []byte("var gatewayCalls int\n")...)
	}
	if transport != "http" {
		factory = append(factory, []byte("var assemblyDuplicate bool\n")...)
	}
	return factory
}

func moduleAssemblyProof(transport, backend string) string {
	var source strings.Builder
	source.WriteString("package bootstrap\nimport(\"errors\";\"reflect\";\"testing\";\"go.uber.org/zap\"\n")
	if backend == "sqlite" {
		source.WriteString("\"context\";\"encoding/base64\";\"path/filepath\";\"example.com/demo/internal/pkg/config\";\"example.com/demo/internal/pkg/database\"\n")
	} else if backend != "none" {
		source.WriteString("\"gorm.io/gorm\"\n")
	}
	if transport == "gateway" && backend != "sqlite" {
		source.WriteString("\"context\"\n")
	}
	if backend == "sqlite" && transport != "http" {
		source.WriteString("\"strings\"\n")
	}
	source.WriteString(")\nfunc TestGeneratedModuleAssembly(t *testing.T){\nassemblyCalls=0;assemblyFailure=nil;defer func(){assemblyFailure=nil}()\ndependencies:=moduleDependencies{Logger:zap.NewNop()}\n")
	if transport == "gateway" {
		source.WriteString("gatewayCalls=0\n")
	}
	if transport != "http" {
		source.WriteString("assemblyDuplicate=false;defer func(){assemblyDuplicate=false}()\n")
	}
	if backend == "sqlite" {
		source.WriteString(`t.Setenv("APP_ENV","development")
t.Setenv("APP_JWT_SECRET",base64.StdEncoding.EncodeToString(make([]byte,32)))
settings,err:=config.Load(filepath.Join(t.TempDir(),"missing.yaml"),false,config.Overrides{});if err!=nil{t.Fatal(err)}
settings.Database.Path=filepath.Join(t.TempDir(),"assembly.db")
db,err:=database.Open(context.Background(),settings.Database,dependencies.Logger);if err!=nil{t.Fatal(err)};defer func(){_ = db.Close()}()
if err:=db.Migrate(context.Background());err!=nil{t.Fatal(err)}
dependencies.Database=db.DB
`)
	} else if backend != "none" {
		source.WriteString("dependencies.Database=&gorm.DB{}\n")
	}
	source.WriteString("services,err:=newModuleServices(dependencies);if err!=nil{t.Fatal(err)};if assemblyCalls!=1{t.Fatal(\"module constructed more than once\")};if assemblyDependencies.Logger!=dependencies.Logger{t.Fatal(\"module did not receive the shared logger\")}\n")
	if transport == "http" || transport == "dual" {
		source.WriteString("httpBusiness:=reflect.ValueOf(services.ordersService.HTTP).Elem().FieldByName(\"biz\")\nmodules:=httpModules(nil,services);if len(modules)!=3||modules[1].Name!=\"orders\"||modules[1].Operations[\"GET\"]!=\"read\"{t.Fatal(\"custom HTTP registration lost\")}\n")
	}
	if transport != "http" {
		source.WriteString("rpcBusiness:=reflect.ValueOf(services.ordersService.RPC).Elem().FieldByName(\"biz\")\npermissions,_,err:=rpcModules(nil,services);if err!=nil{t.Fatal(err)};permission:=permissions[\"/orders.Test/Get\"];if permission.Operation!=\"read\"||permission.Resource!=\"orders\"{t.Fatal(\"custom RPC permission or module identity lost\")}\n")
		method := "Get"
		if backend == "none" {
			method = "Greet"
		}
		fmt.Fprintf(&source, "services.ordersService.Permissions[%q]=\"read\";if _,_,err:=rpcModules(nil,services);err==nil{t.Fatal(\"duplicate FullMethod replaced another module permission\")};delete(services.ordersService.Permissions,%q)\n", "/example.v1.ExampleService/"+method, "/example.v1.ExampleService/"+method)
	}
	if transport == "dual" {
		source.WriteString("if httpBusiness.Pointer()!=rpcBusiness.Pointer(){t.Fatal(\"dual constructed different Biz instances\")}\n")
	}
	if transport == "gateway" {
		source.WriteString("mappings:=gatewayModules(services);if len(mappings)!=3||mappings[1].Name!=\"orders\"||mappings[1].Path!=\"/v1/orders\"||len(mappings[1].Queries[\"GET\"])!=1||!mappings[1].BodyMethods[\"POST\"]{t.Fatal(\"custom Gateway description lost\")};if err:=mappings[1].Register(context.Background(),nil,nil);err!=nil||gatewayCalls!=1{t.Fatal(\"custom Gateway callback lost\")}\n")
	}
	business := "rpcBusiness"
	if transport == "http" || transport == "dual" {
		business = "httpBusiness"
	}
	if backend != "none" {
		fmt.Fprintf(&source, "repository:=%s.Elem().FieldByName(\"repository\").Elem().Elem();if repository.FieldByName(\"db\").Pointer()!=reflect.ValueOf(dependencies.Database).Pointer(){t.Fatal(\"Store did not receive application database\")}\n", business)
	} else {
		fmt.Fprintf(&source, "if %s.Pointer()==0{t.Fatal(\"missing Biz\")}\n", business)
	}
	source.WriteString("assemblyFailure=errors.New(\"module assembly failed\");if _,err:=newModuleServices(dependencies);!errors.Is(err,assemblyFailure){t.Fatal(\"assembly failure was swallowed\")}\n")
	if backend == "sqlite" {
		source.WriteString(`var alive int;if err:=db.Pool.QueryRow("SELECT 1").Scan(&alive);err!=nil||alive!=1{t.Fatal("module assembly closed the shared pool")}
if _,err:=New(settings,"test");!errors.Is(err,assemblyFailure){t.Fatal("startup swallowed module failure",err)}
pool,err:=assemblyDependencies.Database.DB();if err!=nil{t.Fatal(err)};if err:=pool.Ping();err==nil{t.Fatal("module startup failure leaked database pool")}
`)
		if transport != "http" {
			source.WriteString(`assemblyFailure=nil;assemblyDuplicate=true
if _,err:=New(settings,"test");err==nil||!strings.Contains(err.Error(),"duplicate RPC method"){t.Fatal("duplicate RPC permission accepted at startup",err)}
pool,err=assemblyDependencies.Database.DB();if err!=nil{t.Fatal(err)};if err:=pool.Ping();err==nil{t.Fatal("duplicate RPC startup failure leaked database pool")}
`)
		}
	}
	source.WriteString("}\n")
	return source.String()
}
