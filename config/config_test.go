package config

import (
	"os"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	// Clear relevant env vars
	for _, k := range []string{"OC_PROVIDER", "OC_MODEL", "OC_API_KEY", "OC_BASE_URL",
		"ANTHROPIC_API_KEY", "OPENAI_API_KEY"} {
		os.Unsetenv(k)
	}

	cfg := Load()
	if cfg.Provider != "anthropic" {
		t.Errorf("expected default provider 'anthropic', got %q", cfg.Provider)
	}
	if cfg.Model != "claude-sonnet-4-20250514" {
		t.Errorf("expected default model, got %q", cfg.Model)
	}
}

func TestLoadFromEnv(t *testing.T) {
	os.Setenv("OC_PROVIDER", "openai")
	os.Setenv("OC_MODEL", "gpt-4o")
	os.Setenv("OC_API_KEY", "test-key")
	os.Setenv("OC_BASE_URL", "https://api.openai.com")
	defer func() {
		os.Unsetenv("OC_PROVIDER")
		os.Unsetenv("OC_MODEL")
		os.Unsetenv("OC_API_KEY")
		os.Unsetenv("OC_BASE_URL")
	}()

	cfg := Load()
	if cfg.Provider != "openai" {
		t.Errorf("expected 'openai', got %q", cfg.Provider)
	}
	if cfg.Model != "gpt-4o" {
		t.Errorf("expected 'gpt-4o', got %q", cfg.Model)
	}
	if cfg.APIKey != "test-key" {
		t.Errorf("expected 'test-key', got %q", cfg.APIKey)
	}
	if cfg.BaseURL != "https://api.openai.com" {
		t.Errorf("expected base URL, got %q", cfg.BaseURL)
	}
}

func TestLoadFallbackAPIKey(t *testing.T) {
	os.Unsetenv("OC_API_KEY")
	os.Setenv("OC_PROVIDER", "anthropic")
	os.Setenv("ANTHROPIC_API_KEY", "ant-key")
	defer func() {
		os.Unsetenv("OC_PROVIDER")
		os.Unsetenv("ANTHROPIC_API_KEY")
	}()

	cfg := Load()
	if cfg.APIKey != "ant-key" {
		t.Errorf("expected fallback to ANTHROPIC_API_KEY, got %q", cfg.APIKey)
	}
}

func TestLoadOllamaDefaults(t *testing.T) {
	for _, k := range []string{"OC_API_KEY", "OC_BASE_URL", "OC_MODEL",
		"ANTHROPIC_API_KEY", "OPENAI_API_KEY"} {
		os.Unsetenv(k)
	}
	os.Setenv("OC_PROVIDER", "ollama")
	defer os.Unsetenv("OC_PROVIDER")

	cfg := Load()
	if cfg.Model != "llama3" {
		t.Errorf("expected default ollama model 'llama3', got %q", cfg.Model)
	}
	if cfg.BaseURL != "http://localhost:11434" {
		t.Errorf("expected ollama base URL, got %q", cfg.BaseURL)
	}
}

func TestLoadTemperature(t *testing.T) {
	os.Setenv("OC_TEMPERATURE", "0.5")
	defer os.Unsetenv("OC_TEMPERATURE")

	cfg := Load()
	if cfg.Temperature != 0.5 {
		t.Errorf("expected temperature 0.5, got %f", cfg.Temperature)
	}
}

func TestLoadMaxTokens(t *testing.T) {
	os.Setenv("OC_MAX_TOKENS", "4096")
	defer os.Unsetenv("OC_MAX_TOKENS")

	cfg := Load()
	if cfg.MaxTokens != 4096 {
		t.Errorf("expected max_tokens 4096, got %d", cfg.MaxTokens)
	}
}

func TestLoadClaudeMaxDefaults(t *testing.T) {
	for _, k := range []string{"OC_API_KEY", "OC_MODEL", "OC_BASE_URL",
		"ANTHROPIC_API_KEY", "OPENAI_API_KEY"} {
		os.Unsetenv(k)
	}
	os.Setenv("OC_PROVIDER", "claude-max")
	defer os.Unsetenv("OC_PROVIDER")

	cfg := Load()
	if cfg.Model != "claude-sonnet-4-20250514" {
		t.Errorf("expected claude-sonnet model, got %q", cfg.Model)
	}
	// claude-max should not try to load API keys
	if cfg.APIKey != "" {
		t.Errorf("expected empty API key for claude-max, got %q", cfg.APIKey)
	}
}
