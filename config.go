package main

import (
	"fmt"
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
		ProjectTags:    parseTags(envOrDefault("PROJECT_TAGS", "clientsite,campbells,makinen,tradebot,personal,home")),
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
