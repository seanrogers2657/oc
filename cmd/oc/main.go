package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"strings"
	"time"

	"github.com/srogers/oc/assets"
	"github.com/srogers/oc/auth"
	"github.com/srogers/oc/config"
	"github.com/srogers/oc/domain"
	"github.com/srogers/oc/event"
	"github.com/srogers/oc/history"
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
	// Load file config (~/.oc/config.json), warn on parse error
	fc, err := config.LoadFile(config.DefaultPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: %s: %v\n", config.DefaultPath(), err)
	}

	cfg := config.Load(fc)

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
	var providerDetail string
	enableTools := true
	switch cfg.Provider {
	case "anthropic", "claude-max":
		if cfg.Provider == "anthropic" && cfg.APIKey != "" {
			p, err = anthropic.NewAnthropicApiProvider(cfg, nil, anthropic.AnthropicBaseUrl)
		} else {
			p, err = anthropic.NewAnthropicSubscriptionProvider(cfg, nil, anthropic.AnthropicBaseUrl)
		}
	default:
		// OpenAI-compatible (covers openai, ollama, lm studio, vllm, etc.)
		if cfg.BaseURL == "" {
			return fmt.Errorf("base URL required for provider %q (set OC_BASE_URL)", cfg.Provider)
		}
		p = openai.NewOpenAI(cfg.Provider, cfg.APIKey, cfg.BaseURL)
		if cfg.Provider == "ollama" {
			enableTools = checkOllamaTools(cfg.BaseURL, cfg.Model)
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
		tools.Register(tool.NewWebSearch())
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

	// Create history store
	histStore := history.NewStore()

	// Create command registry
	commands := common.NewCommandRegistry()

	// Load key bindings
	km := domain.NewKeyMap()
	if err := domain.LoadKeyMapBytes(km, assets.DefaultKeybindings); err != nil {
		panic("bad default keybindings: " + err.Error())
	}
	if err := domain.LoadKeyMapFile(km, domain.DefaultBindingsPath()); err != nil {
		fmt.Fprintf(os.Stderr, "warning: keybindings: %v\n", err)
	}

	// Build model fetcher (if provider supports listing)
	var fetchModels custom.FetchModels
	if lister, ok := p.(provider.ModelLister); ok {
		fetchModels = func() ([]string, error) {
			return lister.ListModels(context.Background())
		}
	}

	// Create TUI
	ui := custom.New(custom.Deps{
		KeyMap: km,
		Source: bus,
		OnInput: func(text string) {
			histStore.Append(text)
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
			saveConfigField(func(fc *config.FileConfig) { fc.Model = &model })
			sess.AddLocalMessage("Switched to model: " + model)
			bus.Publish(event.TopicMsgDone, "model-switch")
		},
	})

	// Load prompt history into input area
	if entries, err := histStore.Load(); err == nil && len(entries) > 0 {
		ui.InputArea().SetHistory(entries)
	}

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
		Name:        "fps",
		Description: "Toggle FPS counter display",
		Action: func() {
			ui.ToggleFPS()
		},
	})
	commands.Register(common.Command{
		Name:        "doctor",
		Description: "Show provider configuration status",
		Action: func() {
			var buf strings.Builder
			_ = doctor(&buf, p.Name(), buildProviderChecks(cfg))
			sess.AddLocalMessage(buf.String())
			bus.Publish(event.TopicMsgDone, "doctor")
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

type providerCheck struct {
	name  string
	check func() (ok bool, message string)
}

// buildProviderChecks returns the real provider checks for cfg.
func buildProviderChecks(cfg config.Config) []providerCheck {
	// Determine Ollama native API base (strip /v1 if present)
	ollamaBase := "http://localhost:11434"
	if cfg.Provider == "ollama" && cfg.BaseURL != "" {
		ollamaBase = strings.TrimSuffix(cfg.BaseURL, "/")
		ollamaBase = strings.TrimSuffix(ollamaBase, "/v1")
	}

	return []providerCheck{
		{
			name: "anthropic-api",
			check: func() (bool, string) {
				if os.Getenv("ANTHROPIC_API_KEY") == "" {
					return false, "no API key (set ANTHROPIC_API_KEY)"
				}
				return true, "API key configured"
			},
		},
		{
			name: "claude-max",
			check: func() (bool, string) {
				token, err := auth.NewTokenStore().Load()
				if err != nil {
					return false, "error reading credentials: " + err.Error()
				}
				if token == nil || !token.Valid() {
					return false, "not authenticated — run: oc login"
				}
				return true, "authenticated (token expires " + token.ExpiresAt.Format(time.RFC3339) + ")"
			},
		},
		{
			name: "openai",
			check: func() (bool, string) {
				if os.Getenv("OPENAI_API_KEY") == "" {
					return false, "no API key (set OPENAI_API_KEY)"
				}
				return true, "API key configured"
			},
		},
		{
			name: "ollama",
			check: func() (bool, string) {
				models, err := listOllamaModels(ollamaBase)
				if err != nil {
					return false, "not reachable at " + ollamaBase
				}
				n := len(models)
				if n == 0 {
					return true, "running at " + ollamaBase + " (no models)"
				}
				suffix := "s"
				if n == 1 {
					suffix = ""
				}
				return true, fmt.Sprintf("running at %s (%d model%s)", ollamaBase, n, suffix)
			},
		},
	}
}

// doctor writes provider status to w. Extracted for testability.
func doctor(w io.Writer, activeProvider string, checks []providerCheck) error {
	fmt.Fprintln(w, "Provider status:")
	fmt.Fprintln(w)
	for _, pc := range checks {
		ok, msg := pc.check()
		mark := "✓"
		if !ok {
			mark = "✗"
		}
		active := ""
		if pc.name == activeProvider {
			active = " (active)"
		}
		fmt.Fprintf(w, "  %s  %-12s %s%s\n", mark, pc.name, msg, active)
	}
	fmt.Fprintln(w)
	return nil
}

// listOllamaModels queries the Ollama /api/tags endpoint and returns model names.
func listOllamaModels(baseURL string) ([]string, error) {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(baseURL + "/api/tags")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	names := make([]string, len(result.Models))
	for i, m := range result.Models {
		names[i] = m.Name
	}
	return names, nil
}

// saveConfigField loads ~/.oc/config.json, applies mutate, and writes it back.
// Errors are silently ignored — saving config is best-effort.
func saveConfigField(mutate func(fc *config.FileConfig)) {
	path := config.DefaultPath()
	fc, _ := config.LoadFile(path)
	if fc == nil {
		fc = &config.FileConfig{}
	}
	mutate(fc)
	config.SaveFile(path, fc)
}
