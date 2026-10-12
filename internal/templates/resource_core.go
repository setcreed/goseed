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
	GoName string
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
	data := struct {
		Project
		Options
		Name, GoName string
		Fields       []resourceCoreField
	}{Project: project, Options: options, Name: name, GoName: strings.ToUpper(name[:1]) + name[1:]}
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
	factory, err := renderResourceHTTPFactory(project, options, data)
	if err != nil {
		return nil, err
	}
	files = append(files, factory, resourceHTTPGuide(data), resourceOpenAPI(data))
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

func renderResourceHTTPFactory(project Project, options Options, data struct {
	Project
	Options
	Name, GoName string
	Fields       []resourceCoreField
}) (File, error) {
	var source strings.Builder
	source.WriteString("// Module assembly is developer-owned; goseed does not overwrite this file.\npackage bootstrap\n\nimport (\n")
	fmt.Fprintf(&source, "business %q\nstore %q\nhttpHandler %q\n", project.Module+"/internal/apiserver/biz/"+data.Name, project.Module+"/internal/apiserver/store/"+data.Name, project.Module+"/internal/apiserver/handler/http/"+data.Name)
	source.WriteString(")\n\n")
	fmt.Fprintf(&source, "type %sModule struct { HTTP *httpHandler.Handler; Operations map[string]string }\n", data.Name)
	fmt.Fprintf(&source, "func new%sModule(dependencies moduleDependencies) (%sModule, error) {\n", data.GoName, data.Name)
	fmt.Fprintf(&source, "repository := store.New(dependencies.Database)\nif err := repository.CheckSchema(dependencies.Context); err != nil { return %sModule{}, err }\n", data.Name)
	if options.Redis {
		source.WriteString("var dependency business.Dependency = dependencies.Cache\n")
	}
	fmt.Fprintf(&source, "return %sModule{HTTP:httpHandler.New(business.New(repository", data.Name)
	if options.Redis {
		source.WriteString(", dependency")
	}
	source.WriteString(")), Operations:")
	if options.Authz == "casbin" {
		fmt.Fprintf(&source, "%sPermissions().HTTP", data.Name)
	} else {
		source.WriteString("map[string]string{\"GET\":\"read\",\"POST\":\"create\",\"PATCH\":\"update\",\"DELETE\":\"delete\"}")
	}
	source.WriteString("}, nil\n}\n")
	fmt.Fprintf(&source, "func %sPermissions() modulePermissions { return modulePermissions{HTTP:map[string]string{\"GET\":\"read\",\"POST\":\"create\",\"PATCH\":\"update\",\"DELETE\":\"delete\"}} }\n", data.Name)
	formatted, err := formatGo(source.String())
	if err != nil {
		return File{}, err
	}
	return File{Path: "internal/apiserver/bootstrap/module_" + data.Name + ".go", Content: formatted}, nil
}

func resourceHTTPGuide(data struct {
	Project
	Options
	Name, GoName string
	Fields       []resourceCoreField
}) File {
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
	body.WriteString("\n示例：\n\n```sh\ncurl -X POST http://127.0.0.1:8080/v1/" + data.Name + " \\\n  -H 'Authorization: Bearer $TOKEN' -H 'Content-Type: application/json' \\\n  -d '{")
	for i, field := range data.Fields {
		if i > 0 {
			body.WriteString(",")
		}
		switch field.Type {
		case "string":
			fmt.Fprintf(&body, `"%s":"value"`, field.Name)
		case "bool":
			fmt.Fprintf(&body, `"%s":true`, field.Name)
		default:
			fmt.Fprintf(&body, `"%s":1`, field.Name)
		}
	}
	body.WriteString("}'\n```\n\n数据库 schema 必须先通过项目显式迁移建立；服务启动只读核验 schema，不自动迁移。")
	return File{Path: "docs/" + data.Name + ".md", Content: []byte(body.String())}
}

func resourceOpenAPI(data struct {
	Project
	Options
	Name, GoName string
	Fields       []resourceCoreField
}) File {
	var body strings.Builder
	fmt.Fprintf(&body, "swagger: \"2.0\"\npaths:\n  /%s:\n", data.Name)
	fmt.Fprintf(&body, "    post:\n      operationId: %sCreate\n      security:\n        - Bearer: []\n      parameters:\n        - {name: body, in: body, required: true, schema: {$ref: '#/definitions/Create%sRequest'}}\n      responses:\n        '201': {description: Created, schema: {$ref: '#/definitions/%sItem'}}\n        '400': {description: Invalid input}\n        '401': {description: Unauthorized}\n", data.Name, data.GoName, data.GoName)
	body.WriteString("    get:\n      operationId: " + data.Name + "List\n      security:\n        - Bearer: []\n      parameters:\n        - {name: page, in: query, type: integer, default: 1, minimum: 1, maximum: 100000}\n        - {name: page_size, in: query, type: integer, default: 20, minimum: 1, maximum: 100}\n")
	fmt.Fprintf(&body, "      responses:\n        '200': {description: Page, schema: {$ref: '#/definitions/%sPage'}}\n        '400': {description: Invalid pagination}\n", data.GoName)
	fmt.Fprintf(&body, "  /%s/{id}:\n", data.Name)
	fmt.Fprintf(&body, "    parameters:\n      - {name: id, in: path, required: true, type: integer, format: int64, minimum: 1, maximum: 9223372036854775807}\n    get:\n      operationId: %sGet\n      security:\n        - Bearer: []\n      responses:\n        '200': {description: Resource, schema: {$ref: '#/definitions/%sItem'}}\n        '404': {description: Not found}\n    patch:\n      operationId: %sUpdate\n      security:\n        - Bearer: []\n      parameters:\n        - {name: body, in: body, required: true, schema: {$ref: '#/definitions/Update%sRequest'}}\n      responses:\n        '200': {description: Updated, schema: {$ref: '#/definitions/%sItem'}}\n        '400': {description: Invalid input}\n        '404': {description: Not found}\n    delete:\n      operationId: %sDelete\n      security:\n        - Bearer: []\n      responses:\n        '204': {description: Deleted}\n        '404': {description: Not found}\ndefinitions:\n", data.Name, data.GoName, data.Name, data.GoName, data.GoName, data.Name)
	for _, definition := range []struct {
		name     string
		required bool
	}{{"Create" + data.GoName + "Request", true}, {"Update" + data.GoName + "Request", false}} {
		fmt.Fprintf(&body, "  %s:\n    type: object\n", definition.name)
		if !definition.required {
			body.WriteString("    minProperties: 1\n")
		}
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
			fmt.Fprintf(&body, "      %s:\n", field.Name)
			switch field.Type {
			case "string":
				fmt.Fprintf(&body, "        type: string\n        maxLength: %d\n", field.MaxLength)
			case "bool":
				body.WriteString("        type: boolean\n")
			default:
				body.WriteString("        type: integer\n        format: int64\n")
			}
		}
	}
	fmt.Fprintf(&body, "  %sItem:\n    type: object\n    required: [id, created_at, updated_at]\n    properties:\n      id: {type: integer, format: int64}\n", data.GoName)
	for _, field := range data.Fields {
		fmt.Fprintf(&body, "      %s: {type: %s", field.Name, openAPIType(field.Type))
		if field.Type == "string" {
			fmt.Fprintf(&body, ", maxLength: %d", field.MaxLength)
		}
		body.WriteString("}\n")
	}
	body.WriteString("      created_at: {type: string, format: date-time}\n      updated_at: {type: string, format: date-time}\n")
	fmt.Fprintf(&body, "  %sPage:\n    type: object\n    properties:\n      items:\n        type: array\n        items: {$ref: '#/definitions/%sItem'}\n      total: {type: integer, format: int64}\n      page: {type: integer}\n      page_size: {type: integer}\n", data.GoName, data.GoName)
	return File{Path: "api/openapi/" + data.Name + ".yaml", Content: []byte(body.String())}
}

func openAPIType(fieldType string) string {
	switch fieldType {
	case "string":
		return "string"
	case "bool":
		return "boolean"
	default:
		return "integer"
	}
}
