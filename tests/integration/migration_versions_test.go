package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/setcreed/goseed/internal/templates"
)

func TestMigrationVersionProjects(t *testing.T) {
	for _, authz := range []string{"none", "casbin"} {
		t.Run(authz, func(t *testing.T) { verifyMigrationVersionProject(t, "sqlite", authz, nil) })
	}
}

func TestExternalMigrationVersionProjects(t *testing.T) {
	if os.Getenv("GOSEED_TEMPLATE_DB") != "1" {
		t.Skip("run make template-db for isolated real SQL verification")
	}
	for _, backend := range []string{"mysql", "postgres"} {
		for _, authz := range []string{"none", "casbin"} {
			t.Run(backend+"-"+authz, func(t *testing.T) {
				port := os.Getenv("GOSEED_TEST_" + strings.ToUpper(backend) + "_PORT")
				password := os.Getenv("GOSEED_TEST_PASSWORD")
				if port == "" || password == "" {
					t.Fatal("dedicated database credentials required")
				}
				user := "root"
				if backend == "postgres" {
					user = "goseed_test"
				}
				verifyMigrationVersionProject(t, backend, authz, []string{"GOSEED_TEST_EXTERNAL=1", "GOSEED_TEST_DATABASE_HOST=127.0.0.1", "GOSEED_TEST_DATABASE_PORT=" + port, "GOSEED_TEST_DATABASE_USER=" + user, "GOSEED_TEST_DATABASE_PASSWORD=" + password})
			})
		}
	}
}

func verifyMigrationVersionProject(t *testing.T, backend, authz string, env []string) {
	t.Helper()
	directory := renderProjectWith(t, templates.Options{Database: backend, Authz: authz})
	verifyReadonly(t, directory)
	base := 1
	if authz == "casbin" {
		base = 2
	}
	fields := []templates.SchemaField{{Name: "title", Type: "string", MaxLength: 3}, {Name: "enabled", Type: "bool"}, {Name: "count", Type: "int64"}, {Name: "select", Type: "string", MaxLength: 4096}}
	for offset, table := range []string{"orders", "notes"} {
		store, sql, err := templates.RenderResourceSchema(templates.Project{Name: "demo", Module: "example.com/demo"}, backend, table, fields)
		if err != nil {
			t.Fatal(err)
		}
		filename := filepath.Join(directory, store.Path)
		if err := os.MkdirAll(filepath.Dir(filename), 0750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filename, store.Content, 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, "migrations", fmt.Sprintf("%05d_%s.sql", base+offset+1, table)), sql, 0644); err != nil {
			t.Fatal(err)
		}
	}
	settings := `settings:=testutil.Configuration(t).Database`
	imports := `"crypto/rand";"example.com/demo/internal/pkg/config";"example.com/demo/internal/testutil"`
	if backend == "sqlite" {
		settings = `settings:=config.Database{Path:filepath.Join(t.TempDir(),"growth.db"),MaxOpen:2,MaxIdle:1,BusyTimeout:time.Second}`
		imports = `"path/filepath";"example.com/demo/internal/pkg/config"`
	}
	source := fmt.Sprintf(`package database_test
import("context";"fmt";"io/fs";"strings";"testing";"testing/fstest";"time";orders "example.com/demo/internal/apiserver/store/orders";notes "example.com/demo/internal/apiserver/store/notes";"example.com/demo/internal/pkg/database";"example.com/demo/migrations";"github.com/pressly/goose/v3";"go.uber.org/zap";%s)
func TestMigrationGrowth(t *testing.T){
 ctx:=context.Background();%s
 db,err:=database.Open(ctx,settings,zap.NewNop());if err!=nil{t.Fatal(err)};defer func(){_ = db.Close()}()
 if err:=orders.New(db.DB).CheckSchema(ctx);err==nil{t.Fatal("missing resource schema accepted")}
 versions,err:=migrations.Versions();if err!=nil||len(versions)!=%d{t.Fatal("embedded version growth",versions,err)}
 subset:=make(fstest.MapFS)
 for step,version:=range versions {
  entries,err:=fs.ReadDir(migrations.Files,".");if err!=nil{t.Fatal(err)}
  for _,entry:=range entries{data,err:=fs.ReadFile(migrations.Files,entry.Name());if err!=nil{t.Fatal(err)};if entry.Name()[:5]==fmt.Sprintf("%%05d",version){subset[entry.Name()]=&fstest.MapFile{Data:data}}}
  if err:=db.CheckSchema(ctx);err==nil{t.Fatal("pending migrations accepted",step)}
  provider,err:=goose.NewProvider(goose.%s,db.Pool,subset,goose.WithDisableGlobalRegistry(true));if err!=nil{t.Fatal(err)}
  if _,err:=provider.Up(ctx);err!=nil{t.Fatal(err)}
  actual,err:=db.Version(ctx);if err!=nil||actual!=version{t.Fatal("incremental version",actual,err)}
 }
 for range 2{if err:=db.Migrate(ctx);err!=nil{t.Fatal(err)}}
 if err:=orders.New(db.DB).CheckSchema(ctx);err!=nil{t.Fatal(err)};if err:=notes.New(db.DB).CheckSchema(ctx);err!=nil{t.Fatal(err)}
 now:=time.Now().UTC();statement:=%q
 if err:=db.DB.WithContext(ctx).Exec(statement,"subject","中文😀",false,int64(9223372036854775807),strings.Repeat("中",4096),now,now).Error;err!=nil{t.Fatal("Unicode/int64 boundary rejected",err)}
 if err:=db.DB.WithContext(ctx).Exec(statement,"subject","中文😀外",false,int64(0),"",now,now).Error;err==nil{t.Fatal("Unicode length overflow accepted")}
 if err:=db.Close();err!=nil{t.Fatal(err)}
 reopened,err:=database.Open(ctx,settings,zap.NewNop());if err!=nil{t.Fatal(err)};defer func(){_ = reopened.Close()}()
 if err:=reopened.CheckSchema(ctx);err!=nil{t.Fatal("restart schema",err)}
 if err:=orders.New(reopened.DB).CheckSchema(ctx);err!=nil{t.Fatal(err)};if err:=notes.New(reopened.DB).CheckSchema(ctx);err!=nil{t.Fatal(err)}
 var count int;if err:=reopened.Pool.QueryRowContext(ctx,"SELECT count(*) FROM orders").Scan(&count);err!=nil||count!=1{t.Fatal("resource restart persistence",count,err)}
 %s
}
`, imports, settings, base+2, map[string]string{"sqlite": "DialectSQLite3", "mysql": "DialectMySQL", "postgres": "DialectPostgres"}[backend], resourceInsertStatement(backend), applicationAccountCall(backend))
	if backend != "sqlite" {
		source += resourceApplicationAccountTest(backend, authz)
	}
	if err := os.WriteFile(filepath.Join(directory, "internal/pkg/database/migration_growth_test.go"), []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	writeResourceSchemaFailures(t, directory, backend, settings, imports)
	run(t, directory, "gofmt", "-w", "internal/pkg/database/migration_growth_test.go")
	output := runWith(t, directory, env, "go", "test", "-mod=readonly", "-race", "-count=1", "-v", "./migrations", "./internal/pkg/database")
	if env != nil && strings.Contains(string(output), "--- SKIP:") {
		t.Fatal("external migration verification skipped a test")
	}
	run(t, directory, "go", "vet", "-mod=readonly", "./...")
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	run(t, directory, filepath.Join(root, ".tools/bin/golangci-lint"), "run", "./...")
}

func writeResourceSchemaFailures(t *testing.T, directory, backend, settings, imports string) {
	t.Helper()
	_, draft, err := templates.RenderResourceSchema(templates.Project{Name: "demo", Module: "example.com/demo"}, backend, "orders", []templates.SchemaField{{Name: "title", Type: "string", MaxLength: 3}, {Name: "enabled", Type: "bool"}, {Name: "count", Type: "int64"}})
	if err != nil {
		t.Fatal(err)
	}
	quote := func(s string) string { return `"` + s + `"` }
	if backend == "mysql" {
		quote = func(s string) string { return "`" + s + "`" }
	}
	baseSQL := string(draft)
	function := "length"
	if backend == "mysql" {
		function = "CHAR_LENGTH"
	}
	titleType := "TEXT"
	if backend == "postgres" {
		titleType = "VARCHAR(3)"
	}
	invalid := map[string]string{
		"nullable":      strings.Replace(baseSQL, quote("title")+" "+titleType+" NOT NULL", quote("title")+" "+titleType, 1),
		"wrong-type":    strings.Replace(baseSQL, quote("enabled")+" BOOLEAN", quote("enabled")+" BIGINT", 1),
		"missing-field": strings.Replace(baseSQL, quote("count")+" BIGINT NOT NULL", quote("missing_count")+" BIGINT NOT NULL", 1),
		"owner-length":  strings.Replace(baseSQL, "128", "129", 1),
	}
	if backend == "postgres" {
		invalid["length"] = strings.Replace(baseSQL, "VARCHAR(3)", "VARCHAR(4)", 1)
	} else {
		expression := function + "(" + quote("title") + ") <= 3"
		invalid["length"] = strings.Replace(baseSQL, expression, function+"("+quote("title")+") <= 4", 1)
		invalid["tautology"] = strings.Replace(baseSQL, expression, expression+" OR 1=1", 1)
		invalid["wrong-parentheses"] = strings.Replace(baseSQL, expression, function+"("+quote("title")+" <= 3)", 1)
		if backend == "mysql" {
			invalid["wrong-parentheses"] = strings.Replace(baseSQL, expression, function+"("+quote("title")+" <= 3) > 0", 1)
		}
	}
	if backend == "mysql" {
		invalid["unsigned-id"] = strings.Replace(baseSQL, "BIGINT NOT NULL AUTO_INCREMENT", "BIGINT UNSIGNED NOT NULL AUTO_INCREMENT", 1)
	}
	switch backend {
	case "sqlite":
		invalid["owner-collation"] = strings.Replace(baseSQL, quote("owner")+" TEXT NOT NULL", quote("owner")+" TEXT COLLATE NOCASE NOT NULL", 1)
		invalid["owner-index-override"] = strings.Replace(invalid["owner-collation"], "("+quote("owner")+", "+quote("id")+")", "("+quote("owner")+" COLLATE BINARY, "+quote("id")+")", 1)
	case "mysql":
		invalid["owner-collation"] = strings.Replace(baseSQL, quote("owner")+" VARCHAR(128) NOT NULL", quote("owner")+" VARCHAR(128) COLLATE utf8mb4_0900_ai_ci NOT NULL", 1)
	case "postgres":
		invalid["owner-collation"] = strings.Replace(baseSQL, `COLLATE "C"`, `COLLATE "default"`, 1)
	}
	id := "INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT"
	if backend == "mysql" {
		id = "BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY"
	}
	if backend == "postgres" {
		id = "BIGINT GENERATED BY DEFAULT AS IDENTITY NOT NULL PRIMARY KEY"
	}
	invalid["missing-primary"] = strings.Replace(baseSQL, quote("id")+" "+id, quote("id")+" BIGINT NOT NULL", 1)
	var source strings.Builder
	if backend == "sqlite" {
		imports += ";\"time\""
	} else {
		imports = `"example.com/demo/internal/testutil"`
	}
	fmt.Fprintf(&source, `package database_test
import("context";"errors";"testing";"testing/fstest";"example.com/demo/internal/pkg/database";"github.com/pressly/goose/v3";"go.uber.org/zap";%s)
func TestResourceSchemaFailures(t *testing.T){
 cases:=map[string]string{
`, imports)
	for _, name := range []string{"nullable", "wrong-type", "missing-field", "owner-length", "length", "tautology", "wrong-parentheses", "unsigned-id", "missing-primary", "owner-collation", "owner-index-override"} {
		sql, ok := invalid[name]
		if !ok {
			continue
		}
		if sql == baseSQL {
			t.Fatal("schema mutation did not change fixture", name)
		}
		fmt.Fprintf(&source, "%q:%q,\n", name, sql)
	}
	source.WriteString("}\nfor name,draft:=range cases{t.Run(name,func(t *testing.T){ctx:=context.Background();" + settings + "\n")
	fmt.Fprintf(&source, `db,err:=database.Open(ctx,settings,zap.NewNop());if err!=nil{t.Fatal(err)};defer func(){_ = db.Close()}()
 provider,err:=goose.NewProvider(goose.%s,db.Pool,fstest.MapFS{"00001_orders.sql":{Data:[]byte(draft)}},goose.WithDisableGlobalRegistry(true));if err!=nil{t.Fatal(err)};if _,err:=provider.Up(ctx);err!=nil{t.Fatal("invalid schema fixture could not be created",err)}
 fields:=[]database.SchemaField{{Name:"title",Type:"string",MaxLength:3},{Name:"enabled",Type:"bool"},{Name:"count",Type:"int64"}}
 var before,after int; if err:=db.Pool.QueryRowContext(ctx,"SELECT count(*) FROM goose_db_version").Scan(&before);err!=nil{t.Fatal(err)}
 if err:=database.CheckResourceSchema(ctx,db.DB,"orders",fields);err==nil{t.Fatal("incompatible schema accepted")}
 if err:=db.Pool.QueryRowContext(ctx,"SELECT count(*) FROM goose_db_version").Scan(&after);err!=nil||before!=after{t.Fatal("schema check changed history",err)}
 canceled,cancel:=context.WithCancel(ctx);cancel();if err:=database.CheckResourceSchema(canceled,db.DB,"orders",fields);!errors.Is(err,context.Canceled){t.Fatal("schema cancellation lost",err)}
 })}}
`, map[string]string{"sqlite": "DialectSQLite3", "mysql": "DialectMySQL", "postgres": "DialectPostgres"}[backend])
	fmt.Fprintf(&source, "func TestMigrationDDLFailure(t *testing.T){ctx:=context.Background();%s\n", settings)
	fmt.Fprintf(&source, `db,err:=database.Open(ctx,settings,zap.NewNop());if err!=nil{t.Fatal(err)};defer func(){_ = db.Close()}()
 draft:="-- +goose Up\nCREATE TABLE migration_failure(id BIGINT);\nINSERT INTO missing_failure_table(id) VALUES(1);\n-- +goose Down\nDROP TABLE migration_failure;\n"
 provider,err:=goose.NewProvider(goose.%s,db.Pool,fstest.MapFS{"00001_failure.sql":{Data:[]byte(draft)}},goose.WithDisableGlobalRegistry(true));if err!=nil{t.Fatal(err)}
 if _,err:=provider.Up(ctx);err==nil{t.Fatal("broken migration succeeded")}
 version,err:=db.Version(ctx);if err!=nil||version!=0{t.Fatal("failed migration recorded as applied",version,err)}
 if db.DB.WithContext(ctx).Migrator().HasTable("migration_failure")!=%t{t.Fatal("unexpected transactional DDL behavior")}
}
`, map[string]string{"sqlite": "DialectSQLite3", "mysql": "DialectMySQL", "postgres": "DialectPostgres"}[backend], backend == "mysql")
	wide := make([]templates.SchemaField, 32)
	for i := range wide {
		wide[i] = templates.SchemaField{Name: fmt.Sprintf("field%d", i), Type: "string", MaxLength: 4096}
	}
	wide[0].Name = strings.Repeat("a", 63)
	wideTable := strings.Repeat("w", 63)
	_, wideSQL, err := templates.RenderResourceSchema(templates.Project{Name: "demo", Module: "example.com/demo"}, backend, wideTable, wide)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprintf(&source, "func TestWideResourceSchema(t *testing.T){ctx:=context.Background();%s\n", settings)
	fmt.Fprintf(&source, `db,err:=database.Open(ctx,settings,zap.NewNop());if err!=nil{t.Fatal(err)};defer func(){_ = db.Close()}()
 provider,err:=goose.NewProvider(goose.%s,db.Pool,fstest.MapFS{"00001_wide.sql":{Data:[]byte(%q)}},goose.WithDisableGlobalRegistry(true));if err!=nil{t.Fatal(err)};if _,err:=provider.Up(ctx);err!=nil{t.Fatal("maximum DSL schema cannot be applied",err)}
 fields:=[]database.SchemaField{
`, map[string]string{"sqlite": "DialectSQLite3", "mysql": "DialectMySQL", "postgres": "DialectPostgres"}[backend], string(wideSQL))
	for _, field := range wide {
		fmt.Fprintf(&source, "{Name:%q,Type:%q,MaxLength:%d},\n", field.Name, field.Type, field.MaxLength)
	}
	fmt.Fprintf(&source, "};if err:=database.CheckResourceSchema(ctx,db.DB,%q,fields);err!=nil{t.Fatal(err)}\n}\n", wideTable)
	if err := os.WriteFile(filepath.Join(directory, "internal/pkg/database/resource_failures_test.go"), []byte(source.String()), 0644); err != nil {
		t.Fatal(err)
	}
	run(t, directory, "gofmt", "-w", "internal/pkg/database/resource_failures_test.go")
}

func resourceInsertStatement(backend string) string {
	quote := func(name string) string { return `"` + name + `"` }
	if backend == "mysql" {
		quote = func(name string) string { return "`" + name + "`" }
	}
	columns := []string{"owner", "title", "enabled", "count", "select", "created_at", "updated_at"}
	for i := range columns {
		columns[i] = quote(columns[i])
	}
	return "INSERT INTO orders(" + strings.Join(columns, ",") + ") VALUES(?,?,?,?,?,?,?)"
}

func applicationAccountCall(backend string) string {
	if backend == "sqlite" {
		return ""
	}
	return "verifyApplicationAccount(t,ctx,reopened,settings)"
}

func resourceApplicationAccountTest(backend, authz string) string {
	setup, cleanup := resourceAccountStatements(backend, authz)
	return fmt.Sprintf(`
func verifyApplicationAccount(t *testing.T,ctx context.Context,admin *database.Database,settings config.Database){
 t.Helper();user:="goseed_app_"+strings.TrimPrefix(settings.Name,"goseed_test_")[:16];password:=rand.Text()
 exec:=func(statement string){t.Helper();if err:=admin.DB.WithContext(ctx).Exec(statement).Error;err!=nil{t.Fatal("isolated application account setup or cleanup failed")}}
 %s
 defer func(){%s}()
 settings.User=user;settings.Password=password
 application,err:=database.Open(ctx,settings,zap.NewNop());if err!=nil{t.Fatal(err)};defer func(){_ = application.Close()}()
 if err:=application.CheckSchema(ctx);err!=nil{t.Fatal("application account requires extra migration privileges",err)}
 if err:=orders.New(application.DB).CheckSchema(ctx);err!=nil{t.Fatal("application account cannot inspect resource",err)}
 if err:=notes.New(application.DB).CheckSchema(ctx);err!=nil{t.Fatal(err)}
 if err:=application.DB.WithContext(ctx).Exec("CREATE TABLE forbidden_app_ddl(id BIGINT)").Error;err==nil{t.Fatal("application account unexpectedly has DDL privileges")}
 if err:=application.DB.WithContext(ctx).Exec("INSERT INTO orders(owner,title,enabled,count,created_at,updated_at) VALUES(?,?,?,?,?,?)","application","ok",false,int64(0),time.Now().UTC(),time.Now().UTC()).Error;err==nil{t.Fatal("schema allows omitted non-null field")}
 if err:=application.DB.WithContext(ctx).Exec(%q,"application","ok",false,int64(0),"",time.Now().UTC(),time.Now().UTC()).Error;err!=nil{t.Fatal("application account cannot write business rows",err)}
}
`, setup, cleanup, resourceInsertStatement(backend))
}

func resourceAccountStatements(backend, authz string) (string, string) {
	setup := ""
	cleanup := ""
	tables := []string{"examples", "orders", "notes"}
	if authz == "casbin" {
		tables = append(tables, "casbin_rules")
	}
	if backend == "mysql" {
		setup = `exec("CREATE USER '"+user+"'@'%' IDENTIFIED BY '"+password+"'")`
		setup += "\n" + `exec("GRANT SELECT ON "+settings.Name+".goose_db_version TO '"+user+"'@'%'")`
		for _, table := range tables {
			setup += "\n" + `exec("GRANT SELECT,INSERT,UPDATE,DELETE ON "+settings.Name+".` + table + ` TO '"+user+"'@'%'")`
		}
		cleanup = `exec("DROP USER '"+user+"'@'%'")`
	} else {
		setup = `exec("CREATE ROLE "+user+" LOGIN PASSWORD '"+password+"'")`
		setup += "\n" + `exec("GRANT CONNECT ON DATABASE "+settings.Name+" TO "+user)`
		setup += "\n" + `exec("GRANT USAGE ON SCHEMA public TO "+user)`
		setup += "\n" + `exec("GRANT SELECT ON TABLE goose_db_version TO "+user)`
		for _, table := range tables {
			setup += "\n" + `exec("GRANT SELECT,INSERT,UPDATE,DELETE ON TABLE ` + table + ` TO "+user)`
		}
		setup += "\n" + `exec("GRANT USAGE,SELECT ON ALL SEQUENCES IN SCHEMA public TO "+user)`
		cleanup = `exec("DROP OWNED BY "+user);exec("DROP ROLE "+user)`
	}
	return setup, cleanup
}
