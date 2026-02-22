package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"strings"
	"time"

	"github.com/srogers/oc/auth"
	"github.com/srogers/oc/config"
	"github.com/srogers/oc/event"
	"github.com/srogers/oc/provider"
	"github.com/srogers/oc/provider/anthropic"
	"github.com/srogers/oc/provider/openai"
	"github.com/srogers/oc/session"
	"github.com/srogers/oc/tool"
	"github.com/srogers/oc/tui/common"
	"github.com/srogers/oc/tui/custom"
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
		Commands: []*cli.Command{
			{
				Name:  "login",
				Usage: "Authenticate with Claude Max (OAuth)",
				Action: func(c *cli.Context) error {
					oauthCfg := auth.AnthropicOAuth()
					store := auth.NewTokenStore()
					token, err := auth.Login(c.Context, oauthCfg, store)
					if err != nil {
						return fmt.Errorf("login failed: %w", err)
					}
					fmt.Printf("Authenticated successfully (token expires %s)\n", token.ExpiresAt.Format(time.RFC3339))
					return nil
				},
			},
			{
				Name:  "logout",
				Usage: "Remove stored Claude Max credentials",
				Action: func(c *cli.Context) error {
					store := auth.NewTokenStore()
					if err := store.Clear(); err != nil {
						return fmt.Errorf("logout failed: %w", err)
					}
					fmt.Println("Logged out successfully.")
					return nil
				},
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
	config := config.Load()

	// CLI flags override env vars
	if v := c.String("provider"); v != "" {
		config.Provider = v
	}
	if v := c.String("model"); v != "" {
		config.Model = v
	}
	if v := c.String("api-key"); v != "" {
		config.APIKey = v
	}
	if v := c.String("base-url"); v != "" {
		config.BaseURL = v
	}

	// Select provider adapter
	var p provider.Provider
	var providerDetail string
	var err error
	enableTools := true
	switch config.Provider {
	case "anthropic", "claude-max":
		if config.Provider == "anthropic" && config.APIKey != "" {
			p, err = anthropic.NewAnthropicApiProvider(config, nil, anthropic.AnthropicBaseUrl)
		} else {
			p, err = anthropic.NewAnthropicSubscriptionProvider(config, nil, anthropic.AnthropicBaseUrl)
		}
	default:
		// OpenAI-compatible (covers openai, ollama, lm studio, vllm, etc.)
		if config.BaseURL == "" {
			return fmt.Errorf("base URL required for provider %q (set OC_BASE_URL)", config.Provider)
		}
		p = openai.NewOpenAI(config.Provider, config.APIKey, config.BaseURL)
		if config.Provider == "ollama" {
			enableTools = checkOllamaTools(config.BaseURL, config.Model)
		}
	}

	if err != nil {
		return err
	}

	// Register tools (skip for models that don't support tool calling)
	tools := tool.NewToolRegistry()
	if enableTools {
		tools.Register(tool.NewBash())
		tools.Register(tool.NewRead())
		tools.Register(tool.NewWrite())
		tools.Register(tool.NewEdit())
		tools.Register(tool.NewGlob())
		tools.Register(tool.NewGrep())
		tools.Register(tool.NewAsk())
	}

	// Build model config
	modelCfg := provider.ModelConfig{Model: config.Model}
	if config.Temperature != 0 {
		t := config.Temperature
		modelCfg.Temperature = &t
	}
	if config.MaxTokens != 0 {
		m := config.MaxTokens
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

	// Create command registry
	commands := common.NewCommandRegistry()

	// Build model fetcher (if provider supports listing)
	var fetchModels custom.FetchModels
	if lister, ok := p.(provider.ModelLister); ok {
		fetchModels = func() ([]string, error) {
			return lister.ListModels(context.Background())
		}
	}

	// Create TUI
	ui := custom.New(custom.Deps{
		Source: bus,
		OnInput: func(text string) {
			sess.Send(context.Background(), text)
		},
		Status: func() custom.StatusInfo {
			status := "idle"
			switch sess.GetStatus() {
			case session.StatusBusy:
				status = "busy"
			}
			tokens := sess.GetTokens()
			return custom.StatusInfo{
				Model:         sess.GetModel(),
				Provider:      p.Name(),
				Detail:        providerDetail,
				Status:        status,
				Tokens:        tokens.TotalTokens,
				ContextTokens: sess.GetContextTokens(),
				WorkingDir:    workingDir,
			}
		},
		Messages: func() []session.Message {
			return sess.GetMessages()
		},
		Commands: commands,
		FetchModels: fetchModels,
		OnModelSelect: func(model string) {
			sess.SetModel(model)
			sess.AddLocalMessage("Switched to model: " + model)
			bus.Publish(event.TopicMsgDone, "model-switch")
		},
	})

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// Register commands (after ui is created so closures can reference it)
	commands.Register(common.Command{
		Name:        "clear",
		Description: "Clear conversation history",
		Aliases:     []string{"cls", "reset"},
		Action: func() {
			sess.ClearMessages()
			bus.Publish(event.TopicStatus, "idle")
		},
	})
	commands.Register(common.Command{
		Name:        "cancel",
		Description: "Cancel current operation",
		Aliases:     []string{"stop", "interrupt"},
		Action: func() {
			sess.Cancel()
		},
	})
	commands.Register(common.Command{
		Name:        "exit",
		Description: "Exit the application",
		Aliases:     []string{"quit", "q"},
		Action: func() {
			cancel()
		},
	})
	commands.Register(common.Command{
		Name:        "model",
		Description: "Switch to a different model",
		Action: func() {
			ui.OpenModelPicker()
		},
	})
	commands.Register(common.Command{
		Name:        "help",
		Description: "Show available commands",
		Aliases:     []string{"?", "commands"},
		Action: func() {
			helpText := "Available commands:\n"
			for _, cmd := range commands.All() {
				helpText += "  :" + cmd.Name
				if len(cmd.Aliases) > 0 {
					helpText += " ("
					for i, a := range cmd.Aliases {
						if i > 0 {
							helpText += ", "
						}
						helpText += a
					}
					helpText += ")"
				}
				helpText += " - " + cmd.Description + "\n"
			}
			sess.AddLocalMessage(helpText)
			bus.Publish(event.TopicMsgDone, "help")
		},
	})

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

	return slices.Contains(result.Capabilities, "tools")
}
