package main

import (
	"os"
	"testing"
)

func TestParseTags(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"a,b,c", []string{"a", "b", "c"}},
		{" a , b , c ", []string{"a", "b", "c"}},
		{"single", []string{"single"}},
		{"a,,b", []string{"a", "b"}},         // skip empty segments
		{",,,", nil},                          // all empty
		{"national life,BofA", []string{"national life", "BofA"}},
	}
	for _, tt := range tests {
		got := parseTags(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("parseTags(%q) len = %d, want %d: %v", tt.input, len(got), len(tt.want), got)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("parseTags(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestLoadConfig_MissingDatabaseURL(t *testing.T) {
	// Clear env to ensure DATABASE_URL is missing
	orig := os.Getenv("DATABASE_URL")
	os.Unsetenv("DATABASE_URL")
	defer os.Setenv("DATABASE_URL", orig)

	// Also set passphrase hash so that's not the error
	origHash := os.Getenv("APP_PASSPHRASE_HASH")
	os.Setenv("APP_PASSPHRASE_HASH", "somehash")
	defer os.Setenv("APP_PASSPHRASE_HASH", origHash)

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error when DATABASE_URL is missing")
	}
	if err.Error() != "DATABASE_URL is required" {
		t.Errorf("error = %q, want 'DATABASE_URL is required'", err)
	}
}

func TestLoadConfig_MissingPassphraseHash(t *testing.T) {
	origDB := os.Getenv("DATABASE_URL")
	origHash := os.Getenv("APP_PASSPHRASE_HASH")
	os.Setenv("DATABASE_URL", "postgres://localhost/test")
	os.Unsetenv("APP_PASSPHRASE_HASH")
	defer func() {
		os.Setenv("DATABASE_URL", origDB)
		os.Setenv("APP_PASSPHRASE_HASH", origHash)
	}()

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error when APP_PASSPHRASE_HASH is missing")
	}
	if err.Error() != "APP_PASSPHRASE_HASH is required" {
		t.Errorf("error = %q, want 'APP_PASSPHRASE_HASH is required'", err)
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	origDB := os.Getenv("DATABASE_URL")
	origHash := os.Getenv("APP_PASSPHRASE_HASH")
	origPort := os.Getenv("APP_PORT")
	origProvider := os.Getenv("LLM_PROVIDER")

	os.Setenv("DATABASE_URL", "postgres://localhost/test")
	os.Setenv("APP_PASSPHRASE_HASH", "$2a$10$fakehash")
	os.Unsetenv("APP_PORT")
	os.Unsetenv("LLM_PROVIDER")
	defer func() {
		os.Setenv("DATABASE_URL", origDB)
		os.Setenv("APP_PASSPHRASE_HASH", origHash)
		if origPort != "" {
			os.Setenv("APP_PORT", origPort)
		}
		if origProvider != "" {
			os.Setenv("LLM_PROVIDER", origProvider)
		}
	}()

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != "8090" {
		t.Errorf("Port = %q, want 8090", cfg.Port)
	}
	if cfg.LLMProvider != "claude" {
		t.Errorf("LLMProvider = %q, want claude", cfg.LLMProvider)
	}
	if cfg.OllamaURL != "http://localhost:11434" {
		t.Errorf("OllamaURL = %q, want http://localhost:11434", cfg.OllamaURL)
	}
	if cfg.SMTPPort != "587" {
		t.Errorf("SMTPPort = %q, want 587", cfg.SMTPPort)
	}
	if cfg.DigestHour != "7" {
		t.Errorf("DigestHour = %q, want 7", cfg.DigestHour)
	}
}

func TestLoadConfig_EnvOverridesDefaults(t *testing.T) {
	origDB := os.Getenv("DATABASE_URL")
	origHash := os.Getenv("APP_PASSPHRASE_HASH")
	origPort := os.Getenv("APP_PORT")
	origProvider := os.Getenv("LLM_PROVIDER")

	os.Setenv("DATABASE_URL", "postgres://localhost/test")
	os.Setenv("APP_PASSPHRASE_HASH", "$2a$10$fakehash")
	os.Setenv("APP_PORT", "3000")
	os.Setenv("LLM_PROVIDER", "openai")
	defer func() {
		os.Setenv("DATABASE_URL", origDB)
		os.Setenv("APP_PASSPHRASE_HASH", origHash)
		if origPort != "" {
			os.Setenv("APP_PORT", origPort)
		} else {
			os.Unsetenv("APP_PORT")
		}
		if origProvider != "" {
			os.Setenv("LLM_PROVIDER", origProvider)
		} else {
			os.Unsetenv("LLM_PROVIDER")
		}
	}()

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != "3000" {
		t.Errorf("Port = %q, want 3000", cfg.Port)
	}
	if cfg.LLMProvider != "openai" {
		t.Errorf("LLMProvider = %q, want openai", cfg.LLMProvider)
	}
}

func TestLoadConfig_ProjectTagsParsed(t *testing.T) {
	origDB := os.Getenv("DATABASE_URL")
	origHash := os.Getenv("APP_PASSPHRASE_HASH")
	origTags := os.Getenv("PROJECT_TAGS")

	os.Setenv("DATABASE_URL", "postgres://localhost/test")
	os.Setenv("APP_PASSPHRASE_HASH", "$2a$10$fakehash")
	os.Setenv("PROJECT_TAGS", "alpha, beta, gamma")
	defer func() {
		os.Setenv("DATABASE_URL", origDB)
		os.Setenv("APP_PASSPHRASE_HASH", origHash)
		if origTags != "" {
			os.Setenv("PROJECT_TAGS", origTags)
		} else {
			os.Unsetenv("PROJECT_TAGS")
		}
	}()

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.ProjectTags) != 3 {
		t.Fatalf("ProjectTags len = %d, want 3: %v", len(cfg.ProjectTags), cfg.ProjectTags)
	}
	if cfg.ProjectTags[0] != "alpha" || cfg.ProjectTags[1] != "beta" || cfg.ProjectTags[2] != "gamma" {
		t.Errorf("ProjectTags = %v, want [alpha beta gamma]", cfg.ProjectTags)
	}
}

func TestEnvOrDefault(t *testing.T) {
	key := "VOICETASK_TEST_ENV_OR_DEFAULT"
	os.Unsetenv(key)

	got := envOrDefault(key, "fallback")
	if got != "fallback" {
		t.Errorf("envOrDefault(unset) = %q, want fallback", got)
	}

	os.Setenv(key, "custom")
	defer os.Unsetenv(key)
	got = envOrDefault(key, "fallback")
	if got != "custom" {
		t.Errorf("envOrDefault(set) = %q, want custom", got)
	}
}
