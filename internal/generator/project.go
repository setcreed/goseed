package generator

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/setcreed/goseed/internal/openapi"
	"go.yaml.in/yaml/v3"
)

var projectModuleName = regexp.MustCompile(`^[a-z][a-z0-9]*$`)

// ProjectMetadata is the versioned generation manifest written into a project.
type ProjectMetadata struct {
	Version         int              `yaml:"version"`
	TemplateVersion int              `yaml:"template_version"`
	Name            string           `yaml:"name"`
	Module          string           `yaml:"module"`
	Database        string           `yaml:"database"`
	Authentication  string           `yaml:"authentication"`
	Authorization   string           `yaml:"authorization"`
	Redis           bool             `yaml:"redis"`
	Transport       string           `yaml:"transport"`
	Modules         []ModuleMetadata `yaml:"-"`
}

// ModuleMetadata describes one generated business namespace.
type ModuleMetadata struct {
	Name         string `yaml:"name" json:"name"`
	Type         string `yaml:"type" json:"type"`
	Spec         string `yaml:"spec,omitempty" json:"spec,omitempty"`
	FieldVersion int    `yaml:"field_version,omitempty" json:"field_version,omitempty"`
}

// ProjectState is a verified snapshot of the generation-owned OpenAPI inputs
// and their aggregate index. It performs no writes.
type ProjectState struct {
	Root              string
	Metadata          ProjectMetadata
	Sources           []openapi.Source
	CurrentOpenAPI    []byte
	OpenAPI           openapi.Result
	OpenAPIIndexStale bool
}

// LoadProject validates generation metadata, module-owned OpenAPI sources,
// the merged index, and the digest recorded by the generated registry.
func LoadProject(root string) (ProjectState, error) {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return ProjectState{}, fmt.Errorf("resolve project root: %w", err)
	}
	info, err := os.Stat(absoluteRoot)
	if err != nil {
		return ProjectState{}, fmt.Errorf("inspect project root: %w", err)
	}
	if !info.IsDir() {
		return ProjectState{}, fmt.Errorf("project root is not a directory")
	}

	metadataBytes, err := readProjectFile(absoluteRoot, "goseed.yaml")
	if err != nil {
		return ProjectState{}, err
	}
	metadata, err := parseProjectMetadata(metadataBytes)
	if err != nil {
		return ProjectState{}, err
	}
	state := ProjectState{Root: absoluteRoot, Metadata: metadata}
	registry, err := readProjectFile(absoluteRoot, "internal/apiserver/bootstrap/modules_gen.go")
	if err != nil {
		return ProjectState{}, err
	}
	modulesDigest := moduleManifestDigest(metadata.Modules)
	moduleMarker := []byte(`const modulesSHA256 = "` + modulesDigest + `"`)
	if bytes.Count(registry, []byte("const modulesSHA256 = ")) != 1 || bytes.Count(registry, moduleMarker) != 1 {
		return ProjectState{}, fmt.Errorf("module registration digest does not match goseed.yaml modules")
	}
	if metadata.Transport == "grpc" {
		if _, err := os.Lstat(filepath.Join(absoluteRoot, "api/openapi")); !os.IsNotExist(err) {
			if err != nil {
				return ProjectState{}, fmt.Errorf("inspect OpenAPI directory: %w", err)
			}
			return ProjectState{}, fmt.Errorf("gRPC-only project must not contain api/openapi")
		}
		return state, nil
	}

	moduleNames := make([]string, 0, len(metadata.Modules))
	for _, module := range metadata.Modules {
		moduleNames = append(moduleNames, module.Name)
		name := "api/openapi/" + module.Name + ".yaml"
		source, err := readProjectFile(absoluteRoot, name)
		if err != nil {
			return ProjectState{}, fmt.Errorf("read OpenAPI source for module %q: %w", module.Name, err)
		}
		state.Sources = append(state.Sources, openapi.Source{Name: module.Name, YAML: source})
	}
	if err := rejectUnregisteredOpenAPISources(absoluteRoot, moduleNames); err != nil {
		return ProjectState{}, err
	}
	merged, err := openapi.Merge(state.Sources)
	if err != nil {
		return ProjectState{}, fmt.Errorf("merge project OpenAPI sources: %w", err)
	}
	index, err := readProjectFile(absoluteRoot, "api/openapi/swagger.json")
	if err != nil {
		return ProjectState{}, err
	}
	if !json.Valid(index) {
		return ProjectState{}, fmt.Errorf("api/openapi/swagger.json is not valid JSON")
	}
	indexDigest := sha256.Sum256(index)
	currentDigest := hex.EncodeToString(indexDigest[:])
	marker := []byte(`const openAPISHA256 = "` + currentDigest + `"`)
	if bytes.Count(registry, []byte("const openAPISHA256 = ")) != 1 || bytes.Count(registry, marker) != 1 {
		return ProjectState{}, fmt.Errorf("module registration OpenAPI digest does not match api/openapi/swagger.json")
	}
	state.OpenAPI = merged
	state.CurrentOpenAPI = bytes.Clone(index)
	state.OpenAPIIndexStale = !bytes.Equal(index, merged.JSON)
	return state, nil
}

func parseProjectMetadata(data []byte) (ProjectMetadata, error) {
	var document yaml.Node
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&document); err != nil {
		return ProjectMetadata{}, fmt.Errorf("parse goseed.yaml: %w", err)
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return ProjectMetadata{}, fmt.Errorf("goseed.yaml must contain one mapping document")
	}
	if err := validateMetadataNode(document.Content[0]); err != nil {
		return ProjectMetadata{}, fmt.Errorf("validate goseed.yaml: %w", err)
	}
	var trailing yaml.Node
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return ProjectMetadata{}, fmt.Errorf("goseed.yaml must contain one document")
		}
		return ProjectMetadata{}, fmt.Errorf("parse goseed.yaml: %w", err)
	}

	var metadata ProjectMetadata
	strict := yaml.NewDecoder(bytes.NewReader(data))
	strict.KnownFields(true)
	// Module entries are decoded separately to retain compatibility with v1
	// projects generated before entries carried an explicit type.
	var raw struct {
		Version         int       `yaml:"version"`
		TemplateVersion int       `yaml:"template_version"`
		Name            string    `yaml:"name"`
		Module          string    `yaml:"module"`
		Database        string    `yaml:"database"`
		Authentication  string    `yaml:"authentication"`
		Authorization   string    `yaml:"authorization"`
		Redis           bool      `yaml:"redis"`
		Transport       string    `yaml:"transport"`
		Modules         yaml.Node `yaml:"modules"`
	}
	strict.KnownFields(true)
	if err := strict.Decode(&raw); err != nil {
		return ProjectMetadata{}, fmt.Errorf("decode goseed.yaml: %w", err)
	}
	metadata = ProjectMetadata{Version: raw.Version, TemplateVersion: raw.TemplateVersion, Name: raw.Name, Module: raw.Module, Database: raw.Database, Authentication: raw.Authentication, Authorization: raw.Authorization, Redis: raw.Redis, Transport: raw.Transport}
	if raw.Modules.Kind != 0 {
		list := &raw.Modules
		if list.Kind != yaml.SequenceNode {
			return ProjectMetadata{}, fmt.Errorf("goseed.yaml modules must be a sequence")
		}
		for _, item := range list.Content {
			var module ModuleMetadata
			switch item.Kind {
			case yaml.ScalarNode:
				if item.ShortTag() != "!!str" {
					return ProjectMetadata{}, fmt.Errorf("module names must be strings or mappings")
				}
				module = ModuleMetadata{Name: item.Value, Type: "module"}
			case yaml.MappingNode:
				for i := 0; i < len(item.Content); i += 2 {
					key := item.Content[i].Value
					if key != "name" && key != "type" && key != "spec" && key != "field_version" {
						return ProjectMetadata{}, fmt.Errorf("unknown module entry field %q", key)
					}
				}
				if err := item.Decode(&module); err != nil {
					return ProjectMetadata{}, fmt.Errorf("decode module entry: %w", err)
				}
			default:
				return ProjectMetadata{}, fmt.Errorf("module entries must be strings or mappings")
			}
			metadata.Modules = append(metadata.Modules, module)
		}
	}
	if metadata.Version != 1 || metadata.TemplateVersion != 1 {
		return ProjectMetadata{}, fmt.Errorf("unsupported goseed.yaml or template version")
	}
	if metadata.Name == "" || metadata.Module == "" || metadata.Authentication != "jwt" {
		return ProjectMetadata{}, fmt.Errorf("goseed.yaml is missing required project metadata")
	}
	if metadata.Database != "sqlite" && metadata.Database != "mysql" && metadata.Database != "postgres" && metadata.Database != "none" {
		return ProjectMetadata{}, fmt.Errorf("unsupported database %q in goseed.yaml", metadata.Database)
	}
	if metadata.Authorization != "none" && metadata.Authorization != "casbin" {
		return ProjectMetadata{}, fmt.Errorf("unsupported authorization %q in goseed.yaml", metadata.Authorization)
	}
	if metadata.Transport != "http" && metadata.Transport != "grpc" && metadata.Transport != "dual" && metadata.Transport != "gateway" {
		return ProjectMetadata{}, fmt.Errorf("unsupported transport %q in goseed.yaml", metadata.Transport)
	}
	if len(metadata.Modules) == 0 {
		return ProjectMetadata{}, fmt.Errorf("goseed.yaml must register at least one module")
	}
	seen := make(map[string]bool, len(metadata.Modules))
	hasExample := false
	for _, module := range metadata.Modules {
		if !projectModuleName.MatchString(module.Name) || seen[module.Name] {
			return ProjectMetadata{}, fmt.Errorf("invalid or duplicate module name %q in goseed.yaml", module.Name)
		}
		if module.Type != "module" && module.Type != "resource" {
			return ProjectMetadata{}, fmt.Errorf("unsupported module type %q", module.Type)
		}
		if module.Type == "resource" && (!validResourceSpec(module.Spec) || module.FieldVersion != 1) {
			return ProjectMetadata{}, fmt.Errorf("resource %q requires spec and field_version 1", module.Name)
		}
		if module.Type == "module" && (module.Spec != "" || module.FieldVersion != 0) {
			return ProjectMetadata{}, fmt.Errorf("module %q cannot declare resource fields", module.Name)
		}
		seen[module.Name] = true
		hasExample = hasExample || module.Name == "example" && module.Type == "module"
	}
	if !hasExample {
		return ProjectMetadata{}, fmt.Errorf("goseed.yaml must retain the built-in example module")
	}
	return metadata, nil
}

func validResourceSpec(spec string) bool {
	normalized := filepath.ToSlash(filepath.Clean(filepath.FromSlash(spec)))
	if normalized != spec || !filepath.IsLocal(filepath.FromSlash(spec)) || filepath.Ext(spec) != ".yaml" || !strings.HasPrefix(spec, "specs/") {
		return false
	}
	return true
}

func moduleManifestDigest(modules []ModuleMetadata) string {
	hash := sha256.New()
	for _, module := range modules {
		_, _ = fmt.Fprintf(hash, "%s\x00%s\x00%s\x00%d\n", module.Name, module.Type, module.Spec, module.FieldVersion)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func validateMetadataNode(node *yaml.Node) error {
	if node.Anchor != "" || node.Kind == yaml.AliasNode {
		return fmt.Errorf("YAML anchors and aliases are not supported")
	}
	switch node.ShortTag() {
	case "!!map", "!!seq", "!!str", "!!null", "!!bool", "!!int", "!!float":
	default:
		return fmt.Errorf("YAML tag %q is not supported", node.ShortTag())
	}
	if node.Kind == yaml.MappingNode {
		seen := make(map[string]bool, len(node.Content)/2)
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i]
			if key.ShortTag() == "!!merge" {
				return fmt.Errorf("YAML merge keys are not supported")
			}
			if key.Kind != yaml.ScalarNode || key.ShortTag() != "!!str" {
				return fmt.Errorf("mapping keys must be strings")
			}
			if seen[key.Value] {
				return fmt.Errorf("duplicate key %q", key.Value)
			}
			seen[key.Value] = true
		}
	}
	for _, child := range node.Content {
		if err := validateMetadataNode(child); err != nil {
			return err
		}
	}
	return nil
}

func readProjectFile(root, relative string) ([]byte, error) {
	filename := filepath.Join(root, filepath.FromSlash(relative))
	info, err := os.Lstat(filename)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", relative, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("project file %s must be a regular file, not a symlink", relative)
	}
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", relative, err)
	}
	return data, nil
}

func rejectUnregisteredOpenAPISources(root string, modules []string) error {
	entries, err := os.ReadDir(filepath.Join(root, "api/openapi"))
	if err != nil {
		return fmt.Errorf("read api/openapi: %w", err)
	}
	registered := make(map[string]bool, len(modules))
	for _, module := range modules {
		registered[module+".yaml"] = true
	}
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".yaml" && !registered[entry.Name()] {
			return fmt.Errorf("OpenAPI source %s is not registered in goseed.yaml", entry.Name())
		}
	}
	return nil
}
