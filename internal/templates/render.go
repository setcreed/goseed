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
	"strings"
	"text/template"
)

//go:embed base
var sources embed.FS

type File struct {
	Path    string
	Content []byte
}

type Project struct {
	Name   string
	Module string
}

var namePattern = regexp.MustCompile(`^[a-z][a-z0-9]*$`)
var modulePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]*(/[A-Za-z0-9._~-]+)+$`)

// Default builds the entire HTTP/SQLite/JWT file plan before a caller writes it.
func Default(project Project) ([]File, error) {
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
	return render(sources, project)
}

func render(source fs.FS, project Project) ([]File, error) {
	var files []File
	err := fs.WalkDir(source, "base", func(filename string, entry fs.DirEntry, walkError error) error {
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
		relative := strings.TrimSuffix(strings.TrimPrefix(filename, "base/"), ".tmpl")
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
		if err := parsed.Execute(&output, project); err != nil {
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
