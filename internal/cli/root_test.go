package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestCommandReadOnlyAndFailures(t *testing.T) {
	// A fresh working directory proves the placeholder never generates files.
	t.Chdir(t.TempDir())
	tests := []struct {
		name    string
		args    []string
		want    string
		wantErr bool
	}{
		{name: "help", args: []string{"--help"}, want: "under development"},
		{name: "version", args: []string{"--version"}, want: "v0.0.0-test"},
		{name: "generation unavailable", want: "not implemented", wantErr: true},
		{name: "unexpected argument", args: []string{"shop"}, wantErr: true},
		{name: "unknown flag", args: []string{"--db=sqlite"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			cmd := NewCommand("v0.0.0-test")
			cmd.SetArgs(tt.args)
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			err := cmd.Execute()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
			got := out.String()
			if err != nil {
				got += err.Error()
			}
			if !strings.Contains(got, tt.want) {
				t.Fatalf("output %q does not contain %q", got, tt.want)
			}
			files, err := os.ReadDir(".")
			if err != nil {
				t.Fatal(err)
			}
			if len(files) != 0 {
				t.Fatalf("command created %d files", len(files))
			}
		})
	}
}
