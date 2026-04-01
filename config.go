package main

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port           string
	PassphraseHash string
	DatabaseURL    string
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

	cfg := &Config{
		Port:           envOrDefault("APP_PORT", "8090"),
		PassphraseHash: os.Getenv("APP_PASSPHRASE_HASH"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		LLMProvider:    envOrDefault("LLM_PROVIDER", "claude"),
		AnthropicKey:   os.Getenv("ANTHROPIC_API_KEY"),
		OpenAIKey:      os.Getenv("OPENAI_API_KEY"),
		GroqKey:        os.Getenv("GROQ_API_KEY"),
		OllamaURL:      envOrDefault("OLLAMA_URL", "http://localhost:11434"),
		ProjectTags:    parseTags(envOrDefault("PROJECT_TAGS", "campbells,personal,sedalia,BofA,gritton,diment,constellation,national life,cinfin")),
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

	// TEMPORARY: Allow startup without passphrase hash and API key
	// so the /setup route can be used to generate the hash.
	// See SETUP_TODO.md for instructions on restoring these checks.
	if cfg.PassphraseHash == "" {
		slog.Warn("APP_PASSPHRASE_HASH is not set — auth is disabled, /setup route available")
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
