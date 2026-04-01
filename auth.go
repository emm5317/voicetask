package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
)

const (
	sessionCookieName = "session"
	sessionMaxAge     = 30 * 24 * time.Hour // 30 days
)

// App methods for auth are defined here.
// The App struct is defined in main.go.

// AuthRequired is middleware that checks for a valid session cookie.
func (a *App) AuthRequired(c *fiber.Ctx) error {
	cookie := c.Cookies(sessionCookieName)
	if cookie == "" || cookie != a.sessionToken() {
		// For HTMX requests, return 401 so the client can redirect
		if c.Get("HX-Request") == "true" {
			c.Set("HX-Redirect", "/login")
			return c.SendStatus(fiber.StatusUnauthorized)
		}
		return c.Redirect("/login")
	}
	return c.Next()
}

// HandleLoginPage renders the login form.
func (a *App) HandleLoginPage(c *fiber.Ctx) error {
	// If already authenticated, redirect to dashboard
	cookie := c.Cookies(sessionCookieName)
	if cookie != "" && cookie == a.sessionToken() {
		return c.Redirect("/")
	}
	return c.Type("html").SendString(loginHTML(""))
}

// HandleAuth processes the login form submission.
func (a *App) HandleAuth(c *fiber.Ctx) error {
	passphrase := c.FormValue("passphrase")

	if err := bcrypt.CompareHashAndPassword([]byte(a.cfg.PassphraseHash), []byte(passphrase)); err != nil {
		slog.Warn("login failed", "ip", c.IP())
		return c.Type("html").SendString(loginHTML("Invalid passphrase"))
	}

	c.Cookie(&fiber.Cookie{
		Name:     sessionCookieName,
		Value:    a.sessionToken(),
		MaxAge:   int(sessionMaxAge.Seconds()),
		HTTPOnly: true,
		Secure:   true,
		SameSite: "Lax",
	})

	slog.Info("login success", "ip", c.IP())

	// If this was an HTMX request, redirect via header
	if c.Get("HX-Request") == "true" {
		c.Set("HX-Redirect", "/")
		return c.SendStatus(fiber.StatusOK)
	}
	return c.Redirect("/")
}

// HandleLogout clears the session cookie.
func (a *App) HandleLogout(c *fiber.Ctx) error {
	c.Cookie(&fiber.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		MaxAge:   -1,
		HTTPOnly: true,
		Secure:   true,
		SameSite: "Lax",
	})
	return c.Redirect("/login")
}

// sessionToken generates a deterministic HMAC token from the passphrase hash.
// Since this is single-user, the token is always the same for a given hash.
func (a *App) sessionToken() string {
	mac := hmac.New(sha256.New, []byte(a.cfg.PassphraseHash))
	mac.Write([]byte("voicetask-session"))
	return hex.EncodeToString(mac.Sum(nil))
}

// loginHTML returns a minimal login page. This will be replaced by
// html/template rendering in Phase 5, but provides a working auth
// flow for Phase 4 testing.
func loginHTML(errMsg string) string {
	errDiv := ""
	if errMsg != "" {
		errDiv = `<div style="color:#ef4444;margin-bottom:1rem">` + errMsg + `</div>`
	}
	return `<!DOCTYPE html>
<html><head><title>VoiceTask - Login</title>
<meta name="viewport" content="width=device-width,initial-scale=1">
<style>
body{background:#18181b;color:#e4e4e7;font-family:system-ui;display:flex;align-items:center;justify-content:center;min-height:100vh;margin:0}
form{background:#27272a;padding:2rem;border-radius:0.5rem;width:100%;max-width:320px}
h1{margin:0 0 1.5rem;font-size:1.25rem;text-align:center}
input{width:100%;padding:0.75rem;border:1px solid #3f3f46;border-radius:0.375rem;background:#18181b;color:#e4e4e7;font-size:1rem;margin-bottom:1rem;box-sizing:border-box}
button{width:100%;padding:0.75rem;background:#d97706;color:#18181b;border:none;border-radius:0.375rem;font-size:1rem;font-weight:600;cursor:pointer}
button:hover{background:#b45309}
</style></head><body>
<form method="POST" action="/auth">
<h1>VoiceTask</h1>` + errDiv + `
<input type="password" name="passphrase" placeholder="Passphrase" autofocus required>
<button type="submit">Sign In</button>
</form></body></html>`
}
