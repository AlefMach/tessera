package project

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	ModeEmptyProject    = "empty_project"
	ModeExistingProject = "existing_project"
)

type ProjectProfile struct {
	SessionID  string    `json:"session_id"`
	Root       string    `json:"root"`
	Mode       string    `json:"mode"`
	Stack      string    `json:"stack"`
	Stacks     []string  `json:"stacks"`
	Manifests  []string  `json:"manifests"`
	HasGit     bool      `json:"has_git"`
	HasTests   bool      `json:"has_tests"`
	TestPaths  []string  `json:"test_paths"`
	TestRunner string    `json:"test_runner"`
	ProfiledAt time.Time `json:"profiled_at"`
}

func Profile(root string) (ProjectProfile, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return ProjectProfile{}, err
	}

	manifests := detectManifests(absRoot)
	stacks := stacksForManifests(manifests)
	testPaths, err := detectTests(absRoot)
	if err != nil {
		return ProjectProfile{}, err
	}

	profile := ProjectProfile{
		Root:       absRoot,
		Mode:       ModeExistingProject,
		Stack:      stackLabel(stacks),
		Stacks:     stacks,
		Manifests:  manifests,
		HasGit:     pathExists(filepath.Join(absRoot, ".git")),
		HasTests:   len(testPaths) > 0,
		TestPaths:  testPaths,
		TestRunner: testRunnerFor(absRoot, stacks, manifests),
		ProfiledAt: time.Now().UTC(),
	}
	if isEmptyProject(absRoot) {
		profile.Mode = ModeEmptyProject
	}
	if profile.Stack == "" {
		profile.Stack = "unknown"
	}
	if profile.TestRunner == "" {
		profile.TestRunner = "unknown"
	}
	return profile, nil
}

func detectManifests(root string) []string {
	candidates := []string{
		"go.mod",
		"package.json",
		"pyproject.toml",
		"requirements.txt",
		"Cargo.toml",
		"mix.exs",
		"pom.xml",
		"build.gradle",
		"docker-compose.yml",
	}
	manifests := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if pathExists(filepath.Join(root, candidate)) {
			manifests = append(manifests, candidate)
		}
	}
	return manifests
}

func stacksForManifests(manifests []string) []string {
	seen := map[string]bool{}
	var stacks []string
	for _, manifest := range manifests {
		stack := stackForManifest(manifest)
		if stack != "" && !seen[stack] {
			seen[stack] = true
			stacks = append(stacks, stack)
		}
	}
	return stacks
}

func stackForManifest(manifest string) string {
	switch manifest {
	case "go.mod":
		return "Go"
	case "package.json":
		return "Node"
	case "pyproject.toml", "requirements.txt":
		return "Python"
	case "Cargo.toml":
		return "Rust"
	case "mix.exs":
		return "Elixir"
	case "pom.xml", "build.gradle":
		return "Java"
	case "docker-compose.yml":
		return "Docker"
	default:
		return ""
	}
}

func detectTests(root string) ([]string, error) {
	var tests []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := entry.Name()
		if entry.IsDir() {
			if shouldSkipDir(name) && path != root {
				return filepath.SkipDir
			}
			if isTestDir(name) && path != root {
				rel, err := filepath.Rel(root, path)
				if err != nil {
					return err
				}
				tests = append(tests, filepath.ToSlash(rel)+"/")
			}
			return nil
		}
		if isTestFile(name) {
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			tests = append(tests, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(tests)
	return dedupe(tests), nil
}

func isEmptyProject(root string) bool {
	entries, err := os.ReadDir(root)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		switch entry.Name() {
		case ".git", ".tessera":
			continue
		default:
			return false
		}
	}
	return true
}

func shouldSkipDir(name string) bool {
	switch name {
	case ".git", ".tessera", "node_modules", "vendor", "target", "dist", "build", ".venv", "venv", "__pycache__":
		return true
	default:
		return false
	}
}

func isTestDir(name string) bool {
	switch strings.ToLower(name) {
	case "test", "tests", "spec":
		return true
	default:
		return false
	}
}

func isTestFile(name string) bool {
	return strings.Contains(name, "_test.")
}

func testRunnerFor(root string, stacks, manifests []string) string {
	manifestSet := make(map[string]bool, len(manifests))
	for _, manifest := range manifests {
		manifestSet[manifest] = true
	}

	for _, stack := range stacks {
		switch stack {
		case "Go":
			return "go test ./..."
		case "Node":
			switch {
			case pathExists(filepath.Join(root, "pnpm-lock.yaml")):
				return "pnpm test"
			case pathExists(filepath.Join(root, "yarn.lock")):
				return "yarn test"
			default:
				return "npm test"
			}
		case "Python":
			if manifestSet["pyproject.toml"] {
				return "pytest"
			}
			return "python -m pytest"
		case "Rust":
			return "cargo test"
		case "Elixir":
			return "mix test"
		case "Java":
			if manifestSet["pom.xml"] {
				return "mvn test"
			}
			if pathExists(filepath.Join(root, "gradlew")) {
				return "./gradlew test"
			}
			return "gradle test"
		}
	}
	return ""
}

func stackLabel(stacks []string) string {
	if len(stacks) == 0 {
		return ""
	}
	return strings.Join(stacks, ", ")
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func dedupe(values []string) []string {
	if len(values) < 2 {
		return values
	}
	out := values[:0]
	var previous string
	for i, value := range values {
		if i == 0 || value != previous {
			out = append(out, value)
		}
		previous = value
	}
	return out
}
