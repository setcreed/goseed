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
