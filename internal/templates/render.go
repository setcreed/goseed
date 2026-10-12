// Package templates renders standalone source plans without writing files.
package templates

import (
	"bytes"
	"embed"
	"fmt"
	"go/format"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strings"
	"text/template"

	"github.com/setcreed/goseed/internal/openapi"
)

//go:embed base variants
var sources embed.FS

type File struct {
	Path    string
	Content []byte
}

type Project struct {
	Name   string
	Module string
}

// Options selects source at generation time, never at service runtime.
type Options struct {
	Database  string
	Redis     bool
	Authz     string
	Transport string
}

// ModuleEntry is the manifest projection used to render protocol registries.
type ModuleEntry struct {
	Name       string
	Type       string
	GoName     string
	ServiceVar string
	Path       string
}

type templateData struct {
	Project
	Options
	Modules []ModuleEntry
}

var namePattern = regexp.MustCompile(`^[a-z][a-z0-9]*$`)
var modulePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]*(/[A-Za-z0-9._~-]+)+$`)

// Default builds the entire HTTP/SQLite/JWT file plan before a caller writes it.
func Default(project Project) ([]File, error) {
	return Render(project, Options{})
}

// Render prepares a standalone project with the selected transport and storage.
func Render(project Project, options Options) ([]File, error) {
	return RenderWithModules(project, options, []ModuleEntry{{Name: "example", Path: "/v1/examples"}})
}

// RenderWithModules renders a project using the supplied manifest modules.
func RenderWithModules(project Project, options Options, modules []ModuleEntry) ([]File, error) {
	if !namePattern.MatchString(project.Name) {
		return nil, fmt.Errorf("invalid project name")
	}
	if !modulePattern.MatchString(project.Module) {
		return nil, fmt.Errorf("invalid module path")
	}
	for _, part := range strings.Split(project.Module, "/") {
		if part == "." || part == ".." || strings.HasSuffix(part, ".") {
			return nil, fmt.Errorf("invalid module path segment")
		}
	}
	if options.Database == "" {
		options.Database = "sqlite"
	}
	if options.Database != "sqlite" && options.Database != "mysql" && options.Database != "postgres" && options.Database != "none" {
		return nil, fmt.Errorf("unsupported database")
	}
	if options.Authz == "" {
		options.Authz = "none"
	}
	if options.Authz != "none" && options.Authz != "casbin" {
		return nil, fmt.Errorf("unsupported authorization")
	}
	if options.Transport == "" {
		options.Transport = "http"
	}
	if options.Transport != "http" && options.Transport != "grpc" && options.Transport != "dual" && options.Transport != "gateway" {
		return nil, fmt.Errorf("unsupported transport")
	}
	if len(modules) == 0 {
		return nil, fmt.Errorf("at least one module is required")
	}
	seenModules := make(map[string]bool, len(modules))
	hasExample := false
	for i := range modules {
		module := &modules[i]
		if !namePattern.MatchString(module.Name) || seenModules[module.Name] {
			return nil, fmt.Errorf("invalid or duplicate module name %q", module.Name)
		}
		seenModules[module.Name] = true
		module.GoName = strings.ToUpper(module.Name[:1]) + module.Name[1:]
		module.ServiceVar = module.Name + "Service"
		if module.Type == "" {
			module.Type = "module"
		}
		if module.Type != "module" && module.Type != "resource" {
			return nil, fmt.Errorf("unsupported module type %q", module.Type)
		}
		if module.Name == "example" {
			hasExample = true
			if module.Path == "" {
				module.Path = "/v1/examples"
			}
		} else if module.Path == "" {
			module.Path = "/v1/" + module.Name
		}
	}
	if !hasExample {
		return nil, fmt.Errorf("built-in example module is required")
	}
	data := templateData{Project: project, Options: options, Modules: modules}
	base, err := renderTree(sources, "base", data)
	if err != nil {
		return nil, err
	}
	if options.Database == "sqlite" && !options.Redis && options.Authz == "none" && options.Transport == "http" {
		return finalize(base)
	}
	// Variant directories intentionally replace matching base files, and add
	// backend-specific fixtures. Sorting keeps the complete plan deterministic.
	files := make(map[string]File, len(base))
	for _, file := range base {
		files[file.Path] = file
	}
	directories := []string{"variants/sql", "variants/" + options.Database}
	if options.Database == "sqlite" {
		directories = nil
	}
	if options.Database == "none" {
		directories = []string{"variants/none"}
	}
	if options.Redis {
		directories = append(directories, "variants/redis/common", "variants/redis/"+options.Database)
	}
	if options.Authz == "casbin" {
		directories = append(directories, "variants/casbin/common")
		if options.Database != "none" {
			directories = append(directories, "variants/casbin/sql")
		}
		manifest := "plain"
		if options.Redis {
			manifest = "redis"
		}
		directories = append(directories, "variants/casbin/manifests/"+options.Database+"/"+manifest)
	}
	if options.Transport != "http" {
		profile := "plain"
		if options.Authz == "casbin" {
			profile = "casbin"
		}
		if options.Redis {
			profile += "-redis"
		}
		directories = append(directories, "variants/grpc/common")
		if options.Database != "none" {
			directories = append(directories, "variants/grpc/sql")
		}
		switch options.Transport {
		case "grpc":
			directories = append(directories, "variants/grpc/"+options.Database, "variants/grpc/manifests/"+options.Database+"/"+profile)
		case "dual":
			directories = append(directories, "variants/grpc/"+options.Database, "variants/dual/common", "variants/dual/manifests/"+options.Database+"/"+profile)
		case "gateway":
			directories = append(directories, "variants/grpc/"+options.Database, "variants/gateway/common", "variants/gateway/manifests/"+options.Database+"/"+profile)
			if options.Database == "none" {
				directories = append(directories, "variants/gateway/none")
			} else {
				directories = append(directories, "variants/gateway/sql")
			}
		}
	}
	for _, directory := range directories {
		variant, err := renderTree(sources, directory, data)
		if err != nil {
			return nil, err
		}
		for _, file := range variant {
			files[file.Path] = file
		}
	}
	result := make([]File, 0, len(files))
	for _, file := range files {
		result = append(result, file)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Path < result[j].Path })
	return finalize(result)
}

// RenderBusinessModule returns the small, behavior-free source skeleton for a
// registered module. Resource CRUD generation is handled separately.
func RenderBusinessModule(project Project, options Options, name string) ([]File, error) {
	if !modulePattern.MatchString(project.Module) || !namePattern.MatchString(name) || name == "example" {
		return nil, fmt.Errorf("invalid business module identity")
	}
	if options.Database == "" {
		options.Database = "sqlite"
	}
	if options.Database != "sqlite" && options.Database != "mysql" && options.Database != "postgres" && options.Database != "none" {
		return nil, fmt.Errorf("unsupported database")
	}
	if options.Transport == "" {
		options.Transport = "http"
	}
	if options.Transport != "http" && options.Transport != "grpc" && options.Transport != "dual" && options.Transport != "gateway" {
		return nil, fmt.Errorf("unsupported transport")
	}
	files := []File{}
	bizSource := "package biz\n\n"
	if options.Database == "none" {
		bizSource += "type Biz struct{}\n\nfunc New() *Biz { return &Biz{} }\n"
	} else {
		bizSource += "// Repository is the persistence boundary for operations added to this module.\ntype Repository interface{}\n\ntype Biz struct { repository Repository }\n\nfunc New(repository Repository) *Biz { return &Biz{repository: repository} }\n"
		storeSource := "package store\n\nimport (\n" + name + "biz \"" + project.Module + "/internal/apiserver/biz/" + name + "\"\n\"gorm.io/gorm\"\n)\n\ntype Store struct{ db *gorm.DB }\n\nfunc New(db *gorm.DB) *Store { return &Store{db:db} }\n\nvar _ " + name + "biz.Repository = (*Store)(nil)\n"
		formatted, err := formatGo(storeSource)
		if err != nil {
			return nil, err
		}
		files = append(files, File{Path: "internal/apiserver/store/" + name + "/store.go", Content: formatted})
	}
	formatted, err := formatGo(bizSource)
	if err != nil {
		return nil, err
	}
	files = append(files, File{Path: "internal/apiserver/biz/" + name + "/biz.go", Content: formatted})
	if options.Transport == "http" || options.Transport == "dual" {
		handler := "package handler\n\nimport (\n biz \"" + project.Module + "/internal/apiserver/biz/" + name + "\"\n \"github.com/gin-gonic/gin\"\n)\n\ntype Handler struct { biz *biz.Biz }\n\nfunc New(business *biz.Biz) *Handler { return &Handler{biz: business} }\n\nfunc (h *Handler) Register(_ *gin.RouterGroup) {}\n"
		formatted, err := formatGo(handler)
		if err != nil {
			return nil, err
		}
		files = append(files, File{Path: "internal/apiserver/handler/http/" + name + "/handler.go", Content: formatted})
	}
	if options.Transport != "http" {
		handler := "package handler\n\nimport (\n biz \"" + project.Module + "/internal/apiserver/biz/" + name + "\"\n \"google.golang.org/grpc\"\n)\n\nvar Permissions = map[string]string{}\n\ntype Handler struct { biz *biz.Biz }\n\nfunc New(business *biz.Biz) *Handler { return &Handler{biz: business} }\n\nfunc (h *Handler) Register(_ grpc.ServiceRegistrar) {}\n"
		formatted, err := formatGo(handler)
		if err != nil {
			return nil, err
		}
		files = append(files, File{Path: "internal/apiserver/handler/grpc/" + name + "/handler.go", Content: formatted})
	}
	factory, err := renderModuleFactory(project, options, name)
	if err != nil {
		return nil, err
	}
	files = append(files, factory)
	return files, nil
}

func formatGo(source string) ([]byte, error) {
	formatted, err := format.Source([]byte(source))
	if err != nil {
		return nil, fmt.Errorf("format module skeleton: %w", err)
	}
	return formatted, nil
}

func finalize(files []File) ([]File, error) {
	swaggerIndex, registryIndex := -1, -1
	for i := range files {
		switch files[i].Path {
		case "api/openapi/swagger.json":
			swaggerIndex = i
		case "internal/apiserver/bootstrap/modules_gen.go":
			registryIndex = i
		}
	}
	if swaggerIndex == -1 {
		return files, nil
	}
	result, err := openapi.Merge([]openapi.Source{{Name: "example", YAML: files[swaggerIndex].Content}})
	if err != nil {
		return nil, fmt.Errorf("merge bundled OpenAPI document: %w", err)
	}
	if registryIndex == -1 {
		return nil, fmt.Errorf("OpenAPI document has no module registration index")
	}
	marker := []byte(`const openAPISHA256 = ""`)
	if bytes.Count(files[registryIndex].Content, marker) != 1 {
		return nil, fmt.Errorf("module registration index must contain one OpenAPI digest marker")
	}
	replacement := []byte(fmt.Sprintf("const openAPISHA256 = %q", result.Digest))
	files[registryIndex].Content = bytes.Replace(files[registryIndex].Content, marker, replacement, 1)
	files[swaggerIndex].Content = result.JSON
	return files, nil
}

func render(source fs.FS, project Project) ([]File, error) {
	return renderTree(source, "base", templateData{Project: project, Options: Options{Database: "sqlite"}})
}

func renderTree(source fs.FS, root string, data templateData) ([]File, error) {
	var files []File
	err := fs.WalkDir(source, root, func(filename string, entry fs.DirEntry, walkError error) error {
		if walkError != nil {
			return walkError
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Name() == ".DS_Store" || entry.Name() == "Thumbs.db" {
			return nil
		}
		if !strings.HasSuffix(filename, ".tmpl") {
			return fmt.Errorf("unrecognized template: %s", filename)
		}
		relative := strings.TrimSuffix(strings.TrimPrefix(filename, root+"/"), ".tmpl")
		if root == "base" && (data.Transport == "grpc" || data.Transport == "gateway") && relative == "internal/apiserver/bootstrap/modules_gen.go" {
			return nil
		}
		if (data.Transport == "dual" || data.Transport == "gateway") && strings.HasPrefix(root, "variants/grpc/") && relative == "internal/apiserver/bootstrap/providers_test.go" {
			return nil
		}
		if (data.Transport == "dual" || data.Transport == "gateway") && root == "variants/grpc/common" {
			switch relative {
			case "README.md", "AGENTS.md", "ROADMAP.md", "configs/app.example.yaml", "build/docker/Dockerfile", "internal/pkg/config/config.go", "internal/apiserver/bootstrap/providers.go":
				return nil
			}
			if relative == "internal/pkg/config/config_test.go" {
				relative = "internal/pkg/config/grpc_test.go"
			}
		}
		if data.Transport == "gateway" {
			if strings.HasPrefix(relative, "internal/apiserver/handler/http/") || strings.HasPrefix(relative, "internal/pkg/middleware/") || relative == "internal/apiserver/router.go" || relative == "internal/apiserver/router_test.go" || relative == "internal/apiserver/bootstrap/providers_test.go" {
				return nil
			}
		}
		if data.Transport == "grpc" {
			if strings.HasPrefix(relative, "internal/apiserver/handler/http/") || strings.HasPrefix(relative, "internal/pkg/middleware/") || strings.HasPrefix(relative, "internal/pkg/server/") || strings.HasPrefix(relative, "api/openapi/") || relative == "internal/apiserver/router.go" || relative == "internal/apiserver/router_test.go" {
				return nil
			}
		}
		if data.Database == "none" {
			for _, prefix := range []string{"migrations/", "internal/pkg/database/", "internal/apiserver/model/", "internal/apiserver/store/"} {
				if strings.HasPrefix(relative, prefix) {
					return nil
				}
			}
		}
		if name := path.Base(relative); strings.HasPrefix(name, "dot.") {
			relative = path.Join(path.Dir(relative), "."+strings.TrimPrefix(name, "dot."))
		}
		if !fs.ValidPath(relative) || path.Clean(relative) != relative {
			return fmt.Errorf("invalid template path")
		}
		content, err := fs.ReadFile(source, filename)
		if err != nil {
			return err
		}
		parsed, err := template.New(filename).Delims("[[", "]]").Option("missingkey=error").Parse(string(content))
		if err != nil {
			return fmt.Errorf("parse %s: %w", relative, err)
		}
		var output bytes.Buffer
		if err := parsed.Execute(&output, data); err != nil {
			return fmt.Errorf("render %s: %w", relative, err)
		}
		content = output.Bytes()
		if strings.HasSuffix(relative, ".go") {
			content, err = format.Source(content)
			if err != nil {
				return fmt.Errorf("format %s: %w", relative, err)
			}
		}
		files = append(files, File{Path: relative, Content: content})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}
