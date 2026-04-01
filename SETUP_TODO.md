# Setup TODO — Remove Before Production Use

This file tracks temporary relaxations made to allow initial deployment
without all secrets configured. **Complete these steps and then delete
this file.**

---

## 1. Generate and set passphrase hash

1. Visit `https://tasks.emm5317.com/setup`
2. Enter your passphrase and copy the bcrypt hash
3. SSH into your droplet and edit `/opt/voicetask/.env`:
   ```
   APP_PASSPHRASE_HASH=$2a$10$your_hash_here
   ```
4. Restart: `systemctl restart voicetask`
5. Verify login works at `https://tasks.emm5317.com/login`

## 2. Set Anthropic API key

1. Go to [console.anthropic.com](https://console.anthropic.com)
2. Sign up, add billing, create an API key
3. Edit `/opt/voicetask/.env`:
   ```
   ANTHROPIC_API_KEY=sk-ant-api03-your_key_here
   ```
4. Restart: `systemctl restart voicetask`
5. Test by adding a voice/text task — it should get auto-tagged and prioritized

## 3. Restore strict config validation

In `config.go`, replace the temporary block:
```go
// TEMPORARY: Allow startup without passphrase hash and API key
if cfg.PassphraseHash == "" {
    slog.Warn("APP_PASSPHRASE_HASH is not set — auth is disabled, /setup route available")
}
```

With the original strict check:
```go
if cfg.PassphraseHash == "" {
    return nil, fmt.Errorf("APP_PASSPHRASE_HASH is required")
}
```

## 4. Remove /setup route

1. Delete `setup.go`
2. In `main.go`, remove these two lines:
   ```go
   server.Get("/setup", HandleSetup)
   server.Post("/setup", HandleSetup)
   ```
3. Rebuild and redeploy

## 5. Restore strict LLM provider check

In `main.go`, replace:
```go
// TEMPORARY: Allow startup without LLM API key
provider, err := newLLMProvider(cfg)
if err != nil {
    slog.Warn("llm provider not configured", "err", err)
}
```

With:
```go
provider, err := newLLMProvider(cfg)
if err != nil {
    slog.Error("llm provider", "err", err)
    os.Exit(1)
}
```

## 6. Remove auth bypass

In `auth.go`, remove this block from `AuthRequired`:
```go
// TEMPORARY: Skip auth if passphrase hash is not configured
if a.cfg.PassphraseHash == "" {
    return c.Next()
}
```

## 7. Delete this file

```bash
rm SETUP_TODO.md
```

Rebuild, redeploy, done.
