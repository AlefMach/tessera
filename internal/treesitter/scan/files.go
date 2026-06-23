package scan

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func SourceFiles(root string, extensions map[string]bool) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := entry.Name()
		if entry.IsDir() {
			if shouldSkipDir(name) {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() || !extensions[strings.ToLower(filepath.Ext(path))] {
			return nil
		}
		files = append(files, path)
		return nil
	})
	sort.Strings(files)
	return files, err
}

func HasTestsNearby(root, rel string) bool {
	dir := filepath.Dir(filepath.Join(root, rel))
	base := strings.TrimSuffix(filepath.Base(rel), filepath.Ext(rel))
	ext := filepath.Ext(rel)
	candidates := []string{
		filepath.Join(dir, base+"_test"+ext),
		filepath.Join(dir, base+".test"+ext),
		filepath.Join(dir, base+".spec"+ext),
		filepath.Join(dir, "test_"+base+ext),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return true
		}
	}
	lowerRel := strings.ToLower(rel)
	return strings.Contains(lowerRel, "_test.") || strings.Contains(lowerRel, ".test.") || strings.Contains(lowerRel, ".spec.") || strings.Contains(lowerRel, "/test/") || strings.Contains(lowerRel, "/tests/")
}

func shouldSkipDir(name string) bool {
	switch name {
	case ".git", ".tessera", "node_modules", "vendor", "dist", "build", "coverage", ".next", ".venv", "venv", "__pycache__":
		return true
	default:
		return false
	}
}
