package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/csrf"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/gofiber/fiber/v2/utils"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/emm5317/voicetask/db"
	"github.com/emm5317/voicetask/llm"
)

//go:embed static/*
var staticFS embed.FS

// App holds all dependencies for the application.
type App struct {
	cfg      *Config
	pool     *pgxpool.Pool
	queries  *db.Queries
	hub      *SSEHub
	llm      llm.Provider
	renderer *Renderer
}

func main() {
	cfg, err := LoadConfig()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := NewPool(ctx, cfg.DatabaseURL, cfg.DBMaxConns)
	if err != nil {
		slog.Error("database", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := RunMigrations(ctx, pool); err != nil {
		slog.Error("migrations", "err", err)
		os.Exit(1)
	}

	provider, err := newLLMProvider(cfg)
	if err != nil {
		slog.Error("llm provider", "err", err)
		os.Exit(1)
	}

	app := &App{
		cfg:      cfg,
		pool:     pool,
		queries:  db.New(pool),
		hub:      NewSSEHub(),
		llm:      provider,
		renderer: NewRenderer(),
	}

	server := fiber.New(fiber.Config{
		AppName:               "VoiceTask",
		ServerHeader:          "VoiceTask",
		EnableTrustedProxyCheck: true,
		TrustedProxies:         []string{"127.0.0.1", "::1"},
		ProxyHeader:            "X-Forwarded-For",
	})

	// Serve embedded static files (manifest.json, etc.)
	staticSub, _ := fs.Sub(staticFS, "static")
	server.Use("/static", filesystem.New(filesystem.Config{
		Root:       http.FS(staticSub),
		PathPrefix: "",
		Browse:     false,
	}))

	// Global middleware
	server.Use(recover.New())
	server.Use(requestid.New())

	// Public routes (no auth required)
	server.Get("/login", app.HandleLoginPage)

	// Rate limit on auth endpoint: 5 attempts per 15 minutes
	server.Post("/auth", limiter.New(limiter.Config{
		Max:        5,
		Expiration: 15 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
	}), app.HandleAuth)

	server.Post("/logout", app.HandleLogout)

	// Health check (no auth)
	server.Get("/health", func(c *fiber.Ctx) error {
		hctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := app.pool.Ping(hctx); err != nil {
			return c.Status(fiber.StatusServiceUnavailable).SendString("DB unavailable")
		}
		return c.SendString("OK")
	})

	// SSE endpoint (auth required, but no rate limit — long-lived connection)
	server.Get("/tasks/stream", app.AuthRequired, app.hub.HandleStream)

	// CSRF middleware for mutating routes
	csrfMiddleware := csrf.New(csrf.Config{
		KeyLookup:      "header:X-CSRF-Token",
		CookieName:     "csrf_",
		CookieHTTPOnly: false,
		CookieSameSite: "Strict",
		CookieSecure:   true,
		Expiration:     12 * time.Hour,
		KeyGenerator:   utils.UUIDv4,
	})

	// Protected routes with rate limiting: 60 req/min (increased for timer switching)
	protected := server.Group("/", app.AuthRequired, limiter.New(limiter.Config{
		Max:        60,
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
	}), csrfMiddleware)

	protected.Get("/", app.HandleDashboard)
	protected.Get("/tasks/list", app.HandleTaskList)
	protected.Post("/tasks", app.HandleCreateTask)
	protected.Patch("/tasks/:id", app.HandleUpdateTask)
	protected.Delete("/tasks/:id", app.HandleDeleteTask)
	protected.Post("/tasks/reorder", app.HandleReorderTasks)
	protected.Post("/tasks/clear", app.HandleClearCompleted)
	protected.Get("/export/csv", app.HandleExportCSV)
	protected.Get("/export/json", app.HandleExportJSON)

	// Time tracking routes
	protected.Post("/time/switch/:matter", app.HandleSwitchMatter)
	protected.Post("/time/stop", app.HandleStopTimer)
	protected.Post("/time/resume", app.HandleResumeLast)
	protected.Post("/time/manual", app.HandleCreateManualEntry)
	protected.Patch("/time/:id", app.HandleUpdateTimeEntry)
	protected.Delete("/time/:id", app.HandleDeleteTimeEntry)
	protected.Get("/time/list", app.HandleTimeList)
	protected.Get("/time/entries", app.HandleTimeEntries)
	protected.Get("/time/weekly", app.HandleWeeklySummary)
	protected.Get("/time/export/csv", app.HandleTimeExportCSV)
	protected.Post("/time/export/email", app.HandleTimeExportEmail)

	// Crash recovery: stop any timers left running from a previous crash
	app.crashRecovery(ctx)

	app.startDigestScheduler()

	go func() {
		slog.Info("server starting", "port", cfg.Port, "llm", cfg.LLMProvider)
		if err := server.Listen(":" + cfg.Port); err != nil {
			slog.Error("server", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down...")
	app.hub.Close()
	_ = server.Shutdown()
}

// newLLMProvider creates the appropriate LLM provider based on config.
func newLLMProvider(cfg *Config) (llm.Provider, error) {
	switch cfg.LLMProvider {
	case "claude":
		if cfg.AnthropicKey == "" {
			return nil, fmt.Errorf("ANTHROPIC_API_KEY is required when LLM_PROVIDER=claude")
		}
		return llm.NewClaudeProvider(cfg.AnthropicKey, cfg.ProjectTags), nil
	case "openai":
		if cfg.OpenAIKey == "" {
			return nil, fmt.Errorf("OPENAI_API_KEY is required when LLM_PROVIDER=openai")
		}
		return llm.NewOpenAIProvider(cfg.OpenAIKey, cfg.ProjectTags), nil
	case "groq":
		if cfg.GroqKey == "" {
			return nil, fmt.Errorf("GROQ_API_KEY is required when LLM_PROVIDER=groq")
		}
		return llm.NewGroqProvider(cfg.GroqKey, cfg.ProjectTags), nil
	case "ollama":
		return llm.NewOllamaProvider(cfg.OllamaURL, cfg.ProjectTags), nil
	default:
		return nil, fmt.Errorf("unknown LLM_PROVIDER: %s", cfg.LLMProvider)
	}
}
