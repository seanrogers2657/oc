package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/srogers/oc/config"
	"github.com/srogers/oc/event"
	"github.com/srogers/oc/provider"
	"github.com/srogers/oc/session"
	"github.com/srogers/oc/tool"
	"github.com/srogers/oc/tui"
	cli "github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:  "oc",
		Usage: "AI coding assistant in the terminal",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "provider",
				Aliases: []string{"p"},
				Usage:   "AI provider (anthropic, openai, ollama)",
				EnvVars: []string{"OC_PROVIDER"},
			},
			&cli.StringFlag{
				Name:    "model",
				Aliases: []string{"m"},
				Usage:   "Model to use",
				EnvVars: []string{"OC_MODEL"},
			},
			&cli.StringFlag{
				Name:    "api-key",
				Usage:   "API key",
				EnvVars: []string{"OC_API_KEY"},
			},
			&cli.StringFlag{
				Name:    "base-url",
				Usage:   "Base URL for API endpoint",
				EnvVars: []string{"OC_BASE_URL"},
			},
		},
		Action: run,
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(c *cli.Context) error {
	cfg := config.Load()

	// CLI flags override env vars
	if v := c.String("provider"); v != "" {
		cfg.Provider = v
	}
	if v := c.String("model"); v != "" {
		cfg.Model = v
	}
	if v := c.String("api-key"); v != "" {
		cfg.APIKey = v
	}
	if v := c.String("base-url"); v != "" {
		cfg.BaseURL = v
	}

	// Select provider adapter
	var p provider.Provider
	enableTools := true
	switch cfg.Provider {
	case "anthropic":
		if cfg.APIKey == "" {
			return fmt.Errorf("API key required for Anthropic (set ANTHROPIC_API_KEY or OC_API_KEY)")
		}
		p = provider.NewAnthropic(cfg.APIKey)
	default:
		// OpenAI-compatible (covers openai, ollama, lm studio, vllm, etc.)
		if cfg.BaseURL == "" {
			return fmt.Errorf("base URL required for provider %q (set OC_BASE_URL)", cfg.Provider)
		}
		p = provider.NewOpenAI(cfg.Provider, cfg.APIKey, cfg.BaseURL)
		if cfg.Provider == "ollama" {
			enableTools = checkOllamaTools(cfg.BaseURL, cfg.Model)
		}
	}

	// Register tools (skip for models that don't support tool calling)
	tools := tool.NewRegistry()
	if enableTools {
		tools.Register(tool.NewBash())
		tools.Register(tool.NewRead())
		tools.Register(tool.NewWrite())
		tools.Register(tool.NewEdit())
		tools.Register(tool.NewGlob())
		tools.Register(tool.NewGrep())
	}

	// Build model config
	modelCfg := provider.ModelConfig{Model: cfg.Model}
	if cfg.Temperature != 0 {
		t := cfg.Temperature
		modelCfg.Temperature = &t
	}
	if cfg.MaxTokens != 0 {
		m := cfg.MaxTokens
		modelCfg.MaxTokens = &m
	}

	// Capture working directory from invocation
	workingDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getwd: %w", err)
	}

	// Create session
	bus := event.NewBus()
	store := session.NewStore()
	sess := store.Create(session.Deps{
		Model:  p,
		Tools:  tools,
		Events: bus,
	}, modelCfg, workingDir)

	// Create TUI
	ui := tui.New(tui.Deps{
		Source: bus,
		OnInput: func(text string) {
			sess.Send(context.Background(), text)
		},
		Status: func() tui.StatusInfo {
			status := "idle"
			switch sess.GetStatus() {
			case session.StatusBusy:
				status = "busy"
			}
			tokens := sess.GetTokens()
			return tui.StatusInfo{
				Model:         cfg.Model,
				Provider:      p.Name(),
				Status:        status,
				Tokens:        tokens.TotalTokens,
				ContextTokens: sess.GetContextTokens(),
				WorkingDir:    workingDir,
			}
		},
		Messages: func() []session.Message {
			return sess.GetMessages()
		},
	})

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	return ui.Run(ctx)
}

// checkOllamaTools queries the Ollama API to determine if the model supports tool calling.
// Returns false on any error (conservative default for non-tool models).
func checkOllamaTools(baseURL, model string) bool {
	// Derive the Ollama native API base from the OpenAI-compat base URL.
	// e.g. "http://localhost:11434/v1" -> "http://localhost:11434"
	ollamaBase := strings.TrimSuffix(baseURL, "/")
	ollamaBase = strings.TrimSuffix(ollamaBase, "/v1")

	reqBody, _ := json.Marshal(map[string]string{"model": model})
	req, err := http.NewRequest("POST", ollamaBase+"/api/show", bytes.NewReader(reqBody))
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false
	}

	var result struct {
		Capabilities []string `json:"capabilities"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false
	}

	for _, cap := range result.Capabilities {
		if cap == "tools" {
			return true
		}
	}
	return false
}
