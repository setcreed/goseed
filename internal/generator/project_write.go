package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/setcreed/goseed/internal/templates"
)

// writeProjectFiles commits a validated module or resource plan. Callers own
// input/path revalidation; this function exclusively creates new files, replaces
// generated indexes and rolls back only changes owned by this invocation.
func writeProjectFiles(root string, creates []templates.File, updates []fileUpdate, write atomicWriter) error {
	createdFiles := make([]string, 0, len(creates))
	createdDirs := make([]string, 0, len(creates))
	rollbackCreates := func() []string {
		var failures []string
		for i := len(createdFiles) - 1; i >= 0; i-- {
			if err := os.Remove(createdFiles[i]); err != nil && !os.IsNotExist(err) {
				failures = append(failures, createdFiles[i]+": "+err.Error())
			}
		}
		for i := len(createdDirs) - 1; i >= 0; i-- {
			if err := os.Remove(createdDirs[i]); err != nil && !os.IsNotExist(err) {
				failures = append(failures, createdDirs[i]+": "+err.Error())
			}
		}
		return failures
	}
	for _, file := range creates {
		filename := filepath.Join(root, filepath.FromSlash(file.Path))
		dirs, err := makeParents(root, filepath.Dir(filename))
		createdDirs = append(createdDirs, dirs...)
		if err != nil {
			return projectWriteError(fmt.Errorf("create parent for %s: %w", file.Path, err), rollbackCreates())
		}
		output, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if err != nil {
			return projectWriteError(fmt.Errorf("create %s: %w", file.Path, err), rollbackCreates())
		}
		createdFiles = append(createdFiles, filename)
		if _, err := output.Write(file.Content); err != nil {
			_ = output.Close()
			return projectWriteError(fmt.Errorf("write %s: %w", file.Path, err), rollbackCreates())
		}
		if err := output.Close(); err != nil {
			return projectWriteError(fmt.Errorf("close %s: %w", file.Path, err), rollbackCreates())
		}
	}
	written := make([]fileUpdate, 0, len(updates))
	for _, update := range updates {
		filename := filepath.Join(root, filepath.FromSlash(update.Path))
		if err := write(filename, update.Content, update.Mode); err != nil {
			var rollbackErrors []string
			for i := len(written) - 1; i >= 0; i-- {
				previous := written[i]
				path := filepath.Join(root, filepath.FromSlash(previous.Path))
				if restoreErr := write(path, previous.Original, previous.Mode); restoreErr != nil {
					rollbackErrors = append(rollbackErrors, previous.Path+": "+restoreErr.Error())
				}
			}
			cleanupErrors := rollbackCreates()
			return projectWriteError(fmt.Errorf("update %s: %w", update.Path, err), append(rollbackErrors, cleanupErrors...))
		}
		written = append(written, update)
	}
	return nil
}

func projectWriteError(cause error, rollbackErrors []string) error {
	if len(rollbackErrors) > 0 {
		return fmt.Errorf("%w; rollback failed for %s", cause, strings.Join(rollbackErrors, ", "))
	}
	return cause
}
