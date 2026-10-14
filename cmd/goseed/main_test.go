package main

import (
	"strings"
	"testing"
)

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
