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
	Database string
}

type templateData struct {
	Project
	Options
}

var namePattern = regexp.MustCompile(`^[a-z][a-z0-9]*$`)
var modulePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]*(/[A-Za-z0-9._~-]+)+$`)

// Default builds the entire HTTP/SQLite/JWT file plan before a caller writes it.
func Default(project Project) ([]File, error) {
	return Render(project, Options{})
}

// Render prepares a standalone HTTP/JWT project with the selected storage.
func Render(project Project, options Options) ([]File, error) {
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
	data := templateData{Project: project, Options: options}
	base, err := renderTree(sources, "base", data)
	if err != nil {
		return nil, err
	}
	if options.Database == "sqlite" {
		return base, nil
	}
	// Variant directories intentionally replace matching base files, and add
	// backend-specific fixtures. Sorting keeps the complete plan deterministic.
	files := make(map[string]File, len(base))
	for _, file := range base {
		files[file.Path] = file
	}
	directories := []string{"variants/sql", "variants/" + options.Database}
	if options.Database == "none" {
		directories = []string{"variants/none"}
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
	return result, nil
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
