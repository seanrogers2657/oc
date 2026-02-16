package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

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
		p = provider.NewOpenAI(cfg.APIKey, cfg.BaseURL)
	}

	// Register tools
	tools := tool.NewRegistry()
	tools.Register(tool.NewBash())
	tools.Register(tool.NewRead())
	tools.Register(tool.NewWrite())
	tools.Register(tool.NewEdit())
	tools.Register(tool.NewGlob())
	tools.Register(tool.NewGrep())

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

	// Create session
	bus := event.NewBus()
	store := session.NewStore()
	sess := store.Create(session.Deps{
		Model:  p,
		Tools:  tools,
		Events: bus,
	}, modelCfg)

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
				Model:    cfg.Model,
				Provider: p.Name(),
				Status:   status,
				Tokens:   tokens.TotalTokens,
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
