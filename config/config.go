package config

import (
	"os"
	"strconv"
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

// Load reads configuration from environment variables.
// Precedence: OC_* vars > provider-specific vars > defaults.
func Load() Config {
	c := Config{
		Provider:    getEnv("OC_PROVIDER", "anthropic"),
		Model:       getEnv("OC_MODEL", ""),
		APIKey:      getEnv("OC_API_KEY", ""),
		BaseURL:     getEnv("OC_BASE_URL", ""),
		Temperature: getEnvFloat("OC_TEMPERATURE", 0),
		MaxTokens:   getEnvInt("OC_MAX_TOKENS", 0),
	}

	// Fall back to provider-specific API key env vars
	if c.APIKey == "" {
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
		case "anthropic":
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
