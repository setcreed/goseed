package generator

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/setcreed/goseed/internal/openapi"
	"github.com/setcreed/goseed/internal/templates"
	"go.yaml.in/yaml/v3"
)

// ModulePlan is a dry-run-safe addition of a behavior-free business module.
type ModulePlan struct {
	root      string
	name      string
	inputs    map[string][]byte
	updates   []fileUpdate
	creates   []templates.File
	Conflicts []string
}

var errModuleOutputConflict = errors.New("module output conflict")

// PrepareModule validates the project and prepares module source, manifest,
// protocol registry, and OpenAPI changes without writing any files.
func PrepareModule(root, name string) (ModulePlan, error) {
	if !projectModuleName.MatchString(name) || name == "example" {
		return ModulePlan{}, fmt.Errorf("invalid or reserved module name %q", name)
	}
	state, err := LoadProject(root)
	if err != nil {
		return ModulePlan{}, err
	}
	assemblyInputs, err := validateModuleAssembly(state)
	if err != nil {
		return ModulePlan{}, err
	}
	for _, module := range state.Metadata.Modules {
		if module.Name == name {
			return ModulePlan{}, fmt.Errorf("module %q is already registered", name)
		}
	}
	project := templates.Project{Name: state.Metadata.Name, Module: state.Metadata.Module}
	options := state.Metadata.templateOptions()
	moduleFiles, err := templates.RenderBusinessModule(project, options, name)
	if err != nil {
		return ModulePlan{}, err
	}
	creates := append([]templates.File(nil), moduleFiles...)
	if state.Metadata.Transport != "grpc" {
		source, err := emptyOpenAPISource(state.Sources[0].YAML)
		if err != nil {
			return ModulePlan{}, fmt.Errorf("prepare module OpenAPI source: %w", err)
		}
		creates = append(creates, templates.File{Path: "api/openapi/" + name + ".yaml", Content: source})
	}
	metadata := append([]ModuleMetadata(nil), state.Metadata.Modules...)
	metadata = append(metadata, ModuleMetadata{Name: name, Type: "module"})
	entries := make([]templates.ModuleEntry, 0, len(metadata))
	for _, module := range metadata {
		entries = append(entries, templates.ModuleEntry{Name: module.Name, Type: module.Type})
	}
	rendered, err := templates.RenderWithModules(project, options, entries)
	if err != nil {
		return ModulePlan{}, fmt.Errorf("render module registration index: %w", err)
	}
	var registry []byte
	for _, file := range rendered {
		if file.Path == "internal/apiserver/bootstrap/modules_gen.go" {
			registry = bytes.Clone(file.Content)
			break
		}
	}
	if len(registry) == 0 {
		return ModulePlan{}, fmt.Errorf("rendered project has no module registration index")
	}
	if state.Metadata.Transport != "grpc" {
		sources := append([]openapi.Source(nil), state.Sources...)
		newSource := creates[len(creates)-1]
		sources = append(sources, openapi.Source{Name: name, YAML: newSource.Content})
		merged, err := openapi.Merge(sources)
		if err != nil {
			return ModulePlan{}, fmt.Errorf("merge new module OpenAPI source: %w", err)
		}
		registry, err = replaceDigestMarker(registry, "openAPISHA256", merged.Digest)
		if err != nil {
			return ModulePlan{}, err
		}
		creates = append(creates, templates.File{Path: "api/openapi/swagger.json", Content: merged.JSON})
	}
	registry, err = replaceDigestMarker(registry, "modulesSHA256", moduleManifestDigest(metadata))
	if err != nil {
		return ModulePlan{}, err
	}
	metadataContent, err := renderMetadataModules(project, options, metadata)
	if err != nil {
		return ModulePlan{}, err
	}
	registryPath := "internal/apiserver/bootstrap/modules_gen.go"
	metadataPath := "goseed.yaml"
	updates := []fileUpdate{}
	for _, target := range []struct {
		path string
		data []byte
	}{{metadataPath, metadataContent}, {registryPath, registry}} {
		original, err := readProjectFile(state.Root, target.path)
		if err != nil {
			return ModulePlan{}, err
		}
		info, err := os.Lstat(filepath.Join(state.Root, filepath.FromSlash(target.path)))
		if err != nil {
			return ModulePlan{}, fmt.Errorf("inspect %s: %w", target.path, err)
		}
		updates = append(updates, fileUpdate{Path: target.path, Original: original, Content: target.data, Mode: info.Mode().Perm()})
	}
	if state.Metadata.Transport != "grpc" {
		indexPath := "api/openapi/swagger.json"
		info, err := os.Lstat(filepath.Join(state.Root, filepath.FromSlash(indexPath)))
		if err != nil {
			return ModulePlan{}, fmt.Errorf("inspect %s: %w", indexPath, err)
		}
		updates = append(updates, fileUpdate{Path: indexPath, Original: bytes.Clone(state.CurrentOpenAPI), Content: bytes.Clone(creates[len(creates)-1].Content), Mode: info.Mode().Perm()})
		creates = creates[:len(creates)-1]
	}
	sort.Slice(updates, func(i, j int) bool { return updates[i].Path < updates[j].Path })
	sort.Slice(creates, func(i, j int) bool { return creates[i].Path < creates[j].Path })
	inputs := map[string][]byte{metadataPath: nil}
	for path, source := range assemblyInputs {
		inputs[path] = bytes.Clone(source)
	}
	inputs[metadataPath], err = readProjectFile(state.Root, metadataPath)
	if err != nil {
		return ModulePlan{}, err
	}
	inputs[registryPath] = nil
	inputs[registryPath], err = readProjectFile(state.Root, registryPath)
	if err != nil {
		return ModulePlan{}, err
	}
	for _, source := range state.Sources {
		inputs["api/openapi/"+source.Name+".yaml"] = bytes.Clone(source.YAML)
	}
	if state.Metadata.Transport != "grpc" {
		inputs["api/openapi/swagger.json"] = bytes.Clone(state.CurrentOpenAPI)
	}
	var conflicts []string
	for _, file := range creates {
		if err := validateCreatePath(state.Root, file.Path); err != nil {
			if errors.Is(err, errModuleOutputConflict) {
				conflicts = append(conflicts, strings.TrimPrefix(err.Error(), errModuleOutputConflict.Error()+": "))
				continue
			}
			return ModulePlan{}, err
		}
	}
	sort.Strings(conflicts)
	return ModulePlan{root: state.Root, name: name, inputs: inputs, updates: updates, creates: creates, Conflicts: conflicts}, nil
}

func emptyOpenAPISource(source []byte) ([]byte, error) {
	var document yaml.Node
	decoder := yaml.NewDecoder(bytes.NewReader(source))
	if err := decoder.Decode(&document); err != nil {
		return nil, err
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("expected one OpenAPI mapping document")
	}
	root := document.Content[0]
	for i := 0; i < len(root.Content); i += 2 {
		if root.Content[i].Value == "paths" {
			root.Content[i+1] = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
			var output bytes.Buffer
			encoder := yaml.NewEncoder(&output)
			encoder.SetIndent(2)
			if err := encoder.Encode(root); err != nil {
				return nil, err
			}
			if err := encoder.Close(); err != nil {
				return nil, err
			}
			return output.Bytes(), nil
		}
	}
	return nil, fmt.Errorf("OpenAPI source has no paths mapping")
}

func replaceDigestMarker(source []byte, marker, value string) ([]byte, error) {
	needle := []byte(`const ` + marker + ` = "`)
	if bytes.Count(source, needle) != 1 {
		return nil, fmt.Errorf("module registration index must contain one %s marker", marker)
	}
	start := bytes.Index(source, needle) + len(needle)
	end := bytes.IndexByte(source[start:], '"')
	if end < 0 {
		return nil, fmt.Errorf("module registration index has an invalid %s marker", marker)
	}
	end += start
	result := make([]byte, 0, len(source)+len(value))
	result = append(result, source[:start]...)
	result = append(result, value...)
	result = append(result, source[end:]...)
	return result, nil
}

// Report describes added files and regenerated indexes without side effects.
func (p ModulePlan) Report(w io.Writer, dryRun bool) error {
	if _, err := fmt.Fprintf(w, "Module: %s\nFiles (%d):\n", p.name, len(p.creates)+len(p.updates)); err != nil {
		return err
	}
	for _, file := range p.creates {
		if _, err := fmt.Fprintf(w, "  + %s\n", file.Path); err != nil {
			return err
		}
	}
	for _, file := range p.updates {
		if _, err := fmt.Fprintf(w, "  ~ %s\n", file.Path); err != nil {
			return err
		}
	}
	for _, conflict := range p.Conflicts {
		if _, err := fmt.Fprintf(w, "  ! %s\n", conflict); err != nil {
			return err
		}
	}
	if dryRun {
		_, err := io.WriteString(w, "Dry run: no files written.\n")
		return err
	}
	return nil
}

// Write revalidates the prepared plan, creates new source files exclusively,
// and atomically replaces generated indexes with rollback on failure.
func (p ModulePlan) Write() error {
	return p.writeWith(writeAtomic)
}

func (p ModulePlan) writeWith(write atomicWriter) error {
	if len(p.Conflicts) > 0 {
		return fmt.Errorf("generation conflicts: %s", strings.Join(p.Conflicts, ", "))
	}
	if _, err := LoadProject(p.root); err != nil {
		return fmt.Errorf("revalidate project before module generation: %w", err)
	}
	for relative, expected := range p.inputs {
		actual, err := readProjectFile(p.root, relative)
		if err != nil {
			return err
		}
		if !bytes.Equal(actual, expected) {
			return fmt.Errorf("project input %s changed after the module plan was prepared", relative)
		}
	}
	for _, update := range p.updates {
		actual, err := readProjectFile(p.root, update.Path)
		if err != nil {
			return err
		}
		if !bytes.Equal(actual, update.Original) {
			return fmt.Errorf("generated file %s changed after the module plan was prepared", update.Path)
		}
		if err := validateModulePath(p.root, update.Path); err != nil {
			return err
		}
	}
	for _, file := range p.creates {
		if err := validateCreatePath(p.root, file.Path); err != nil {
			return err
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
			cleanupErrors := rollbackCreates()
			return moduleWriteError(fmt.Errorf("update %s: %w", update.Path, err), append(rollbackErrors, cleanupErrors...))
		}
		written = append(written, update)
	}
	return nil
}

func validateCreatePath(root, relative string) error {
	if !filepath.IsLocal(filepath.FromSlash(relative)) {
		return fmt.Errorf("unsafe output path %s", relative)
	}
	filename := filepath.Join(root, filepath.FromSlash(relative))
	for parent := filepath.Dir(filename); parent != root && parent != "."; parent = filepath.Dir(parent) {
		info, err := os.Lstat(parent)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("inspect output parent %s: %w", parent, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("unsafe output parent %s", filepath.ToSlash(parent))
		}
	}
	if _, err := os.Lstat(filename); err == nil {
		return fmt.Errorf("%w: output already exists %s", errModuleOutputConflict, relative)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect output %s: %w", relative, err)
	}
	return nil
}

func validateModulePath(root, relative string) error {
	if !filepath.IsLocal(filepath.FromSlash(relative)) {
		return fmt.Errorf("unsafe generated file update %s", relative)
	}
	for parent := filepath.Dir(filepath.Join(root, filepath.FromSlash(relative))); parent != root && parent != "."; parent = filepath.Dir(parent) {
		info, err := os.Lstat(parent)
		if err != nil {
			return fmt.Errorf("inspect update parent %s: %w", parent, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("unsafe update parent %s", filepath.ToSlash(parent))
		}
	}
	return nil
}

func moduleWriteError(cause error, rollbackErrors []string) error {
	if len(rollbackErrors) > 0 {
		return fmt.Errorf("%w; rollback failed for %s", cause, strings.Join(rollbackErrors, ", "))
	}
	return cause
}
