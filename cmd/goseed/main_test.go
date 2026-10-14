package main

import (
	"runtime/debug"
	"strings"
	"testing"
)

func TestFormatVersionFromInstallBuildInfo(t *testing.T) {
	tests := []struct {
		name, version, commit, moduleVersion, want string
	}{
		{"tagged install", "dev", "unknown", "v0.1.0", "v0.1.0 (commit unknown;"},
		{"pseudo-version install", "dev", "unknown", "v0.0.0-20261014000000-0123456789ab", "v0.0.0-20261014000000-0123456789ab (commit unknown;"},
		{"local build", "dev", "unknown", "(devel)", "dev (commit unknown;"},
		{"empty module version", "dev", "unknown", "", "dev (commit unknown;"},
		{"injected release", "v0.2.0", "abcdef0123456789", "v0.1.0", "v0.2.0 (commit abcdef012345;"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &debug.BuildInfo{Main: debug.Module{Version: tt.moduleVersion}}
			if got := formatVersion(tt.version, tt.commit, info); !strings.HasPrefix(got, tt.want) {
				t.Fatalf("formatVersion() = %q, want prefix %q", got, tt.want)
			}
		})
	}
	info := &debug.BuildInfo{Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "0123456789abcdef"}}}
	if got := formatVersion("dev", "unknown", info); !strings.Contains(got, "commit 0123456789ab;") {
		t.Fatalf("missing VCS revision: %q", got)
	}
	if got := formatVersion("dev", "", nil); !strings.Contains(got, "commit unknown;") {
		t.Fatalf("missing fallback for absent build info: %q", got)
	}
}

func TestVersionStringIncludesIndependentVersionsAndCommit(t *testing.T) {
	oldVersion, oldCommit := version, commit
	t.Cleanup(func() {
		version, commit = oldVersion, oldCommit
	})
	version = "v0.1.0"
	commit = "0123456789abcdef0123456789abcdef01234567"

	got := versionString()
	for _, want := range []string{"v0.1.0", "0123456789ab", "template 1", "resource spec 1"} {
		if !strings.Contains(got, want) {
			t.Fatalf("version string %q does not contain %q", got, want)
		}
	}
}

func TestVersionStringKeepsDevelopmentVersion(t *testing.T) {
	oldVersion, oldCommit := version, commit
	t.Cleanup(func() {
		version, commit = oldVersion, oldCommit
	})
	version = "dev"
	commit = "unknown"

	if got := versionString(); !strings.HasPrefix(got, "dev (commit ") {
		t.Fatalf("versionString() = %q, want dev build", got)
	}
}
