package generator

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func migrationInputs(root string) ([]string, error) {
	if err := validateModulePath(root, "migrations/migrations.go"); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(filepath.Join(root, "migrations"))
	if err != nil {
		return nil, fmt.Errorf("read project migrations: %w", err)
	}
	names := make([]string, 0, len(entries))
	versions := make(map[int64]bool)
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 || entry.IsDir() {
			return nil, fmt.Errorf("migration input %s must be a regular file", entry.Name())
		}
		names = append(names, entry.Name())
		if strings.EqualFold(filepath.Ext(entry.Name()), ".sql") {
			version, err := resourceMigrationVersion(entry.Name())
			if err != nil {
				return nil, err
			}
			if versions[version] {
				return nil, fmt.Errorf("duplicate migration version %d", version)
			}
			versions[version] = true
		}
	}
	if len(versions) == 0 {
		return nil, fmt.Errorf("project migrations contain no versioned SQL")
	}
	return names, nil
}

func resourceMigrationVersion(name string) (int64, error) {
	prefix, _, separated := strings.Cut(name, "_")
	if filepath.Ext(name) != ".sql" || !separated {
		return 0, fmt.Errorf("invalid migration filename %s", name)
	}
	version, err := strconv.ParseInt(prefix, 10, 64)
	if err != nil || version < 1 {
		return 0, fmt.Errorf("invalid migration version in %s", name)
	}
	return version, nil
}

func nextResourceMigration(name string, inputs []string) (string, error) {
	var latest int64
	for _, filename := range inputs {
		if filepath.Ext(filename) != ".sql" {
			continue
		}
		version, err := resourceMigrationVersion(filename)
		if err != nil {
			return "", err
		}
		if version > latest {
			latest = version
		}
	}
	if latest == math.MaxInt64 {
		return "", fmt.Errorf("cannot allocate a resource migration version after %d", latest)
	}
	return fmt.Sprintf("migrations/%05d_%s.sql", latest+1, name), nil
}
