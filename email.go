package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/emm5317/voicetask/db"
)

// startDigestScheduler runs a background goroutine that sends a daily
// email digest at the configured hour. Does nothing if email is not configured.
func (a *App) startDigestScheduler() {
	if a.cfg.SMTPHost == "" || a.cfg.EmailTo == "" {
		slog.Info("email digest disabled (SMTP_HOST or EMAIL_TO not set)")
		return
	}

	hour, err := strconv.Atoi(a.cfg.DigestHour)
	if err != nil || hour < 0 || hour > 23 {
		hour = 7
	}

	slog.Info("email digest scheduled", "hour", hour, "to", a.cfg.EmailTo)

	go func() {
		for {
			now := time.Now()
			next := time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, now.Location())
			if !next.After(now) {
				next = next.Add(24 * time.Hour)
			}
			time.Sleep(time.Until(next))

			if err := a.sendDigest(); err != nil {
				slog.Error("digest email failed", "err", err)
			}
		}
	}()
}

func (a *App) sendDigest() error {
	ctx := context.Background()
	tasks, err := a.queries.ListTasks(ctx)
	if err != nil {
		return fmt.Errorf("list tasks: %w", err)
	}

	// Filter to open tasks only
	var open []db.Task
	for _, t := range tasks {
		if !t.Completed {
			open = append(open, t)
		}
	}

	// Fetch yesterday's time data
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	yesterday := today.Add(-24 * time.Hour)
	timeSums, _ := a.queries.SumDurationByMatter(ctx, yesterday, today)

	if len(open) == 0 && len(timeSums) == 0 {
		slog.Info("digest: no open tasks or time entries, skipping email")
		return nil
	}

	body := buildDigestHTML(open)

	// Append time tracking section if there was time tracked yesterday
	if len(timeSums) > 0 {
		body = appendTimeDigest(body, timeSums, yesterday)
	}

	subject := fmt.Sprintf("VoiceTask — %d open tasks", len(open))

	overdue := 0
	for _, t := range open {
		if t.Deadline != nil && t.Deadline.Before(today) {
			overdue++
		}
	}
	if overdue > 0 {
		subject = fmt.Sprintf("VoiceTask — %d open tasks (%d overdue)", len(open), overdue)
	}

	return a.sendEmail(subject, body)
}

func appendTimeDigest(body string, sums []db.SumDurationByMatterRow, date time.Time) string {
	// Insert before the closing </div>
	idx := strings.LastIndex(body, "</div>")
	if idx < 0 {
		return body
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf(`<h3 style="color:#b87040;font-size:13px;text-transform:uppercase;letter-spacing:0.05em;margin:16px 0 8px;">Time Tracked — %s</h3>`, date.Format("Mon, Jan 2")))

	var total float64
	for _, s := range sums {
		meta := getProjectMeta(s.Matter)
		b.WriteString(fmt.Sprintf(`<p style="margin:4px 0;font-size:14px;"><span style="color:%s;">&#9632;</span> %s — <strong>%.1f hrs</strong> (%d entries)</p>`,
			meta.Accent, meta.Label, s.TotalBillable, s.EntryCount))
		total += s.TotalBillable
	}
	b.WriteString(fmt.Sprintf(`<p style="margin:8px 0;font-size:14px;font-weight:700;">Total: %.1f hrs</p>`, total))

	return body[:idx] + b.String() + body[idx:]
}

func buildDigestHTML(tasks []db.Task) string {
	var b strings.Builder
	b.WriteString(`<div style="font-family:system-ui,sans-serif;max-width:500px;margin:0 auto;padding:20px;color:#333;">`)
	b.WriteString(`<h2 style="color:#b87040;margin:0 0 20px;font-size:18px;">VoiceTask Daily Digest</h2>`)

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// Overdue section
	var overdue []db.Task
	var dueToday []db.Task
	for _, t := range tasks {
		if t.Deadline != nil {
			dl := time.Date(t.Deadline.Year(), t.Deadline.Month(), t.Deadline.Day(), 0, 0, 0, 0, now.Location())
			if dl.Before(today) {
				overdue = append(overdue, t)
			} else if dl.Equal(today) {
				dueToday = append(dueToday, t)
			}
		}
	}

	if len(overdue) > 0 {
		b.WriteString(`<h3 style="color:#dc4040;font-size:13px;text-transform:uppercase;letter-spacing:0.05em;margin:16px 0 8px;">Overdue</h3>`)
		for _, t := range overdue {
			days := int(today.Sub(time.Date(t.Deadline.Year(), t.Deadline.Month(), t.Deadline.Day(), 0, 0, 0, 0, now.Location())).Hours() / 24)
			b.WriteString(fmt.Sprintf(`<p style="margin:4px 0;font-size:14px;color:#dc4040;">&#9632; %s — %s (%dd overdue)</p>`, t.Title, t.ProjectTag, days))
		}
	}

	if len(dueToday) > 0 {
		b.WriteString(`<h3 style="color:#d4975c;font-size:13px;text-transform:uppercase;letter-spacing:0.05em;margin:16px 0 8px;">Due Today</h3>`)
		for _, t := range dueToday {
			b.WriteString(fmt.Sprintf(`<p style="margin:4px 0;font-size:14px;color:#d4975c;">&#9632; %s — %s</p>`, t.Title, t.ProjectTag))
		}
	}

	// Group remaining by project
	grouped := make(map[string][]db.Task)
	for _, t := range tasks {
		grouped[t.ProjectTag] = append(grouped[t.ProjectTag], t)
	}

	for tag, tagTasks := range grouped {
		meta := getProjectMeta(tag)
		b.WriteString(fmt.Sprintf(`<h3 style="color:%s;font-size:13px;text-transform:uppercase;letter-spacing:0.05em;margin:16px 0 8px;">%s (%d)</h3>`, meta.Accent, meta.Label, len(tagTasks)))
		for _, t := range tagTasks {
			pri := ""
			if t.Priority != "normal" {
				pri = fmt.Sprintf(" <span style='color:#999;font-size:12px;'>(%s)</span>", t.Priority)
			}
			b.WriteString(fmt.Sprintf(`<p style="margin:4px 0;font-size:14px;">&#9632; %s%s</p>`, t.Title, pri))
		}
	}

	b.WriteString(`<hr style="border:none;border-top:1px solid #eee;margin:20px 0;">`)
	b.WriteString(fmt.Sprintf(`<p style="font-size:12px;color:#999;">Sent at %s</p>`, time.Now().Format("3:04 PM, Mon Jan 2")))
	b.WriteString(`</div>`)

	return b.String()
}

func (a *App) sendEmail(subject, htmlBody string) error {
	from := a.cfg.SMTPUser
	to := a.cfg.EmailTo
	host := a.cfg.SMTPHost
	port := a.cfg.SMTPPort
	addr := host + ":" + port

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		from, to, subject, htmlBody)

	auth := smtp.PlainAuth("", a.cfg.SMTPUser, a.cfg.SMTPPassword, host)

	if err := smtp.SendMail(addr, auth, from, []string{to}, []byte(msg)); err != nil {
		return fmt.Errorf("smtp send: %w", err)
	}

	slog.Info("digest email sent", "to", to, "subject", subject)
	return nil
}
