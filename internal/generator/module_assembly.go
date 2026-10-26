package generator

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"

	"github.com/setcreed/goseed/internal/templates"
)

// validateModuleAssembly checks the stable startup contract without rewriting
// developer-owned bootstrap files. Snapshot these inputs for Write as well.
func validateModuleAssembly(state ProjectState) (map[string][]byte, error) {
	registryPath := "internal/apiserver/bootstrap/modules_gen.go"
	registry, err := readProjectFile(state.Root, registryPath)
	if err != nil {
		return nil, err
	}
	marker := []byte("const moduleAssemblyVersion = 2")
	if bytes.Count(registry, marker) != 1 {
		return nil, fmt.Errorf("project lacks module assembly version 2; explicitly upgrade the bootstrap entry, pure permission declarations and module factories before generating (no files written)")
	}
	providerPath := "internal/apiserver/bootstrap/providers.go"
	if err := validateModulePath(state.Root, providerPath); err != nil {
		return nil, err
	}
	providers, err := readProjectFile(state.Root, providerPath)
	if err != nil {
		return nil, err
	}
	if err := validateAssemblyCalls(providers, state.Metadata.Transport); err != nil {
		return nil, err
	}
	inputs := map[string][]byte{providerPath: providers}
	permissionPath := "internal/apiserver/bootstrap/module_permissions.go"
	if err := validateModulePath(state.Root, permissionPath); err != nil {
		return nil, err
	}
	permissionSource, err := readProjectFile(state.Root, permissionPath)
	if err != nil {
		return nil, err
	}
	inputs[permissionPath] = permissionSource
	if err := validatePurePermissionEntry(permissionSource, "examplePermissions"); err != nil {
		return nil, err
	}
	entries := make([]templates.ModuleEntry, 0, len(state.Metadata.Modules))
	for _, module := range state.Metadata.Modules {
		entries = append(entries, templates.ModuleEntry{Name: module.Name, Type: module.Type})
		if module.Name == "example" {
			continue
		}
		path := "internal/apiserver/bootstrap/module_" + module.Name + ".go"
		if err := validateModulePath(state.Root, path); err != nil {
			return nil, err
		}
		factory, err := readProjectFile(state.Root, path)
		if err != nil {
			return nil, fmt.Errorf("module %q assembly is missing or unsafe: %w", module.Name, err)
		}
		inputs[path] = factory
		if err := validatePurePermissionEntry(factory, module.Name+"Permissions"); err != nil {
			return nil, err
		}
	}
	project := templates.Project{Name: state.Metadata.Name, Module: state.Metadata.Module}
	options := state.Metadata.templateOptions()
	files, err := templates.RenderWithModules(project, options, entries)
	if err != nil {
		return nil, err
	}
	var expected []byte
	for _, file := range files {
		if file.Path == registryPath {
			expected = file.Content
			break
		}
	}
	expected, err = replaceDigestMarker(expected, "modulesSHA256", moduleManifestDigest(state.Metadata.Modules))
	if err != nil {
		return nil, err
	}
	if state.Metadata.Transport != "grpc" {
		expected, err = replaceDigestMarker(expected, "openAPISHA256", digest(state.CurrentOpenAPI))
		if err != nil {
			return nil, err
		}
	}
	if !bytes.Equal(registry, expected) {
		return nil, fmt.Errorf("generated module registration index differs from its manifest projection; preserve custom assembly in module_<name>.go, not modules_gen.go")
	}
	return inputs, nil
}

func validatePurePermissionEntry(source []byte, name string) error {
	file, err := parser.ParseFile(token.NewFileSet(), "module factory", source, 0)
	if err != nil {
		return fmt.Errorf("parse module permissions: %w", err)
	}
	count := 0
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != name {
			continue
		}
		count++
		if function.Recv != nil || function.Type.TypeParams.NumFields() != 0 || function.Type.Params.NumFields() != 0 || function.Type.Results.NumFields() != 1 {
			return fmt.Errorf("module requires pure %s() modulePermissions declaration", name)
		}
		result, ok := function.Type.Results.List[0].Type.(*ast.Ident)
		if !ok || result.Name != "modulePermissions" {
			return fmt.Errorf("module requires pure %s() modulePermissions declaration", name)
		}
	}
	if count != 1 {
		return fmt.Errorf("module requires one pure %s() modulePermissions declaration", name)
	}
	return nil
}

func validateAssemblyCalls(source []byte, transport string) error {
	file, err := parser.ParseFile(token.NewFileSet(), "providers.go", source, 0)
	if err != nil {
		return fmt.Errorf("parse project bootstrap: %w", err)
	}
	required := map[string]int{"newModuleServices": 1}
	if transport == "http" || transport == "dual" {
		required["httpModules"] = 2
	}
	if transport != "http" {
		required["rpcModules"] = 2
	}
	if transport == "gateway" {
		required["gatewayModules"] = 1
	}
	counts := make(map[string]int, len(required))
	var invalid string
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		name, ok := call.Fun.(*ast.Ident)
		if !ok {
			return true
		}
		arguments, needed := required[name.Name]
		if !needed {
			return true
		}
		counts[name.Name]++
		if len(call.Args) != arguments || call.Ellipsis.IsValid() {
			invalid = name.Name
		}
		return true
	})
	if invalid != "" {
		return fmt.Errorf("project bootstrap has an incompatible %s call; explicitly update its module assembly entry", invalid)
	}
	for _, name := range []string{"newModuleServices", "httpModules", "rpcModules", "gatewayModules"} {
		if _, needed := required[name]; !needed {
			continue
		}
		if counts[name] == 0 || (name == "newModuleServices" && counts[name] != 1) {
			return fmt.Errorf("project bootstrap requires one shared newModuleServices construction and protocol registration; missing or repeated %s entry", name)
		}
	}
	return nil
}
