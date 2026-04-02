package main

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func testApp(t *testing.T) (*App, *fiber.App) {
	t.Helper()

	// Hash of "testpass" generated with bcrypt cost 4 for speed
	// $2a$04$LxPq3xTq3xRqYz5z5z5z5O... — we generate on the fly
	cfg := &Config{
		Port:           "0",
		PassphraseHash: "$2a$10$bVuCnVgwyoPAhhCVk9nrDeXUhN56f9f0iGHBRAJmnvOlmgrfJXodm", // "test123"
		DatabaseURL:    "postgres://dummy:dummy@localhost/dummy",
		LLMProvider:    "claude",
		ProjectTags:    []string{"personal", "campbells"},
	}

	app := &App{
		cfg:      cfg,
		renderer: NewRenderer(),
		hub:      NewSSEHub(),
	}

	server := fiber.New()
	server.Get("/login", app.HandleLoginPage)
	server.Post("/auth", app.HandleAuth)
	server.Get("/logout", app.HandleLogout)
	server.Get("/protected", app.AuthRequired, func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	return app, server
}

func TestSessionTokenDeterministic(t *testing.T) {
	app, _ := testApp(t)
	t1 := app.sessionToken()
	t2 := app.sessionToken()
	if t1 != t2 {
		t.Errorf("session token should be deterministic: got %q and %q", t1, t2)
	}
	if len(t1) != 64 { // SHA256 hex = 64 chars
		t.Errorf("session token should be 64 hex chars, got %d", len(t1))
	}
}

func TestAuthRequired_NoCookie(t *testing.T) {
	_, server := testApp(t)

	req, _ := http.NewRequest("GET", "/protected", nil)
	resp, err := server.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != 302 {
		t.Errorf("expected redirect 302, got %d", resp.StatusCode)
	}
	loc := resp.Header.Get("Location")
	if loc != "/login" {
		t.Errorf("expected redirect to /login, got %q", loc)
	}
}

func TestAuthRequired_InvalidCookie(t *testing.T) {
	_, server := testApp(t)

	req, _ := http.NewRequest("GET", "/protected", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "invalid-token"})
	resp, err := server.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != 302 {
		t.Errorf("expected redirect 302, got %d", resp.StatusCode)
	}
}

func TestAuthRequired_ValidCookie(t *testing.T) {
	app, server := testApp(t)

	req, _ := http.NewRequest("GET", "/protected", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: app.sessionToken()})
	resp, err := server.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "OK" {
		t.Errorf("expected body 'OK', got %q", string(body))
	}
}

func TestAuthRequired_HTMXRequest(t *testing.T) {
	_, server := testApp(t)

	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("HX-Request", "true")
	resp, err := server.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != 401 {
		t.Errorf("expected 401 for HTMX request without cookie, got %d", resp.StatusCode)
	}
	if resp.Header.Get("HX-Redirect") != "/login" {
		t.Errorf("expected HX-Redirect to /login, got %q", resp.Header.Get("HX-Redirect"))
	}
}

func TestLogin_WrongPassphrase(t *testing.T) {
	_, server := testApp(t)

	form := url.Values{"passphrase": {"wrongpassword"}}
	req, _ := http.NewRequest("POST", "/auth", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := server.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200 (with error message), got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Invalid passphrase") {
		t.Error("expected error message in response body")
	}
}

func TestLogin_CorrectPassphrase(t *testing.T) {
	_, server := testApp(t)

	form := url.Values{"passphrase": {"test123"}}
	req, _ := http.NewRequest("POST", "/auth", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := server.Test(req, -1)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != 302 {
		t.Errorf("expected redirect 302 after successful login, got %d", resp.StatusCode)
	}
	// Check session cookie is set
	cookies := resp.Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == "session" && c.Value != "" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected session cookie to be set after login")
	}
}

func TestLoginPage_AlreadyAuthenticated(t *testing.T) {
	app, server := testApp(t)

	req, _ := http.NewRequest("GET", "/login", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: app.sessionToken()})
	resp, err := server.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != 302 {
		t.Errorf("expected redirect away from login when already authenticated, got %d", resp.StatusCode)
	}
}

func TestAuthNoHash_RedirectsToLogin(t *testing.T) {
	cfg := &Config{
		PassphraseHash: "", // empty hash = no bypass, redirects to login
		ProjectTags:    []string{"personal"},
	}
	app := &App{cfg: cfg, renderer: NewRenderer(), hub: NewSSEHub()}
	server := fiber.New()
	server.Get("/protected", app.AuthRequired, func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	req, _ := http.NewRequest("GET", "/protected", nil)
	resp, err := server.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != 302 {
		t.Errorf("expected 302 redirect when passphrase hash is empty, got %d", resp.StatusCode)
	}
}
