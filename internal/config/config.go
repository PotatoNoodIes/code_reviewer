// Package config loads the reviewer's settings from GitHub Actions inputs
// (INPUT_* env vars), falling back to plain env vars and then defaults —
// in that precedence order, so the same binary works as a Docker container
// action and as a bare CLI during local development.
package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jhenriquez/ai-code-reviewer/internal/ollama"
)

// Config holds every setting the reviewer needs, independent of *which* PR
// is being reviewed (that comes from the triggering event or CLI flags).
type Config struct {
	GitHubToken       string
	OllamaURL         string
	OllamaModel       string
	OllamaFormatMode  ollama.FormatMode
	OllamaConcurrency int
	OllamaTimeout     time.Duration
	MaxFiles          int
	MaxFileLines      int
	IgnorePatterns    []string
	ReviewEvent       string // GitHub review "event": COMMENT, APPROVE, REQUEST_CHANGES
	DryRun            bool
}

// DefaultIgnorePatterns covers common generated/vendored/binary paths that
// are rarely worth an LLM's attention and often blow past size caps.
var DefaultIgnorePatterns = []string{
	"**/vendor/**",
	"**/node_modules/**",
	"**/dist/**",
	"**/build/**",
	"**/*.min.js",
	"**/*.lock",
	"go.sum",
	"package-lock.json",
	"yarn.lock",
	"pnpm-lock.yaml",
}

// getenv is a lookup function; tests substitute a fake to avoid touching
// the real process environment.
type getenvFunc func(key string) string

// Load reads configuration for the given input name using GetenvFunc's
// INPUT_<NAME> convention first (GitHub Actions maps action input
// "ollama-url" to env var INPUT_OLLAMA-URL), then a plain env var name,
// then falls back to the provided default.
func Load(getenv getenvFunc) (*Config, error) {
	cfg := &Config{
		GitHubToken: lookup(getenv, "github-token", "GITHUB_TOKEN", ""),
		OllamaURL:   lookup(getenv, "ollama-url", "OLLAMA_URL", "http://localhost:11434"),
		OllamaModel: lookup(getenv, "ollama-model", "OLLAMA_MODEL", "qwen2.5-coder:7b"),
		ReviewEvent: strings.ToUpper(lookup(getenv, "review-event", "REVIEW_EVENT", "COMMENT")),
	}

	formatMode := ollama.FormatMode(lookup(getenv, "ollama-format-mode", "OLLAMA_FORMAT_MODE", string(ollama.FormatSchema)))
	switch formatMode {
	case ollama.FormatSchema, ollama.FormatJSON, ollama.FormatNone:
		cfg.OllamaFormatMode = formatMode
	default:
		return nil, fmt.Errorf("config: invalid ollama-format-mode %q (want schema|json|none)", formatMode)
	}

	var err error
	if cfg.OllamaConcurrency, err = lookupInt(getenv, "ollama-concurrency", "OLLAMA_CONCURRENCY", 1); err != nil {
		return nil, err
	}
	timeoutSeconds, err := lookupInt(getenv, "ollama-timeout-seconds", "OLLAMA_TIMEOUT_SECONDS", 120)
	if err != nil {
		return nil, err
	}
	cfg.OllamaTimeout = time.Duration(timeoutSeconds) * time.Second

	if cfg.MaxFiles, err = lookupInt(getenv, "max-files", "MAX_FILES", 15); err != nil {
		return nil, err
	}
	if cfg.MaxFileLines, err = lookupInt(getenv, "max-file-lines", "MAX_FILE_LINES", 1200); err != nil {
		return nil, err
	}

	if raw := lookup(getenv, "ignore-patterns", "IGNORE_PATTERNS", ""); raw != "" {
		for _, p := range strings.Split(raw, ",") {
			if p = strings.TrimSpace(p); p != "" {
				cfg.IgnorePatterns = append(cfg.IgnorePatterns, p)
			}
		}
	} else {
		cfg.IgnorePatterns = DefaultIgnorePatterns
	}

	cfg.DryRun = lookupBool(getenv, "dry-run", "DRY_RUN", false)

	// dry-run only skips posting comments; reading the real PR/files still
	// needs a token, so it's always required.
	if cfg.GitHubToken == "" {
		return nil, fmt.Errorf("config: github-token is required")
	}

	return cfg, nil
}

func lookup(getenv getenvFunc, inputName, envName, def string) string {
	if v := getenv(inputEnvVar(inputName)); v != "" {
		return v
	}
	if v := getenv(envName); v != "" {
		return v
	}
	return def
}

func lookupInt(getenv getenvFunc, inputName, envName string, def int) (int, error) {
	raw := lookup(getenv, inputName, envName, "")
	if raw == "" {
		return def, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("config: %s must be an integer, got %q", envName, raw)
	}
	return n, nil
}

func lookupBool(getenv getenvFunc, inputName, envName string, def bool) bool {
	raw := strings.ToLower(lookup(getenv, inputName, envName, ""))
	switch raw {
	case "true", "1", "yes":
		return true
	case "false", "0", "no":
		return false
	default:
		return def
	}
}

// inputEnvVar mirrors GitHub Actions' INPUT_<NAME> convention: uppercase,
// hyphens preserved (not converted to underscores).
func inputEnvVar(inputName string) string {
	return "INPUT_" + strings.ToUpper(inputName)
}
