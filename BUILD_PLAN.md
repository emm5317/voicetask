# VoiceTask — Build Plan

## What This Is

A self-hosted, voice-to-task capture web app. Speak or type on any device — an LLM extracts structured, tagged tasks. Tasks sync across all connected devices via SSE. The Android tablet on your desk is an always-on dashboard.

**Monthly cost:** ~$4.77 ($4 droplet + ~$0.77 Claude Sonnet API)

---

## Architecture

```
Browser (any device)
  │
  ├─ Web Speech API (on-device transcription)
  │
  ▼
Caddy (auto-HTTPS) → Fiber Server (Go)
  │
  ├─ POST /tasks → LLM extracts tasks → Postgres INSERT → SSE broadcast
  ├─ GET /tasks/stream → SSE connection (real-time updates)
  ├─ PATCH/DELETE /tasks/:id → Postgres UPDATE/DELETE → SSE broadcast
  │
  ├─ Claude Sonnet API (default, swappable to OpenAI/Groq/Ollama)
  └─ PostgreSQL (local on droplet)
```

**End-to-end flow:** Tap mic → browser transcribes → POST to server → LLM extracts structured tasks → insert into Postgres → broadcast SSE → all devices update. ~2-3 seconds.

---

## Data Model

```sql
CREATE TABLE tasks (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title          TEXT NOT NULL,
    project_tag    TEXT NOT NULL DEFAULT 'personal',
    priority       TEXT NOT NULL DEFAULT 'normal',
    deadline       DATE,
    raw_transcript TEXT,
    completed      BOOLEAN NOT NULL DEFAULT FALSE,
    completed_at   TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sort_order     INTEGER NOT NULL DEFAULT 0
);
```

**Priority values:** `urgent` | `high` | `normal` | `low`
**Project tags:** Configured via `PROJECT_TAGS` env var. Default: `clientsite, campbells, makinen, tradebot, personal, home`
**Display order:** Incomplete first → priority (urgent→low) → sort_order → newest first, grouped by project tag.

---

## API Routes

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/login` | Login page |
| POST | `/auth` | Authenticate with passphrase |
| GET | `/` | Dashboard (full page) |
| POST | `/tasks` | Create task(s) from voice/text |
| PATCH | `/tasks/:id` | Toggle complete or edit task |
| DELETE | `/tasks/:id` | Delete task |
| POST | `/tasks/clear` | Bulk delete completed tasks |
| GET | `/tasks/stream` | SSE endpoint |

All mutation endpoints return an HTML fragment (re-rendered task list) for HTMX to swap into `#task-list`.

---

## LLM Integration

### Provider Interface

```go
type Provider interface {
    ExtractTasks(ctx context.Context, transcript string) ([]ExtractedTask, error)
}
```

### Providers

| Provider | Env Value | Format | Cost |
|----------|-----------|--------|------|
| Claude Sonnet | `claude` | Anthropic Messages API | ~$0.77/mo |
| OpenAI GPT-4o-mini | `openai` | OpenAI Chat Completions | ~$0.15/mo |
| Groq (Llama 3.3 70B) | `groq` | OpenAI-compatible | $0 |
| Ollama (local) | `ollama` | OpenAI-compatible | $0 |

Switch with `LLM_PROVIDER=groq` in `.env`.

### System Prompt

```
You are a task extraction assistant. Return ONLY valid JSON — no markdown, no backticks.

Known projects: {PROJECT_TAGS}. Infer the tag from context. Default to "personal".

Priority: "urgent" for time-sensitive legal deadlines, "high" for this-week,
"normal" for standard, "low" for someday/maybe.

Parse deadline language relative to today: {TODAY}.
Multiple tasks in one input → multiple items in response.

Response format: {"tasks":[{"title":"...","project_tag":"...","priority":"...","deadline":"YYYY-MM-DD"}]}
```

### Fallback Behavior

If LLM fails (network error, rate limit, malformed JSON): create a single task with raw transcript as title, `personal` tag, `normal` priority. Input is never lost.

`cleanJSON()` strips markdown fencing before parsing. Validates tags against known list, priorities against allowed values. Logs raw LLM response on parse failure.

---

## Authentication

- Single-user, passphrase-based. No user table.
- Passphrase bcrypt hash stored in `.env` (`APP_PASSPHRASE_HASH`).
- Session: HMAC token in HttpOnly/Secure/SameSite=Lax cookie, 30-day expiry.
- Generate hash: `make hash PASS="your-passphrase"`
- Rate limited: 5 attempts per 15 minutes on `/auth`.

---

## Frontend

- **html/template** with `embed.FS` — templates parsed once at startup, auto-escaping, composable partials.
- **HTMX** — all mutations return HTML fragments swapped into `#task-list`.
- **Alpine.js** — voice capture component (Web Speech API), lightweight UI state.
- **Tailwind CSS (CDN)** — dark theme (zinc-900 bg), amber accent, responsive.
- **HTMX SSE extension** — listens for `tasks-updated` events, triggers task list refresh.
- **Alpine.js + HTMX interaction** — capture bar (with Alpine voice state) is outside the `#task-list` HTMX swap target to avoid state loss. Use `htmx:afterSettle` if Alpine reinit needed inside swapped regions.

### Voice Capture

- Chrome/Edge: full Web Speech API support (Google cloud speech engine).
- Safari iOS 14.5+: supported, requires user gesture to start.
- Firefox: not supported — mic button hidden, text-only input.
- Tap mic → listen → auto-submit on speech end. Interim results shown in real time.

### PWA

`manifest.json` enables "Add to Home Screen" on Android. Standalone display, no browser chrome.

---

## Real-Time Sync (SSE)

In-memory `SSEHub` with `map[chan string]bool` + `sync.RWMutex`.

```html
<div hx-ext="sse" sse-connect="/tasks/stream" style="display:none">
    <div hx-get="/" hx-trigger="sse:tasks-updated"
         hx-target="#task-list" hx-select="#task-list" hx-swap="innerHTML">
    </div>
</div>
```

On mutation → server broadcasts `tasks-updated` → HTMX fetches fresh task list → swap. Originating device updates from the fetch response; other devices update via SSE. Same render function for both paths.

---

## Security (Day One)

- **Rate limiting**: Fiber middleware. 5 req/15min on `/auth`, 30 req/min on `/tasks`.
- **HTTPS**: Caddy auto-provisions TLS via Let's Encrypt.
- **Security headers** (Caddy): Content-Security-Policy, X-Content-Type-Options, X-Frame-Options, Referrer-Policy.
- **Auth**: bcrypt passphrase, HMAC session cookie, HttpOnly/Secure/SameSite.
- **SQL injection**: pgx parameterized queries only.
- **XSS**: html/template auto-escaping.

---

## Project Structure

```
voicetask/
├── main.go                    # Entry: config, DI, routes, graceful shutdown
├── config.go                  # Env var loading into Config struct
├── auth.go                    # Passphrase auth middleware + login handler
├── handlers.go                # HTTP handlers
├── render.go                  # html/template loading via embed.FS, render helpers
├── sse.go                     # SSE hub
├── llm/
│   ├── provider.go            # Provider interface, ExtractedTask, system prompt, cleanJSON
│   ├── claude.go              # Anthropic Messages API
│   └── openai_compat.go       # OpenAI/Groq/Ollama shared implementation
├── db/
│   ├── db.go                  # Connection pool, CRUD queries
│   └── migrations/
│       └── 001_create_tasks.sql
├── templates/
│   ├── layout.html            # Base layout (head, CDN links, body)
│   ├── dashboard.html         # Main page (capture bar + task list)
│   ├── login.html             # Login form
│   └── partials/
│       └── tasklist.html      # Task list fragment (HTMX swap target)
├── cmd/hashpass/main.go       # CLI: generate bcrypt hash
├── static/manifest.json       # PWA manifest
├── testdata/
│   └── llm_responses.json     # LLM parsing test fixtures
├── handlers_test.go
├── llm/provider_test.go
├── db/db_test.go
├── Caddyfile
├── voicetask.service
├── Makefile
├── .env.example
├── .gitignore
├── CLAUDE.md
├── BUILD_PLAN.md
└── LICENSE
```

---

## Build Phases

### Phase 1: Scaffolding (30 min)

| File | What |
|------|------|
| `go.mod` | `go mod init`, install Fiber v2, pgx/v5, godotenv, bcrypt |
| `.gitignore` | Binary, .env, IDE files, OS files, coverage output |
| `.env.example` | All config vars with placeholder values |
| `config.go` | Load env vars into typed Config struct |
| `Makefile` | build, build-linux, run, test, deploy, hash, migrate targets |
| `CLAUDE.md` | AI development guide |
| `BUILD_PLAN.md` | This document |

**Milestone:** `go build .` succeeds.

### Phase 2: Database (45 min)

| File | What |
|------|------|
| `db/migrations/001_create_tasks.sql` | Tasks table + indexes |
| `db/db.go` | `NewPool()`, `RunMigrations()`, `InsertTask()`, `ListTasks()`, `ToggleTask()`, `UpdateTask()`, `DeleteTask()`, `ClearCompleted()` |
| `db/db_test.go` | CRUD integration tests (requires test Postgres) |

**Milestone:** `make test` passes DB tests.

### Phase 3: LLM Integration (1 hr)

| File | What |
|------|------|
| `llm/provider.go` | `Provider` interface, `ExtractedTask` struct, system prompt template, `cleanJSON()` |
| `llm/claude.go` | Anthropic Messages API client |
| `llm/openai_compat.go` | OpenAI-compatible client (shared by OpenAI, Groq, Ollama) |
| `llm/provider_test.go` | Unit tests: valid JSON, markdown-wrapped, multi-task, malformed, empty |
| `testdata/llm_responses.json` | Test fixtures |

**Milestone:** `make test` passes LLM parsing tests.

### Phase 4: Core Server + Handlers (1.5 hr)

| File | What |
|------|------|
| `main.go` | Load config, init DB/LLM/SSE, create Fiber app, middleware, routes, graceful shutdown |
| `handlers.go` | `handleCreateTask`, `handleUpdateTask`, `handleDeleteTask`, `handleClearCompleted` |
| `auth.go` | `AuthRequired` middleware, `handleLogin` (GET), `handleAuth` (POST), HMAC cookie |
| `sse.go` | `SSEHub` with `Subscribe()`, `Unsubscribe()`, `Broadcast()`, `handleStream()` |
| `cmd/hashpass/main.go` | Bcrypt hash generator CLI |

**Milestone:** `curl -X POST -d "input=test task" localhost:8090/tasks` creates a task.

### Phase 5: Templates + Frontend (2 hr)

| File | What |
|------|------|
| `render.go` | `//go:embed templates/*`, template loading with FuncMap, render functions |
| `templates/layout.html` | Head (Tailwind/HTMX/Alpine CDN), dark theme body wrapper |
| `templates/dashboard.html` | Capture bar, SSE listener, task list container, clear button |
| `templates/partials/tasklist.html` | Tasks grouped by tag, priority badges, deadline formatting |
| `templates/login.html` | Passphrase form with error feedback |
| `static/manifest.json` | PWA manifest |

**Milestone:** Full working app locally — voice and text capture, grouped tasks, complete/delete, SSE sync.

### Phase 6: Integration + Polish (1 hr)

| Task | What |
|------|------|
| Wire SSE | Broadcasts in all mutation handlers |
| Keyboard shortcuts | `/` focuses input, `Enter` submits |
| `handlers_test.go` | Integration test: POST /tasks with mock LLM → verify DB + HTML |
| Browser testing | Voice in Chrome, text fallback in Firefox, Safari iOS |

**Milestone:** `make test` passes all tests. Manual cross-device SSE confirmed.

### Phase 7: Deployment (1 hr)

| File/Task | What |
|-----------|------|
| `Caddyfile` | Reverse proxy + security headers (CSP, X-Frame-Options, etc.) |
| `voicetask.service` | systemd unit file |
| Droplet setup | Ubuntu 24.04, Postgres, Caddy, DNS, TLS |
| `make deploy` | Cross-compile → scp → restart |

**Milestone:** `https://tasks.emm5317.com` is live with auth, voice capture, and real-time sync.

---

## Testing Strategy

| Layer | Scope | Method | When |
|-------|-------|--------|------|
| LLM parsing | `cleanJSON()`, JSON→ExtractedTask | Unit tests with fixtures | Phase 3 |
| DB CRUD | Insert, List, Toggle, Delete, Clear | Integration tests (test DB) | Phase 2 |
| Auth | HMAC sign/verify, cookie lifecycle | Unit tests | Phase 4 |
| Handlers | POST /tasks end-to-end | Mock LLM, real DB | Phase 6 |
| Voice/SSE/UI | Browser behavior | Manual testing | Phase 6-7 |

All tests use Go stdlib `testing`. No test framework deps. Use `-short` flag to skip DB-dependent tests.

---

## Day-One Extras (During Build)

- **Fail2ban** (15 min) — ban IPs after 5 failed login attempts
- **Nightly pg_dump** (15 min) — cron job to Backblaze B2

## Future Roadmap

| Session | Feature | Effort |
|---------|---------|--------|
| 2 | Push notifications via Ntfy | 1-2 hr |
| 3 | Whisper.cpp on Proxmox (better accuracy) | half day |
| 4 | sqlc migration (type-safe queries) | 2-3 hr |
| 5 | Templ for type-safe HTML | 3-4 hr |
| 6 | Recurring tasks | 2-3 hr |
| 7 | Daily digest email | 2 hr |
| 8 | Obsidian sync | 1-2 hr |
| 9 | Time tracking | half day |
| 10 | Archive & history view | 1-2 hr |

---

## Environment Variables

```bash
APP_PORT=8090
APP_PASSPHRASE_HASH=$2a$10$...          # go run ./cmd/hashpass "passphrase"
DATABASE_URL=postgres://voicetask:pw@localhost:5432/voicetask?sslmode=disable
LLM_PROVIDER=claude                      # claude | openai | groq | ollama
ANTHROPIC_API_KEY=sk-ant-api03-...
# OPENAI_API_KEY=sk-...
# GROQ_API_KEY=gsk_...
# OLLAMA_URL=http://192.168.x.x:11434
PROJECT_TAGS=clientsite,campbells,makinen,tradebot,personal,home
```
