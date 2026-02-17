// Package config loads application configuration from environment variables.
//
// The Config struct is populated by Load(), which reads OC_* env vars with
// fallbacks to provider-specific vars (ANTHROPIC_API_KEY, OPENAI_API_KEY)
// and sensible defaults per provider.
//
// Key env vars: OC_PROVIDER, OC_MODEL, OC_API_KEY, OC_BASE_URL,
// OC_TEMPERATURE, OC_MAX_TOKENS.
//
// This package has no dependencies beyond stdlib.
package config
