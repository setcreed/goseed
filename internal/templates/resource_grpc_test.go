package templates

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestGatewayOpenAPIMergerMatchesGenerator(t *testing.T) {
	root, err := os.ReadFile("../openapi/merge.go")
	if err != nil {
		t.Fatal(err)
	}
	generated, err := sources.ReadFile("variants/gateway/common/scripts/openapi/merger/merge.go.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(string(root), "package openapi", "package merger", 1)
	if string(generated) != want {
		t.Fatal("generated-project OpenAPI merger has drifted from generator validation")
	}
}

func TestRenderResourceGRPCAndDualFactory(t *testing.T) {
	project := Project{Name: "fixture", Module: "example.com/fixture"}
	fields := []ResourceField{
		{SchemaField: SchemaField{Name: "title", Type: "string", MaxLength: 80}, Required: true},
		{SchemaField: SchemaField{Name: "enabled", Type: "bool"}},
		{SchemaField: SchemaField{Name: "count", Type: "int64"}},
	}

	grpcFiles, err := RenderResourceGRPC(project, Options{Database: "sqlite", Transport: "grpc"}, "task", fields)
	if err != nil {
		t.Fatal(err)
	}
	var proto, handler, factory []byte
	for _, file := range grpcFiles {
		switch file.Path {
		case "api/proto/task/v1/task.proto":
			proto = file.Content
		case "internal/apiserver/handler/grpc/task/handler.go":
			handler = file.Content
		case "internal/apiserver/bootstrap/module_task.go":
			factory = file.Content
		}
	}
	for name, content := range map[string][]byte{"proto": proto, "handler": handler, "factory": factory} {
		if len(content) == 0 {
			t.Fatalf("missing %s output", name)
		}
	}
	for _, want := range []string{"optional string title = 1;", "optional bool enabled = 2;", "optional int64 count = 3;", "optional string title = 2;"} {
		if !bytes.Contains(proto, []byte(want)) {
			t.Errorf("Proto is missing %q", want)
		}
	}
	gatewayFiles, err := RenderResourceGRPC(project, Options{Database: "sqlite", Transport: "gateway", Authz: "casbin"}, "task", fields)
	if err != nil {
		t.Fatal(err)
	}
	var gatewayFactory, httpMapping, openAPIConfig, guide []byte
	for _, file := range gatewayFiles {
		switch file.Path {
		case "internal/apiserver/bootstrap/module_task.go":
			gatewayFactory = file.Content
		case "api/proto/task/v1/http.yaml":
			httpMapping = file.Content
		case "api/proto/task/v1/openapi.yaml":
			openAPIConfig = file.Content
		case "docs/task.md":
			guide = file.Content
		}
	}
	for label, content := range map[string][]byte{"Gateway factory": gatewayFactory, "HTTP mapping": httpMapping, "OpenAPI config": openAPIConfig, "module guide": guide} {
		if len(content) == 0 {
			t.Fatalf("missing %s", label)
		}
	}
	for _, want := range []string{"api.RegisterTaskServiceHandler", "\"page\", \"page_size\"", "\"POST\": true", "\"PATCH\": true"} {
		if !bytes.Contains(gatewayFactory, []byte(want)) {
			t.Errorf("Gateway factory missing %q:\n%s", want, gatewayFactory)
		}
	}
	for _, want := range []string{"/v1/task", "/v1/task/{id}", "task.v1.TaskService.Create", "task.v1.TaskService.Update"} {
		if !bytes.Contains(httpMapping, []byte(want)) {
			t.Errorf("HTTP mapping missing %q:\n%s", want, httpMapping)
		}
	}
	if !bytes.Contains(guide, []byte("snake_case")) || !bytes.Contains(guide, []byte("64 位整数")) {
		t.Errorf("Gateway guide must describe ProtoJSON wire differences: %s", guide)
	}
	for _, want := range []string{"request.Title == nil", "Title:   request.Title", "api.TaskService_Create_FullMethodName", "api.TaskService_Delete_FullMethodName"} {
		if !bytes.Contains(handler, []byte(want)) {
			t.Errorf("RPC handler is missing %q:\n%s", want, handler)
		}
	}
	if strings.Count(string(factory), "business.New(") != 1 || strings.Count(string(factory), "store.New(") != 1 {
		t.Fatalf("gRPC factory must construct one Store and one Biz:\n%s", factory)
	}

	httpFiles, err := RenderResourceHTTP(project, Options{Database: "sqlite", Transport: "dual"}, "task", fields)
	if err != nil {
		t.Fatal(err)
	}
	dualRPCFiles, err := RenderResourceGRPC(project, Options{Database: "sqlite", Transport: "dual"}, "task", fields)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	var dualFactory []byte
	for _, file := range append(httpFiles, dualRPCFiles...) {
		if file.Path == "internal/apiserver/bootstrap/module_task.go" {
			count++
			dualFactory = file.Content
		}
	}
	if count != 1 {
		t.Fatalf("dual render produced %d module factories, want 1", count)
	}
	if strings.Count(string(dualFactory), "business.New(") != 1 || strings.Count(string(dualFactory), "httpHandler.New(businessService)") != 1 || strings.Count(string(dualFactory), "grpcHandler.New(businessService)") != 1 {
		t.Fatalf("dual factory must share one Biz across HTTP and RPC:\n%s", dualFactory)
	}
	if !strings.Contains(string(dualFactory), "RPC: map[string]string") || !strings.Contains(string(dualFactory), "HTTP: map[string]string") {
		t.Fatalf("dual factory is missing protocol permission descriptions:\n%s", dualFactory)
	}
}
