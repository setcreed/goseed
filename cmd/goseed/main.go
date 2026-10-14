package main

import (
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	"github.com/setcreed/goseed/internal/cli"
	"github.com/setcreed/goseed/internal/generator"
)

var version = "dev"
var commit = "unknown"

func main() {
	if err := cli.NewCommand(versionString()).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func versionString() string {
	resolvedVersion := version
	resolvedCommit := commit
	if info, ok := debug.ReadBuildInfo(); ok {
		if resolvedCommit == "unknown" {
			for _, setting := range info.Settings {
				if setting.Key == "vcs.revision" {
					resolvedCommit = setting.Value
					break
				}
			}
		}
	}
	if len(resolvedCommit) > 12 {
		resolvedCommit = resolvedCommit[:12]
	}
	if strings.TrimSpace(resolvedCommit) == "" {
		resolvedCommit = "unknown"
	}
	return fmt.Sprintf("%s (commit %s; template %s; resource spec %d)", resolvedVersion, resolvedCommit, generator.TemplateVersion, generator.ResourceSpecVersion)
}
