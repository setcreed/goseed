package templates

import (
	"bytes"
	"testing"
	"testing/fstest"
)

func TestDefaultPlan(t *testing.T) {
	first, err := Default(Project{Name: "demo", Module: "example.com/demo"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := Default(Project{Name: "demo", Module: "example.com/demo"})
	if err != nil {
		t.Fatal(err)
	}
	if len(first) == 0 || len(first) != len(second) {
		t.Fatal("empty or inconsistent file plan")
	}
	for i, file := range first {
		if file.Path != second[i].Path || !bytes.Equal(file.Content, second[i].Content) {
			t.Fatal("non-deterministic rendering")
		}
		if bytes.Contains(file.Content, []byte("[[.")) {
			t.Fatalf("unexpanded variable: %s", file.Path)
		}
	}
}

func TestInvalidProject(t *testing.T) {
	for _, project := range []Project{{Name: "../demo", Module: "example.com/demo"}, {Name: "demo", Module: "example.com/../demo"}, {Name: "demo", Module: "example.com/demo\";panic(1)"}, {Name: "demo", Module: ""}} {
		if files, err := Default(project); err == nil || files != nil {
			t.Fatalf("accepted invalid project: %+v", project)
		}
	}
}

func TestRenderFailuresReturnNoPartialPlan(t *testing.T) {
	for _, invalid := range []string{"[[.Missing]]", "[[", "package invalid; func {"} {
		source := fstest.MapFS{"base/a.txt.tmpl": &fstest.MapFile{Data: []byte("valid")}, "base/b.go.tmpl": &fstest.MapFile{Data: []byte(invalid)}}
		if files, err := render(source, Project{}); err == nil || files != nil {
			t.Fatal("render failure returned partial plan")
		}
	}
}
