package config

import (
	"testing"

	"github.com/jhenriquez/ai-code-reviewer/internal/ollama"
)

func fakeGetenv(vars map[string]string) getenvFunc {
	return func(key string) string { return vars[key] }
}

func TestLoad_Defaults(t *testing.T) {
	cfg, err := Load(fakeGetenv(map[string]string{"GITHUB_TOKEN": "tok"}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.OllamaURL != "http://localhost:11434" {
		t.Errorf("OllamaURL = %q", cfg.OllamaURL)
	}
	if cfg.MaxFiles != 15 {
		t.Errorf("MaxFiles = %d", cfg.MaxFiles)
	}
	if cfg.OllamaFormatMode != ollama.FormatSchema {
		t.Errorf("OllamaFormatMode = %q", cfg.OllamaFormatMode)
	}
}

func TestLoad_InputEnvTakesPrecedenceOverPlainEnv(t *testing.T) {
	cfg, err := Load(fakeGetenv(map[string]string{
		"GITHUB_TOKEN":       "plain-token",
		"INPUT_GITHUB-TOKEN": "action-input-token",
		"OLLAMA_MODEL":       "plain-model",
		"INPUT_OLLAMA-MODEL": "action-input-model",
	}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.GitHubToken != "action-input-token" {
		t.Errorf("GitHubToken = %q, want action-input-token", cfg.GitHubToken)
	}
	if cfg.OllamaModel != "action-input-model" {
		t.Errorf("OllamaModel = %q, want action-input-model", cfg.OllamaModel)
	}
}

func TestLoad_MissingTokenAlwaysErrors(t *testing.T) {
	// dry-run only skips posting; reading the real PR/files still needs a
	// token, so it must be required regardless of dry-run.
	if _, err := Load(fakeGetenv(map[string]string{})); err == nil {
		t.Fatalf("expected error when github-token is missing")
	}
	if _, err := Load(fakeGetenv(map[string]string{"DRY_RUN": "true"})); err == nil {
		t.Fatalf("expected error when github-token is missing even in dry-run")
	}

	cfg, err := Load(fakeGetenv(map[string]string{"GITHUB_TOKEN": "tok", "DRY_RUN": "true"}))
	if err != nil {
		t.Fatalf("Load with dry-run: %v", err)
	}
	if !cfg.DryRun {
		t.Errorf("expected DryRun=true")
	}
}

func TestLoad_InvalidFormatMode(t *testing.T) {
	_, err := Load(fakeGetenv(map[string]string{
		"GITHUB_TOKEN":             "tok",
		"INPUT_OLLAMA-FORMAT-MODE": "yaml",
	}))
	if err == nil {
		t.Fatalf("expected error for invalid ollama-format-mode")
	}
}

func TestLoad_IgnorePatternsCustomOverridesDefault(t *testing.T) {
	cfg, err := Load(fakeGetenv(map[string]string{
		"GITHUB_TOKEN":          "tok",
		"INPUT_IGNORE-PATTERNS": "**/foo/**, bar.txt",
	}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := []string{"**/foo/**", "bar.txt"}
	if len(cfg.IgnorePatterns) != len(want) {
		t.Fatalf("IgnorePatterns = %v", cfg.IgnorePatterns)
	}
	for i := range want {
		if cfg.IgnorePatterns[i] != want[i] {
			t.Errorf("IgnorePatterns[%d] = %q, want %q", i, cfg.IgnorePatterns[i], want[i])
		}
	}
}
