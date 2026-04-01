package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/emm5317/voicetask/llm"
)

// App holds all dependencies for the application.
type App struct {
	cfg  *Config
	pool *pgxpool.Pool
	hub  *SSEHub
	llm  llm.Provider
}

func main() {
	cfg, err := LoadConfig()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := NewPool(ctx, cfg.DatabaseURL)
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
		cfg:  cfg,
		pool: pool,
		hub:  NewSSEHub(),
		llm:  provider,
	}

	server := fiber.New(fiber.Config{
		AppName:      "VoiceTask",
		ServerHeader: "VoiceTask",
	})

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

	server.Get("/logout", app.HandleLogout)

	// SSE endpoint (auth required, but no rate limit — long-lived connection)
	server.Get("/tasks/stream", app.AuthRequired, app.hub.HandleStream)

	// Protected routes with rate limiting: 30 req/min
	protected := server.Group("/", app.AuthRequired, limiter.New(limiter.Config{
		Max:        30,
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
	}))

	protected.Get("/", app.HandleDashboard)
	protected.Post("/tasks", app.HandleCreateTask)
	protected.Patch("/tasks/:id", app.HandleUpdateTask)
	protected.Delete("/tasks/:id", app.HandleDeleteTask)
	protected.Post("/tasks/clear", app.HandleClearCompleted)

	go func() {
		slog.Info("server starting", "port", cfg.Port, "llm", cfg.LLMProvider)
		if err := server.Listen(":" + cfg.Port); err != nil {
			slog.Error("server", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down...")
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
