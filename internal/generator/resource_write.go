package generator

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/setcreed/goseed/internal/openapi"
	"github.com/setcreed/goseed/internal/templates"
)

func resourceGRPCGuide(name string, fields []templates.ResourceField) []byte {
	var output strings.Builder
	fmt.Fprintf(&output, "# %s RPC\n\n资源由 gRPC Handler 调用共享 Biz；所有 RPC 要求 JWT，启用 Casbin 时按 `create`、`read`、`update`、`delete` 授权。字段存在性用于区分未提供与显式零值；Proto `int64` 保留 64 位精度。数据库 schema 需先显式迁移，服务启动只做只读核验。\n\n", name)
	output.WriteString("| 字段 | 类型 | 创建必需 | 约束 |\n|---|---|---|---|\n")
	for _, field := range fields {
		required, constraint := "否", ""
		if field.Required {
			required = "是"
		}
		if field.Type == "string" {
			constraint = fmt.Sprintf("最多 %d 个 Unicode 字符", field.MaxLength)
		}
		fmt.Fprintf(&output, "| `%s` | `%s` | %s | %s |\n", field.Name, field.Type, required, constraint)
	}
	return []byte(output.String())
}

func (p *ResourcePlan) prepareCommit(state ProjectState) error {
	files := make([]templates.File, 0, len(p.coreFiles)+len(p.httpFiles)+len(p.rpcFiles)+1)
	files = append(files, p.coreFiles...)
	files = append(files, p.httpFiles...)
	files = append(files, p.rpcFiles...)
	if p.migrationDraft != nil {
		files = append(files, *p.migrationDraft)
	}
	created := make(map[string]templates.File, len(files))
	for _, file := range files {
		if _, exists := created[file.Path]; exists {
			return fmt.Errorf("duplicate resource output %s", file.Path)
		}
		created[file.Path] = templates.File{Path: file.Path, Content: bytes.Clone(file.Content)}
	}
	metadata := append([]ModuleMetadata(nil), state.Metadata.Modules...)
	metadata = append(metadata, ModuleMetadata{Name: p.name, Type: "resource", Spec: p.specPath, FieldVersion: p.spec.Version})
	entries := make([]templates.ModuleEntry, 0, len(metadata))
	for _, module := range metadata {
		entries = append(entries, templates.ModuleEntry{Name: module.Name, Type: module.Type})
	}
	project := templates.Project{Name: state.Metadata.Name, Module: state.Metadata.Module}
	options := templates.Options{Database: state.Metadata.Database, Redis: state.Metadata.Redis, Authz: state.Metadata.Authorization, Transport: state.Metadata.Transport, Metrics: state.Metadata.Metrics, Tracing: state.Metadata.Tracing}
	registryFiles, err := templates.RenderWithModules(project, options, entries)
	if err != nil {
		return fmt.Errorf("render resource registration index: %w", err)
	}
	var registry []byte
	for _, file := range registryFiles {
		if file.Path == "internal/apiserver/bootstrap/modules_gen.go" {
			registry = bytes.Clone(file.Content)
			break
		}
	}
	if len(registry) == 0 {
		return fmt.Errorf("rendered project has no module registration index")
	}
	registry, err = replaceDigestMarker(registry, "modulesSHA256", moduleManifestDigest(metadata))
	if err != nil {
		return err
	}
	updates := make([]fileUpdate, 0, 3)
	metadataContent, err := renderMetadataModules(project, options, metadata)
	if err != nil {
		return err
	}
	if state.Metadata.Transport != "grpc" {
		var source []byte
		for _, file := range files {
			if file.Path == "api/openapi/"+p.name+".yaml" {
				source = bytes.Clone(file.Content)
				break
			}
		}
		if len(source) == 0 {
			return fmt.Errorf("resource OpenAPI source was not prepared")
		}
		sources := make([]openapi.Source, 0, len(state.Sources)+1)
		for _, item := range state.Sources {
			sources = append(sources, openapi.Source{Name: item.Name, YAML: item.YAML})
		}
		sources = append(sources, openapi.Source{Name: p.name, YAML: source})
		merged, err := openapi.Merge(sources)
		if err != nil {
			return fmt.Errorf("merge resource OpenAPI source: %w", err)
		}
		registry, err = replaceDigestMarker(registry, "openAPISHA256", merged.Digest)
		if err != nil {
			return err
		}
		created["api/openapi/swagger.json"] = templates.File{Path: "api/openapi/swagger.json", Content: bytes.Clone(merged.JSON)}
	}
	registryPath := "internal/apiserver/bootstrap/modules_gen.go"
	created[registryPath] = templates.File{Path: registryPath, Content: registry}
	created["goseed.yaml"] = templates.File{Path: "goseed.yaml", Content: metadataContent}

	for _, path := range []string{"goseed.yaml", registryPath} {
		update, err := resourceUpdate(p.root, path, created[path].Content)
		if err != nil {
			return err
		}
		updates = append(updates, update)
	}
	if state.Metadata.Transport != "grpc" {
		update, err := resourceUpdate(p.root, "api/openapi/swagger.json", created["api/openapi/swagger.json"].Content)
		if err != nil {
			return err
		}
		updates = append(updates, update)
	}
	delete(created, "goseed.yaml")
	delete(created, registryPath)
	delete(created, "api/openapi/swagger.json")
	for _, file := range created {
		p.creates = append(p.creates, file)
	}
	sort.Slice(p.creates, func(i, j int) bool { return p.creates[i].Path < p.creates[j].Path })
	sort.Slice(updates, func(i, j int) bool { return updates[i].Path < updates[j].Path })
	p.updates = updates
	return nil
}

func resourceUpdate(root, path string, content []byte) (fileUpdate, error) {
	if err := validateModulePath(root, path); err != nil {
		return fileUpdate{}, err
	}
	original, err := readProjectFile(root, path)
	if err != nil {
		return fileUpdate{}, err
	}
	info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		return fileUpdate{}, fmt.Errorf("inspect %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fileUpdate{}, fmt.Errorf("inspect %s: expected a regular file", path)
	}
	return fileUpdate{Path: path, Original: original, Content: bytes.Clone(content), Mode: info.Mode().Perm()}, nil
}

// Report shows the exact files prepared for an all-or-nothing resource
// generation. It performs no writes.
func (p ResourcePlan) Report(w io.Writer, dryRun ...bool) error {
	write := func(format string, args ...any) error {
		n, err := fmt.Fprintf(w, format, args...)
		if err == nil && n != len(fmt.Sprintf(format, args...)) {
			return io.ErrShortWrite
		}
		return err
	}
	if err := write("Resource: %s\nSpec: %s\nFiles (%d):\n", p.name, p.specPath, len(p.creates)+len(p.updates)); err != nil {
		return err
	}
	for _, file := range p.creates {
		if err := write("  + %s\n", file.Path); err != nil {
			return err
		}
	}
	for _, file := range p.updates {
		if err := write("  ~ %s\n", file.Path); err != nil {
			return err
		}
	}
	for _, tool := range p.tools {
		if err := write("  tool %s %s (%s)\n", tool.Name, tool.Version, tool.Path); err != nil {
			return err
		}
	}
	for _, conflict := range p.conflicts {
		if err := write("  ! %s\n", conflict); err != nil {
			return err
		}
	}
	if len(dryRun) > 0 && dryRun[0] {
		return write("Dry run: no files written.\n")
	}
	return write("Complete resource generation prepared.\n")
}

// Write revalidates every snapshotted input, exclusively creates business
// files, then updates generated indexes with rollback if any step fails.
func (p ResourcePlan) Write() error { return p.writeWith(writeAtomic) }

func (p ResourcePlan) writeWith(write atomicWriter) error {
	if err := p.Revalidate(); err != nil {
		return err
	}
	for _, update := range p.updates {
		actual, err := readProjectFile(p.root, update.Path)
		if err != nil {
			return err
		}
		if !bytes.Equal(actual, update.Original) {
			return fmt.Errorf("generated file %s changed after the resource plan was prepared", update.Path)
		}
	}
	createdFiles := make([]string, 0, len(p.creates))
	createdDirs := make([]string, 0, len(p.creates))
	rollbackCreates := func() []string {
		var failures []string
		for i := len(createdFiles) - 1; i >= 0; i-- {
			if err := os.Remove(createdFiles[i]); err != nil && !os.IsNotExist(err) {
				failures = append(failures, createdFiles[i]+": "+err.Error())
			}
		}
		for i := len(createdDirs) - 1; i >= 0; i-- {
			if err := os.Remove(createdDirs[i]); err != nil && !os.IsNotExist(err) {
				failures = append(failures, createdDirs[i]+": "+err.Error())
			}
		}
		return failures
	}
	for _, file := range p.creates {
		filename := filepath.Join(p.root, filepath.FromSlash(file.Path))
		dirs, err := makeParents(p.root, filepath.Dir(filename))
		createdDirs = append(createdDirs, dirs...)
		if err != nil {
			return moduleWriteError(fmt.Errorf("create parent for %s: %w", file.Path, err), rollbackCreates())
		}
		output, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if err != nil {
			return moduleWriteError(fmt.Errorf("create %s: %w", file.Path, err), rollbackCreates())
		}
		createdFiles = append(createdFiles, filename)
		if _, err := output.Write(file.Content); err != nil {
			_ = output.Close()
			return moduleWriteError(fmt.Errorf("write %s: %w", file.Path, err), rollbackCreates())
		}
		if err := output.Close(); err != nil {
			return moduleWriteError(fmt.Errorf("close %s: %w", file.Path, err), rollbackCreates())
		}
	}
	written := make([]fileUpdate, 0, len(p.updates))
	for _, update := range p.updates {
		filename := filepath.Join(p.root, filepath.FromSlash(update.Path))
		if err := write(filename, update.Content, update.Mode); err != nil {
			var rollbackErrors []string
			for i := len(written) - 1; i >= 0; i-- {
				previous := written[i]
				path := filepath.Join(p.root, filepath.FromSlash(previous.Path))
				if restoreErr := write(path, previous.Original, previous.Mode); restoreErr != nil {
					rollbackErrors = append(rollbackErrors, previous.Path+": "+restoreErr.Error())
				}
			}
			cleanup := rollbackCreates()
			return moduleWriteError(fmt.Errorf("update %s: %w", update.Path, err), append(rollbackErrors, cleanup...))
		}
		written = append(written, update)
	}
	return nil
}
