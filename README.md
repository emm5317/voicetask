# VoiceTask

A self-hosted, voice-to-task capture web app. Speak or type on any device — an LLM extracts structured, tagged tasks with priorities and deadlines. Tasks sync across all connected devices in real time via Server-Sent Events.

**Monthly cost:** ~$4.77 ($4 DigitalOcean droplet + ~$0.77 Claude Sonnet API)

## How It Works

1. Tap the mic or type a task on any device (phone, tablet, PC)
2. Browser's Web Speech API transcribes voice on-device (continuous mode with 3s silence auto-stop)
3. Server sends the transcript to an LLM (Claude Sonnet by default)
4. LLM extracts structured tasks: title, project tag, priority, deadline
5. Tasks are stored in PostgreSQL and broadcast via SSE
6. All connected devices update instantly

A single rambling voice note like *"I need to draft that Campbell's motion to compel by Thursday and also follow up with Kayla about the demo"* becomes two clean, tagged, prioritized task items.

## Features

- **Voice capture** — Web Speech API with continuous listening, 3s silence auto-stop, real-time transcript preview
- **LLM extraction** — Automatic tagging, prioritization, and deadline parsing (conservative defaults — only marks urgent/high when you explicitly say so)
- **Real-time sync** — SSE broadcasts to all connected devices with 5s auto-reconnect
- **Multi-provider LLM** — Claude, OpenAI, Groq, or Ollama (switch with one env var)
- **Dark/light theme** — Toggle in header, persisted to localStorage. Dark theme optimized for always-on tablet display
- **Inline task editing** — Double-click or tap edit to change title, project tag, priority, and deadline with date picker
- **Project grouping** — Tasks grouped by configurable tags with color coding and progress bars
- **Custom tags** — Type any tag in the edit form (autocomplete suggests existing tags)
- **Drag-and-drop reorder** — SortableJS within project groups
- **Priority badges** — Urgent, high, normal, low with color-coded indicators
- **Deadline display** — Overdue count, today, tomorrow, or full date (e.g., "Sat, Apr 5")
- **Push notifications** — Ntfy integration for mobile alerts when tasks are created
- **Daily digest email** — Morning summary of open tasks via SMTP
- **PWA support** — Add to home screen on Android/iOS for app-like experience
- **CSV/JSON export** — Download all tasks for backup or analysis
- **Single binary** — Templates, migrations, static files all embedded via `go:embed`
- **Passphrase auth** — bcrypt + HMAC session cookie, single-user, no registration
- **Rate limiting** — 5 req/15min on auth, 30 req/min on protected routes

## Tech Stack

| Component | Choice | Version |
|-----------|--------|---------|
| Language | Go | 1.24 |
| Web framework | Fiber | v2.52 |
| HTML rendering | `html/template` + `embed.FS` | stdlib |
| Frontend | HTMX + Alpine.js | 2.0 / 3.14 |
| Database | PostgreSQL | 16 |
| DB driver | pgx/v5 + pgxpool | v5.7 |
| Query generation | sqlc | v1.28 |
| LLM (default) | Claude Sonnet | Anthropic API |
| Reverse proxy | Caddy | v2 |
| Voice capture | Web Speech API | browser-native |
| Logging | `log/slog` | stdlib |
| CI | GitHub Actions | lint + test + build |

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
# Install Air for live reload
go install github.com/air-verse/air@latest

# Run with hot reload (watches .go, .html, .json files)
make dev

# Run tests
make test

# Run linter
make lint
```

## Configuration

All configuration is via environment variables (or `.env` file):

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `APP_PORT` | No | `8090` | Server port |
| `APP_PASSPHRASE_HASH` | Yes | — | bcrypt hash of login passphrase |
| `DATABASE_URL` | Yes | — | Postgres connection string |
| `LLM_PROVIDER` | No | `claude` | `claude`, `openai`, `groq`, or `ollama` |
| `ANTHROPIC_API_KEY` | If claude | — | Anthropic API key |
| `OPENAI_API_KEY` | If openai | — | OpenAI API key |
| `GROQ_API_KEY` | If groq | — | Groq API key |
| `OLLAMA_URL` | If ollama | `http://localhost:11434` | Ollama endpoint |
| `PROJECT_TAGS` | No | see below | Comma-separated project tags |
| `NTFY_URL` | No | — | Ntfy server URL for push notifications |
| `NTFY_TOPIC` | No | — | Ntfy topic to publish to |
| `SMTP_HOST` | No | — | SMTP server for daily digest email |
| `SMTP_PORT` | No | `587` | SMTP port |
| `SMTP_USER` | No | — | SMTP username |
| `SMTP_PASSWORD` | No | — | SMTP password |
| `EMAIL_TO` | No | — | Digest recipient email |
| `DIGEST_HOUR` | No | `7` | Hour (0-23) to send daily digest |

Default project tags: `campbells,personal,sedalia,BofA,gritton,diment,constellation,national life,cinfin`

## API Routes

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/login` | Login page |
| `POST` | `/auth` | Authenticate (rate limited: 5/15min) |
| `GET` | `/` | Dashboard |
| `POST` | `/tasks` | Create task(s) via LLM extraction |
| `PATCH` | `/tasks/:id` | Toggle complete, edit task, or update deadline |
| `DELETE` | `/tasks/:id` | Delete task |
| `POST` | `/tasks/reorder` | Reorder tasks (drag-and-drop) |
| `POST` | `/tasks/clear` | Clear all completed tasks |
| `GET` | `/tasks/list` | Task list partial (HTMX/SSE) |
| `GET` | `/tasks/stream` | SSE endpoint (real-time updates) |
| `GET` | `/export/csv` | Download tasks as CSV |
| `GET` | `/export/json` | Download tasks as JSON |

Protected routes are rate limited at 30 requests/minute.

## Deployment

VoiceTask is designed for a single $4/month DigitalOcean droplet (512MB RAM, Ubuntu 24.04).

```bash
# Cross-compile
make build-linux

# Deploy (requires SERVER env var)
SERVER=your-droplet-ip make deploy
```

The deploy target copies the binary and restarts the systemd service. The binary is fully self-contained — no runtime file dependencies.

### Server Setup

1. Install PostgreSQL 16 and Caddy
2. Create the database (migrations run automatically on startup)
3. Copy the `Caddyfile` to `/etc/caddy/Caddyfile`
4. Copy `voicetask.service` to `/etc/systemd/system/`
5. Create `/opt/voicetask/.env` with your configuration
6. Enable and start: `systemctl enable --now voicetask`

Caddy auto-provisions HTTPS via Let's Encrypt.

**Note:** On a 512MB droplet, add 1GB swap before building from source:
```bash
fallocate -l 1G /swapfile && chmod 600 /swapfile && mkswap /swapfile && swapon /swapfile
echo '/swapfile none swap sw 0 0' >> /etc/fstab
```

## Architecture

```
Browser → Caddy (HTTPS) → Fiber (recover → requestid → auth → rate limiter)
  → Handler → LLM provider → PostgreSQL → SSE hub → All connected clients
```

- **App struct** — All dependencies (pool, queries, hub, llm, renderer) in one struct. No global state.
- **SSE hub** — In-memory `map[chan string]bool` with mutex. Non-blocking broadcast, 30s keepalive, 5s reconnect.
- **LLM provider interface** — Swappable via env var. Claude uses Anthropic API; OpenAI/Groq/Ollama share one implementation. Conservative priority defaults — only marks urgent/high when explicitly requested.
- **sqlc** — Type-safe generated queries with native Go types via overrides. Zero hand-written `Scan()` calls.
- **embed.FS** — Templates, migrations, and static files baked into the binary.
- **Ntfy** — Optional push notifications via HTTP POST on task creation. Non-blocking goroutine.
- **Email digest** — Optional daily summary via SMTP. Background goroutine fires at configured hour.

## Testing

36 tests across 4 test files, all passing with `-race`:

| File | Tests | Coverage |
|------|-------|----------|
| `llm/provider_test.go` | 13 | JSON parsing, markdown fencing, validation, fallback |
| `auth_test.go` | 9 | HMAC tokens, middleware, login/logout, auth bypass |
| `handlers_test.go` | 8 | Full request cycle with mock LLM and real DB |
| `db_test.go` | 6 | CRUD operations against PostgreSQL |

```bash
make test                    # all tests (skips DB if no Postgres)
go test -race ./...          # with race detector
go test -short ./...         # skip DB-dependent tests
```

CI runs lint, test (with Postgres service container), and build on every push to `main`.

## Browser Compatibility

| Browser | Voice Capture | Notes |
|---------|--------------|-------|
| Chrome/Edge | Full support | Google cloud speech engine, continuous mode |
| Safari iOS 14.5+ | Supported | Requires tap gesture to start |
| Chrome Android | Full support | Most accurate |
| Firefox | Not supported | Text-only input (mic button hidden) |

## License

MIT
