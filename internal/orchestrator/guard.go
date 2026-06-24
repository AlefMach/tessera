package orchestrator

import (
	"fmt"
	"path/filepath"
	"strings"
)

func cleanWorkspaceRelPath(rel string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(strings.TrimSpace(rel)))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || filepath.IsAbs(clean) {
		return "", fmt.Errorf("path outside workspace is not allowed: %s", rel)
	}
	return clean, nil
}

func isSensitiveWorkspacePath(path string) bool {
	path = filepath.ToSlash(strings.TrimSpace(path))
	if path == "" {
		return false
	}

	lower := strings.ToLower(path)
	base := strings.ToLower(filepath.Base(path))

	if lower == ".git" || strings.HasPrefix(lower, ".git/") {
		return true
	}
	if lower == ".tessera" || strings.HasPrefix(lower, ".tessera/") {
		return true
	}
	if strings.HasPrefix(base, ".env") {
		return true
	}

	sensitiveNames := map[string]bool{
		"id_rsa":               true,
		"id_dsa":               true,
		"id_ecdsa":             true,
		"id_ed25519":           true,
		"credentials":          true,
		"credentials.json":     true,
		"service-account.json": true,
	}
	if sensitiveNames[base] {
		return true
	}

	return strings.HasSuffix(base, ".pem") ||
		strings.HasSuffix(base, ".key") ||
		strings.HasSuffix(base, ".p12") ||
		strings.HasSuffix(base, ".pfx")
}
