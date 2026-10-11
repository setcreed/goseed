// Package generator plans and writes standalone projects from embedded templates.
package generator

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/setcreed/goseed/internal/templates"
)

const TemplateVersion = "1"

// Plan contains a complete rendered project and the conflicts found at its target.
type Plan struct {
	Root      string
	Project   templates.Project
	Options   templates.Options
	Files     []templates.File
	Conflicts []string
}

// Prepare renders every output before inspecting the target directory.
func Prepare(root string, project templates.Project, options templates.Options) (Plan, error) {
	files, err := templates.Render(project, options)
	if err != nil {
		return Plan{}, fmt.Errorf("prepare project templates: %w", err)
	}
	metadata, err := renderMetadata(project, options)
	if err != nil {
		return Plan{}, err
	}
	files = append(files, templates.File{Path: "goseed.yaml", Content: metadata})
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })

	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return Plan{}, fmt.Errorf("resolve target directory: %w", err)
	}
	plan := Plan{Root: absoluteRoot, Project: project, Options: options, Files: files}
	plan.Conflicts, err = inspect(plan.Root, files)
	if err != nil {
		return Plan{}, err
	}
	return plan, nil
}

// Report writes a stable, reviewable summary without changing the target.
func (p Plan) Report(w io.Writer, dryRun bool) error {
	if _, err := fmt.Fprintf(w, "Project: %s\nModule: %s\nDatabase: %s\nJWT: enabled\nAuthorization: %s\nRedis: %t\nTransport: %s\nFiles (%d):\n", p.Project.Name, p.Project.Module, optionOr(p.Options.Database, "sqlite"), optionOr(p.Options.Authz, "none"), p.Options.Redis, optionOr(p.Options.Transport, "http"), len(p.Files)); err != nil {
		return err
	}
	for _, file := range p.Files {
		if _, err := fmt.Fprintf(w, "  + %s\n", file.Path); err != nil {
			return err
		}
	}
	dependencies := requiredDependencies(p.Files)
	if _, err := fmt.Fprintf(w, "Dependencies (%d):\n", len(dependencies)); err != nil {
		return err
	}
	for _, dependency := range dependencies {
		if _, err := fmt.Fprintf(w, "  - %s\n", dependency); err != nil {
			return err
		}
	}
	if len(p.Conflicts) > 0 {
		if _, err := io.WriteString(w, "Conflicts:\n"); err != nil {
			return err
		}
		for _, conflict := range p.Conflicts {
			if _, err := fmt.Fprintf(w, "  ! %s\n", conflict); err != nil {
				return err
			}
		}
	}
	if dryRun {
		_, err := io.WriteString(w, "Dry run: no files written.\n")
		return err
	}
	return nil
}

// Write rechecks conflicts immediately before creating files. It never replaces
// an existing file and removes only paths created by this invocation on failure.
func (p Plan) Write() error {
	conflicts, err := inspect(p.Root, p.Files)
	if err != nil {
		return err
	}
	if len(conflicts) > 0 {
		return fmt.Errorf("generation conflicts: %s", strings.Join(conflicts, ", "))
	}
	createdFiles := make([]string, 0, len(p.Files))
	createdDirs := make([]string, 0, len(p.Files))
	rollback := func(cause error) error {
		var cleanup []string
		for i := len(createdFiles) - 1; i >= 0; i-- {
			if err := os.Remove(createdFiles[i]); err != nil && !os.IsNotExist(err) {
				cleanup = append(cleanup, createdFiles[i]+": "+err.Error())
			}
		}
		for i := len(createdDirs) - 1; i >= 0; i-- {
			if err := os.Remove(createdDirs[i]); err != nil && !os.IsNotExist(err) {
				cleanup = append(cleanup, createdDirs[i]+": "+err.Error())
			}
		}
		if len(cleanup) > 0 {
			return fmt.Errorf("%w; rollback left files or directories: %s", cause, strings.Join(cleanup, ", "))
		}
		return cause
	}

	for _, file := range p.Files {
		filename := filepath.Join(p.Root, filepath.FromSlash(file.Path))
		created, err := makeParents(p.Root, filepath.Dir(filename))
		createdDirs = append(createdDirs, created...)
		if err != nil {
			return rollback(fmt.Errorf("create directory for %s: %w", file.Path, err))
		}
		output, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if err != nil {
			return rollback(fmt.Errorf("create %s: %w", file.Path, err))
		}
		createdFiles = append(createdFiles, filename)
		if _, err := io.Copy(output, bytes.NewReader(file.Content)); err != nil {
			_ = output.Close()
			return rollback(fmt.Errorf("write %s: %w", file.Path, err))
		}
		if err := output.Close(); err != nil {
			return rollback(fmt.Errorf("close %s: %w", file.Path, err))
		}
	}
	return nil
}

func inspect(root string, files []templates.File) ([]string, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("inspect target directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("target is not a directory")
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("read target directory: %w", err)
	}
	var conflicts []string
	for _, entry := range entries {
		if entry.Name() == ".git" || entry.Name() == ".DS_Store" || entry.Name() == "Thumbs.db" {
			continue
		}
		conflicts = append(conflicts, "target directory already contains "+entry.Name())
	}
	seen := make(map[string]struct{}, len(files))
	for _, file := range files {
		if !filepath.IsLocal(filepath.FromSlash(file.Path)) {
			conflicts = append(conflicts, "unsafe output path "+file.Path)
			continue
		}
		if _, ok := seen[file.Path]; ok {
			conflicts = append(conflicts, "duplicate output path "+file.Path)
			continue
		}
		seen[file.Path] = struct{}{}
		destination := filepath.Join(root, filepath.FromSlash(file.Path))
		relative, err := filepath.Rel(root, destination)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			conflicts = append(conflicts, "output escapes target directory "+file.Path)
			continue
		}
		for parent := filepath.Dir(destination); parent != root && parent != "."; parent = filepath.Dir(parent) {
			info, err := os.Lstat(parent)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return nil, fmt.Errorf("inspect output parent %s: %w", parent, err)
			}
			if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
				conflicts = append(conflicts, "unsafe output parent "+filepath.ToSlash(parent))
				break
			}
		}
		if _, err := os.Lstat(destination); err == nil {
			conflicts = append(conflicts, "output already exists "+file.Path)
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("inspect output %s: %w", file.Path, err)
		}
	}
	sort.Strings(conflicts)
	return conflicts, nil
}

func makeParents(root, directory string) ([]string, error) {
	var missing []string
	for current := directory; current != root && current != "."; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
				return nil, fmt.Errorf("unsafe parent path %s", current)
			}
			break
		}
		if !os.IsNotExist(err) {
			return nil, err
		}
		missing = append(missing, current)
	}
	created := make([]string, 0, len(missing))
	for i := len(missing) - 1; i >= 0; i-- {
		if err := os.Mkdir(missing[i], 0755); err != nil {
			return created, err
		}
		created = append(created, missing[i])
	}
	return created, nil
}

func renderMetadata(project templates.Project, options templates.Options) ([]byte, error) {
	if options.Database == "" {
		options.Database = "sqlite"
	}
	if options.Authz == "" {
		options.Authz = "none"
	}
	if options.Transport == "" {
		options.Transport = "http"
	}
	return []byte(fmt.Sprintf("version: 1\ntemplate_version: %s\nname: %s\nmodule: %s\ndatabase: %s\nauthentication: jwt\nauthorization: %s\nredis: %t\ntransport: %s\nmodules:\n  - example\n", TemplateVersion, project.Name, project.Module, options.Database, options.Authz, options.Redis, options.Transport)), nil
}

func optionOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func requiredDependencies(files []templates.File) []string {
	var manifest string
	for _, file := range files {
		if file.Path == "go.mod" {
			manifest = string(file.Content)
			break
		}
	}
	var dependencies []string
	inRequireBlock := false
	for _, line := range strings.Split(manifest, "\n") {
		line = strings.TrimSpace(line)
		if line == "require (" {
			inRequireBlock = true
			continue
		}
		if inRequireBlock && line == ")" {
			inRequireBlock = false
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[0] == "require" && strings.HasPrefix(fields[2], "v") {
			dependencies = append(dependencies, fields[1]+" "+fields[2])
			continue
		}
		if inRequireBlock && len(fields) >= 2 && strings.HasPrefix(fields[1], "v") {
			dependencies = append(dependencies, fields[0]+" "+fields[1])
		}
	}
	sort.Strings(dependencies)
	return dependencies
}
