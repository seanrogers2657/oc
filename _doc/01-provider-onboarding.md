# 01 — Provider Onboarding in the TUI

## Status

`draft`

---

## Goal

Let users configure and switch AI providers from within the TUI — select a
provider, enter API credentials, and start using it — without restarting the
application or editing environment variables.

---

## Context

Today the provider is fixed at startup. `cmd/oc/main.go:run()` constructs a
single `provider.Provider` from env vars / `~/.oc/config.json` / CLI flags,
injects it into the session via `session.Deps.Model`, and it never changes.
The model picker (`:model`) lets you switch models *within* the current
provider, but there is no way to switch *providers* at runtime.

Packages involved:
- `config/` — per-provider config types, serialization, validation
- `cmd/oc/` — provider construction, wiring, onboarding flow
- `session/` — no changes (the `ModelClient` interface stays the same)
- `tui/common/` — new picker overlay
- `tui/custom/` — new port callbacks, overlay wiring

Port interfaces crossed:
- `session.ModelClient` ← satisfied by a new `providerSwapper` wrapper in `cmd/oc/`
- `provider.ModelLister` ← the swapper delegates to the current provider
- `custom.EventSource` ← existing, used for prompt-mode events during credential collection

Constraints:
- `domain/` stays stdlib-only
- No new external dependencies
- The provider swap must be safe while the session is idle (not mid-stream)

---

## API

| Name | Kind | Package | Status | Meaning |
|------|------|---------|--------|---------|
| `AuthMethod` | interface | `config/` | **new** | Marker interface for auth configs (`APIKeyAuth`, `OAuthAuth`, or nil) |
| `APIKeyAuth` | type | `config/` | **new** | Auth via static API key |
| `OAuthAuth` | type | `config/` | **new** | Auth via OAuth PKCE tokens (access, refresh, expiry) |
| `ProviderConfig` | interface | `config/` | **new** | Common interface for typed provider configs: name, type, model, auth |
| `AnthropicProvider` | type | `config/` | **new** | Typed config for Anthropic providers (any auth method) |
| `OpenAIProvider` | type | `config/` | **new** | Typed config for OpenAI-compatible providers (base URL + optional auth) |
| `authEntry` | type | `config/` | **new** | Unexported nested struct — JSON serialization for auth block |
| `providerEntry` | type | `config/` | **new** | Unexported flat struct — JSON serialization format only |
| `FileConfig.DefaultProvider` | field | `config/` | **new** | Name of the active provider (replaces top-level `provider`) |
| `FileConfig.Providers` | field | `config/` | **new** | `[]providerEntry` on disk, loaded into `map[string]ProviderConfig` in memory |
| `LoadProviders` | func | `config/` | **new** | Parses `[]providerEntry` → `map[string]ProviderConfig` with validation |
| `SaveProviders` | func | `config/` | **new** | Converts `map[string]ProviderConfig` → `[]providerEntry` for persistence |
| `OnProviderSelect` | type | `tui/custom/` | **new** | Callback invoked when the user picks a provider from the picker |
| `FetchProviders` | type | `tui/custom/` | **new** | Returns the list of configured provider names for the picker |
| `ProviderPicker` | type | `tui/common/` | **new** | Overlay for selecting from available providers |
| `ModelClient` | interface | `session/` | unchanged | The session never sees the swap |
| `TopicToolPrompt` | constant | `event/` | unchanged | Reused for credential prompts during onboarding |
| `StatusInfo.Provider` | field | `tui/custom/` | unchanged | Updates reactively after swap |

---

## Approach

### 1. Config type system

Three layers: JSON (flat), typed in-memory configs, and a conversion boundary
between them.

#### On-disk format (`~/.oc/config.json`)

Providers are a JSON array. Each entry is a self-contained object with a
`name`, `providerType` discriminator, and an optional nested `auth` block:

```json
{
  "defaultProvider": "my-anthropic",
  "providers": [
    {
      "name": "my-anthropic",
      "providerType": "anthropic",
      "model": "claude-sonnet-4-20250514",
      "auth": {
        "type": "api_key",
        "api_key": "sk-ant-..."
      }
    },
    {
      "name": "claude-max",
      "providerType": "anthropic",
      "model": "claude-sonnet-4-20250514",
      "auth": {
        "type": "oauth",
        "access_token": "eyJ...",
        "refresh_token": "ref-...",
        "expires_at": "2026-03-01T12:00:00Z",
        "token_type": "Bearer"
      }
    },
    {
      "name": "my-openai",
      "providerType": "openai",
      "model": "gpt-4o",
      "base_url": "https://api.openai.com/v1",
      "auth": {
        "type": "api_key",
        "api_key": "sk-..."
      }
    },
    {
      "name": "ollama",
      "providerType": "openai",
      "model": "llama3",
      "base_url": "http://localhost:11434/v1"
    },
    {
      "name": "corp-llm",
      "providerType": "openai",
      "base_url": "https://llm.corp.com/v1",
      "auth": {
        "type": "api_key",
        "api_key": "corp-key-..."
      }
    }
  ]
}
```

- `name` — user-chosen label, what `defaultProvider` references, what the
  picker shows
- `providerType` — wire protocol (`"anthropic"` or `"openai"`), not the
  display name. Multiple entries can share a type (e.g. `ollama` and
  `corp-llm` are both `providerType: "openai"`)
- `auth` — nested object with a `type` discriminator (`"api_key"` or
  `"oauth"`). Omitted entirely for unauthenticated providers (Ollama, local
  endpoints). Auth is orthogonal to provider type — an anthropic provider
  can use API key auth or OAuth PKCE
- Remaining fields are type-specific; irrelevant fields are simply absent

#### Serialization types (unexported)

```go
// config/provider.go

// authEntry is the nested JSON shape for auth configuration.
// Unexported — purely a serialization detail.
type authEntry struct {
    Type         string     `json:"type"`                    // "api_key" or "oauth"
    APIKey       *string    `json:"api_key,omitempty"`
    AccessToken  *string    `json:"access_token,omitempty"`
    RefreshToken *string    `json:"refresh_token,omitempty"`
    ExpiresAt    *time.Time `json:"expires_at,omitempty"`
    TokenType    *string    `json:"token_type,omitempty"`
}

// providerEntry is the JSON shape for a provider.
// Unexported — purely a serialization detail.
type providerEntry struct {
    Name         string     `json:"name"`
    ProviderType string     `json:"providerType"`
    Model        *string    `json:"model,omitempty"`
    BaseURL      *string    `json:"base_url,omitempty"`
    Auth         *authEntry `json:"auth,omitempty"`          // nil = no auth
}
```

Standard `encoding/json` handles this — no custom marshaler needed. The
`auth` field is a pointer so it serializes as absent (not `null`) when nil.

#### Auth types

Auth is orthogonal to provider type. Two auth methods exist today, plus nil
for unauthenticated endpoints:

```go
// AuthMethod is a marker interface for auth configurations.
// nil means no authentication (e.g. local Ollama).
type AuthMethod interface{ authMethod() }

// APIKeyAuth authenticates via a static API key (Bearer token).
type APIKeyAuth struct {
    APIKey string
}

// OAuthAuth authenticates via OAuth PKCE tokens.
// Tokens are managed by `oc login` and refreshed transparently.
type OAuthAuth struct {
    AccessToken  string
    RefreshToken string
    ExpiresAt    time.Time
    TokenType    string
}
```

#### Typed in-memory configs

```go
// ProviderConfig is the interface the rest of the app works with.
type ProviderConfig interface {
    GetName() string
    GetProviderType() string
    GetModel() *string
    SetModel(model string)
    GetAuth() AuthMethod
    SetAuth(auth AuthMethod)
}

// AnthropicProvider holds config for Anthropic providers.
// Auth is APIKeyAuth (direct API) or OAuthAuth (Claude Max).
type AnthropicProvider struct {
    Name  string
    Model *string
    Auth  AuthMethod
}

// OpenAIProvider holds config for any OpenAI-compatible endpoint.
// Auth is APIKeyAuth, or nil for unauthenticated local endpoints.
type OpenAIProvider struct {
    Name    string
    Model   *string
    BaseURL string      // required
    Auth    AuthMethod  // nil = no auth (Ollama, local endpoints)
}
```

`AnthropicProvider` has no `BaseURL` — it structurally cannot exist.
`OpenAIProvider` always has a `BaseURL` — it's a plain `string`, not a
pointer; the conversion layer enforces this. Auth is decoupled from
provider type — the same `AnthropicProvider` struct serves both direct API
key users and Claude Max OAuth users.

#### Conversion boundary

```go
// Load: providerEntry → typed config (validates on the way in)
func parseProvider(e providerEntry) (ProviderConfig, error)

// Save: typed config → providerEntry (lossless round-trip)
func toEntry(p ProviderConfig) providerEntry

// Batch versions for FileConfig integration
func LoadProviders(entries []providerEntry) (map[string]ProviderConfig, []error)
func SaveProviders(providers map[string]ProviderConfig) []providerEntry
```

`parseProvider` validates per-type constraints:
- `anthropic` requires `auth` (either `api_key` or `oauth`)
- `openai` requires `base_url`; `auth` is optional
- Unknown `providerType` returns an error
- Unknown `auth.type` returns an error

`parseAuth` validates per-auth-type constraints:
- `api_key` requires `api_key` field
- `oauth` requires `access_token`, `refresh_token`, `expires_at`
- `nil` auth block is valid (unauthenticated)

Validation happens once at load time. Invalid entries produce errors that are
logged as warnings; valid entries still load (partial success).

#### FileConfig changes

```go
type FileConfig struct {
    DefaultProvider *string          `json:"defaultProvider,omitempty"`
    Providers       []providerEntry  `json:"providers,omitempty"`

    // --- legacy fields (backward compat with env vars / CLI flags) ---
    Provider    *string  `json:"provider,omitempty"`     // superseded by defaultProvider
    Model       *string  `json:"model,omitempty"`
    APIKey      *string  `json:"api_key,omitempty"`
    BaseURL     *string  `json:"base_url,omitempty"`
    Temperature *float64 `json:"temperature,omitempty"`
    MaxTokens   *int     `json:"max_tokens,omitempty"`

    // --- legacy OAuth fields (now live inside provider auth blocks) ---
    AccessToken  *string    `json:"access_token,omitempty"`
    RefreshToken *string    `json:"refresh_token,omitempty"`
    ExpiresAt    *time.Time `json:"expires_at,omitempty"`
    TokenType    *string    `json:"token_type,omitempty"`
}
```

Backward compat: if `Providers` is empty, `config.Load()` falls back to the
legacy top-level fields. If `DefaultProvider` is nil, falls back to `Provider`.
Top-level OAuth fields are migrated into a provider auth block on first save.
Existing configs keep working without modification.

### 2. Provider swapper (runtime swap)

A `providerSwapper` in `cmd/oc/` wraps the current provider behind the
`session.ModelClient` interface:

```go
type providerSwapper struct {
    mu      sync.RWMutex
    current provider.Provider
}

func (s *providerSwapper) Stream(ctx context.Context, cfg provider.ModelConfig, ...) (<-chan provider.StreamEvent, error) {
    s.mu.RLock()
    p := s.current
    s.mu.RUnlock()
    return p.Stream(ctx, cfg, msgs, tools)
}

func (s *providerSwapper) Swap(p provider.Provider) {
    s.mu.Lock()
    s.current = p
    s.mu.Unlock()
}
```

The session receives `&providerSwapper{current: initialProvider}` as its
`ModelClient`. All closures in `run()` (`Status`, `FetchModels`, etc.) read
through the swapper, so they automatically reflect the current provider after
a swap.

### 3. Provider factory

Extract the provider construction logic from `run()` into a function that
takes a typed config. The factory switches on both provider type and auth
method:

```go
func createProvider(cfg config.ProviderConfig) (provider.Provider, error) {
    switch c := cfg.(type) {
    case *config.AnthropicProvider:
        switch a := c.Auth.(type) {
        case *config.APIKeyAuth:
            return anthropic.NewAnthropicApiProvider(a.APIKey, ...)
        case *config.OAuthAuth:
            return anthropic.NewAnthropicSubscriptionProvider(a, ...)
        default:
            return nil, fmt.Errorf("anthropic provider %q requires auth", c.Name)
        }
    case *config.OpenAIProvider:
        apiKey := ""
        if a, ok := c.Auth.(*config.APIKeyAuth); ok {
            apiKey = a.APIKey
        }
        return openai.NewOpenAI(c.Name, apiKey, c.BaseURL), nil
    }
}
```

Type switch — no string-based dispatch. Called both at startup and at runtime
during the onboarding flow. Adding a new provider type or auth method is: new
struct, new case here, new case in `parseProvider`/`toEntry`/`parseAuth`.

### 4. ProviderPicker overlay

A new overlay in `tui/common/` structurally identical to `ModelPicker` but
simpler: static list (no async fetch), no fuzzy search needed (few items).
Shows configured providers by name plus an "Add provider" option:

```
  anthropic      * anthropic
  my-openai        openai
  ollama           openai
  corp-llm         openai
  + Add provider
```

Left column is the name, right column is the `providerType` (dimmed). The `*`
marks `defaultProvider`.

### 5. Onboarding flow

When the user selects a provider from the picker:

**Already configured** (entry exists in providers list):
→ Swap immediately. Call `createProvider` with the typed config, swap into
  `providerSwapper`, persist the new `defaultProvider`, show confirmation
  message. Restore the last-used model from `ProviderConfig.GetModel()`.

**"Add provider":**
→ Multi-step prompt flow using the existing `TopicToolPrompt` mechanism:

1. Prompt for provider name (the `name` field)
2. Prompt for provider type (`anthropic` or `openai`)
3. Prompt for auth method (`api_key`, `oauth`, or `none`)
   - `api_key`: prompt for API key
   - `oauth`: tell user to run `oc login` instead (PKCE requires a browser)
   - `none`: skip auth (local endpoints)
4. Based on provider type, prompt for remaining fields:
   - `openai`: prompt for base URL
5. Construct the typed config (`&AnthropicProvider{...}` or
   `&OpenAIProvider{...}`) with the chosen `AuthMethod` — no intermediate
   raw entry
6. Call `createProvider`, swap, persist via `SaveProviders`

The flow runs in a goroutine (same pattern as `session.Send` / tool
execution) and blocks on each prompt response via the channel.

### 6. Update reactive closures

After a swap, several closures need to reflect the new provider:
- `Status()` — reads through the swapper, works automatically
- `FetchModels` — the swapper also implements `ModelLister` by delegation
- `OnModelSelect` — persists model to the active `ProviderConfig` via
  `SetModel()`, then writes back through `SaveProviders`

### Alternatives Considered

**Flat union config (no typed split):** One `ProviderConfig` struct with all
fields and a `providerType` discriminator. Simpler serialization but weaker
Go-side guarantees — nothing prevents a `BaseURL` on an anthropic config, and
every consumer switches on a string. The typed split catches misconfiguration
at load time and gives the factory concrete types to work with.

**Auth baked into provider type:** Original design had `AnthropicProvider.APIKey`
directly and treated Claude Max as a separate provider type. This conflates
wire protocol with authentication. Separating auth into its own axis means the
same `AnthropicProvider` struct covers both API key and OAuth users, and adding
a new auth method doesn't require new provider types.

**Prompt-only (no picker overlay):** Use `TopicToolPrompt` for everything,
including provider selection. Simpler to implement but worse UX — typing a
name is less discoverable than selecting from a list.

**Session.SetModelClient():** Rejected — pushes runtime-swap concerns into
the session package. The swapper wrapper in `cmd/oc/` keeps session clean.

**Map instead of array for JSON providers:** `"providers": {"name": {...}}`
puts the name one level up as the key. Array with explicit `name` field keeps
each provider as a self-contained object — easier to read, reorder, and
copy-paste.

---

## Steps

- [ ] Step 1 — **Config type system.** Add `authEntry`, `providerEntry`
  (both unexported), `AuthMethod` interface, `APIKeyAuth`, `OAuthAuth`,
  `ProviderConfig` interface, `AnthropicProvider`, `OpenAIProvider` to
  `config/`. Implement `parseAuth`, `parseProvider`, `toEntry`,
  `LoadProviders`, `SaveProviders`. Add `DefaultProvider` and `Providers`
  fields to `FileConfig`. Write tests for: round-trip serialization,
  per-type validation, per-auth-type validation, backward compat (configs
  without `providers` still load, top-level OAuth fields migrate into auth
  blocks).

- [ ] Step 2 — **Provider factory.** Extract `createProvider(cfg
  config.ProviderConfig) (provider.Provider, error)` from `run()`. Refactor
  `run()` to: load `FileConfig` → `LoadProviders` → look up
  `defaultProvider` → `createProvider`. No behavior change — pure refactor.

- [ ] Step 3 — **Provider swapper.** Create `providerSwapper` in `cmd/oc/`.
  Wire it into `run()` so the session's `ModelClient` goes through the
  swapper. Wire `Status()` and `FetchModels` closures through it. No behavior
  change — the swapper starts with the initial provider and never swaps yet.

- [ ] Step 4 — **ProviderPicker overlay.** Implement in `tui/common/`. Follow
  the `ModelPicker` pattern: `Active()`, `HandleAction()`, `InsertRune()`,
  `Render()`, `Open()`, `Close()`. Static item list, highlight current
  provider. Unit test the selection logic.

- [ ] Step 5 — **Wire onboarding flow.** Add `OnProviderSelect` and
  `FetchProviders` to `custom.Deps`. Add `:provider` command to the registry.
  Implement the orchestration goroutine in `cmd/oc/`: picker → credential
  prompts → construct typed config → `createProvider` → swap → persist via
  `SaveProviders`. Wire the `ProviderPicker` into `ui.go` overlay dispatch.

- [ ] Step 6 — **Model restoration.** When switching providers, restore the
  last-used model from `ProviderConfig.GetModel()`. Update `OnModelSelect`
  to call `SetModel()` on the active `ProviderConfig` and persist.

- [ ] Step 7 — **Manual testing and polish.** Full flow: start with
  anthropic → `:provider` → switch to openai (enter API key + base URL) →
  chat → switch back to anthropic (instant, credentials stored) → verify
  model picker works with new provider.

---

## Affected Files

| File | Status | Change |
|------|--------|--------|
| `config/provider.go` | **new** | `authEntry`, `providerEntry`, `AuthMethod`, `APIKeyAuth`, `OAuthAuth`, `ProviderConfig`, `AnthropicProvider`, `OpenAIProvider`, parse/convert funcs |
| `config/provider_test.go` | **new** | Round-trip, validation, backward compat tests |
| `config/config.go` | existing | Add `DefaultProvider`, `Providers` to `FileConfig`; update `Load()` to resolve from providers |
| `config/config_test.go` | existing | Tests for new `Load()` resolution path |
| `cmd/oc/main.go` | existing | Extract `createProvider`, add `providerSwapper`, wire onboarding flow, `:provider` command |
| `tui/common/providerpicker.go` | **new** | ProviderPicker overlay component |
| `tui/common/providerpicker_test.go` | **new** | Unit tests for selection logic |
| `tui/custom/ports.go` | existing | Add `OnProviderSelect`, `FetchProviders` types |
| `tui/custom/ui.go` | existing | Wire ProviderPicker into overlay dispatch, render |

---

## Tests

- [ ] `config/provider_test.go` — `parseAuth` validates per-auth-type (missing api_key → error, missing oauth fields → error, unknown type → error, nil → ok) — Unit
- [ ] `config/provider_test.go` — `parseProvider` validates per-type (anthropic without auth → error, openai without base_url → error, unknown type → error) — Unit
- [ ] `config/provider_test.go` — `toEntry` → `parseProvider` round-trip is lossless for each provider type × auth type combination — Unit
- [ ] `config/provider_test.go` — `LoadProviders` partial success (valid + invalid entries) — Unit
- [ ] `config/config_test.go` — `Load()` resolves `defaultProvider` → `ProviderConfig` fields into flat `Config` — Unit
- [ ] `config/config_test.go` — Backward compat: config without `providers` uses legacy fields — Unit
- [ ] `config/config_test.go` — Backward compat: top-level OAuth fields produce an anthropic provider with `OAuthAuth` — Unit
- [ ] `cmd/oc/` — `createProvider` type-switches correctly for each provider type × auth type — Unit
- [ ] `cmd/oc/` — `providerSwapper.Swap` changes which provider `Stream` delegates to — Unit
- [ ] `tui/common/` — ProviderPicker selection logic, highlights current — Unit
- [ ] Manual — Full onboarding flow end-to-end (described in Step 7)

---

## Open Questions

- [ ] Should "Add provider" support arbitrary names (e.g. "my-company-llm")
  or only known types? Leaning toward arbitrary — any OpenAI-compatible
  endpoint just needs a name + base URL + optional API key.
- [ ] Should we validate the provider connection (e.g. call `ListModels`)
  before completing the switch? Adds latency but catches bad API keys early.
- [ ] Should the session history be cleared when switching providers?
  Different providers may not understand each other's tool call formats.
  Leaning toward clearing with a warning message.
- [x] ~~Claude Max (OAuth) provider: should it be addable through the TUI
  onboarding flow, or only via `oc login`?~~ **Resolved:** Claude Max is an
  anthropic provider with `OAuthAuth`. The "Add provider" flow detects the
  `oauth` auth choice and directs the user to `oc login` (PKCE requires a
  browser). Once `oc login` writes the tokens, the provider entry appears in
  the picker like any other configured provider.

---

## Risks & Watch-outs

- **Concurrency:** The swap must only happen while the session is idle
  (not mid-stream). `providerSwapper` uses a `RWMutex` — `Stream()` takes
  a read lock, `Swap()` takes a write lock. If a swap is requested while
  busy, show an error ("cancel current operation first").
- **Import rules:** `providerSwapper` and `createProvider` live in `cmd/oc/`
  to avoid adding provider-specific imports to `session/` or `tui/`.
  `config/` has no provider imports — it defines the typed configs, not the
  provider constructors.
- **API key security:** API keys typed into the prompt area bypass
  `session.Send()` so they won't appear in chat history. The key is visible
  in the input area while typing — consider masking (future enhancement).
- **OAuth token refresh:** `OAuthAuth` tokens expire. The existing
  `provider.Authenticator` interface handles refresh transparently during
  requests. `createProvider` must wire the token refresh callback to persist
  updated tokens back into the provider's auth block via `SetAuth()` +
  `SaveProviders`.
- **Backward compat:** Existing configs without `providers` / `defaultProvider`
  must keep working. `Load()` falls back to legacy top-level fields. The
  `provider` JSON field is superseded but still read. Top-level OAuth fields
  (`access_token`, etc.) are treated as a legacy anthropic provider with
  `OAuthAuth` when no `providers` array exists.
- **Duplicate names:** `LoadProviders` should reject duplicate `name` values
  in the array with a clear error.

---

## Definition of Done

- [ ] `go build ./...` passes
- [ ] `go test ./...` passes
- [ ] No new import-graph cycles
- [ ] Can switch from anthropic to openai via `:provider`, entering credentials when prompted
- [ ] Can switch back to anthropic without re-entering credentials
- [ ] Can add a new custom OpenAI-compatible provider via `:provider` → "Add provider"
- [ ] Provider and model selection persisted to `~/.oc/config.json`
- [ ] Existing env var / CLI flag configuration still works (backward compat)
- [ ] `:doctor` command still works and shows correct active provider
- [ ] `CLAUDE.md` updated with new command, config fields, and provider types
