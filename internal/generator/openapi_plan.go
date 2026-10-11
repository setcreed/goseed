package generator

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// fileUpdate is a generated file replacement guarded by its original bytes.
type fileUpdate struct {
	Path     string
	Original []byte
	Content  []byte
	Mode     os.FileMode
}

// OpenAPIUpdatePlan safely refreshes the aggregate Swagger index and its
// generated digest after a registered module source changes.
type OpenAPIUpdatePlan struct {
	root        string
	inputs      map[string][]byte
	updates     []fileUpdate
	desiredJSON []byte
}

// PrepareOpenAPIRefresh computes the new aggregate without changing the
// project. The plan is empty when all registered sources are already indexed.
func PrepareOpenAPIRefresh(root string) (OpenAPIUpdatePlan, error) {
	state, err := LoadProject(root)
	if err != nil {
		return OpenAPIUpdatePlan{}, err
	}
	plan := OpenAPIUpdatePlan{root: state.Root, inputs: make(map[string][]byte)}
	if state.Metadata.Transport == "grpc" {
		return OpenAPIUpdatePlan{}, fmt.Errorf("gRPC-only projects do not have an OpenAPI index")
	}
	metadata, err := readProjectFile(state.Root, "goseed.yaml")
	if err != nil {
		return OpenAPIUpdatePlan{}, err
	}
	plan.inputs["goseed.yaml"] = metadata
	for _, source := range state.Sources {
		name := "api/openapi/" + source.Name + ".yaml"
		plan.inputs[name] = bytes.Clone(source.YAML)
	}
	if !state.OpenAPIIndexStale {
		return plan, nil
	}
	plan.desiredJSON = bytes.Clone(state.OpenAPI.JSON)

	indexPath := "api/openapi/swagger.json"
	indexInfo, err := os.Lstat(filepath.Join(state.Root, filepath.FromSlash(indexPath)))
	if err != nil {
		return OpenAPIUpdatePlan{}, fmt.Errorf("inspect %s: %w", indexPath, err)
	}
	if indexInfo.Mode()&os.ModeSymlink != 0 || !indexInfo.Mode().IsRegular() {
		return OpenAPIUpdatePlan{}, fmt.Errorf("inspect %s: expected a regular file", indexPath)
	}
	registryPath := "internal/apiserver/bootstrap/modules_gen.go"
	registryInfo, err := os.Lstat(filepath.Join(state.Root, filepath.FromSlash(registryPath)))
	if err != nil {
		return OpenAPIUpdatePlan{}, fmt.Errorf("inspect %s: %w", registryPath, err)
	}
	if registryInfo.Mode()&os.ModeSymlink != 0 || !registryInfo.Mode().IsRegular() {
		return OpenAPIUpdatePlan{}, fmt.Errorf("inspect %s: expected a regular file", registryPath)
	}
	registry, err := readProjectFile(state.Root, registryPath)
	if err != nil {
		return OpenAPIUpdatePlan{}, err
	}
	oldDigest := digest(state.CurrentOpenAPI)
	newDigest := state.OpenAPI.Digest
	oldMarker := []byte(`const openAPISHA256 = "` + oldDigest + `"`)
	newMarker := []byte(`const openAPISHA256 = "` + newDigest + `"`)
	if bytes.Count(registry, oldMarker) != 1 {
		return OpenAPIUpdatePlan{}, fmt.Errorf("module registration index has no unique digest marker for the current Swagger index")
	}
	updatedRegistry := bytes.Replace(registry, oldMarker, newMarker, 1)

	plan.updates = []fileUpdate{
		{Path: indexPath, Original: bytes.Clone(state.CurrentOpenAPI), Content: bytes.Clone(state.OpenAPI.JSON), Mode: indexInfo.Mode().Perm()},
		{Path: registryPath, Original: registry, Content: updatedRegistry, Mode: registryInfo.Mode().Perm()},
	}
	sort.Slice(plan.updates, func(i, j int) bool { return plan.updates[i].Path < plan.updates[j].Path })
	return plan, nil
}

// Report writes the files that would be updated without modifying the project.
func (p OpenAPIUpdatePlan) Report(w io.Writer, dryRun bool) error {
	if len(p.updates) == 0 {
		_, err := io.WriteString(w, "OpenAPI index is up to date.\n")
		return err
	}
	if _, err := io.WriteString(w, "OpenAPI index refresh:\n"); err != nil {
		return err
	}
	for _, update := range p.updates {
		if _, err := fmt.Fprintf(w, "  ~ %s\n", update.Path); err != nil {
			return err
		}
	}
	if dryRun {
		_, err := io.WriteString(w, "Dry run: no files written.\n")
		return err
	}
	return nil
}

// Write revalidates all inputs and targets before atomically replacing each
// generated file. If a later replacement fails, earlier replacements roll back.
func (p OpenAPIUpdatePlan) Write() error {
	return p.writeWith(writeAtomic)
}

type atomicWriter func(path string, data []byte, mode os.FileMode) error

func (p OpenAPIUpdatePlan) writeWith(write atomicWriter) error {
	if len(p.updates) == 0 {
		return nil
	}
	state, err := LoadProject(p.root)
	if err != nil {
		return fmt.Errorf("revalidate project before OpenAPI update: %w", err)
	}
	if !bytes.Equal(state.OpenAPI.JSON, p.desiredJSON) {
		return fmt.Errorf("registered OpenAPI sources changed after the update plan was prepared")
	}
	inputNames := make([]string, 0, len(p.inputs))
	for relative := range p.inputs {
		inputNames = append(inputNames, relative)
	}
	sort.Strings(inputNames)
	for _, relative := range inputNames {
		expected := p.inputs[relative]
		actual, err := readProjectFile(p.root, relative)
		if err != nil {
			return err
		}
		if !bytes.Equal(actual, expected) {
			return fmt.Errorf("project input %s changed after the update plan was prepared", relative)
		}
	}
	for _, update := range p.updates {
		if !allowedOpenAPIUpdate(update.Path) {
			return fmt.Errorf("unsupported generated file update %s", update.Path)
		}
		actual, err := readProjectFile(p.root, update.Path)
		if err != nil {
			return err
		}
		if !bytes.Equal(actual, update.Original) {
			return fmt.Errorf("generated file %s changed after the update plan was prepared", update.Path)
		}
		if err := validateUpdateParents(p.root, update.Path); err != nil {
			return err
		}
	}

	written := make([]fileUpdate, 0, len(p.updates))
	for _, update := range p.updates {
		filename := filepath.Join(p.root, filepath.FromSlash(update.Path))
		if err := write(filename, update.Content, update.Mode); err != nil {
			rollbackErrors := make([]string, 0)
			for i := len(written) - 1; i >= 0; i-- {
				previous := written[i]
				path := filepath.Join(p.root, filepath.FromSlash(previous.Path))
				if restoreErr := write(path, previous.Original, previous.Mode); restoreErr != nil {
					rollbackErrors = append(rollbackErrors, previous.Path+": "+restoreErr.Error())
				}
			}
			cause := fmt.Errorf("update %s: %w", update.Path, err)
			if len(rollbackErrors) > 0 {
				return fmt.Errorf("%w; rollback failed for %s", cause, strings.Join(rollbackErrors, ", "))
			}
			return cause
		}
		written = append(written, update)
	}
	return nil
}

func validateUpdateParents(root, relative string) error {
	if !filepath.IsLocal(filepath.FromSlash(relative)) || !allowedOpenAPIUpdate(relative) {
		return fmt.Errorf("unsafe generated file update %s", relative)
	}
	for parent := filepath.Dir(filepath.Join(root, filepath.FromSlash(relative))); parent != root && parent != "."; parent = filepath.Dir(parent) {
		info, err := os.Lstat(parent)
		if err != nil {
			return fmt.Errorf("inspect update parent %s: %w", parent, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("unsafe update parent %s", parent)
		}
	}
	return nil
}

func allowedOpenAPIUpdate(relative string) bool {
	return relative == "api/openapi/swagger.json" || relative == "internal/apiserver/bootstrap/modules_gen.go"
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func writeAtomic(filename string, data []byte, mode os.FileMode) (result error) {
	directory := filepath.Dir(filename)
	temporary, err := os.CreateTemp(directory, ".goseed-update-*")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer func() {
		_ = os.Remove(temporaryName)
	}()
	if err := temporary.Chmod(mode); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryName, filename); err != nil {
		return err
	}
	return nil
}
