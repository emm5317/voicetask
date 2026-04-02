# CLAUDE.md

## Project Overview

VoiceTask is a self-hosted, voice-to-task capture web app. Users speak or type into any device (Android tablet, iPhone, PC browser), an LLM extracts structured tasks (title, project tag, priority, deadline), and tasks sync across all connected devices in real time via Server-Sent Events.

Single-user app. Passphrase-based auth with bcrypt + HMAC session cookie. No registration, no user table.

## Tech Stack

| Component       | Choice                        | Version     |
|-----------------|-------------------------------|-------------|
| Language        | Go                            | 1.24+       |
| Web framework   | Fiber                         | v2.52       |
| HTML rendering  | Templ (type-safe Go templates)| v0.3        |
| Frontend        | HTMX + Alpine.js              | 2.0 / 3.14  |
| CSS             | Tailwind CSS v4 (standalone)  | v4          |
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
make build          # templ generate + tailwind css + go build -o voicetask .
make build-linux    # templ generate + tailwind css + GOOS=linux GOARCH=amd64 go build
make run            # go run .
make dev            # templ watch + tailwind watch + air (live reload)
make test           # go test ./...
make templ          # templ generate (compile .templ → Go code)
make css            # tailwind standalone CLI → static/styles.css
make css-watch      # tailwind watch mode for development
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

**HTML Rendering** (`render.go` + `templates/`): Templ components generate type-safe Go code at build time. Handlers render components directly to the response writer. Helper functions for deadline formatting, priority badges, and project colors live in `templates/components/helpers.go`. Shared data types live in `templates/model/types.go`.

**Alpine.js + HTMX**: Capture bar (voice input with Alpine `x-data`) is OUTSIDE the `#task-list` HTMX swap target. This preserves Alpine state (listening, transcript) during HTMX DOM swaps. Delete confirmation uses Alpine inline toggle, not `hx-confirm`.

**Static Files**: Embedded via `//go:embed static/*` and served through Fiber's filesystem middleware. Everything is in the binary — no filesystem dependencies at runtime.

**Rate Limiting**: Fiber limiter middleware. 5 req/15min on `/auth`, 30 req/min on protected routes. SSE endpoint excluded (long-lived connection).

## Code Conventions

- **Go style**: `gofmt`, `go vet`. Short variable names in small scopes (`c` for Ctx, `tx` for transaction). Descriptive package-level names.
- **Error handling**: Return errors, never panic. Wrap with `fmt.Errorf("context: %w", err)`.
- **Logging**: Use `log/slog` throughout. Structured key-value pairs: `slog.Info("msg", "key", val)`.
- **No global state**: All deps in `App` struct, initialized in `main.go`, passed to handlers as methods.
- **Database**: Raw SQL with pgx. Parameterized queries (`$1, $2`). No ORM. Pool max 5 connections. Migration SQL embedded via `//go:embed`.
- **Templates**: Templ components compiled to Go code at build time via `templ generate`. Type-safe — no runtime template parsing. CSS via Tailwind utility classes with dynamic project colors as inline styles.
- **LLM JSON parsing**: `cleanJSON()` strips markdown fencing before unmarshal. On any parse failure, create a raw task with the original text as title. Always log raw LLM response on failure.

## Project Structure

```
voicetask/
├── main.go              # Entry: App struct, config, DI, routes, middleware, graceful shutdown
├── config.go            # Env var loading into Config struct
├── auth.go              # Passphrase auth middleware, login/logout handlers
├── handlers.go          # HTTP handlers (dashboard, create, toggle, edit, delete, clear, task list)
├── handlers_time.go     # Time tracking handlers (timer, entries, weekly summary, export)
├── render.go            # Data building, type aliases, Renderer struct (Templ-based)
├── db.go                # Connection pool, embedded migration
├── sse.go               # SSE hub (subscribe, unsubscribe, broadcast, keepalive, retry)
├── llm/
│   ├── provider.go      # Provider interface, ExtractedTask, system prompt, cleanJSON
│   ├── claude.go        # Anthropic Messages API implementation
│   ├── openai_compat.go # OpenAI/Groq/Ollama shared implementation
│   └── provider_test.go # 13 unit tests for JSON parsing
├── db/
│   ├── models.go        # Task and TimeEntry structs
│   ├── queries.go       # SQL queries (CRUD)
│   └── migrations/
│       ├── 001_create_tasks.sql
│       └── 002_create_time_entries.sql
├── templates/
│   ├── layout.templ     # HTML shell (head, scripts, body wrapper)
│   ├── login.templ      # Standalone login page
│   ├── dashboard.templ  # Dashboard orchestrator (split-panel grid)
│   ├── model/
│   │   └── types.go     # Shared data types (DashboardData, TaskGroup, etc.)
│   └── components/
│       ├── helpers.go    # Format functions (deadline, priority, project colors)
│       ├── header.templ  # Wordmark, clock, theme toggle
│       ├── capture_bar.templ  # Voice/text input with Alpine.js
│       ├── task_group.templ   # Task section with progress bar
│       ├── task_row.templ     # Single task with inline editing
│       ├── time_panel.templ   # Timer, matter buttons, time entries
│       └── weekly_summary.templ # Weekly breakdown table
├── static/
│   ├── input.css        # Tailwind CSS v4 theme config + animations
│   ├── styles.css       # Generated (not committed) — Tailwind output
│   ├── app.js           # All Alpine.js components + HTMX handlers
│   └── manifest.json    # PWA manifest
├── cmd/hashpass/main.go # CLI: generate bcrypt hash
├── auth_test.go         # 9 auth tests (HMAC, middleware, login flow)
├── handlers_test.go     # 8 handler integration tests (mock LLM, real DB)
├── db_test.go           # 6 DB CRUD tests
├── Makefile             # Build pipeline (templ generate → tailwind → go build)
├── .env.example
├── .gitignore
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
- **No JS build step** — All frontend deps via CDN. No webpack, npm, or node_modules. Tailwind uses standalone CLI binary (no Node.js).
- **Fiber v2 not v3** — v3 requires Go 1.25+ and its context.Context methods are nops due to fasthttp. v2 is stable and well-known. Upgrade when v3 matures.
- **Templ not html/template** — Type-safe templates with compile-time checks. Components receive Go structs directly. No runtime template parsing or FuncMap.
- **Tailwind CSS v4 standalone** — No Node.js required. Single binary generates purged/minified CSS. Custom theme in `static/input.css` with `@theme {}` block.
- **UUID IDs not sequential** — No sequential IDs exposed in URLs. UUIDs via pgcrypto.
- **embed.FS for static files** — CSS, JS, and manifest embedded in binary. Single binary deployment with zero filesystem dependencies.
