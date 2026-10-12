package integration

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/setcreed/goseed/internal/generator"
	"github.com/setcreed/goseed/internal/templates"
)

func TestModulePermissionCatalogProjects(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	base, err := os.MkdirTemp(filepath.Join(root, ".tmp"), "d6-module-permissions-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(base); err != nil {
			t.Error(err)
		}
	})
	for _, transport := range []string{"http", "grpc", "dual", "gateway"} {
		for _, backend := range []string{"sqlite", "none", "mysql", "postgres"} {
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
				project, err := generator.Prepare(directory, templates.Project{Name: "demo", Module: "example.com/demo"}, templates.Options{Transport: transport, Database: backend, Authz: "casbin"})
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
				factoryPath := filepath.Join(directory, "internal/apiserver/bootstrap/module_orders.go")
				factory, err := os.ReadFile(factoryPath)
				if err != nil {
					t.Fatal(err)
				}
				if transport == "http" || transport == "dual" {
					factory = bytes.Replace(factory, []byte("map[string]string{}"), []byte("map[string]string{\"GET\":\"read\"}"), 1)
				}
				if transport != "http" {
					factory = bytes.Replace(factory, []byte("rpcHandler.Permissions"), []byte("map[string]string{\"/orders.v1.OrdersService/Get\":\"read\"}"), 1)
				}
				anchor := []byte("func newOrdersModule(dependencies moduleDependencies) (ordersModule, error) {")
				if bytes.Count(factory, anchor) != 1 {
					t.Fatal("missing module constructor")
				}
				factory = bytes.Replace(factory, anchor, append(bytes.Clone(anchor), []byte("\nif rejectCatalogConstruction {panic(\"permission lookup constructed business\")}\n")...), 1)
				mismatch := "permissions.HTTP=map[string]string{\"GET\":\"delete\"}"
				if transport != "http" && transport != "dual" {
					mismatch = "permissions.RPC=map[string]string{\"/orders.v1.OrdersService/Get\":\"delete\"}"
				}
				factory = bytes.Replace(factory, []byte("permissions := ordersPermissions()"), []byte("permissions := ordersPermissions()\nif rejectPermissionMismatch {"+mismatch+"}"), 1)
				factory = append(factory, []byte("\nvar rejectCatalogConstruction bool\nvar rejectPermissionMismatch bool\n")...)
				if err := os.WriteFile(factoryPath, factory, 0644); err != nil {
					t.Fatal(err)
				}
				proof := filepath.Join(directory, "internal/apiserver/bootstrap/permission_catalog_test.go")
				if err := os.WriteFile(proof, []byte(permissionCatalogProof()), 0644); err != nil {
					t.Fatal(err)
				}
				run(t, directory, "gofmt", "-w", factoryPath, proof)
				if backend == "sqlite" || backend == "none" {
					run(t, directory, "go", "test", "-mod=readonly", "-race", "./...")
				} else {
					// External SQL transaction tests run explicitly via template-db.
					run(t, directory, "go", "test", "-mod=readonly", "-run", "^TestGeneratedPermission(Catalog|Mismatch)$", "./...")
				}
				run(t, directory, "go", "vet", "-mod=readonly", "./...")
				run(t, directory, filepath.Join(root, ".tools/bin/golangci-lint"), "run", "./...")
				if backend == "sqlite" {
					binary := filepath.Join(directory, "bin/apiserver")
					run(t, directory, "go", "build", "-mod=readonly", "-o", binary, "./cmd/apiserver")
					run(t, directory, binary, "dev", "init")
					run(t, directory, binary, "migrate", "up")
					policy := filepath.Join(directory, "configs/orders.csv")
					if err := os.WriteFile(policy, []byte("p,role:reader,orders,read\ng,user:alice,role:reader\n"), 0600); err != nil {
						t.Fatal(err)
					}
					run(t, directory, binary, "policy", "replace", "--file", policy)
				}
			})
		}
	}
}

func permissionCatalogProof() string {
	return `package bootstrap
import("testing";"slices";"strings")
func TestGeneratedPermissionCatalog(t *testing.T){
 rejectCatalogConstruction=true;defer func(){rejectCatalogConstruction=false}()
 catalog,err:=PermissionCatalog();if err!=nil{t.Fatal(err)}
 if !slices.Equal(catalog["orders"],[]string{"read"}){t.Fatalf("resource permission missing: %v",catalog)}
 catalog["orders"][0]="delete"
 fresh,err:=PermissionCatalog();if err!=nil||!slices.Equal(fresh["orders"],[]string{"read"}){t.Fatal("caller mutated pure permissions")}
 if _,err:=permissionCatalog(map[string]modulePermissions{"orders":{HTTP:map[string]string{"GET":"*"}}});err==nil{t.Fatal("wildcard permission accepted")}
 if _,err:=permissionCatalog(map[string]modulePermissions{"orders":{RPC:map[string]string{"not-a-full-method":"read"}}});err==nil{t.Fatal("invalid full method accepted")}
 if _,err:=permissionCatalog(map[string]modulePermissions{"orders":{RPC:map[string]string{"/test.Service/Get":"read"}},"other":{RPC:map[string]string{"/test.Service/Get":"read"}}});err==nil{t.Fatal("duplicate full method accepted")}
}
func TestGeneratedPermissionMismatch(t *testing.T){
 rejectPermissionMismatch=true;defer func(){rejectPermissionMismatch=false}()
 if _,err:=newModuleServices(moduleDependencies{});err==nil||!strings.Contains(err.Error(),"pure declaration"){t.Fatalf("factory permission mismatch accepted: %v",err)}
}
`
}
