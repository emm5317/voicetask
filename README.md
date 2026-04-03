# VoiceTask

![Go](https://img.shields.io/badge/Go-1.24-00ADD8?logo=go&logoColor=white)
![Fiber](https://img.shields.io/badge/Fiber-v2.52-00ACD7?logo=go&logoColor=white)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-4169E1?logo=postgresql&logoColor=white)
![Tailwind CSS](https://img.shields.io/badge/Tailwind_CSS-v4-06B6D4?logo=tailwindcss&logoColor=white)
![HTMX](https://img.shields.io/badge/HTMX-2.0-3366CC)
![Alpine.js](https://img.shields.io/badge/Alpine.js-3.14-8BC0D0?logo=alpine.js&logoColor=white)
![License](https://img.shields.io/badge/License-MIT-green)

A self-hosted, voice-to-task capture web app. Speak or type on any device — an LLM extracts structured, tagged tasks with priorities and deadlines. Tasks sync across all connected devices in real time via Server-Sent Events.

## How It Works

1. Tap the mic or type a task on any device (phone, tablet, PC)
2. Browser's Web Speech API transcribes voice on-device (continuous mode with 3s silence auto-stop)
3. Server sends the transcript to an LLM (Claude Sonnet by default)
4. LLM extracts structured tasks: title, project tag, priority, deadline
5. Tasks are stored in PostgreSQL and broadcast via SSE
6. All connected devices update instantly

A single rambling voice note like *"I need to draft that motion to compel by Thursday and also follow up with Kayla about the demo"* becomes two clean, tagged, prioritized task items.

## Features

**Task Capture**
- Voice capture via Web Speech API — continuous listening, 3s silence auto-stop, real-time transcript preview
- LLM extraction — automatic tagging, prioritization, and deadline parsing (conservative defaults)
- Multi-provider LLM — Claude, OpenAI, Groq, or Ollama (switch with one env var)
- Inline editing — change title, project tag, priority, and deadline with date picker
- Drag-and-drop reorder within project groups (SortableJS)

**Time Tracking**
- Built-in timer with matter/project switching
- Decimal time extraction from voice notes (e.g., "call with Bob .2" sets 0.2h)
- Daily totals, weekly summary grid, manual entry
- CSV export and email reports via SMTP

**Organization & Sync**
- Real-time sync via SSE with 5s auto-reconnect
- Project grouping with color coding and progress bars
- Custom tags with autocomplete
- Priority badges (urgent, high, normal, low) and deadline display
- CSV/JSON export for backup or analysis

**Notifications**
- Push notifications via Ntfy integration
- Daily digest email — morning summary of open tasks via SMTP

**Infrastructure**
- Single binary — static files, migrations, and CSS embedded via `go:embed`
- Passphrase auth — bcrypt + HMAC session cookie with CSRF protection
- Rate limiting — 5 req/15min on auth, 60 req/min on protected routes
- Health endpoint (`/health`) with DB connectivity check
- Dark/light theme persisted to localStorage
- PWA support — add to home screen on Android/iOS

## Quick Start

### Prerequisites

- Go 1.24+
- PostgreSQL 16
- An Anthropic API key (or OpenAI/Groq key, or local Ollama)

### Setup

```bash
# Clone
git clone https://github.com/emm5317/voicetask.git
cd voicetask

# Create database
sudo -u postgres psql <<EOF
CREATE USER voicetask WITH PASSWORD 'your-password';
CREATE DATABASE voicetask OWNER voicetask;
\c voicetask
CREATE EXTENSION IF NOT EXISTS pgcrypto;
EOF

# Configure
cp .env.example .env
# Edit .env with your database URL, API key, and passphrase hash

# Generate passphrase hash
go run ./cmd/hashpass "your-passphrase"
# Paste the output into .env as APP_PASSPHRASE_HASH

# Run
make run
# Visit http://localhost:8090
```

### Development

```bash
# Install development tools
go install github.com/air-verse/air@latest          # live reload
go install github.com/a-h/templ/cmd/templ@latest    # templ compiler

# Download Tailwind CSS standalone CLI (no Node.js needed)
make bin/tailwindcss

# Run with hot reload (watches .go, .templ, .css files)
make dev

# Build pipeline: templ generate -> tailwind css -> go build
make build

# Individual steps
make templ       # compile .templ files to Go code
make css         # generate static/styles.css from Tailwind
make test        # run all tests
make lint        # run golangci-lint
```

## Configuration

All configuration is via environment variables (or `.env` file):

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `APP_PORT` | No | `8090` | Server port |
| `APP_PASSPHRASE_HASH` | Yes | — | bcrypt hash of login passphrase |
| `APP_SESSION_SECRET` | No | derived | HMAC key for session cookies |
| `DATABASE_URL` | Yes | — | Postgres connection string |
| `DB_MAX_CONNS` | No | `5` | Max database pool connections |
| `LLM_PROVIDER` | No | `claude` | `claude`, `openai`, `groq`, or `ollama` |
| `ANTHROPIC_API_KEY` | If claude | — | Anthropic API key |
| `OPENAI_API_KEY` | If openai | — | OpenAI API key |
| `GROQ_API_KEY` | If groq | — | Groq API key |
| `OLLAMA_URL` | If ollama | `http://localhost:11434` | Ollama endpoint |
| `PROJECT_TAGS` | No | — | Comma-separated project tags |
| `NTFY_URL` | No | — | Ntfy server URL for push notifications |
| `NTFY_TOPIC` | No | — | Ntfy topic to publish to |
| `SMTP_HOST` | No | — | SMTP server for daily digest email |
| `SMTP_PORT` | No | `587` | SMTP port |
| `SMTP_USER` | No | — | SMTP username |
| `SMTP_PASSWORD` | No | — | SMTP password |
| `EMAIL_TO` | No | — | Digest recipient email |
| `DIGEST_HOUR` | No | `7` | Hour (0-23) to send daily digest |

## Architecture

```
Browser -> Caddy (HTTPS + HSTS) -> Fiber (recover -> requestid -> csrf -> auth -> rate limiter)
  -> Handler -> LLM provider -> PostgreSQL -> SSE hub -> All connected clients
```

- **App struct** — All dependencies (pool, queries, hub, llm, renderer) in one struct. No global state.
- **SSE hub** — In-memory `map[chan string]bool` with mutex. Non-blocking broadcast, 30s keepalive, 5s reconnect. Graceful shutdown via `Close()`.
- **LLM provider interface** — Swappable via env var. Claude uses Anthropic API; OpenAI/Groq/Ollama share one implementation. Retry on empty response before fallback.
- **sqlc** — Type-safe generated queries with native Go types. Zero hand-written `Scan()` calls.
- **embed.FS** — Migrations and static files baked into the binary. Templ components compile to native Go code.
- **CSRF protection** — Fiber CSRF middleware with `X-CSRF-Token` header for HTMX requests.

## API Routes

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/health` | Health check (DB ping, no auth) |
| `GET` | `/login` | Login page |
| `POST` | `/auth` | Authenticate (rate limited: 5/15min) |
| `POST` | `/logout` | End session |
| `GET` | `/` | Dashboard (split-panel: tasks + time tracking) |
| `POST` | `/tasks` | Create task(s) via LLM extraction |
| `PATCH` | `/tasks/:id` | Toggle complete, edit task, or update deadline |
| `DELETE` | `/tasks/:id` | Delete task |
| `POST` | `/tasks/reorder` | Reorder tasks (drag-and-drop) |
| `POST` | `/tasks/clear` | Clear all completed tasks |
| `GET` | `/tasks/list` | Task list partial (HTMX/SSE) |
| `GET` | `/tasks/stream` | SSE endpoint (real-time updates) |
| `GET` | `/export/csv` | Download tasks as CSV |
| `GET` | `/export/json` | Download tasks as JSON |
| `POST` | `/time/switch/:matter` | Start/switch timer for a matter |
| `POST` | `/time/stop` | Stop active timer |
| `POST` | `/time/resume` | Resume last stopped timer |
| `POST` | `/time/manual` | Create manual time entry |
| `PATCH` | `/time/:id` | Edit time entry (times, description) |
| `DELETE` | `/time/:id` | Delete time entry |
| `GET` | `/time/list` | Time panel partial (HTMX/SSE) |
| `GET` | `/time/entries` | Time entries for a date |
| `GET` | `/time/weekly` | Weekly summary grid |
| `GET` | `/time/export/csv` | Download time entries as CSV |
| `POST` | `/time/export/email` | Send time report via email |

Protected routes are rate limited at 60 requests/minute.

## Testing

80 tests across 9 test files, all passing with `-race`:

| File | Tests | Coverage |
|------|-------|----------|
| `templates/components/helpers_test.go` | 19 | Deadline formatting, priority colors/labels, project meta, date/time helpers |
| `llm/provider_test.go` | 13 | JSON parsing, markdown fencing, validation, fallback |
| `render_test.go` | 11 | Dashboard data grouping, progress %, urgent counts, unconfigured tags |
| `auth_test.go` | 9 | HMAC tokens, middleware, login/logout, session validation |
| `handlers_test.go` | 8 | Full request cycle with mock LLM and real DB |
| `config_test.go` | 7 | Config validation, defaults, env overrides, tag parsing |
| `sse_test.go` | 6 | Subscribe/unsubscribe, broadcast, concurrency, full-channel skip |
| `db_test.go` | 6 | CRUD operations against PostgreSQL |
| `handlers_time_test.go` | 1 | Decimal time extraction from voice descriptions |

```bash
make test                    # all tests (skips DB if no Postgres)
go test -race ./...          # with race detector
go test -short ./...         # skip DB-dependent tests
```

CI runs lint, test (with Postgres service container), and build on every push to `main`.

## Deployment

VoiceTask is designed for a single small VPS (512MB RAM is sufficient).

```bash
# Cross-compile
make build-linux

# Deploy (requires SERVER env var)
SERVER=your-server-ip make deploy
```

The deploy target copies the binary and restarts the systemd service. The binary is fully self-contained — no runtime file dependencies.

### Server Setup

1. Install PostgreSQL 16 and Caddy
2. Create the database (migrations run automatically on startup)
3. Copy the `Caddyfile` to `/etc/caddy/Caddyfile`
4. Copy `voicetask.service` to `/etc/systemd/system/`
5. Create `/opt/voicetask/.env` with your configuration
6. Enable and start: `systemctl enable --now voicetask`

Caddy auto-provisions HTTPS via Let's Encrypt with HSTS enabled.

**Note:** On a 512MB VPS, add 1GB swap before building from source:
```bash
fallocate -l 1G /swapfile && chmod 600 /swapfile && mkswap /swapfile && swapon /swapfile
echo '/swapfile none swap sw 0 0' >> /etc/fstab
```

## Browser Compatibility

| Browser | Voice Capture | Notes |
|---------|--------------|-------|
| Chrome/Edge | Full support | Google cloud speech engine, continuous mode |
| Safari iOS 14.5+ | Supported | Requires tap gesture to start |
| Chrome Android | Full support | Most accurate |
| Firefox | Not supported | Text-only input (mic button hidden) |

## License

MIT
