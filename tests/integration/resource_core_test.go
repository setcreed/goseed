package integration

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
			options := templates.Options{Database: "sqlite", Redis: optional, Transport: "http"}
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
		t.Run(fields[0].Name, func(t *testing.T) {
			verifyResourceCoreProject(t, templates.Options{Database: "sqlite", Transport: "http"}, fields, nil)
		})
	}
}

func TestResourceGRPCProjects(t *testing.T) {
	for _, transport := range []string{"grpc", "dual", "gateway"} {
		t.Run(transport, func(t *testing.T) {
			verifyResourceCoreProject(t, templates.Options{Database: "sqlite", Transport: transport}, coreFields, nil)
		})
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
		if options.Transport == "http" || options.Transport == "dual" {
			httpFiles, err := templates.RenderResourceHTTP(project, options, name, fields)
			if err != nil {
				t.Fatal(err)
			}
			files = append(files, httpFiles...)
		}
		if options.Transport == "grpc" || options.Transport == "dual" || options.Transport == "gateway" {
			rpcFiles, err := templates.RenderResourceGRPC(project, options, name, fields)
			if err != nil {
				t.Fatal(err)
			}
			files = append(files, rpcFiles...)
		}
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
	if options.Transport == "http" || options.Transport == "grpc" || options.Transport == "dual" || options.Transport == "gateway" {
		registry, err := templates.RenderWithModules(project, options, []templates.ModuleEntry{{Name: "example"}, {Name: "orders", Type: "resource"}, {Name: "notes", Type: "resource"}})
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range registry {
			if file.Path != "internal/apiserver/bootstrap/modules_gen.go" {
				continue
			}
			for i := range files {
				if files[i].Path == file.Path {
					files[i] = file
					break
				}
			}

		}
	}
	if options.Database == "sqlite" && options.Transport == "http" {
		integrationFile, err := templates.RenderResourceHTTPIntegrationTest(project, options, fields)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(directory, integrationFile.Path)
		if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, integrationFile.Content, 0644); err != nil {
			t.Fatal(err)
		}
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
	if options.Transport == "grpc" || options.Transport == "dual" || options.Transport == "gateway" {
		if err := os.Symlink(filepath.Join(root, ".tools"), filepath.Join(directory, ".tools")); err != nil {
			t.Fatal(err)
		}
		run(t, directory, "make", "proto-generate")
		verifyResourceProtoReproduction(t, directory)
	}
	if options.Database != "sqlite" {
		writeResourceApplicationTest(t, directory, options)
	}
	verifyReadonly(t, directory)
	output := runWith(t, directory, env, "go", "test", "-mod=readonly", "-race", "-count=1", "-v", "./internal/apiserver/model", "./internal/apiserver/biz/orders", "./internal/apiserver/biz/notes", "./internal/apiserver/store/orders", "./internal/apiserver/store/notes")
	if options.Transport == "http" || options.Transport == "dual" {
		output = runWith(t, directory, env, "go", "test", "-mod=readonly", "-race", "-count=1", "./internal/apiserver/handler/http/orders", "./internal/apiserver/handler/http/notes")
		runWith(t, directory, env, "go", "test", "-mod=readonly", "-run", "^$", "./internal/apiserver/bootstrap")
	}
	if options.Transport == "grpc" || options.Transport == "dual" || options.Transport == "gateway" {
		output = runWith(t, directory, env, "go", "test", "-mod=readonly", "-race", "-count=1", "./internal/apiserver/handler/grpc/orders", "./internal/apiserver/handler/grpc/notes")
		runWith(t, directory, env, "go", "test", "-mod=readonly", "-run", "^$", "./internal/apiserver/bootstrap")
		if options.Database == "sqlite" {
			verifyResourceRPCProcess(t, directory, options.Transport)
		}
	}
	if options.Database == "sqlite" && options.Transport == "http" {
		runWith(t, directory, env, "go", "test", "-mod=readonly", "-race", "-count=1", "-run", "^TestGeneratedResourceHTTP$", "./internal/apiserver")
	}
	if env != nil && strings.Contains(string(output), "--- SKIP:") {
		t.Fatal("external resource verification skipped")
	}
	run(t, directory, "go", "vet", "-mod=readonly", "./...")
	run(t, directory, filepath.Join(root, ".tools/bin/golangci-lint"), "run", "./...")
	run(t, directory, "go", "build", "-mod=readonly", "./...")
}

func verifyResourceRPCProcess(t *testing.T, directory, transport string) {
	t.Helper()
	clientSource := `package main
import (
 "context"; "crypto/tls"; "crypto/x509"; "errors"; "flag"; "fmt"; "net"; "os"; "time"
 orders "example.com/demo/pkg/api/orders/v1"; notes "example.com/demo/pkg/api/notes/v1"
 "google.golang.org/grpc"; "google.golang.org/grpc/credentials"; "google.golang.org/grpc/credentials/insecure"; "google.golang.org/grpc/metadata"; "google.golang.org/grpc/health/grpc_health_v1"; "google.golang.org/protobuf/encoding/protojson"; "google.golang.org/protobuf/proto"
)
func run() error {
 address:=flag.String("address","127.0.0.1:9090",""); ca:=flag.String("ca","",""); resource:=flag.String("resource","orders",""); action:=flag.String("action","list",""); id:=flag.Uint64("id",0,""); title:=flag.String("title","",""); enabled:=flag.Bool("enabled",false,""); amount:=flag.Int64("amount",0,""); page:=flag.Int("page",1,""); pageSize:=flag.Int("page-size",20,""); timeout:=flag.Duration("timeout",5*time.Second,""); unknown:=flag.Bool("unknown",false,""); flag.Parse(); provided:=map[string]bool{};flag.Visit(func(f *flag.Flag){provided[f.Name]=true})
 host,_,err:=net.SplitHostPort(*address);if err!=nil{return err};var transport credentials.TransportCredentials;if *ca!=""{roots:=x509.NewCertPool();data,err:=os.ReadFile(*ca);if err!=nil||!roots.AppendCertsFromPEM(data){return errors.New("invalid CA")};transport=credentials.NewTLS(&tls.Config{RootCAs:roots,ServerName:host,MinVersion:tls.VersionTLS12})}else{transport=insecure.NewCredentials()}
 connection,err:=grpc.NewClient(*address,grpc.WithTransportCredentials(transport),grpc.WithDisableRetry());if err!=nil{return err};defer func(){_ = connection.Close()}();ctx,cancel:=context.WithTimeout(context.Background(),*timeout);defer cancel();if token:=os.Getenv("APP_CLIENT_TOKEN");token!=""{ctx=metadata.AppendToOutgoingContext(ctx,"authorization","Bearer "+token)}
 if *action=="health"{value,err:=grpc_health_v1.NewHealthClient(connection).Check(ctx,&grpc_health_v1.HealthCheckRequest{});if err!=nil{return err};fmt.Printf("{\"status\":%q}",value.Status.String());return nil}
	var response proto.Message
 switch *resource {
 case "orders":
  client:=orders.NewOrdersServiceClient(connection)
  switch *action {
  case "create":request:=&orders.CreateOrdersRequest{};if provided["title"]{request.Title=title};if provided["enabled"]{request.Enabled=enabled};if provided["amount"]{request.Amount=amount};if *unknown{request.ProtoReflect().SetUnknown([]byte{0xa0,0x06,0x01})};response,err=client.Create(ctx,request)
  case "get":response,err=client.Get(ctx,&orders.GetOrdersRequest{Id:*id})
  case "list":request:=&orders.ListOrdersRequest{};if provided["page"]{v:=int32(*page);request.Page=&v};if provided["page-size"]{v:=int32(*pageSize);request.PageSize=&v};response,err=client.List(ctx,request)
  case "update":request:=&orders.UpdateOrdersRequest{Id:*id};if provided["title"]{request.Title=title};if provided["enabled"]{request.Enabled=enabled};if provided["amount"]{request.Amount=amount};response,err=client.Update(ctx,request)
  case "delete":response,err=client.Delete(ctx,&orders.DeleteOrdersRequest{Id:*id})
  default:return errors.New("unknown action")
  }
 case "notes":
  client:=notes.NewNotesServiceClient(connection)
  switch *action {
  case "create":request:=&notes.CreateNotesRequest{};if provided["title"]{request.Title=title};if provided["enabled"]{request.Enabled=enabled};if provided["amount"]{request.Amount=amount};response,err=client.Create(ctx,request)
  case "get":response,err=client.Get(ctx,&notes.GetNotesRequest{Id:*id})
  case "list":request:=&notes.ListNotesRequest{};if provided["page"]{v:=int32(*page);request.Page=&v};if provided["page-size"]{v:=int32(*pageSize);request.PageSize=&v};response,err=client.List(ctx,request)
  case "update":request:=&notes.UpdateNotesRequest{Id:*id};if provided["title"]{request.Title=title};if provided["enabled"]{request.Enabled=enabled};if provided["amount"]{request.Amount=amount};response,err=client.Update(ctx,request)
  case "delete":response,err=client.Delete(ctx,&notes.DeleteNotesRequest{Id:*id})
  default:return errors.New("unknown action")
  }
 default:return errors.New("unknown resource")
 }
 if err!=nil{return err};data,err:=protojson.MarshalOptions{UseProtoNames:true,EmitUnpopulated:true}.Marshal(response);if err!=nil{return err};fmt.Println(string(data));return nil
}

func main(){if err:=run();err!=nil{fmt.Fprintln(os.Stderr,err);os.Exit(1)}}
`
	clientPath := filepath.Join(directory, "cmd/resourceclient/main.go")
	if err := os.MkdirAll(filepath.Dir(clientPath), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(clientPath, []byte(clientSource), 0644); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(directory, "bin/apiserver")
	client := filepath.Join(directory, "bin/resourceclient")
	run(t, directory, "go", "build", "-mod=readonly", "-o", binary, "./cmd/apiserver")
	run(t, directory, "go", "build", "-mod=readonly", "-o", client, "./cmd/resourceclient")
	run(t, directory, binary, "dev", "init")
	run(t, directory, binary, "migrate", "up")
	alice := strings.TrimSpace(string(run(t, directory, binary, "dev", "token", "--subject", "alice")))
	bob := strings.TrimSpace(string(run(t, directory, binary, "dev", "token", "--subject", "bob")))
	certificate, key := grpcCertificate(t, directory)
	secure := []string{"APP_GRPC_TLS_CERT=" + certificate, "APP_GRPC_TLS_KEY=" + key}
	var process *serviceProcess
	var grpcAddress string
	clientOptions := []string{"--ca", certificate}
	if transport == "grpc" {
		process = startGRPC(t, directory, binary, client, secure, clientOptions)
		grpcAddress = process.url
	} else {
		clientHTTP := &http.Client{Timeout: 3 * time.Second}
		env := append(append([]string{}, secure...), "APP_GRPC_ADDRESS=127.0.0.1:0")
		if transport == "gateway" {
			env = append(env, "APP_GATEWAY_CA_FILE="+certificate, "APP_GATEWAY_SERVER_NAME=127.0.0.1")
		}
		process = startServiceWith(t, directory, binary, clientHTTP, "http", env)
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			data, err := os.ReadFile(process.log)
			if err != nil {
				t.Fatal(err)
			}
			for _, line := range strings.Split(string(data), "\n") {
				var entry struct {
					Message string `json:"msg"`
					Address string `json:"address"`
				}
				if json.Unmarshal([]byte(line), &entry) == nil && entry.Message == "gRPC server started" {
					grpcAddress = entry.Address
				}
			}
			if grpcAddress != "" {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		if grpcAddress == "" {
			t.Fatal("dual resource gRPC listener did not become ready")
		}
	}
	call := func(token string, args ...string) []byte {
		t.Helper()
		return rpcCall(t, directory, client, grpcAddress, token, nil, clientOptions, args, "")
	}
	rpcCall(t, directory, client, grpcAddress, alice, nil, clientOptions, []string{"--resource", "orders", "--action", "create"}, "InvalidArgument")
	rpcCall(t, directory, client, grpcAddress, alice, nil, clientOptions, []string{"--resource", "orders", "--action", "create", "--title", "one", "--unknown"}, "InvalidArgument")
	rpcCall(t, directory, client, grpcAddress, alice, nil, clientOptions, []string{"--resource", "orders", "--action", "get", "--id", "0"}, "InvalidArgument")
	rpcCall(t, directory, client, grpcAddress, alice, nil, clientOptions, []string{"--resource", "orders", "--action", "list", "--page", "0"}, "InvalidArgument")
	rpcCall(t, directory, client, grpcAddress, alice, nil, clientOptions, []string{"--resource", "orders", "--action", "list", "--timeout", "1ns"}, "DeadlineExceeded")
	created := call(alice, "--resource", "orders", "--action", "create", "--title", "one", "--enabled=true", "--amount=4")
	var item struct {
		ID      string `json:"id"`
		Enabled bool   `json:"enabled"`
		Amount  string `json:"amount"`
	}
	if err := json.Unmarshal(created, &item); err != nil || item.ID == "" || !item.Enabled || item.Amount != "4" {
		t.Fatalf("RPC create: %s (%v)", created, err)
	}
	call(alice, "--resource", "notes", "--action", "create", "--title", "two")
	rpcCall(t, directory, client, grpcAddress, bob, nil, clientOptions, []string{"--resource", "orders", "--action", "get", "--id", item.ID}, "NotFound")
	updated := call(alice, "--resource", "orders", "--action", "update", "--id", item.ID, "--enabled=false", "--amount=0")
	if !strings.Contains(string(updated), `"enabled":false`) || !strings.Contains(string(updated), `"amount":"0"`) {
		t.Fatalf("RPC PATCH lost explicit zero values: %s", updated)
	}
	if transport == "dual" || transport == "gateway" {
		if transport == "gateway" {
			for _, check := range []struct {
				token string
				path  string
				want  int
			}{{bob, "/v1/orders/" + item.ID, http.StatusNotFound}, {"", "/v1/orders", http.StatusUnauthorized}, {alice, "/v1/orders?unexpected=x", http.StatusBadRequest}} {
				request, err := http.NewRequest(http.MethodGet, process.url+check.path, nil)
				if err != nil {
					t.Fatal(err)
				}
				if check.token != "" {
					request.Header.Set("Authorization", "Bearer "+check.token)
				}
				response, err := http.DefaultClient.Do(request)
				if err != nil {
					t.Fatal(err)
				}
				_ = response.Body.Close()
				if response.StatusCode != check.want {
					t.Fatalf("Gateway GET %s status=%d want=%d", check.path, response.StatusCode, check.want)
				}
			}
		}
		request, err := http.NewRequest(http.MethodGet, process.url+"/v1/orders/"+item.ID, nil)
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Authorization", "Bearer "+alice)
		if transport == "gateway" {
			request.Header.Set("X-Request-ID", "resource-gateway-1")
		}
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		body, readErr := io.ReadAll(response.Body)
		_ = response.Body.Close()
		if transport == "gateway" && response.Header.Get("X-Request-ID") != "resource-gateway-1" {
			t.Fatalf("Gateway did not propagate request ID: %v", response.Header)
		}
		wireAmount := `"amount":0`
		if transport == "gateway" {
			wireAmount = `"amount":"0"`
		}
		if readErr != nil || response.StatusCode != http.StatusOK || !strings.Contains(string(body), `"enabled":false`) || !strings.Contains(string(body), wireAmount) {
			t.Fatalf("HTTP could not read RPC-created resource: status=%d body=%s error=%v", response.StatusCode, body, readErr)
		}
		patchBody := `{"enabled":true,"amount":8}`
		if transport == "gateway" {
			patchBody = `{"enabled":true,"amount":"8"}`
		}
		request, err = http.NewRequest(http.MethodPatch, process.url+"/v1/orders/"+item.ID, strings.NewReader(patchBody))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Authorization", "Bearer "+alice)
		request.Header.Set("Content-Type", "application/json")
		response, err = http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		body, readErr = io.ReadAll(response.Body)
		_ = response.Body.Close()
		if readErr != nil || response.StatusCode != http.StatusOK {
			t.Fatalf("HTTP update failed: status=%d body=%s error=%v", response.StatusCode, body, readErr)
		}
		readBack := call(alice, "--resource", "orders", "--action", "get", "--id", item.ID)
		if !strings.Contains(string(readBack), `"enabled":true`) || !strings.Contains(string(readBack), `"amount":"8"`) {
			t.Fatalf("RPC could not read HTTP update: %s", readBack)
		}
	}
}
func verifyResourceProtoReproduction(t *testing.T, directory string) {
	t.Helper()
	paths := []string{
		"pkg/api/example/v1/example.pb.go", "pkg/api/example/v1/example_grpc.pb.go",
		"pkg/api/orders/v1/orders.pb.go", "pkg/api/orders/v1/orders_grpc.pb.go",
		"pkg/api/notes/v1/notes.pb.go", "pkg/api/notes/v1/notes_grpc.pb.go",
	}
	if _, err := os.Stat(filepath.Join(directory, "pkg/api/example/v1/example.pb.gw.go")); err == nil {
		paths = append(paths, "pkg/api/example/v1/example.pb.gw.go", "pkg/api/orders/v1/orders.pb.gw.go", "pkg/api/notes/v1/notes.pb.gw.go")
		paths = append(paths, "api/openapi/example.yaml", "api/openapi/orders.yaml", "api/openapi/notes.yaml", "api/openapi/swagger.json", "internal/apiserver/bootstrap/modules_gen.go")
	}
	original := make(map[string][]byte, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(filepath.Join(directory, path))
		if err != nil {
			t.Fatal(err)
		}
		original[path] = data
	}
	run(t, directory, "make", "proto-generate")
	if _, err := os.Stat(filepath.Join(directory, "pkg/api/example/v1/example.pb.gw.go")); err == nil {
		swagger, err := os.ReadFile(filepath.Join(directory, "api/openapi/swagger.json"))
		if err != nil {
			t.Fatal(err)
		}
		var document struct {
			Paths map[string]json.RawMessage `json:"paths"`
		}
		if err := json.Unmarshal(swagger, &document); err != nil {
			t.Fatal(err)
		}
		for _, path := range []string{"/v1/examples", "/v1/orders", "/v1/notes"} {
			if _, ok := document.Paths[path]; !ok {
				t.Fatalf("aggregated Gateway Swagger lost path %s", path)
			}
		}
		digest := sha256.Sum256(swagger)
		registry, err := os.ReadFile(filepath.Join(directory, "internal/apiserver/bootstrap/modules_gen.go"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(registry), `const openAPISHA256 = "`+hex.EncodeToString(digest[:])+`"`) {
			t.Fatal("Gateway OpenAPI digest does not match aggregate")
		}
	}
	for _, path := range paths {
		data, err := os.ReadFile(filepath.Join(directory, path))
		if err != nil || string(data) != string(original[path]) {
			t.Fatalf("multi-module Proto reproduction differs: %s (%v)", path, err)
		}
	}
	consumer := filepath.Join(directory, ".tmp/resource-client-import")
	if err := os.MkdirAll(consumer, 0750); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"go.mod", "go.sum"} {
		data, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil {
			t.Fatal(err)
		}
		if name == "go.mod" {
			data = []byte(strings.Replace(string(data), "module example.com/demo", "module example.com/resource-consumer", 1) + "\nrequire example.com/demo v0.0.0\nreplace example.com/demo => ../..\n")
		}
		if err := os.WriteFile(filepath.Join(consumer, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	client := "package consumer\nimport (orders \"example.com/demo/pkg/api/orders/v1\"; notes \"example.com/demo/pkg/api/notes/v1\")\nvar OrdersClient = orders.NewOrdersServiceClient\nvar NotesClient = notes.NewNotesServiceClient\n"
	if err := os.WriteFile(filepath.Join(consumer, "client.go"), []byte(client), 0600); err != nil {
		t.Fatal(err)
	}
	run(t, consumer, "go", "test", "-mod=readonly", "./...")
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
