package generator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/setcreed/goseed/internal/templates"
)

// renderResourceProtocol compiles only the new module's Proto inputs in a
// private temporary directory. It never edits the project during preparation.
func renderResourceProtocol(root, module, name string, transport string, files []templates.File) ([]templates.File, error) {
	find := func(path string) ([]byte, error) {
		for _, file := range files {
			if file.Path == path {
				return file.Content, nil
			}
		}
		return nil, fmt.Errorf("resource protocol input %s is missing", path)
	}
	protoPath := "api/proto/" + name + "/v1/" + name + ".proto"
	proto, err := find(protoPath)
	if err != nil {
		return nil, err
	}
	temporary, err := os.MkdirTemp("", "goseed-resource-proto-")
	if err != nil {
		return nil, fmt.Errorf("create temporary protocol workspace: %w", err)
	}
	defer func() { _ = os.RemoveAll(temporary) }()
	input := filepath.Join(temporary, name+".proto")
	if err := os.WriteFile(input, proto, 0600); err != nil {
		return nil, fmt.Errorf("stage resource Proto: %w", err)
	}
	output := filepath.Join(temporary, "out")
	if err := os.Mkdir(output, 0700); err != nil {
		return nil, fmt.Errorf("create temporary protocol output: %w", err)
	}
	protoc := filepath.Join(root, ".tools/protoc/bin/protoc")
	include := filepath.Join(root, ".tools/protoc/include")
	goPackage := module + "/pkg/api/" + name + "/v1;" + name + "v1"
	args := []string{"-I" + temporary, "-I" + include,
		"--go_out=paths=source_relative:" + output,
		"--go-grpc_out=paths=source_relative:" + output,
		"--go_opt=M" + name + ".proto=" + goPackage,
		"--go-grpc_opt=M" + name + ".proto=" + goPackage}
	env := make([]string, 0, len(os.Environ())+1)
	for _, variable := range os.Environ() {
		if !strings.HasPrefix(variable, "PATH=") {
			env = append(env, variable)
		}
	}
	env = append(env, "PATH="+filepath.Join(root, ".tools/bin")+string(os.PathListSeparator)+os.Getenv("PATH"))
	if transport == "gateway" {
		httpConfig, err := find("api/proto/" + name + "/v1/http.yaml")
		if err != nil {
			return nil, err
		}
		openAPIConfig, err := find("api/proto/" + name + "/v1/openapi.yaml")
		if err != nil {
			return nil, err
		}
		httpPath, openAPIPath := filepath.Join(temporary, "http.yaml"), filepath.Join(temporary, "openapi.yaml")
		if err := os.WriteFile(httpPath, httpConfig, 0600); err != nil {
			return nil, err
		}
		if err := os.WriteFile(openAPIPath, openAPIConfig, 0600); err != nil {
			return nil, err
		}
		args = append(args,
			"--grpc-gateway_out=paths=source_relative,grpc_api_configuration="+httpPath+":"+output,
			"--grpc-gateway_opt=M"+name+".proto="+goPackage,
			"--openapiv2_out=json_names_for_fields=false,openapi_configuration="+openAPIPath+",grpc_api_configuration="+httpPath+":"+output)
		args = append(args, input)
		cmd := exec.Command(protoc, args...)
		cmd.Dir = root
		cmd.Env = env
		if data, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("generate Gateway resource protocol: %w: %s", err, strings.TrimSpace(string(data)))
		}
	} else {
		args = append(args, input)
		cmd := exec.Command(protoc, args...)
		cmd.Dir = root
		cmd.Env = env
		if data, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("generate resource protocol: %w: %s", err, strings.TrimSpace(string(data)))
		}
	}
	result := []templates.File{}
	for _, suffix := range []string{".pb.go", "_grpc.pb.go"} {
		filename := filepath.Join(output, name+suffix)
		data, err := os.ReadFile(filename)
		if err != nil {
			return nil, fmt.Errorf("read generated protocol file %s: %w", filename, err)
		}
		result = append(result, templates.File{Path: "pkg/api/" + name + "/v1/" + name + suffix, Content: data})
	}
	if transport == "gateway" {
		filename := filepath.Join(output, name+".pb.gw.go")
		data, err := os.ReadFile(filename)
		if err != nil {
			return nil, fmt.Errorf("read generated Gateway file: %w", err)
		}
		result = append(result, templates.File{Path: "pkg/api/" + name + "/v1/" + name + ".pb.gw.go", Content: data})
		filename = filepath.Join(output, name+".swagger.json")
		swagger, err := os.ReadFile(filename)
		if err != nil {
			return nil, fmt.Errorf("read generated Gateway Swagger: %w", err)
		}
		canonical, err := canonicalJSON(swagger)
		if err != nil {
			return nil, fmt.Errorf("canonicalize generated Gateway Swagger: %w", err)
		}
		result = append(result, templates.File{Path: "api/openapi/" + name + ".yaml", Content: canonical})
	}
	return result, nil
}

func canonicalJSON(data []byte) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}
