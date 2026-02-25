package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// Config holds application configuration loaded from environment.
type Config struct {
	Provider    string  // "openai", "anthropic", "ollama", etc.
	Model       string  // model identifier
	APIKey      string  // API key
	BaseURL     string  // custom endpoint (for OpenAI-compatible)
	Temperature float64 // generation temperature
	MaxTokens   int     // max output tokens
}

// FileConfig represents configuration loaded from ~/.oc/config.json.
// Pointer fields distinguish "not set" from zero values.
type FileConfig struct {
	Provider    *string  `json:"provider,omitempty"`
	Model       *string  `json:"model,omitempty"`
	APIKey      *string  `json:"api_key,omitempty"`
	BaseURL     *string  `json:"base_url,omitempty"`
	Temperature *float64 `json:"temperature,omitempty"`
	MaxTokens   *int     `json:"max_tokens,omitempty"`

	// OAuth token fields (previously in auth.json)
	AccessToken  *string    `json:"access_token,omitempty"`
	RefreshToken *string    `json:"refresh_token,omitempty"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	TokenType    *string    `json:"token_type,omitempty"`
}

// DefaultPath returns the default config file path: ~/.oc/config.json.
func DefaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".oc", "config.json")
}

// LoadFile reads and unmarshals a JSON config file.
// Returns (nil, nil) if the file does not exist.
func LoadFile(path string) (*FileConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var fc FileConfig
	if err := json.Unmarshal(data, &fc); err != nil {
		return nil, err
	}
	return &fc, nil
}

// SaveFile writes a FileConfig to disk as indented JSON.
// Creates the parent directory with 0700 and the file with 0600.
func SaveFile(path string, fc *FileConfig) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(fc, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0600)
}

// Load reads configuration from environment variables, optionally layered
// on top of a file config.
// Precedence: env vars > file config > hardcoded defaults.
func Load(fcs ...*FileConfig) Config {
	var fc *FileConfig
	if len(fcs) > 0 {
		fc = fcs[0]
	}

	// Build defaults from file config (falls back to "" / 0 when unset)
	defProvider := "anthropic"
	var defModel, defAPIKey, defBaseURL string
	var defTemp float64
	var defMaxTokens int
	if fc != nil {
		if fc.Provider != nil {
			defProvider = *fc.Provider
		}
		if fc.Model != nil {
			defModel = *fc.Model
		}
		if fc.APIKey != nil {
			defAPIKey = *fc.APIKey
		}
		if fc.BaseURL != nil {
			defBaseURL = *fc.BaseURL
		}
		if fc.Temperature != nil {
			defTemp = *fc.Temperature
		}
		if fc.MaxTokens != nil {
			defMaxTokens = *fc.MaxTokens
		}
	}

	c := Config{
		Provider:    getEnv("OC_PROVIDER", defProvider),
		Model:       getEnv("OC_MODEL", defModel),
		APIKey:      getEnv("OC_API_KEY", defAPIKey),
		BaseURL:     getEnv("OC_BASE_URL", defBaseURL),
		Temperature: getEnvFloat("OC_TEMPERATURE", defTemp),
		MaxTokens:   getEnvInt("OC_MAX_TOKENS", defMaxTokens),
	}

	// Fall back to provider-specific API key env vars (claude-max uses OAuth, not API keys)
	if c.APIKey == "" && c.Provider != "claude-max" {
		switch c.Provider {
		case "anthropic":
			c.APIKey = getEnv("ANTHROPIC_API_KEY", "")
		case "openai":
			c.APIKey = getEnv("OPENAI_API_KEY", "")
		default:
			// Try both
			c.APIKey = getEnv("OPENAI_API_KEY", "")
			if c.APIKey == "" {
				c.APIKey = getEnv("ANTHROPIC_API_KEY", "")
			}
		}
	}

	// Default models per provider
	if c.Model == "" {
		switch c.Provider {
		case "anthropic", "claude-max":
			c.Model = "claude-sonnet-4-20250514"
		case "openai":
			c.Model = "gpt-4o"
		default:
			c.Model = "llama3"
		}
	}

	// Default base URL for OpenAI-compatible providers
	if c.BaseURL == "" {
		switch c.Provider {
		case "openai":
			c.BaseURL = "https://api.openai.com"
		case "ollama":
			c.BaseURL = "http://localhost:11434"
		}
	}

	return c
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvFloat(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
