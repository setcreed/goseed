package generator

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/setcreed/goseed/internal/templates"
)

func TestProjectWriteRollsBackEveryIndexFailure(t *testing.T) {
	for failAt := 1; failAt <= 3; failAt++ {
		t.Run(fmt.Sprint(failAt), func(t *testing.T) {
			root := generatedProject(t, templates.Options{})
			var updates []fileUpdate
			for _, path := range []string{"goseed.yaml", "internal/apiserver/bootstrap/modules_gen.go", "api/openapi/swagger.json"} {
				update, err := resourceUpdate(root, path, []byte("replacement"))
				if err != nil {
					t.Fatal(err)
				}
				updates = append(updates, update)
			}
			before := resourceTree(t, root)
			cause := errors.New("injected index failure")
			writes := 0
			err := writeProjectFiles(root, []templates.File{{Path: "new/nested/business.go", Content: []byte("new")}}, updates, func(path string, data []byte, mode os.FileMode) error {
				writes++
				if writes == failAt {
					return cause
				}
				return writeAtomic(path, data, mode)
			})
			if !errors.Is(err, cause) {
				t.Fatalf("original error lost: %v", err)
			}
			if !reflect.DeepEqual(before, resourceTree(t, root)) {
				t.Fatal("rollback changed original bytes, permissions or paths")
			}
		})
	}
}

func TestProjectWritePreservesNewOutputConflict(t *testing.T) {
	root := generatedProject(t, templates.Options{})
	before := resourceTree(t, root)
	err := writeProjectFiles(root, []templates.File{
		{Path: "new/business.go", Content: []byte("new")},
		{Path: "README.md", Content: []byte("must not replace")},
	}, nil, writeAtomic)
	if err == nil {
		t.Fatal("output conflict accepted")
	}
	if !reflect.DeepEqual(before, resourceTree(t, root)) {
		t.Fatal("conflict damaged existing file or left created paths")
	}
}

func TestProjectWriteReportsRollbackFailureAndCleansCreates(t *testing.T) {
	root := generatedProject(t, templates.Options{})
	first, err := resourceUpdate(root, "goseed.yaml", []byte("replacement"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := resourceUpdate(root, "api/openapi/swagger.json", []byte("replacement"))
	if err != nil {
		t.Fatal(err)
	}
	cause := errors.New("injected commit failure")
	calls := 0
	err = writeProjectFiles(root, []templates.File{{Path: "new/business.go", Content: []byte("new")}}, []fileUpdate{first, second}, func(path string, data []byte, mode os.FileMode) error {
		calls++
		if calls == 2 {
			return cause
		}
		if calls == 3 {
			return errors.New("injected restore failure")
		}
		return writeAtomic(path, data, mode)
	})
	if !errors.Is(err, cause) || !strings.Contains(err.Error(), "injected restore failure") || !strings.Contains(err.Error(), "goseed.yaml") {
		t.Fatalf("commit or rollback evidence lost: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(root, "new")); !os.IsNotExist(err) {
		t.Fatal("new files survived failed restore")
	}
}
