# CLAUDE.md

## Project Overview

VoiceTask is a self-hosted, voice-to-task capture web app. Users speak or type into any device (Android tablet, iPhone, PC browser), an LLM extracts structured tasks (title, project tag, priority, deadline), and tasks sync across all connected devices in real time via Server-Sent Events.

Single-user app. Passphrase-based auth with bcrypt + HMAC session cookie. No registration, no user table.

## Tech Stack

| Component       | Choice                        | Version     |
|-----------------|-------------------------------|-------------|
| Language        | Go                            | 1.24+       |
| Web framework   | Fiber                         | v2.52       |
| HTML rendering  | `html/template` (stdlib)      | -           |
| Template embed  | `embed.FS`                    | -           |
| Frontend        | HTMX + Alpine.js              | 2.0 / 3.14  |
| CSS             | Inline styles (dark theme)    | -           |
| Database        | PostgreSQL                    | 16          |
| DB driver       | pgx/v5 + pgxpool              | v5.7        |
| LLM (default)   | Claude Sonnet (Anthropic API) | -           |
| LLM (alt)       | OpenAI, Groq, Ollama          | -           |
| Reverse proxy   | Caddy                         | v2          |
| Voice capture   | Web Speech API (browser)      | -           |
| Auth            | bcrypt + HMAC cookie          | -           |
| Logging         | `log/slog` (stdlib)           | -           |

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
Browser → Caddy (HTTPS) → Fiber (recover → requestid → auth → rate limiter) → Handler
  → LLM provider (extract tasks) → PostgreSQL (store) → SSE hub (broadcast)
  → All connected clients refresh task list via HTMX
```

### Key Design Patterns

**App Struct** (`main.go`): All dependencies (pool, hub, llm, renderer, config) held in an `App` struct. Handlers are methods on `App`. No global state.

**SSE Hub** (`sse.go`): In-memory broadcast hub. `map[chan string]bool` protected by `sync.RWMutex`. Clients subscribe on `GET /tasks/stream`, unsubscribe on disconnect. Broadcasts are non-blocking (skip full channels). 30s keepalive pings. 5s reconnect retry. Uses fasthttp `SetBodyStreamWriter` for long-lived connections.

**LLM Provider Interface** (`llm/provider.go`):
```go
type Provider interface {
    ExtractTasks(ctx context.Context, transcript string) ([]ExtractedTask, error)
}
```
Claude uses the Anthropic Messages API. OpenAI/Groq/Ollama share one OpenAI-compatible implementation. Switch providers via `LLM_PROVIDER` env var. On failure, falls back to raw transcript as task title.

**Auth Middleware** (`auth.go`): Checks `session` cookie containing HMAC token derived from passphrase hash. Single-user so token is deterministic. 30-day expiry, HttpOnly, Secure, SameSite=Lax. HTMX-aware: returns `HX-Redirect` header on 401 for HTMX requests.

**HTML Rendering** (`render.go`): Templates loaded at startup via `//go:embed templates/*`. `html/template` with custom FuncMap for deadline formatting, priority badges, and project colors. `RenderDashboard` returns full page; `RenderTaskList` returns partial for HTMX swaps.

**Alpine.js + HTMX**: Capture bar (voice input with Alpine `x-data`) is OUTSIDE the `#task-list` HTMX swap target. This preserves Alpine state (listening, transcript) during HTMX DOM swaps. Delete confirmation uses Alpine inline toggle, not `hx-confirm`.

**Static Files**: Embedded via `//go:embed static/*` and served through Fiber's filesystem middleware. Everything is in the binary — no filesystem dependencies at runtime.

**Rate Limiting**: Fiber limiter middleware. 5 req/15min on `/auth`, 30 req/min on protected routes. SSE endpoint excluded (long-lived connection).

## Code Conventions

- **Go style**: `gofmt`, `go vet`. Short variable names in small scopes (`c` for Ctx, `tx` for transaction). Descriptive package-level names.
- **Error handling**: Return errors, never panic. Wrap with `fmt.Errorf("context: %w", err)`.
- **Logging**: Use `log/slog` throughout. Structured key-value pairs: `slog.Info("msg", "key", val)`.
- **No global state**: All deps in `App` struct, initialized in `main.go`, passed to handlers as methods.
- **Database**: Raw SQL with pgx. Parameterized queries (`$1, $2`). No ORM. Pool max 5 connections. Migration SQL embedded via `//go:embed`.
- **Templates**: Parsed once at startup with `template.Must()`. Never parsed at request time. FuncMap for deadline/priority/project formatting.
- **LLM JSON parsing**: `cleanJSON()` strips markdown fencing before unmarshal. On any parse failure, create a raw task with the original text as title. Always log raw LLM response on failure.

## Project Structure

```
voicetask/
├── main.go              # Entry: App struct, config, DI, routes, middleware, graceful shutdown
├── config.go            # Env var loading into Config struct
├── auth.go              # Passphrase auth middleware, login/logout handlers
├── handlers.go          # HTTP handlers (dashboard, create, toggle, edit, delete, clear, task list)
├── render.go            # html/template renderer with embed.FS, FuncMap, project colors
├── db.go                # Task struct, connection pool, embedded migration, CRUD queries
├── sse.go               # SSE hub (subscribe, unsubscribe, broadcast, keepalive, retry)
├── setup.go             # TEMPORARY: /setup route for bcrypt hash generation
├── llm/
│   ├── provider.go      # Provider interface, ExtractedTask, system prompt, cleanJSON
│   ├── claude.go        # Anthropic Messages API implementation
│   ├── openai_compat.go # OpenAI/Groq/Ollama shared implementation
│   └── provider_test.go # 13 unit tests for JSON parsing
├── db/
│   └── migrations/
│       └── 001_create_tasks.sql
├── templates/
│   ├── layout.html      # Base layout (fonts, HTMX, Alpine, CSS, error toast)
│   ├── dashboard.html   # Main page (wordmark, clock, capture bar, voice component, SSE)
│   ├── login.html       # Login form (standalone, dark theme)
│   └── partials/
│       └── tasklist.html # Task list grouped by project (HTMX swap target)
├── static/
│   └── manifest.json    # PWA manifest
├── cmd/hashpass/main.go # CLI: generate bcrypt hash
├── auth_test.go         # 9 auth tests (HMAC, middleware, login flow)
├── handlers_test.go     # 8 handler integration tests (mock LLM, real DB)
├── db_test.go           # 6 DB CRUD tests
├── Caddyfile            # Reverse proxy config (not yet created)
├── voicetask.service    # systemd unit file (not yet created)
├── Makefile
├── .env.example
├── .gitignore
├── BUILD_PLAN.md
├── SETUP_TODO.md        # TEMPORARY: steps to restore strict validation
├── go.mod
└── go.sum
```

## Testing

```bash
make test                    # runs all tests (skips DB tests if no Postgres)
go test -race ./...          # run with race detector
go test -short ./...         # skip DB-dependent tests
go test -v -run TestHandle   # run handler tests only
```

**Test DB setup:**
```bash
sudo -u postgres createdb voicetask_test
sudo -u postgres psql -c "CREATE USER voicetask WITH PASSWORD 'voicetask';"
sudo -u postgres psql -c "GRANT ALL ON DATABASE voicetask_test TO voicetask;"
export TEST_DATABASE_URL="postgres://voicetask:voicetask@localhost:5432/voicetask_test?sslmode=disable"
```

| File | Tests | What |
|------|-------|------|
| `llm/provider_test.go` | 13 | JSON parsing, markdown fencing, validation, fallback |
| `db_test.go` | 6 | CRUD operations against real Postgres |
| `auth_test.go` | 9 | HMAC tokens, middleware, login/logout, auth bypass |
| `handlers_test.go` | 8 | Full request cycle with mock LLM and real DB |

All 36 tests pass with `-race` flag (zero data races).

## Design Decisions (Why Not X?)

- **No Docker** — Single binary + Postgres. Docker adds complexity for zero benefit at this scale.
- **No Redis** — SSE hub is in-memory. 2-3 clients, not 2,000.
- **No ORM** — One table, ~5 queries. pgx is simpler and faster.
- **No JS build step** — All frontend deps via CDN. No webpack, npm, or node_modules.
- **Fiber v2 not v3** — v3 requires Go 1.25+ and its context.Context methods are nops due to fasthttp. v2 is stable and well-known. Upgrade when v3 matures.
- **html/template not Templ** — Stdlib is sufficient for 3 templates + 1 partial. Migrate to Templ if UI grows significantly.
- **html/template not fmt.Sprintf** — Auto-escaping prevents XSS. Composable partials. Minimal overhead.
- **UUID IDs not sequential** — No sequential IDs exposed in URLs. UUIDs via pgcrypto.
- **Inline CSS not Tailwind** — Dark theme with custom design language (warm blacks, amber accents). Inline styles keep everything in templates with no build step. Move to Tailwind if the CSS grows unwieldy.
- **embed.FS for everything** — Templates, migrations, and static files all embedded. Single binary deployment with zero filesystem dependencies.
