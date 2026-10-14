package generator

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/setcreed/goseed/internal/openapi"
	"github.com/setcreed/goseed/internal/templates"
)

// ResourceOptions selects the specification and an optional migration draft.
// Runtime capabilities come exclusively from the verified project manifest.
type ResourceOptions struct {
	Spec      string
	Migration bool
}

// ResourceTarget describes an intended output, not a rendered source file.
type ResourceTarget struct {
	Path   string
	Action string
}

type resourceFingerprint struct {
	Digest [sha256.Size]byte
	Mode   os.FileMode
}

// ResourcePlan is a read-only target and business-core plan. Protocol rendering
// and Write remain absent until the complete safe generation path is verified.
type ResourcePlan struct {
	root           string
	name           string
	specPath       string
	spec           ResourceSpec
	targets        []ResourceTarget
	tools          []ToolRequirement
	inputs         map[string]resourceFingerprint
	migrations     []string
	conflicts      []string
	schemaStore    templates.File
	migrationDraft *templates.File
	coreFiles      []templates.File
	httpFiles      []templates.File
	rpcFiles       []templates.File
}

// PrepareResource validates resource inputs and plans output paths without
// creating files, rendering protocol placeholders, or connecting to services.
func PrepareResource(root, name string, options ResourceOptions) (ResourcePlan, error) {
	if !projectModuleName.MatchString(name) || len(name) > 63 || name == "example" {
		return ResourcePlan{}, fmt.Errorf("invalid or reserved resource name %q", name)
	}
	if name == "examples" {
		return ResourcePlan{}, fmt.Errorf("resource table %q is reserved by the built-in example module", name)
	}
	if !validResourceSpec(options.Spec) {
		return ResourcePlan{}, fmt.Errorf("resource spec must be a normalized project-relative .yaml path under specs/")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return ResourcePlan{}, fmt.Errorf("resolve resource project root: %w", err)
	}
	canonical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return ResourcePlan{}, fmt.Errorf("resolve resource project root: %w", err)
	}
	if err := validateModulePath(canonical, "internal/apiserver/bootstrap/modules_gen.go"); err != nil {
		return ResourcePlan{}, err
	}
	// LoadProject reads OpenAPI sources. Reject linked parents before that read.
	if _, err := os.Lstat(filepath.Join(canonical, "api/openapi")); !os.IsNotExist(err) {
		if err != nil {
			return ResourcePlan{}, err
		}
		if err := validateModulePath(canonical, "api/openapi/swagger.json"); err != nil {
			return ResourcePlan{}, err
		}
	}
	initial := make(map[string]resourceFingerprint)
	for _, path := range []string{"goseed.yaml", "internal/apiserver/bootstrap/modules_gen.go"} {
		fingerprint, err := fingerprintResourceInput(canonical, path)
		if err != nil {
			return ResourcePlan{}, err
		}
		initial[path] = fingerprint
	}
	state, err := LoadProject(canonical)
	if err != nil {
		return ResourcePlan{}, err
	}
	if state.Metadata.Database == "none" {
		return ResourcePlan{}, fmt.Errorf("resource generation requires SQLite, MySQL, or PostgreSQL; use gen module for a project without a database")
	}
	for _, module := range state.Metadata.Modules {
		if module.Name == name {
			return ResourcePlan{}, fmt.Errorf("module or resource %q is already registered", name)
		}
	}
	assembly, err := validateModuleAssembly(state)
	if err != nil {
		return ResourcePlan{}, err
	}
	if err := validateModulePath(state.Root, options.Spec); err != nil {
		return ResourcePlan{}, err
	}
	data, err := readProjectFile(state.Root, options.Spec)
	if err != nil {
		return ResourcePlan{}, err
	}
	spec, err := ParseResourceSpec(data)
	if err != nil {
		return ResourcePlan{}, fmt.Errorf("resource spec %s: %w", options.Spec, err)
	}
	plan := ResourcePlan{root: state.Root, name: name, specPath: options.Spec, spec: spec, inputs: make(map[string]resourceFingerprint)}
	inputs := []string{"goseed.yaml", "internal/apiserver/bootstrap/modules_gen.go", options.Spec}
	for path := range assembly {
		inputs = append(inputs, path)
	}
	for _, module := range state.Metadata.Modules {
		if module.Type == "resource" {
			inputs = append(inputs, module.Spec)
		}
	}
	if state.Metadata.Transport != "grpc" {
		inputs = append(inputs, "api/openapi/swagger.json")
		for _, source := range state.Sources {
			inputs = append(inputs, "api/openapi/"+source.Name+".yaml")
		}
	}
	for _, path := range inputs {
		if err := plan.snapshot(path); err != nil {
			return ResourcePlan{}, err
		}
	}
	// Do not accept bytes changed between semantic validation and fingerprinting.
	validated := map[string][]byte{options.Spec: data}
	for path, source := range assembly {
		validated[path] = source
	}
	for path, source := range validated {
		if plan.inputs[path].Digest != sha256.Sum256(source) {
			return ResourcePlan{}, fmt.Errorf("project input %s changed while preparing the resource plan", path)
		}
	}
	for path, expected := range initial {
		if plan.inputs[path] != expected {
			return ResourcePlan{}, fmt.Errorf("project input %s changed while preparing the resource plan", path)
		}
	}
	for _, source := range state.Sources {
		if plan.inputs["api/openapi/"+source.Name+".yaml"].Digest != sha256.Sum256(source.YAML) {
			return ResourcePlan{}, fmt.Errorf("OpenAPI source %s changed while preparing the resource plan", source.Name)
		}
	}
	if state.Metadata.Transport != "grpc" && plan.inputs["api/openapi/swagger.json"].Digest != sha256.Sum256(state.CurrentOpenAPI) {
		return ResourcePlan{}, fmt.Errorf("OpenAPI index changed while preparing the resource plan")
	}
	plan.migrations, err = migrationInputs(state.Root)
	if err != nil {
		return ResourcePlan{}, err
	}
	for _, path := range plan.migrations {
		if err := plan.snapshot("migrations/" + path); err != nil {
			return ResourcePlan{}, err
		}
	}
	plan.targets = resourceTargets(name, state.Metadata.Transport)
	fields := make([]templates.SchemaField, 0, len(spec.Fields))
	for _, field := range spec.Fields {
		fields = append(fields, templates.SchemaField{Name: field.Name, Type: field.Type, MaxLength: field.MaxLength})
	}
	store, sql, err := templates.RenderResourceSchema(templates.Project{Name: state.Metadata.Name, Module: state.Metadata.Module}, state.Metadata.Database, name, fields)
	if err != nil {
		return ResourcePlan{}, err
	}
	plan.schemaStore = store
	coreFields := make([]templates.ResourceField, len(spec.Fields))
	for i, field := range spec.Fields {
		coreFields[i] = templates.ResourceField{SchemaField: fields[i], Required: field.Required}
	}
	plan.coreFiles, err = templates.RenderResourceCore(templates.Project{Name: state.Metadata.Name, Module: state.Metadata.Module}, templates.Options{Database: state.Metadata.Database, Transport: state.Metadata.Transport, Authz: state.Metadata.Authorization, Redis: state.Metadata.Redis}, name, coreFields)
	if err != nil {
		return ResourcePlan{}, err
	}
	if state.Metadata.Transport == "http" || state.Metadata.Transport == "dual" {
		plan.httpFiles, err = templates.RenderResourceHTTP(templates.Project{Name: state.Metadata.Name, Module: state.Metadata.Module}, templates.Options{Database: state.Metadata.Database, Transport: state.Metadata.Transport, Authz: state.Metadata.Authorization, Redis: state.Metadata.Redis}, name, coreFields)
		if err != nil {
			return ResourcePlan{}, err
		}
		sources := make([]openapi.Source, 0, len(state.Sources)+1)
		for _, source := range state.Sources {
			sources = append(sources, openapi.Source{Name: source.Name, YAML: source.YAML})
		}
		for _, file := range plan.httpFiles {
			if file.Path == "api/openapi/"+name+".yaml" {
				sources = append(sources, openapi.Source{Name: name, YAML: file.Content})
			}
		}
		if _, err := openapi.Merge(sources); err != nil {
			return ResourcePlan{}, fmt.Errorf("merge resource OpenAPI source: %w", err)
		}
	}
	if state.Metadata.Transport == "grpc" || state.Metadata.Transport == "dual" {
		plan.rpcFiles, err = templates.RenderResourceGRPC(templates.Project{Name: state.Metadata.Name, Module: state.Metadata.Module}, templates.Options{Database: state.Metadata.Database, Transport: state.Metadata.Transport, Authz: state.Metadata.Authorization, Redis: state.Metadata.Redis}, name, coreFields)
		if err != nil {
			return ResourcePlan{}, err
		}
	}
	if options.Migration {
		path, err := nextResourceMigration(name, plan.migrations)
		if err != nil {
			return ResourcePlan{}, err
		}
		plan.targets = append(plan.targets, ResourceTarget{Path: path, Action: "create"})
		plan.migrationDraft = &templates.File{Path: path, Content: sql}
	}
	plan.tools = requiredResourceTools(state.Metadata.Transport)
	if len(plan.tools) > 0 {
		if err := plan.snapshot("scripts/proto.py"); err != nil {
			return ResourcePlan{}, err
		}
	}
	for _, tool := range plan.tools {
		if err := plan.snapshot(tool.Path); err != nil {
			return ResourcePlan{}, fmt.Errorf("protocol tool %s is unavailable; prepare the pinned project-local tools explicitly: %w", tool.Name, err)
		}
		if err := checkResourceTool(plan.root, tool); err != nil {
			return ResourcePlan{}, err
		}
	}
	if len(plan.tools) > 0 {
		for _, name := range []string{"empty", "timestamp"} {
			if err := plan.snapshot(".tools/protoc/include/google/protobuf/" + name + ".proto"); err != nil {
				return ResourcePlan{}, fmt.Errorf("protocol standard include is unavailable: %w", err)
			}
		}
	}
	sort.Slice(plan.targets, func(i, j int) bool { return plan.targets[i].Path < plan.targets[j].Path })
	for _, target := range plan.targets {
		if target.Action != "create" {
			continue
		}
		if err := validateCreatePath(plan.root, target.Path); err != nil {
			if !errors.Is(err, errModuleOutputConflict) {
				return ResourcePlan{}, err
			}
			plan.conflicts = append(plan.conflicts, strings.TrimPrefix(err.Error(), errModuleOutputConflict.Error()+": "))
		}
	}
	if len(plan.conflicts) == 0 {
		if err := plan.Revalidate(); err != nil {
			return ResourcePlan{}, err
		}
	}
	return plan, nil
}

func resourceTargets(name, transport string) []ResourceTarget {
	paths := []string{
		"internal/apiserver/model/" + name + ".go",
		"internal/apiserver/model/" + name + "_test.go",
		"internal/apiserver/biz/" + name + "/biz.go",
		"internal/apiserver/biz/" + name + "/biz_test.go",
		"internal/apiserver/store/" + name + "/store.go",
		"internal/apiserver/store/" + name + "/store_test.go",
		"internal/apiserver/bootstrap/module_" + name + ".go",
		"docs/" + name + ".md",
	}
	for _, protocol := range []string{"http", "grpc"} {
		if protocol == "http" && transport != "http" && transport != "dual" || protocol == "grpc" && transport == "http" {
			continue
		}
		paths = append(paths, "internal/apiserver/handler/"+protocol+"/"+name+"/handler.go", "internal/apiserver/handler/"+protocol+"/"+name+"/handler_test.go")
	}
	if transport != "http" {
		paths = append(paths, "api/proto/"+name+"/v1/"+name+".proto", "pkg/api/"+name+"/v1/"+name+".pb.go", "pkg/api/"+name+"/v1/"+name+"_grpc.pb.go")
	}
	if transport == "gateway" {
		paths = append(paths, "api/proto/"+name+"/v1/http.yaml", "api/proto/"+name+"/v1/openapi.yaml", "pkg/api/"+name+"/v1/"+name+".pb.gw.go")
	}
	if transport != "grpc" {
		paths = append(paths, "api/openapi/"+name+".yaml")
	}
	targets := make([]ResourceTarget, 0, len(paths)+3)
	for _, path := range paths {
		targets = append(targets, ResourceTarget{Path: path, Action: "create"})
	}
	for _, path := range []string{"goseed.yaml", "internal/apiserver/bootstrap/modules_gen.go"} {
		targets = append(targets, ResourceTarget{Path: path, Action: "update"})
	}
	if transport != "grpc" {
		targets = append(targets, ResourceTarget{Path: "api/openapi/swagger.json", Action: "update"})
	}
	return targets
}

func (p *ResourcePlan) snapshot(path string) error {
	fingerprint, err := fingerprintResourceInput(p.root, path)
	if err != nil {
		return err
	}
	p.inputs[path] = fingerprint
	return nil
}

func fingerprintResourceInput(root, path string) (resourceFingerprint, error) {
	if err := validateModulePath(root, path); err != nil {
		return resourceFingerprint{}, err
	}
	filename := filepath.Join(root, filepath.FromSlash(path))
	info, err := os.Lstat(filename)
	if err != nil {
		return resourceFingerprint{}, fmt.Errorf("inspect resource input %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return resourceFingerprint{}, fmt.Errorf("resource input %s must be a regular file, not a symlink", path)
	}
	file, err := os.Open(filename)
	if err != nil {
		return resourceFingerprint{}, fmt.Errorf("read resource input %s: %w", path, err)
	}
	defer func() { _ = file.Close() }()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return resourceFingerprint{}, fmt.Errorf("hash resource input %s: %w", path, err)
	}
	var fingerprint resourceFingerprint
	copy(fingerprint.Digest[:], hash.Sum(nil))
	fingerprint.Mode = info.Mode().Perm()
	return fingerprint, nil
}

// Revalidate refuses stale inputs and new target conflicts. It never writes.
func (p ResourcePlan) Revalidate() error {
	if p.root == "" || len(p.inputs) == 0 {
		return fmt.Errorf("resource plan is not prepared")
	}
	if len(p.conflicts) > 0 {
		return fmt.Errorf("generation conflicts: %s", strings.Join(p.conflicts, ", "))
	}
	paths := make([]string, 0, len(p.inputs))
	for path := range p.inputs {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		actual, err := fingerprintResourceInput(p.root, path)
		if err != nil {
			return err
		}
		if actual != p.inputs[path] {
			return fmt.Errorf("project input %s changed after the resource plan was prepared", path)
		}
	}
	migrations, err := migrationInputs(p.root)
	if err != nil {
		return err
	}
	if !slices.Equal(migrations, p.migrations) {
		return fmt.Errorf("migration directory changed after the resource plan was prepared")
	}
	if _, err := LoadProject(p.root); err != nil {
		return err
	}
	for _, target := range p.targets {
		if target.Action == "create" {
			if err := validateCreatePath(p.root, target.Path); err != nil {
				return err
			}
		}
	}
	return nil
}

// Targets, Tools, Conflicts, and Specification return independent copies.
func (p ResourcePlan) Targets() []ResourceTarget { return append([]ResourceTarget(nil), p.targets...) }
func (p ResourcePlan) Tools() []ToolRequirement  { return append([]ToolRequirement(nil), p.tools...) }
func (p ResourcePlan) Conflicts() []string       { return append([]string(nil), p.conflicts...) }
func (p ResourcePlan) Specification() ResourceSpec {
	spec := p.spec
	spec.Fields = append([]ResourceField(nil), spec.Fields...)
	return spec
}

// SchemaStore and MigrationDraft return reviewable schema inputs, not CRUD or
// permission to write/apply them. Revalidate still guards later execution.
func (p ResourcePlan) SchemaStore() templates.File {
	return templates.File{Path: p.schemaStore.Path, Content: bytes.Clone(p.schemaStore.Content)}
}

// CoreFiles returns independently reviewable business sources and tests. It
// does not include protocol handlers, factories, registry updates, or Write.
func (p ResourcePlan) CoreFiles() []templates.File {
	files := make([]templates.File, len(p.coreFiles))
	for i, file := range p.coreFiles {
		files[i] = templates.File{Path: file.Path, Content: bytes.Clone(file.Content)}
	}
	return files
}

// HTTPFiles returns the HTTP adapter, developer-owned factory, module guide,
// and module Swagger source as independent copies. It does not write or update
// the project registry or merged OpenAPI index.
func (p ResourcePlan) HTTPFiles() []templates.File {
	files := make([]templates.File, len(p.httpFiles))
	for i, file := range p.httpFiles {
		files[i] = templates.File{Path: file.Path, Content: append([]byte(nil), file.Content...)}
	}
	return files
}

// RPCFiles returns the RPC adapter and Proto source as independent copies.
func (p ResourcePlan) RPCFiles() []templates.File {
	files := make([]templates.File, len(p.rpcFiles))
	for i, file := range p.rpcFiles {
		files[i] = templates.File{Path: file.Path, Content: bytes.Clone(file.Content)}
	}
	return files
}
func (p ResourcePlan) MigrationDraft() (templates.File, bool) {
	if p.migrationDraft == nil {
		return templates.File{}, false
	}
	return templates.File{Path: p.migrationDraft.Path, Content: bytes.Clone(p.migrationDraft.Content)}, true
}

// Report labels this intermediate preview explicitly; it promises no CRUD code.
func (p ResourcePlan) Report(w io.Writer) error {
	var output bytes.Buffer
	fmt.Fprintf(&output, "Resource target plan: %s\nSpec: %s\nTargets (%d):\n", p.name, p.specPath, len(p.targets))
	for _, target := range p.targets {
		marker := "+"
		if target.Action == "update" {
			marker = "~"
		}
		fmt.Fprintf(&output, "  %s %s\n", marker, target.Path)
	}
	fmt.Fprintf(&output, "Protocol tools (%d):\n", len(p.tools))
	for _, tool := range p.tools {
		fmt.Fprintf(&output, "  - %s %s (%s)\n", tool.Name, tool.Version, tool.Path)
	}
	for _, conflict := range p.conflicts {
		fmt.Fprintf(&output, "  ! %s\n", conflict)
	}
	output.WriteString("Target preview only: complete generation and writing are not available yet.\nBusiness core is available through CoreFiles.\n")
	if len(p.httpFiles) > 0 {
		output.WriteString("HTTP adapter, factory, module guide, and Swagger source are available through HTTPFiles.\n")
	} else {
		output.WriteString("Selected protocol adapter rendering is not available yet.\n")
	}
	if len(p.rpcFiles) > 0 {
		output.WriteString("RPC adapter and Proto source are available through RPCFiles.\n")
	}
	output.WriteString("No files written.\n")
	_, err := output.WriteTo(w)
	return err
}
