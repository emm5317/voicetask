package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port           string
	PassphraseHash string
	SessionSecret  string
	DatabaseURL    string
	DBMaxConns     int32
	LLMProvider    string
	AnthropicKey   string
	OpenAIKey      string
	GroqKey        string
	OllamaURL      string
	ProjectTags    []string
	// Ntfy push notifications
	NtfyURL   string
	NtfyTopic string
	// Email digest
	SMTPHost     string
	SMTPPort     string
	SMTPUser     string
	SMTPPassword string
	EmailTo      string
	DigestHour   string
}

func LoadConfig() (*Config, error) {
	_ = godotenv.Load() // ignore error — env vars may be set directly

	maxConns := int32(5)
	if v := os.Getenv("DB_MAX_CONNS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxConns = int32(n)
		}
	}

	cfg := &Config{
		Port:           envOrDefault("APP_PORT", "8090"),
		PassphraseHash: os.Getenv("APP_PASSPHRASE_HASH"),
		SessionSecret:  os.Getenv("APP_SESSION_SECRET"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		DBMaxConns:     maxConns,
		LLMProvider:    envOrDefault("LLM_PROVIDER", "claude"),
		AnthropicKey:   os.Getenv("ANTHROPIC_API_KEY"),
		OpenAIKey:      os.Getenv("OPENAI_API_KEY"),
		GroqKey:        os.Getenv("GROQ_API_KEY"),
		OllamaURL:      envOrDefault("OLLAMA_URL", "http://localhost:11434"),
		ProjectTags:    parseTags(envOrDefault("PROJECT_TAGS", "personal")),
		NtfyURL:        os.Getenv("NTFY_URL"),
		NtfyTopic:      os.Getenv("NTFY_TOPIC"),
		SMTPHost:        os.Getenv("SMTP_HOST"),
		SMTPPort:        envOrDefault("SMTP_PORT", "587"),
		SMTPUser:        os.Getenv("SMTP_USER"),
		SMTPPassword:    os.Getenv("SMTP_PASSWORD"),
		EmailTo:         os.Getenv("EMAIL_TO"),
		DigestHour:      envOrDefault("DIGEST_HOUR", "7"),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	if cfg.PassphraseHash == "" {
		return nil, fmt.Errorf("APP_PASSPHRASE_HASH is required")
	}

	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseTags(s string) []string {
	parts := strings.Split(s, ",")
	tags := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t != "" {
			tags = append(tags, t)
		}
	}
	return tags
}
