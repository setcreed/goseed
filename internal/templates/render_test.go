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

func TestVariantPlansAndInvalidOptions(t *testing.T) {
	project := Project{Name: "demo", Module: "example.com/demo"}
	for _, backend := range []string{"mysql", "postgres", "none"} {
		first, err := Render(project, Options{Database: backend})
		if err != nil {
			t.Fatal(err)
		}
		second, err := Render(project, Options{Database: backend})
		if err != nil || len(first) != len(second) {
			t.Fatal("inconsistent variant plan")
		}
		for i, file := range first {
			if file.Path != second[i].Path || !bytes.Equal(file.Content, second[i].Content) {
				t.Fatal("non-deterministic variant plan")
			}
			if i > 0 && first[i-1].Path >= file.Path {
				t.Fatal("plan order or duplicate path")
			}
			if file.Path == "configs/app.example.yaml" && bytes.Contains(file.Content, []byte("busy_timeout")) {
				t.Fatal("SQLite configuration leaked into SQL project")
			}
		}
	}
	for _, invalid := range []string{"mongodb", "MYSQL", "../mysql", "mysql\n"} {
		if files, err := Render(project, Options{Database: invalid}); err == nil || files != nil {
			t.Fatal("invalid option returned a file plan")
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

func TestCasbinPlansAndInvalidOptions(t *testing.T) {
	project := Project{Name: "demo", Module: "example.com/demo"}
	for _, backend := range []string{"sqlite", "mysql", "postgres", "none"} {
		for _, redis := range []bool{false, true} {
			options := Options{Database: backend, Redis: redis, Authz: "casbin"}
			first, err := Render(project, options)
			if err != nil {
				t.Fatal(err)
			}
			second, err := Render(project, options)
			if err != nil || len(first) != len(second) {
				t.Fatal("inconsistent Casbin plan")
			}
			for i, file := range first {
				if file.Path != second[i].Path || !bytes.Equal(file.Content, second[i].Content) {
					t.Fatal("non-deterministic Casbin plan")
				}
				if i > 0 && first[i-1].Path >= file.Path {
					t.Fatal("plan order or duplicate")
				}
			}
		}
	}
	for _, authz := range []string{"CASBIN", "rbac", "../casbin", "casbin\n"} {
		if files, err := Render(project, Options{Authz: authz}); err == nil || files != nil {
			t.Fatal("invalid authorization produced a plan")
		}
	}
}

func TestRedisPlanDeterminism(t *testing.T) {
	project := Project{Name: "demo", Module: "example.com/demo"}
	for _, backend := range []string{"sqlite", "mysql", "postgres", "none"} {
		t.Run(backend, func(t *testing.T) {
			options := Options{Database: backend, Redis: true}
			first, err := Render(project, options)
			if err != nil {
				t.Fatal(err)
			}
			second, err := Render(project, options)
			if err != nil || len(first) == 0 || len(first) != len(second) {
				t.Fatal("inconsistent Redis plan")
			}
			for i, file := range first {
				if file.Path != second[i].Path || !bytes.Equal(file.Content, second[i].Content) {
					t.Fatal("non-deterministic Redis plan")
				}
				if i > 0 && first[i-1].Path >= file.Path {
					t.Fatal("plan order or duplicate path")
				}
			}
		})
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

func TestSystemMetadataAndDotFiles(t *testing.T) {
	source := fstest.MapFS{
		"base/.DS_Store":          &fstest.MapFile{Data: []byte("OS metadata")},
		"base/Thumbs.db":          &fstest.MapFile{Data: []byte("OS metadata")},
		"base/dot.gitignore.tmpl": &fstest.MapFile{Data: []byte("/data/\n")},
	}
	files, err := render(source, Project{})
	if err != nil || len(files) != 1 || files[0].Path != ".gitignore" {
		t.Fatalf("metadata/dot file handling: %v %+v", err, files)
	}
}
