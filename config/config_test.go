package config

import (
	"os"
	"path/filepath"
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

func strPtr(s string) *string    { return &s }
func floatPtr(f float64) *float64 { return &f }
func intPtr(n int) *int           { return &n }

func TestLoadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	os.WriteFile(path, []byte(`{
		"provider": "openai",
		"model": "gpt-4o",
		"api_key": "sk-test",
		"base_url": "https://api.openai.com",
		"temperature": 0.7,
		"max_tokens": 2048
	}`), 0600)

	fc, err := LoadFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fc == nil {
		t.Fatal("expected non-nil FileConfig")
	}
	if *fc.Provider != "openai" {
		t.Errorf("provider: got %q, want %q", *fc.Provider, "openai")
	}
	if *fc.Model != "gpt-4o" {
		t.Errorf("model: got %q, want %q", *fc.Model, "gpt-4o")
	}
	if *fc.APIKey != "sk-test" {
		t.Errorf("api_key: got %q, want %q", *fc.APIKey, "sk-test")
	}
	if *fc.BaseURL != "https://api.openai.com" {
		t.Errorf("base_url: got %q, want %q", *fc.BaseURL, "https://api.openai.com")
	}
	if *fc.Temperature != 0.7 {
		t.Errorf("temperature: got %f, want %f", *fc.Temperature, 0.7)
	}
	if *fc.MaxTokens != 2048 {
		t.Errorf("max_tokens: got %d, want %d", *fc.MaxTokens, 2048)
	}
}

func TestLoadFileMissing(t *testing.T) {
	fc, err := LoadFile(filepath.Join(t.TempDir(), "nonexistent.json"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fc != nil {
		t.Errorf("expected nil for missing file, got %+v", fc)
	}
}

func TestLoadFilePartial(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	os.WriteFile(path, []byte(`{"provider": "ollama"}`), 0600)

	fc, err := LoadFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fc.Provider == nil || *fc.Provider != "ollama" {
		t.Errorf("provider: got %v, want ollama", fc.Provider)
	}
	if fc.Model != nil {
		t.Errorf("expected nil model, got %q", *fc.Model)
	}
	if fc.Temperature != nil {
		t.Errorf("expected nil temperature, got %f", *fc.Temperature)
	}
}

func TestLoadWithFileConfig(t *testing.T) {
	// Clear env vars so file config takes effect
	for _, k := range []string{"OC_PROVIDER", "OC_MODEL", "OC_API_KEY", "OC_BASE_URL",
		"OC_TEMPERATURE", "OC_MAX_TOKENS", "ANTHROPIC_API_KEY", "OPENAI_API_KEY"} {
		os.Unsetenv(k)
	}

	fc := &FileConfig{
		Provider:    strPtr("openai"),
		Model:       strPtr("gpt-4o"),
		APIKey:      strPtr("file-key"),
		BaseURL:     strPtr("https://custom.api.com"),
		Temperature: floatPtr(0.9),
		MaxTokens:   intPtr(1024),
	}

	cfg := Load(fc)
	if cfg.Provider != "openai" {
		t.Errorf("provider: got %q, want %q", cfg.Provider, "openai")
	}
	if cfg.Model != "gpt-4o" {
		t.Errorf("model: got %q, want %q", cfg.Model, "gpt-4o")
	}
	if cfg.APIKey != "file-key" {
		t.Errorf("api_key: got %q, want %q", cfg.APIKey, "file-key")
	}
	if cfg.BaseURL != "https://custom.api.com" {
		t.Errorf("base_url: got %q, want %q", cfg.BaseURL, "https://custom.api.com")
	}
	if cfg.Temperature != 0.9 {
		t.Errorf("temperature: got %f, want %f", cfg.Temperature, 0.9)
	}
	if cfg.MaxTokens != 1024 {
		t.Errorf("max_tokens: got %d, want %d", cfg.MaxTokens, 1024)
	}
}

func TestLoadEnvOverridesFileConfig(t *testing.T) {
	fc := &FileConfig{
		Provider: strPtr("openai"),
		Model:    strPtr("gpt-4o"),
		APIKey:   strPtr("file-key"),
	}

	os.Setenv("OC_PROVIDER", "anthropic")
	os.Setenv("OC_MODEL", "claude-sonnet-4-20250514")
	os.Setenv("OC_API_KEY", "env-key")
	defer func() {
		os.Unsetenv("OC_PROVIDER")
		os.Unsetenv("OC_MODEL")
		os.Unsetenv("OC_API_KEY")
	}()

	cfg := Load(fc)
	if cfg.Provider != "anthropic" {
		t.Errorf("env should override file: provider got %q", cfg.Provider)
	}
	if cfg.Model != "claude-sonnet-4-20250514" {
		t.Errorf("env should override file: model got %q", cfg.Model)
	}
	if cfg.APIKey != "env-key" {
		t.Errorf("env should override file: api_key got %q", cfg.APIKey)
	}
}

func TestSaveFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "config.json")

	fc := &FileConfig{
		Provider:    strPtr("anthropic"),
		Model:       strPtr("claude-sonnet-4-20250514"),
		Temperature: floatPtr(0.5),
	}

	if err := SaveFile(path, fc); err != nil {
		t.Fatalf("SaveFile: %v", err)
	}

	// Round-trip
	got, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if got.Provider == nil || *got.Provider != "anthropic" {
		t.Errorf("provider round-trip failed")
	}
	if got.Model == nil || *got.Model != "claude-sonnet-4-20250514" {
		t.Errorf("model round-trip failed")
	}
	if got.Temperature == nil || *got.Temperature != 0.5 {
		t.Errorf("temperature round-trip failed")
	}
	if got.APIKey != nil {
		t.Errorf("expected nil api_key, got %q", *got.APIKey)
	}
}

func TestSaveFilePermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	fc := &FileConfig{Provider: strPtr("openai")}
	if err := SaveFile(path, fc); err != nil {
		t.Fatalf("SaveFile: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("file permissions: got %o, want 0600", perm)
	}
}
