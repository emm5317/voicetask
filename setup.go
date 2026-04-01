package main

import (
	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
)

// HandleSetup is a temporary route for generating bcrypt hashes.
// Remove this route after configuring APP_PASSPHRASE_HASH in .env.
func HandleSetup(c *fiber.Ctx) error {
	if c.Method() == "GET" {
		return c.Type("html").SendString(setupHTML("", ""))
	}

	passphrase := c.FormValue("passphrase")
	if passphrase == "" {
		return c.Type("html").SendString(setupHTML("", "Passphrase cannot be empty"))
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(passphrase), bcrypt.DefaultCost)
	if err != nil {
		return c.Type("html").SendString(setupHTML("", "Error generating hash: "+err.Error()))
	}

	return c.Type("html").SendString(setupHTML(string(hash), ""))
}

func setupHTML(hash, errMsg string) string {
	result := ""
	if hash != "" {
		result = `<div style="margin-top:1.5rem">
<p style="color:#22c55e;font-weight:600;margin:0 0 0.5rem">Your bcrypt hash:</p>
<pre style="background:#18181b;border:1px solid #3f3f46;border-radius:0.375rem;padding:0.75rem;font-size:0.8rem;word-break:break-all;white-space:pre-wrap;user-select:all;cursor:text">` + hash + `</pre>
<p style="color:#d97706;font-size:0.875rem;margin-top:1rem"><strong>Next steps:</strong></p>
<ol style="color:#a1a1aa;font-size:0.875rem;padding-left:1.25rem">
<li>SSH into your server</li>
<li>Edit <code>/opt/voicetask/.env</code></li>
<li>Set <code>APP_PASSPHRASE_HASH=</code> to the hash above</li>
<li>Run <code>systemctl restart voicetask</code></li>
<li>Visit <a href="/login" style="color:#d97706">/login</a> to sign in</li>
</ol>
</div>`
	}

	errDiv := ""
	if errMsg != "" {
		errDiv = `<div style="color:#ef4444;margin-bottom:1rem">` + errMsg + `</div>`
	}

	return `<!DOCTYPE html>
<html><head><title>VoiceTask - Setup</title>
<meta name="viewport" content="width=device-width,initial-scale=1">
<style>
body{background:#18181b;color:#e4e4e7;font-family:system-ui;display:flex;align-items:center;justify-content:center;min-height:100vh;margin:0}
.card{background:#27272a;padding:2rem;border-radius:0.5rem;width:100%;max-width:420px}
h1{margin:0 0 0.25rem;font-size:1.25rem}
p.sub{color:#a1a1aa;font-size:0.875rem;margin:0 0 1.5rem}
input{width:100%;padding:0.75rem;border:1px solid #3f3f46;border-radius:0.375rem;background:#18181b;color:#e4e4e7;font-size:1rem;margin-bottom:1rem;box-sizing:border-box}
button{width:100%;padding:0.75rem;background:#d97706;color:#18181b;border:none;border-radius:0.375rem;font-size:1rem;font-weight:600;cursor:pointer}
button:hover{background:#b45309}
code{background:#18181b;padding:0.125rem 0.375rem;border-radius:0.25rem;font-size:0.8rem}
</style></head><body>
<div class="card">
<h1>VoiceTask Setup</h1>
<p class="sub">Generate a bcrypt hash for your passphrase</p>` + errDiv + `
<form method="POST" action="/setup">
<input type="password" name="passphrase" placeholder="Enter your passphrase" autofocus required>
<button type="submit">Generate Hash</button>
</form>` + result + `
</div></body></html>`
}
