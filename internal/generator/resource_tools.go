package generator

import (
	"context"
	"debug/buildinfo"
	"fmt"
	"go/version"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ToolRequirement is a pinned, project-local protocol executable.
type ToolRequirement struct {
	Name    string
	Path    string
	Version string
	Module  string
}

func requiredResourceTools(transport string) []ToolRequirement {
	if transport == "http" {
		return nil
	}
	tools := []ToolRequirement{
		{Name: "protoc", Path: ".tools/protoc/bin/protoc", Version: "36.2"},
		{Name: "protoc-gen-go", Path: ".tools/bin/protoc-gen-go", Version: "v1.36.12", Module: "google.golang.org/protobuf"},
		{Name: "protoc-gen-go-grpc", Path: ".tools/bin/protoc-gen-go-grpc", Version: "v1.6.2", Module: "google.golang.org/grpc/cmd/protoc-gen-go-grpc"},
	}
	if transport == "gateway" {
		tools = append(tools,
			ToolRequirement{Name: "protoc-gen-grpc-gateway", Path: ".tools/bin/protoc-gen-grpc-gateway", Version: "v2.31.0", Module: "github.com/grpc-ecosystem/grpc-gateway/v2"},
			ToolRequirement{Name: "protoc-gen-openapiv2", Path: ".tools/bin/protoc-gen-openapiv2", Version: "v2.31.0", Module: "github.com/grpc-ecosystem/grpc-gateway/v2"})
	}
	return tools
}

func checkResourceTool(root string, tool ToolRequirement) error {
	filename := filepath.Join(root, filepath.FromSlash(tool.Path))
	info, err := os.Lstat(filename)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
		return fmt.Errorf("protocol tool %s must be a regular executable at %s", tool.Name, tool.Path)
	}
	if tool.Name == "protoc" {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		output, err := exec.CommandContext(ctx, filename, "--version").Output()
		if err != nil {
			return fmt.Errorf("cannot check protocol tool %s: %w", tool.Name, err)
		}
		if strings.TrimSpace(string(output)) != "libprotoc "+tool.Version {
			return fmt.Errorf("protocol tool %s version mismatch; expected %s", tool.Name, tool.Version)
		}
		return nil
	}
	infoBuild, err := buildinfo.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("cannot inspect protocol tool %s build identity: %w", tool.Name, err)
	}
	if infoBuild.Main.Path != tool.Module || infoBuild.Main.Version != tool.Version || infoBuild.Main.Replace != nil || !compatibleProtocolGo(infoBuild.GoVersion) {
		return fmt.Errorf("protocol tool %s build identity mismatch; expected %s %s built with Go >= 1.26.4", tool.Name, tool.Module, tool.Version)
	}
	return nil
}

func compatibleProtocolGo(value string) bool {
	return version.IsValid(value) && version.Compare(value, "go1.26.4") >= 0
}
