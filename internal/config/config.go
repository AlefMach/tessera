package config

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	defaultProvider      = "ollama"
	defaultModel         = "local-model"
	defaultOllamaURL     = "http://localhost:11434"
	defaultContextTokens = 4096
	defaultMaxTokens     = 1024
)

type Flags struct {
	Provider      string
	Model         string
	OllamaURL     string
	SQLitePath    string
	ContextTokens int
	MaxTokens     int
	ConfigPath    string
}

type Config struct {
	Provider      string
	Model         string
	OllamaURL     string
	SQLitePath    string
	ContextTokens int
	MaxTokens     int
	ConfigPath    string
	TesseraDir    string
}

func Load(flags Flags) (Config, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return Config{}, err
	}

	tesseraDir := filepath.Join(cwd, ".tessera")
	configPath := flags.ConfigPath
	if configPath == "" {
		configPath = filepath.Join(tesseraDir, "config.toml")
	}

	fileValues, err := readConfigFile(configPath)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		Provider:      pickString(flags.Provider, os.Getenv("TESSERA_PROVIDER"), fileValues["provider"], defaultProvider),
		Model:         pickString(flags.Model, os.Getenv("TESSERA_MODEL"), fileValues["model"], defaultModel),
		OllamaURL:     pickString(flags.OllamaURL, os.Getenv("TESSERA_OLLAMA_URL"), fileValues["ollama_url"], defaultOllamaURL),
		ContextTokens: pickInt(flags.ContextTokens, os.Getenv("TESSERA_CONTEXT_TOKENS"), fileValues["context_tokens"], defaultContextTokens),
		MaxTokens:     pickInt(flags.MaxTokens, os.Getenv("TESSERA_MAX_TOKENS"), fileValues["max_tokens"], defaultMaxTokens),
		ConfigPath:    configPath,
		TesseraDir:    tesseraDir,
	}
	cfg.SQLitePath = pickString(flags.SQLitePath, os.Getenv("TESSERA_SQLITE_PATH"), fileValues["sqlite_path"], filepath.Join(tesseraDir, "memory.db"))

	return cfg, nil
}

func (c Config) SessionsDir() string {
	return filepath.Join(c.TesseraDir, "sessions")
}

func (c Config) CurrentSessionPath() string {
	return filepath.Join(c.SessionsDir(), "current.json")
}

func readConfigFile(path string) (map[string]string, error) {
	values := map[string]string{}
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return values, nil
		}
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "[") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		values[key] = value
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return values, nil
}

func pickString(flagValue, envValue, fileValue, defaultValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if envValue != "" {
		return envValue
	}
	if fileValue != "" {
		return fileValue
	}
	return defaultValue
}

func pickInt(flagValue int, envValue, fileValue string, defaultValue int) int {
	if flagValue > 0 {
		return flagValue
	}
	if envValue != "" {
		if parsed, err := strconv.Atoi(envValue); err == nil && parsed > 0 {
			return parsed
		}
	}
	if fileValue != "" {
		if parsed, err := strconv.Atoi(fileValue); err == nil && parsed > 0 {
			return parsed
		}
	}
	return defaultValue
}
