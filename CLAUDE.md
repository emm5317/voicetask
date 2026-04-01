# CLAUDE.md

## Project Overview

VoiceTask is a self-hosted, voice-to-task capture web app. Users speak or type into any device (Android tablet, iPhone, PC browser), an LLM extracts structured tasks (title, project tag, priority, deadline), and tasks sync across all connected devices in real time via Server-Sent Events.

Single-user app. Passphrase-based auth with bcrypt + HMAC session cookie. No registration, no user table.

## Tech Stack

| Component       | Choice                        | Version     |
|-----------------|-------------------------------|-------------|
| Language        | Go                            | 1.22+       |
| Web framework   | Fiber                         | v2          |
| HTML rendering  | `html/template` (stdlib)      | -           |
| Template embed  | `embed.FS`                    | -           |
| Frontend        | HTMX + Alpine.js              | 2.x / 3.x  |
| CSS             | Tailwind CSS (CDN)            | -           |
| Database        | PostgreSQL                    | 16          |
| DB driver       | pgx/v5 + pgxpool              | v5          |
| LLM (default)   | Claude Sonnet (Anthropic API) | -           |
| LLM (alt)       | OpenAI, Groq, Ollama          | -           |
| Reverse proxy   | Caddy                         | v2          |
| Voice capture   | Web Speech API (browser)      | -           |
| Auth            | bcrypt + HMAC cookie          | -           |

## Commands

```bash
make build          # go build -o voicetask .
make build-linux    # GOOS=linux GOARCH=amd64 go build -o voicetask .
make run            # go run .
make test           # go test ./...
make deploy         # cross-compile + scp + restart (requires SERVER env var)
make hash PASS=x    # generate bcrypt hash for passphrase
make migrate        # run SQL migrations on remote server
```

## Environment Variables

See `.env.example` for the full list. Key vars:

- `APP_PORT` — server port (default 8090)
- `APP_PASSPHRASE_HASH` — bcrypt hash of login passphrase
- `DATABASE_URL` — Postgres connection string
- `LLM_PROVIDER` — one of: `claude`, `openai`, `groq`, `ollama`
- `ANTHROPIC_API_KEY` — required when LLM_PROVIDER=claude
- `PROJECT_TAGS` — comma-separated list passed to LLM and used for UI grouping

## Architecture

### Request Flow
```
Browser → Caddy (HTTPS) → Fiber (auth middleware → rate limiter) → Handler
  → LLM provider (extract tasks) → PostgreSQL (store) → SSE hub (broadcast)
  → All connected clients refresh via HTMX
```

### Key Design Patterns

**SSE Hub** (`sse.go`): In-memory broadcast hub. `map[chan string]bool` protected by `sync.RWMutex`. Clients subscribe on `GET /tasks/stream`, unsubscribe on disconnect. Broadcasts are non-blocking (skip full channels). Fine for 2-3 concurrent clients.

**LLM Provider Interface** (`llm/provider.go`):
```go
type Provider interface {
    ExtractTasks(ctx context.Context, transcript string) ([]ExtractedTask, error)
}
```
Claude uses the Anthropic Messages API. OpenAI/Groq/Ollama share one OpenAI-compatible implementation. Switch providers via `LLM_PROVIDER` env var.

**Auth Middleware** (`auth.go`): Checks `session` cookie containing HMAC token derived from passphrase hash. Single-user so token is deterministic. 30-day expiry, HttpOnly, Secure, SameSite=Lax.

**HTML Rendering** (`render.go`): Templates loaded at startup via `//go:embed templates/*`. `html/template` with custom FuncMap for date formatting and priority CSS classes. Partials returned for HTMX swaps, full page for initial load.

**Alpine.js + HTMX**: Use `x-data` scoped to elements that are NOT swapped by HTMX. The capture bar (voice input) is outside the `#task-list` swap target. If Alpine state inside swapped regions is needed, use `htmx:afterSettle` to reinitialize.

## Code Conventions

- **Go style**: `gofmt`, `go vet`. Short variable names in small scopes (`c` for Ctx, `tx` for transaction). Descriptive package-level names.
- **Error handling**: Return errors, never panic. Wrap with `fmt.Errorf("context: %w", err)`.
- **No global state**: Dependencies held in a struct (or closures). DB pool, SSE hub, LLM provider, and template renderer are initialized in `main.go` and passed to handlers.
- **Database**: Raw SQL with pgx. Parameterized queries (`$1, $2`). No ORM. Pool max 5 connections.
- **Templates**: Parsed once at startup with `template.Must()`. Never parsed at request time.
- **LLM JSON parsing**: `cleanJSON()` strips markdown fencing before unmarshal. On any parse failure, create a raw task with the original text as title. Always log raw LLM response on failure.

## Project Structure

```
voicetask/
├── main.go              # Entry: config, DI, routes, graceful shutdown
├── config.go            # Env var loading into Config struct
├── auth.go              # Passphrase auth middleware + login handler
├── handlers.go          # HTTP handlers (create, toggle, edit, delete, clear)
├── render.go            # Template loading via embed.FS, render helpers
├── sse.go               # SSE hub (subscribe, unsubscribe, broadcast)
├── llm/
│   ├── provider.go      # Provider interface, ExtractedTask, system prompt, cleanJSON
│   ├── claude.go        # Anthropic Messages API implementation
│   └── openai_compat.go # OpenAI/Groq/Ollama shared implementation
├── db/
│   ├── db.go            # Connection pool, CRUD queries
│   └── migrations/
│       └── 001_create_tasks.sql
├── templates/
│   ├── layout.html      # Base layout (head, CDN links, body wrapper)
│   ├── dashboard.html   # Main page (capture bar + task list)
│   ├── login.html       # Login form
│   └── partials/
│       └── tasklist.html # Task list fragment (HTMX swap target)
├── cmd/hashpass/main.go # CLI: generate bcrypt hash
├── static/manifest.json # PWA manifest
├── Caddyfile            # Reverse proxy config
├── voicetask.service    # systemd unit file
├── Makefile
├── .env.example
├── .gitignore
├── go.mod
└── go.sum
```

## Testing

```bash
make test    # runs all tests
```

- **LLM parsing** (`llm/provider_test.go`): Unit tests with fixtures for valid JSON, markdown-wrapped JSON, multi-task, malformed, empty responses.
- **DB CRUD** (`db/db_test.go`): Integration tests against a test Postgres DB. Skip with `-short` flag if DB unavailable.
- **Handlers** (`handlers_test.go`): Mock LLM provider, verify task creation and HTML response.
- **Auth** (`auth_test.go`): HMAC signing/verification round-trips.

## Design Decisions (Why Not X?)

- **No Docker** — Single binary + Postgres. Docker adds complexity for zero benefit at this scale.
- **No Redis** — SSE hub is in-memory. 2-3 clients, not 2,000.
- **No ORM** — One table, ~5 queries. pgx is simpler and faster.
- **No JS build step** — All frontend deps via CDN. No webpack, npm, or node_modules.
- **Fiber v2 not v3** — v3 requires Go 1.25+ and its context.Context methods are nops due to fasthttp. v2 is stable and well-known. Upgrade when v3 matures.
- **html/template not Templ** — Stdlib is sufficient for 2 pages + 1 partial. Migrate to Templ if UI grows significantly.
- **html/template not fmt.Sprintf** — Auto-escaping prevents XSS. Composable partials. Minimal overhead.
- **UUID IDs not sequential** — No sequential IDs exposed in URLs. UUIDs via pgcrypto.
