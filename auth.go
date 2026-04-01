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
	// TEMPORARY: Skip auth if passphrase hash is not configured
	if a.cfg.PassphraseHash == "" {
		return c.Next()
	}
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
	return a.renderLogin(c, "")
}

// HandleAuth processes the login form submission.
func (a *App) HandleAuth(c *fiber.Ctx) error {
	passphrase := c.FormValue("passphrase")

	if err := bcrypt.CompareHashAndPassword([]byte(a.cfg.PassphraseHash), []byte(passphrase)); err != nil {
		slog.Warn("login failed", "ip", c.IP())
		return a.renderLogin(c, "Invalid passphrase")
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

func (a *App) renderLogin(c *fiber.Ctx, errMsg string) error {
	html, err := a.renderer.RenderLogin(errMsg)
	if err != nil {
		slog.Error("render login", "err", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to render login page")
	}
	return c.Type("html").SendString(html)
}
