package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/setcreed/goseed/internal/templates"
)

var coreFields = []templates.ResourceField{
	{SchemaField: templates.SchemaField{Name: "title", Type: "string", MaxLength: 3}, Required: true},
	{SchemaField: templates.SchemaField{Name: "select", Type: "string", MaxLength: 4}},
	{SchemaField: templates.SchemaField{Name: "enabled", Type: "bool"}},
	{SchemaField: templates.SchemaField{Name: "amount", Type: "int64"}},
}

func TestResourceCoreProjects(t *testing.T) {
	for _, optional := range []bool{false, true} {
		t.Run(fmt.Sprint(optional), func(t *testing.T) {
			options := templates.Options{Database: "sqlite", Redis: optional}
			if optional {
				options.Authz = "casbin"
			}
			verifyResourceCoreProject(t, options, coreFields, nil)
		})
	}
	for _, fields := range [][]templates.ResourceField{
		{{SchemaField: templates.SchemaField{Name: "flag", Type: "bool"}}},
		{{SchemaField: templates.SchemaField{Name: "number", Type: "int64"}, Required: true}},
		{{SchemaField: templates.SchemaField{Name: "owner_info", Type: "string", MaxLength: 1}, Required: true}},
	} {
		t.Run(fields[0].Name, func(t *testing.T) { verifyResourceCoreProject(t, templates.Options{Database: "sqlite"}, fields, nil) })
	}
}

func TestExternalResourceCoreProjects(t *testing.T) {
	if os.Getenv("GOSEED_TEMPLATE_DB") != "1" {
		t.Skip("run make template-db for real resource CRUD verification")
	}
	for _, database := range []string{"mysql", "postgres"} {
		for _, optional := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s-%t", database, optional), func(t *testing.T) {
				port := os.Getenv("GOSEED_TEST_" + strings.ToUpper(database) + "_PORT")
				password := os.Getenv("GOSEED_TEST_PASSWORD")
				if port == "" || password == "" {
					t.Fatal("dedicated credentials required")
				}
				user := "root"
				if database == "postgres" {
					user = "goseed_test"
				}
				options := templates.Options{Database: database, Redis: optional}
				if optional {
					options.Authz = "casbin"
				}
				verifyResourceCoreProject(t, options, coreFields, []string{"GOSEED_TEST_EXTERNAL=1", "GOSEED_TEST_DATABASE_HOST=127.0.0.1", "GOSEED_TEST_DATABASE_PORT=" + port, "GOSEED_TEST_DATABASE_USER=" + user, "GOSEED_TEST_DATABASE_PASSWORD=" + password})
			})
		}
	}
}

func verifyResourceCoreProject(t *testing.T, options templates.Options, fields []templates.ResourceField, env []string) {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	directory, err := os.MkdirTemp(filepath.Join(root, ".tmp"), "d6-resource-core-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(directory); err != nil {
			t.Error(err)
		}
	})
	project := templates.Project{Name: "demo", Module: "example.com/demo"}
	files, err := templates.Render(project, options)
	if err != nil {
		t.Fatal(err)
	}
	base := 1
	if options.Authz == "casbin" {
		base = 2
	}
	for i, name := range []string{"orders", "notes"} {
		core, err := templates.RenderResourceCore(project, options, name, fields)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, core...)
		schema := make([]templates.SchemaField, len(fields))
		for j, field := range fields {
			schema[j] = field.SchemaField
		}
		_, sql, err := templates.RenderResourceSchema(project, options.Database, name, schema)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, templates.File{Path: fmt.Sprintf("migrations/%05d_%s.sql", base+i+1, name), Content: sql})
	}
	for _, file := range files {
		path := filepath.Join(directory, file.Path)
		if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, file.Content, 0644); err != nil {
			t.Fatal(err)
		}
	}
	if options.Database != "sqlite" {
		writeResourceApplicationTest(t, directory, options)
	}
	verifyReadonly(t, directory)
	output := runWith(t, directory, env, "go", "test", "-mod=readonly", "-race", "-count=1", "-v", "./internal/apiserver/model", "./internal/apiserver/biz/orders", "./internal/apiserver/biz/notes", "./internal/apiserver/store/orders", "./internal/apiserver/store/notes")
	if env != nil && strings.Contains(string(output), "--- SKIP:") {
		t.Fatal("external resource verification skipped")
	}
	run(t, directory, "go", "vet", "-mod=readonly", "./...")
	run(t, directory, filepath.Join(root, ".tools/bin/golangci-lint"), "run", "./...")
	run(t, directory, "go", "build", "-mod=readonly", "./...")
}

func writeResourceApplicationTest(t *testing.T, directory string, options templates.Options) {
	t.Helper()
	setup, cleanup := resourceAccountStatements(options.Database, options.Authz)
	dependency := ""
	if options.Redis {
		dependency = ",dependency{}"
	}
	source := fmt.Sprintf(`package orders_test
import("context";"crypto/rand";"errors";"strings";"testing";"example.com/demo/internal/pkg/database";"example.com/demo/internal/testutil";"example.com/demo/internal/pkg/contextx";"example.com/demo/internal/pkg/errno";business "example.com/demo/internal/apiserver/biz/orders";store "example.com/demo/internal/apiserver/store/orders";"go.uber.org/zap")
func TestApplicationAccountCRUD(t *testing.T){
 ctx:=context.Background();settings:=testutil.Configuration(t).Database
 admin,err:=database.Open(ctx,settings,zap.NewNop());if err!=nil{t.Fatal(err)};defer func(){_ = admin.Close()}()
 if err:=admin.Migrate(ctx);err!=nil{t.Fatal(err)}
 user:="goseed_app_"+strings.TrimPrefix(settings.Name,"goseed_test_")[:16];password:=rand.Text()
 exec:=func(statement string){t.Helper();if err:=admin.DB.WithContext(ctx).Exec(statement).Error;err!=nil{t.Fatal("isolated account setup or cleanup failed")}}
 %s
 defer func(){%s}()
 settings.User=user;settings.Password=password
 app,err:=database.Open(ctx,settings,zap.NewNop());if err!=nil{t.Fatal(err)};defer func(){_ = app.Close()}()
 if err:=app.CheckSchema(ctx);err!=nil{t.Fatal(err)}
 repository:=store.New(app.DB);if err:=repository.CheckSchema(ctx);err!=nil{t.Fatal(err)}
 biz:=business.New(repository%s);alice:=contextx.WithSubject(ctx,"Alice");bob:=contextx.WithSubject(ctx,"alice")
 value,err:=biz.Create(alice,input());if err!=nil{t.Fatal(err)};other,err:=biz.Create(bob,input());if err!=nil{t.Fatal(err)}
 if _,err:=biz.Get(bob,value.ID);!errors.Is(err,errno.NotFound){t.Fatal("cross-owner read",err)}
 if _,err:=biz.Update(bob,value.ID,business.UpdateInput(input()));!errors.Is(err,errno.NotFound){t.Fatal("cross-owner patch",err)}
 if err:=biz.Delete(bob,value.ID);!errors.Is(err,errno.NotFound){t.Fatal("cross-owner delete",err)}
 if _,err:=biz.Get(alice,value.ID);err!=nil{t.Fatal(err)};if _,err:=biz.Update(alice,value.ID,business.UpdateInput(input()));err!=nil{t.Fatal(err)}
 page,err:=biz.List(alice,1,20);if err!=nil||page.Total!=1{t.Fatal("scoped count",err)}
 if err:=biz.Delete(alice,value.ID);err!=nil{t.Fatal(err)};if _,err:=biz.Get(bob,other.ID);err!=nil{t.Fatal("another subject was modified",err)}
 if err:=app.DB.WithContext(ctx).Exec("CREATE TABLE forbidden_app_ddl(id BIGINT)").Error;err==nil{t.Fatal("application account has DDL privileges")}
}
`, setup, cleanup, dependency)
	path := filepath.Join(directory, "internal/apiserver/store/orders/application_test.go")
	if err := os.WriteFile(path, []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	run(t, directory, "gofmt", "-w", "internal/apiserver/store/orders/application_test.go")
}
