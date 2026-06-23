package trust

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"time"
)

const envTrustStore = "TESSERA_TRUST_STORE"

type Store struct {
	path string
}

type trustedFile struct {
	Paths []trustedPath `json:"paths"`
}

type trustedPath struct {
	Path      string    `json:"path"`
	TrustedAt time.Time `json:"trusted_at"`
}

func NewStore() (Store, error) {
	if path := os.Getenv(envTrustStore); path != "" {
		return Store{path: path}, nil
	}

	configDir, err := os.UserConfigDir()
	if err != nil {
		return Store{}, err
	}
	return Store{path: filepath.Join(configDir, "tessera", "trusted-folders.json")}, nil
}

func NewStoreAt(path string) Store {
	return Store{path: path}
}

func (s Store) Path() string {
	return s.path
}

func (s Store) IsTrusted(path string) (bool, error) {
	normalized, err := normalizePath(path)
	if err != nil {
		return false, err
	}

	file, err := s.read()
	if err != nil {
		return false, err
	}

	return slices.ContainsFunc(file.Paths, func(item trustedPath) bool {
		return item.Path == normalized
	}), nil
}

func (s Store) Trust(path string) error {
	normalized, err := normalizePath(path)
	if err != nil {
		return err
	}

	file, err := s.read()
	if err != nil {
		return err
	}

	for i, item := range file.Paths {
		if item.Path == normalized {
			file.Paths[i].TrustedAt = time.Now().UTC()
			return s.write(file)
		}
	}

	file.Paths = append(file.Paths, trustedPath{
		Path:      normalized,
		TrustedAt: time.Now().UTC(),
	})
	return s.write(file)
}

func (s Store) read() (trustedFile, error) {
	payload, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return trustedFile{}, nil
		}
		return trustedFile{}, err
	}
	if len(payload) == 0 {
		return trustedFile{}, nil
	}

	var file trustedFile
	if err := json.Unmarshal(payload, &file); err != nil {
		return trustedFile{}, err
	}
	return file, nil
}

func (s Store) write(file trustedFile) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	return os.WriteFile(s.path, payload, 0o600)
}

func normalizePath(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err == nil {
		absolute = resolved
	}
	return filepath.Clean(absolute), nil
}
