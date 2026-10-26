package templates

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"text/template"
)

type ResourceField struct {
	SchemaField
	Required bool
}

type resourceCoreField struct {
	ResourceField
	GoName, ProtoCreateName, ProtoUpdateName, ProtoItemName string
}

type resourceData struct {
	Project
	Options
	Name, GoName                   string
	Fields                         []resourceCoreField
	ProtoCreatedAt, ProtoUpdatedAt string
}

// RenderResourceCore is protocol-independent and only returns source files.
func RenderResourceCore(project Project, options Options, name string, fields []ResourceField) ([]File, error) {
	if options.Database == "" {
		options.Database = "sqlite"
	}
	schema := make([]SchemaField, len(fields))
	mapped := make([]resourceCoreField, len(fields))
	seen := map[string]bool{"ID": true, "Owner": true, "CreatedAt": true, "UpdatedAt": true, "TableName": true}
	hasString := false
	for i, field := range fields {
		schema[i] = field.SchemaField
		parts := strings.Split(field.Name, "_")
		var goName strings.Builder
		for _, part := range parts {
			if part == "" {
				return nil, fmt.Errorf("invalid resource field name")
			}
			goName.WriteString(strings.ToUpper(part[:1]) + part[1:])
		}
		if seen[goName.String()] {
			return nil, fmt.Errorf("resource field Go name %q is reserved or duplicated", goName.String())
		}
		seen[goName.String()] = true
		mapped[i] = resourceCoreField{ResourceField: field, GoName: goName.String()}
		hasString = hasString || field.Type == "string"
	}
	if _, _, err := RenderResourceSchema(project, options.Database, name, schema); err != nil {
		return nil, err
	}
	if !namePattern.MatchString(name) {
		return nil, fmt.Errorf("invalid resource name")
	}
	data := struct {
		Project
		Options
		Name, GoName string
		Fields       []resourceCoreField
		HasString    bool
	}{project, options, name, strings.ToUpper(name[:1]) + name[1:], mapped, hasString}
	paths := map[string]string{
		"model.go":      "internal/apiserver/model/" + name + ".go",
		"model_test.go": "internal/apiserver/model/" + name + "_test.go",
		"biz.go":        "internal/apiserver/biz/" + name + "/biz.go",
		"biz_test.go":   "internal/apiserver/biz/" + name + "/biz_test.go",
		"store.go":      "internal/apiserver/store/" + name + "/store.go",
		"store_test.go": "internal/apiserver/store/" + name + "/store_test.go",
	}
	files := make([]File, 0, len(paths))
	for filename, path := range paths {
		content, err := sources.ReadFile("resource/core/" + filename + ".tmpl")
		if err != nil {
			return nil, err
		}
		parsed, err := template.New(filename).Delims("[[", "]]").Option("missingkey=error").Parse(string(content))
		if err != nil {
			return nil, err
		}
		var output bytes.Buffer
		if err := parsed.Execute(&output, data); err != nil {
			return nil, err
		}
		formatted, err := formatGo(output.String())
		if err != nil {
			return nil, fmt.Errorf("resource %s: %w", filename, err)
		}
		files = append(files, File{Path: path, Content: formatted})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

// RenderResourceHTTP renders the HTTP adapter, developer-owned assembly factory,
// Swagger source, and module guide for an SQL-backed HTTP-capable project.
func RenderResourceHTTP(project Project, options Options, name string, fields []ResourceField) ([]File, error) {
	if options.Transport != "http" && options.Transport != "dual" {
		return nil, fmt.Errorf("HTTP resource rendering requires http or dual transport")
	}
	if _, err := RenderResourceCore(project, options, name, fields); err != nil {
		return nil, err
	}
	data := resourceData{Project: project, Options: options, Name: name, GoName: strings.ToUpper(name[:1]) + name[1:]}
	for _, field := range fields {
		parts := strings.Split(field.Name, "_")
		var goName strings.Builder
		for _, part := range parts {
			goName.WriteString(strings.ToUpper(part[:1]) + part[1:])
		}
		data.Fields = append(data.Fields, resourceCoreField{ResourceField: field, GoName: goName.String()})
	}
	handlerSource, err := sources.ReadFile("resource/http.go.tmpl")
	if err != nil {
		return nil, err
	}
	parsed, err := template.New("http.go").Delims("[[", "]]").Option("missingkey=error").Parse(string(handlerSource))
	if err != nil {
		return nil, err
	}
	var output bytes.Buffer
	if err := parsed.Execute(&output, data); err != nil {
		return nil, err
	}
	formatted, err := formatGo(output.String())
	if err != nil {
		return nil, fmt.Errorf("resource HTTP handler: %w", err)
	}
	files := []File{{Path: "internal/apiserver/handler/http/" + name + "/handler.go", Content: formatted}}
	testSource, err := sources.ReadFile("resource/http_test.go.tmpl")
	if err != nil {
		return nil, err
	}
	testTemplate, err := template.New("http_test.go").Delims("[[", "]]").Option("missingkey=error").Parse(string(testSource))
	if err != nil {
		return nil, err
	}
	output.Reset()
	if err := testTemplate.Execute(&output, data); err != nil {
		return nil, err
	}
	testFormatted, err := formatGo(output.String())
	if err != nil {
		return nil, fmt.Errorf("resource HTTP tests: %w", err)
	}
	files = append(files, File{Path: "internal/apiserver/handler/http/" + name + "/handler_test.go", Content: testFormatted})
	factory, err := renderResourceModuleFactory(project, options, data)
	if err != nil {
		return nil, err
	}
	files = append(files, factory, resourceHTTPGuide(data), resourceOpenAPI(data))
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

// RenderResourceGRPC renders the Proto contract, RPC Handler and tests. For
// dual projects the matching HTTP renderer owns the single shared module factory.
func RenderResourceGRPC(project Project, options Options, name string, fields []ResourceField) ([]File, error) {
	if options.Transport != "grpc" && options.Transport != "dual" && options.Transport != "gateway" {
		return nil, fmt.Errorf("RPC resource rendering requires grpc, dual, or gateway transport")
	}
	if _, err := RenderResourceCore(project, options, name, fields); err != nil {
		return nil, err
	}
	data := resourceData{Project: project, Options: options, Name: name, GoName: strings.ToUpper(name[:1]) + name[1:]}
	createNames := make([]string, 0, len(fields))
	for _, field := range fields {
		createNames = append(createNames, field.Name)
	}
	updateNames := append([]string{"id"}, createNames...)
	itemNames := append(append([]string{"id"}, createNames...), "created_at", "updated_at")
	createGo := protoFieldNames(createNames, 0, len(createNames))
	updateGo := protoFieldNames(updateNames, 1, len(updateNames))
	itemGo := protoFieldNames(itemNames, 0, 0)
	data.ProtoCreatedAt, data.ProtoUpdatedAt = itemGo[len(itemGo)-2], itemGo[len(itemGo)-1]
	for i, field := range fields {
		parts := strings.Split(field.Name, "_")
		var goName strings.Builder
		for _, part := range parts {
			if part == "" {
				return nil, fmt.Errorf("invalid resource field name")
			}
			goName.WriteString(strings.ToUpper(part[:1]) + part[1:])
		}
		data.Fields = append(data.Fields, resourceCoreField{ResourceField: field, GoName: goName.String(), ProtoCreateName: createGo[i], ProtoUpdateName: updateGo[i+1], ProtoItemName: itemGo[i+1]})
	}
	files := make([]File, 0, 8)
	for _, spec := range []struct {
		name string
		path string
	}{
		{name: "grpc.go", path: "internal/apiserver/handler/grpc/" + name + "/handler.go"},
		{name: "grpc_test.go", path: "internal/apiserver/handler/grpc/" + name + "/handler_test.go"},
	} {
		content, err := sources.ReadFile("resource/" + spec.name + ".tmpl")
		if err != nil {
			return nil, err
		}
		parsed, err := template.New(spec.name).Delims("[[", "]]").Option("missingkey=error").Parse(string(content))
		if err != nil {
			return nil, err
		}
		var output bytes.Buffer
		if err := parsed.Execute(&output, data); err != nil {
			return nil, err
		}
		formatted, err := formatGo(output.String())
		if err != nil {
			return nil, fmt.Errorf("resource RPC %s: %w", spec.name, err)
		}
		files = append(files, File{Path: spec.path, Content: formatted})
	}
	protoSource, err := sources.ReadFile("resource/grpc.proto.tmpl")
	if err != nil {
		return nil, err
	}
	protoTemplate, err := template.New("grpc.proto").Delims("[[", "]]").Funcs(template.FuncMap{
		"add": func(left, right int) int { return left + right },
	}).Option("missingkey=error").Parse(string(protoSource))
	if err != nil {
		return nil, err
	}
	var protoOutput bytes.Buffer
	if err := protoTemplate.Execute(&protoOutput, data); err != nil {
		return nil, err
	}
	files = append(files, File{Path: "api/proto/" + name + "/v1/" + name + ".proto", Content: protoOutput.Bytes()})
	if options.Transport == "grpc" || options.Transport == "gateway" {
		factory, err := renderResourceModuleFactory(project, options, data)
		if err != nil {
			return nil, err
		}
		files = append(files, factory)
	}
	if options.Transport == "gateway" {
		for _, spec := range []struct{ source, path string }{
			{"resource/gateway_http.yaml.tmpl", "api/proto/" + name + "/v1/http.yaml"},
			{"resource/gateway_openapi.yaml.tmpl", "api/proto/" + name + "/v1/openapi.yaml"},
		} {
			content, err := sources.ReadFile(spec.source)
			if err != nil {
				return nil, err
			}
			parsed, err := template.New(spec.path).Delims("[[", "]]").Option("missingkey=error").Parse(string(content))
			if err != nil {
				return nil, err
			}
			var output bytes.Buffer
			if err := parsed.Execute(&output, data); err != nil {
				return nil, err
			}
			files = append(files, File{Path: spec.path, Content: output.Bytes()})
		}
		files = append(files, resourceGatewayGuide(data))
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

// RenderResourceHTTPIntegrationTest builds the two-resource SQLite request
// stack fixture used only by generator integration tests.
func RenderResourceHTTPIntegrationTest(project Project, options Options, fields []ResourceField) (File, error) {
	data := struct {
		Project
		Options
		Fields               []resourceCoreField
		CreateJSON, ZeroJSON string
	}{Project: project, Options: options}
	for _, field := range fields {
		parts := strings.Split(field.Name, "_")
		var goName strings.Builder
		for _, part := range parts {
			goName.WriteString(strings.ToUpper(part[:1]) + part[1:])
		}
		data.Fields = append(data.Fields, resourceCoreField{ResourceField: field, GoName: goName.String()})
	}
	if len(data.Fields) == 0 {
		return File{}, fmt.Errorf("resource HTTP integration fixture requires fields")
	}
	var createJSON strings.Builder
	createJSON.WriteByte('{')
	for i, field := range data.Fields {
		if i > 0 {
			createJSON.WriteByte(',')
		}
		fmt.Fprintf(&createJSON, "%q:", field.Name)
		switch field.Type {
		case "string":
			createJSON.WriteString(`"x"`)
		case "bool":
			createJSON.WriteString("true")
		default:
			createJSON.WriteString("1")
		}
	}
	createJSON.WriteByte('}')
	data.CreateJSON = createJSON.String()
	first := data.Fields[0]
	zeroValue := "0"
	switch first.Type {
	case "string":
		zeroValue = `""`
	case "bool":
		zeroValue = "false"
	}
	data.ZeroJSON = fmt.Sprintf("{%q:%s}", first.Name, zeroValue)
	source, err := sources.ReadFile("resource/http_integration_test.go.tmpl")
	if err != nil {
		return File{}, err
	}
	parsed, err := template.New("resource_http_test.go").Delims("[[", "]]").Option("missingkey=error").Parse(string(source))
	if err != nil {
		return File{}, err
	}
	var output bytes.Buffer
	if err := parsed.Execute(&output, data); err != nil {
		return File{}, err
	}
	formatted, err := formatGo(output.String())
	if err != nil {
		return File{}, fmt.Errorf("resource HTTP integration fixture: %w", err)
	}
	return File{Path: "internal/apiserver/resource_http_test.go", Content: formatted}, nil
}

func renderResourceModuleFactory(project Project, options Options, data resourceData) (File, error) {
	hasHTTP := options.Transport == "http" || options.Transport == "dual"
	hasRPC := options.Transport == "grpc" || options.Transport == "dual" || options.Transport == "gateway"
	var source strings.Builder
	source.WriteString("// Module assembly is developer-owned; goseed does not overwrite this file.\npackage bootstrap\n\nimport (\n")
	fmt.Fprintf(&source, "business %q\nstore %q\n", project.Module+"/internal/apiserver/biz/"+data.Name, project.Module+"/internal/apiserver/store/"+data.Name)
	if hasHTTP {
		fmt.Fprintf(&source, "httpHandler %q\n", project.Module+"/internal/apiserver/handler/http/"+data.Name)
	}
	if hasRPC {
		fmt.Fprintf(&source, "grpcHandler %q\napi %q\n", project.Module+"/internal/apiserver/handler/grpc/"+data.Name, project.Module+"/pkg/api/"+data.Name+"/v1")
	}
	if options.Transport == "gateway" {
		fmt.Fprintf(&source, "\ngateway %q\n", project.Module+"/internal/pkg/gateway")
	}
	source.WriteString(")\n\n")
	fmt.Fprintf(&source, "type %sModule struct {", data.Name)
	if hasHTTP {
		source.WriteString(" HTTP *httpHandler.Handler; Operations map[string]string;")
	}
	if hasRPC {
		source.WriteString(" RPC *grpcHandler.Handler; Permissions map[string]string;")
	}
	if options.Transport == "gateway" {
		source.WriteString(" Gateway gateway.Registration;")
	}
	source.WriteString(" }\n")
	fmt.Fprintf(&source, "func new%sModule(dependencies moduleDependencies) (%sModule, error) {\n", data.GoName, data.Name)
	fmt.Fprintf(&source, "repository := store.New(dependencies.Database)\nif err := repository.CheckSchema(dependencies.Context); err != nil { return %sModule{}, err }\n", data.Name)
	if options.Redis {
		source.WriteString("var dependency business.Dependency = dependencies.Cache\n")
	}
	fmt.Fprintf(&source, "businessService := business.New(repository")
	if options.Redis {
		source.WriteString(", dependency")
	}
	source.WriteString(")\n")
	fmt.Fprintf(&source, "module := %sModule{", data.Name)
	if hasHTTP {
		source.WriteString("HTTP:httpHandler.New(businessService), Operations:")
		fmt.Fprintf(&source, "%sPermissions().HTTP,", data.Name)
	}
	if hasRPC {
		source.WriteString("RPC:grpcHandler.New(businessService), Permissions:")
		fmt.Fprintf(&source, "%sPermissions().RPC,", data.Name)
	}
	if options.Transport == "gateway" {
		fmt.Fprintf(&source, "Gateway:gateway.Registration{Queries:map[string][]string{\"GET\":{\"page\",\"page_size\"}},BodyMethods:map[string]bool{\"POST\":true,\"PATCH\":true},Register:api.Register%sServiceHandler},", data.GoName)
	}
	source.WriteString("}\nreturn module, nil\n}\n")
	fmt.Fprintf(&source, "func %sPermissions() modulePermissions { return modulePermissions{", data.Name)
	if hasHTTP {
		source.WriteString("HTTP:map[string]string{\"GET\":\"read\",\"POST\":\"create\",\"PATCH\":\"update\",\"DELETE\":\"delete\"},")
	}
	if hasRPC {
		fmt.Fprintf(&source, "RPC:map[string]string{api.%sService_Create_FullMethodName:\"create\",api.%sService_Get_FullMethodName:\"read\",api.%sService_List_FullMethodName:\"read\",api.%sService_Update_FullMethodName:\"update\",api.%sService_Delete_FullMethodName:\"delete\"},", data.GoName, data.GoName, data.GoName, data.GoName, data.GoName)
	}
	source.WriteString("} }\n")
	formatted, err := formatGo(source.String())
	if err != nil {
		return File{}, err
	}
	return File{Path: "internal/apiserver/bootstrap/module_" + data.Name + ".go", Content: formatted}, nil
}

func resourceHTTPGuide(data resourceData) File {
	var body strings.Builder
	fmt.Fprintf(&body, "# %s\n\nCRUD 接口为 `/v1/%s` 与 `/v1/%s/{id}`。所有接口要求 JWT；启用 Casbin 时按 `create`、`read`、`update`、`delete` 操作授权。资源归属从认证主体取得，响应不包含 owner。\n\n", data.Name, data.Name, data.Name)
	body.WriteString("| 方法 | 路径 | 行为 | 成功状态 |\n|---|---|---|---|\n| POST | `/v1/" + data.Name + "` | 创建 | 201 |\n| GET | `/v1/" + data.Name + "` | 分页列表 | 200 |\n| GET | `/v1/" + data.Name + "/{id}` | 读取主体自己的记录 | 200 |\n| PATCH | `/v1/" + data.Name + "/{id}` | 部分更新 | 200 |\n| DELETE | `/v1/" + data.Name + "/{id}` | 删除主体自己的记录 | 204 |\n\n创建和 PATCH 接受以下业务字段；字段缺省表示未提供，JSON `null`、重复键、未知键与 owner 均拒绝。分页参数为 `page`（默认 1）与 `page_size`（默认 20），范围分别为 1 至 100000 和 1 至 100。\n\n| 字段 | 类型 | 创建必需 | 约束 |\n|---|---|---|---|---\n")
	for _, field := range data.Fields {
		required := "否"
		if field.Required {
			required = "是"
		}
		constraint := ""
		if field.Type == "string" {
			constraint = fmt.Sprintf("最多 %d 个 Unicode 字符；不自动去空白", field.MaxLength)
		}
		fmt.Fprintf(&body, "| `%s` | `%s` | %s | %s |\n", field.Name, field.Type, required, constraint)
	}
	body.WriteString("\n示例：\n\n```sh\n: \"${TOKEN:?set TOKEN first}\"\n: \"${API_URL:=http://127.0.0.1:8080}\"\ncurl --fail --silent --show-error -X POST \"${API_URL}/v1/" + data.Name + "\" \\\n  -H \"Authorization: Bearer ${TOKEN}\" -H 'Content-Type: application/json' \\\n  -d '{")
	for i, field := range data.Fields {
		if i > 0 {
			body.WriteString(",")
		}
		switch field.Type {
		case "string":
			value := strings.Repeat("x", min(field.MaxLength, 8))
			fmt.Fprintf(&body, `"%s":"%s"`, field.Name, value)
		case "bool":
			fmt.Fprintf(&body, `"%s":true`, field.Name)
		default:
			fmt.Fprintf(&body, `"%s":1`, field.Name)
		}
	}
	body.WriteString("}'\n```\n\n数据库 schema 必须先通过项目显式迁移建立；服务启动只读核验 schema，不自动迁移。")
	return File{Path: "docs/" + data.Name + ".md", Content: []byte(body.String())}
}

func resourceGatewayGuide(data resourceData) File {
	var body strings.Builder
	fmt.Fprintf(&body, "# %s Gateway\n\nGateway 将 HTTP 请求映射到 `%s.v1.%sService`；认证、授权和业务校验仍由 RPC 链执行。路径为 `/v1/%s` 与 `/v1/%s/{id}`，所有业务路由要求 JWT。", data.Name, data.Name, data.GoName, data.Name, data.Name)
	body.WriteString("\n\nProtoJSON 使用 snake_case 字段名，`int64` 在 JSON 中以十进制字符串表示；创建和更新字段保留 optional 存在性。GET 集合允许 `page`、`page_size`，其余 query 参数拒绝。POST/PATCH 仅接受 JSON，拒绝 null、重复/未知字段和空更新。\n\n| HTTP | RPC | 路径 |\n|---|---|---|\n")
	for _, row := range []struct{ method, rpc, path string }{{"POST", "Create", "/v1/" + data.Name}, {"GET", "List", "/v1/" + data.Name}, {"GET", "Get", "/v1/" + data.Name + "/{id}"}, {"PATCH", "Update", "/v1/" + data.Name + "/{id}"}, {"DELETE", "Delete", "/v1/" + data.Name + "/{id}"}} {
		fmt.Fprintf(&body, "| %s | `%sService/%s` | `%s` |\n", row.method, data.GoName, row.rpc, row.path)
	}
	body.WriteString("\n| 字段 | 类型 | 创建必需 | 约束 |\n|---|---|---|---|\n")
	for _, field := range data.Fields {
		required := "否"
		if field.Required {
			required = "是"
		}
		constraint := ""
		if field.Type == "string" {
			constraint = fmt.Sprintf("最多 %d 个 Unicode 字符", field.MaxLength)
		}
		fmt.Fprintf(&body, "| `%s` | `%s` | %s | %s |\n", field.Name, field.Type, required, constraint)
	}
	body.WriteString("\nHTTP 成功响应使用 ProtoJSON；64 位整数请按字符串解析。RPC 客户端使用相同的字段存在性和主体隔离规则。")
	return File{Path: "docs/" + data.Name + ".md", Content: []byte(body.String())}
}

func resourceOpenAPI(data resourceData) File {
	var body strings.Builder
	fmt.Fprintf(&body, "swagger: \"2.0\"\npaths:\n  /v1/%s:\n", data.Name)
	fmt.Fprintf(&body, "    post:\n      operationId: %sCreate\n      security:\n        - Bearer: []\n      parameters:\n        - {name: body, in: body, required: true, schema: {$ref: '#/definitions/Create%sRequest'}}\n      responses:\n        '201': {description: Created, schema: {$ref: '#/definitions/%sItem'}}\n        '400': {description: Invalid input}\n        '401': {description: Unauthorized}\n", data.Name, data.GoName, data.GoName)
	body.WriteString("    get:\n      operationId: " + data.Name + "List\n      security:\n        - Bearer: []\n      parameters:\n        - {name: page, in: query, type: integer, default: 1, minimum: 1, maximum: 100000}\n        - {name: page_size, in: query, type: integer, default: 20, minimum: 1, maximum: 100}\n")
	fmt.Fprintf(&body, "      responses:\n        '200': {description: Page, schema: {$ref: '#/definitions/%sPage'}}\n        '400': {description: Invalid pagination}\n", data.GoName)
	fmt.Fprintf(&body, "  /v1/%s/{id}:\n", data.Name)
	fmt.Fprintf(&body, "    parameters:\n      - {name: id, in: path, required: true, type: integer, format: int64, minimum: 1, maximum: 9223372036854775807}\n    get:\n      operationId: %sGet\n      security:\n        - Bearer: []\n      responses:\n        '200': {description: Resource, schema: {$ref: '#/definitions/%sItem'}}\n        '404': {description: Not found}\n    patch:\n      operationId: %sUpdate\n      security:\n        - Bearer: []\n      parameters:\n        - {name: body, in: body, required: true, schema: {$ref: '#/definitions/Update%sRequest'}}\n      responses:\n        '200': {description: Updated, schema: {$ref: '#/definitions/%sItem'}}\n        '400': {description: Invalid input}\n        '404': {description: Not found}\n    delete:\n      operationId: %sDelete\n      security:\n        - Bearer: []\n      responses:\n        '204': {description: Deleted}\n        '404': {description: Not found}\ndefinitions:\n", data.Name, data.GoName, data.Name, data.GoName, data.GoName, data.Name)
	for _, definition := range []struct {
		name     string
		required bool
	}{{"Create" + data.GoName + "Request", true}, {"Update" + data.GoName + "Request", false}} {
		fmt.Fprintf(&body, "  %s:\n    type: object\n", definition.name)
		if !definition.required {
			body.WriteString("    minProperties: 1\n")
		}
		body.WriteString("    additionalProperties: false\n")
		if definition.required {
			var required []string
			for _, field := range data.Fields {
				if field.Required {
					required = append(required, field.Name)
				}
			}
			if len(required) > 0 {
				body.WriteString("    required:\n")
				for _, field := range required {
					fmt.Fprintf(&body, "      - %s\n", field)
				}
			}
		}
		body.WriteString("    properties:\n")
		for _, field := range data.Fields {
			fmt.Fprintf(&body, "      %s: %s\n", field.Name, resourceOpenAPIField(field))
		}
	}
	fmt.Fprintf(&body, "  %sItem:\n    type: object\n    required: [id", data.GoName)
	for _, field := range data.Fields {
		fmt.Fprintf(&body, ", %s", field.Name)
	}
	body.WriteString(", created_at, updated_at]\n    properties:\n      id: {type: integer, format: int64, minimum: 1, maximum: 9223372036854775807}\n")
	for _, field := range data.Fields {
		fmt.Fprintf(&body, "      %s: %s\n", field.Name, resourceOpenAPIField(field))
	}
	body.WriteString("      created_at: {type: string, format: date-time}\n      updated_at: {type: string, format: date-time}\n")
	fmt.Fprintf(&body, "  %sPage:\n    type: object\n    required: [items, total, page, page_size]\n    properties:\n      items:\n        type: array\n        items: {$ref: '#/definitions/%sItem'}\n      total: {type: integer, format: int64, minimum: 0}\n      page: {type: integer, minimum: 1, maximum: 100000}\n      page_size: {type: integer, minimum: 1, maximum: 100}\n", data.GoName, data.GoName)
	return File{Path: "api/openapi/" + data.Name + ".yaml", Content: []byte(body.String())}
}

func resourceOpenAPIField(field resourceCoreField) string {
	switch field.Type {
	case "string":
		return fmt.Sprintf("{type: string, maxLength: %d}", field.MaxLength)
	case "bool":
		return "{type: boolean}"
	default:
		return "{type: integer, format: int64}"
	}
}
