package main

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/emm5317/voicetask/llm"
)

// HandleDashboard serves the main page.
// In Phase 5 this will render via html/template; for now returns a
// minimal working page so handlers can be tested end-to-end.
func (a *App) HandleDashboard(c *fiber.Ctx) error {
	tasks, err := ListTasks(c.UserContext(), a.pool)
	if err != nil {
		slog.Error("list tasks", "err", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to load tasks")
	}
	return c.Type("html").SendString(dashboardHTML(tasks, a.cfg.ProjectTags))
}

// HandleCreateTask processes voice/text input, extracts tasks via LLM,
// inserts them into the database, and broadcasts an SSE update.
func (a *App) HandleCreateTask(c *fiber.Ctx) error {
	input := strings.TrimSpace(c.FormValue("input"))
	if input == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Input is required")
	}

	slog.Info("creating task", "input", input, "source", c.FormValue("source"))

	var extracted []llm.ExtractedTask
	if a.llm != nil {
		var err error
		extracted, err = a.llm.ExtractTasks(c.UserContext(), input)
		if err != nil {
			slog.Error("llm extraction failed, using fallback", "err", err, "input", input)
		}
	}
	if len(extracted) == 0 {
		extracted = []llm.ExtractedTask{{
			Title:      input,
			ProjectTag: "personal",
			Priority:   "normal",
		}}
	}

	for _, et := range extracted {
		task := Task{
			Title:         et.Title,
			ProjectTag:    et.ProjectTag,
			Priority:      et.Priority,
			RawTranscript: &input,
		}

		if et.Deadline != "" {
			if d, err := time.Parse("2006-01-02", et.Deadline); err == nil {
				task.Deadline = &d
			} else {
				slog.Warn("invalid deadline from LLM", "deadline", et.Deadline, "err", err)
			}
		}

		if err := InsertTask(c.UserContext(), a.pool, &task); err != nil {
			slog.Error("insert task", "err", err, "title", task.Title)
			continue
		}
		slog.Info("task created", "id", task.ID, "title", task.Title, "tag", task.ProjectTag, "priority", task.Priority)
	}

	a.hub.Broadcast("tasks-updated", "reload")

	// Return updated task list
	tasks, err := ListTasks(c.UserContext(), a.pool)
	if err != nil {
		slog.Error("list tasks after create", "err", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to load tasks")
	}

	return c.Type("html").SendString(taskListHTML(tasks))
}

// HandleUpdateTask handles PATCH /tasks/:id for toggling or editing tasks.
func (a *App) HandleUpdateTask(c *fiber.Ctx) error {
	id := c.Params("id")
	action := c.FormValue("action")

	var err error
	switch action {
	case "toggle":
		_, err = ToggleTask(c.UserContext(), a.pool, id)
		if err != nil {
			slog.Error("toggle task", "id", id, "err", err)
			return c.Status(fiber.StatusInternalServerError).SendString("Failed to toggle task")
		}
		slog.Info("task toggled", "id", id)

	case "edit":
		title := c.FormValue("title")
		tag := c.FormValue("project_tag")
		priority := c.FormValue("priority")
		_, err = UpdateTask(c.UserContext(), a.pool, id, title, tag, priority)
		if err != nil {
			slog.Error("update task", "id", id, "err", err)
			return c.Status(fiber.StatusInternalServerError).SendString("Failed to update task")
		}
		slog.Info("task updated", "id", id)

	default:
		return c.Status(fiber.StatusBadRequest).SendString("Invalid action")
	}

	a.hub.Broadcast("tasks-updated", "reload")

	tasks, err := ListTasks(c.UserContext(), a.pool)
	if err != nil {
		slog.Error("list tasks after update", "err", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to load tasks")
	}
	return c.Type("html").SendString(taskListHTML(tasks))
}

// HandleDeleteTask handles DELETE /tasks/:id.
func (a *App) HandleDeleteTask(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := DeleteTask(c.UserContext(), a.pool, id); err != nil {
		slog.Error("delete task", "id", id, "err", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to delete task")
	}
	slog.Info("task deleted", "id", id)

	a.hub.Broadcast("tasks-updated", "reload")

	tasks, err := ListTasks(c.UserContext(), a.pool)
	if err != nil {
		slog.Error("list tasks after delete", "err", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to load tasks")
	}
	return c.Type("html").SendString(taskListHTML(tasks))
}

// HandleClearCompleted handles POST /tasks/clear.
func (a *App) HandleClearCompleted(c *fiber.Ctx) error {
	count, err := ClearCompleted(c.UserContext(), a.pool)
	if err != nil {
		slog.Error("clear completed", "err", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to clear tasks")
	}
	slog.Info("cleared completed tasks", "count", count)

	a.hub.Broadcast("tasks-updated", "reload")

	tasks, err := ListTasks(c.UserContext(), a.pool)
	if err != nil {
		slog.Error("list tasks after clear", "err", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to load tasks")
	}
	return c.Type("html").SendString(taskListHTML(tasks))
}

// --- Temporary HTML rendering (replaced by html/template in Phase 5) ---

func dashboardHTML(tasks []Task, tags []string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html><head><title>VoiceTask</title>
<meta name="viewport" content="width=device-width,initial-scale=1">
<script src="https://unpkg.com/htmx.org@2.0.4"></script>
<script src="https://unpkg.com/htmx-ext-sse@2.2.2/sse.js"></script>
<style>
body{background:#18181b;color:#e4e4e7;font-family:system-ui;margin:0;padding:1rem}
h1{font-size:1.25rem;margin:0 0 1rem}
.capture{display:flex;gap:0.5rem;margin-bottom:1.5rem}
.capture input{flex:1;padding:0.75rem;border:1px solid #3f3f46;border-radius:0.375rem;background:#27272a;color:#e4e4e7;font-size:1rem}
.capture button{padding:0.75rem 1rem;background:#d97706;color:#18181b;border:none;border-radius:0.375rem;font-weight:600;cursor:pointer}
.tag-header{color:#a1a1aa;font-size:0.75rem;font-weight:600;letter-spacing:0.1em;text-transform:uppercase;margin:1rem 0 0.5rem;border-bottom:1px solid #3f3f46;padding-bottom:0.25rem}
.task{display:flex;align-items:center;gap:0.5rem;padding:0.5rem 0}
.task.done{opacity:0.5;text-decoration:line-through}
.badge{font-size:0.7rem;padding:0.125rem 0.5rem;border-radius:9999px;font-weight:600}
.badge-urgent{background:#dc2626;color:#fff}
.badge-high{background:#d97706;color:#18181b}
.badge-normal{background:#3f3f46;color:#a1a1aa}
.badge-low{background:#27272a;color:#52525b}
.actions{margin-left:auto;display:flex;gap:0.25rem}
.actions button{background:none;border:none;color:#71717a;cursor:pointer;font-size:0.875rem;padding:0.25rem}
.actions button:hover{color:#e4e4e7}
.clear-btn{margin-top:1rem;padding:0.5rem 1rem;background:#27272a;color:#a1a1aa;border:1px solid #3f3f46;border-radius:0.375rem;cursor:pointer;font-size:0.875rem}
.clear-btn:hover{background:#3f3f46}
.deadline{font-size:0.75rem;color:#a1a1aa}
.deadline-overdue{color:#ef4444}
.deadline-soon{color:#d97706}
</style></head><body>
<h1>VoiceTask</h1>
<form class="capture" hx-post="/tasks" hx-target="#task-list" hx-swap="innerHTML">
<input type="text" name="input" placeholder="Type a task or tap mic to speak..." autocomplete="off">
<button type="submit">Add</button>
</form>
<div hx-ext="sse" sse-connect="/tasks/stream" style="display:none">
<div hx-get="/" hx-trigger="sse:tasks-updated" hx-target="#task-list" hx-select="#task-list" hx-swap="innerHTML"></div>
</div>
<div id="task-list">%s</div>
<form hx-post="/tasks/clear" hx-target="#task-list" hx-swap="innerHTML" hx-confirm="Clear all completed tasks?">
<button type="submit" class="clear-btn">Clear Completed</button>
</form>
</body></html>`, renderTasks(tasks))
}

func taskListHTML(tasks []Task) string {
	return fmt.Sprintf(`<div id="task-list">%s</div>`, renderTasks(tasks))
}

func renderTasks(tasks []Task) string {
	if len(tasks) == 0 {
		return `<p style="color:#71717a;text-align:center;padding:2rem">No tasks yet. Add one above.</p>`
	}

	var b strings.Builder
	currentTag := ""
	for _, t := range tasks {
		if t.ProjectTag != currentTag {
			currentTag = t.ProjectTag
			fmt.Fprintf(&b, `<div class="tag-header">%s</div>`, strings.ToUpper(currentTag))
		}

		doneClass := ""
		if t.Completed {
			doneClass = " done"
		}

		deadlineStr := ""
		if t.Deadline != nil {
			deadlineStr = formatDeadline(*t.Deadline)
		}

		fmt.Fprintf(&b, `<div class="task%s">`, doneClass)

		// Toggle checkbox
		fmt.Fprintf(&b, `<input type="checkbox" hx-patch="/tasks/%s" hx-vals='{"action":"toggle"}' hx-target="#task-list" hx-swap="innerHTML"%s>`,
			t.ID, checkedAttr(t.Completed))

		fmt.Fprintf(&b, `<span>%s</span>`, t.Title)

		if deadlineStr != "" {
			fmt.Fprintf(&b, ` <span class="%s">%s</span>`, deadlineClass(*t.Deadline), deadlineStr)
		}

		fmt.Fprintf(&b, ` <span class="badge badge-%s">%s</span>`, t.Priority, t.Priority)

		// Delete button
		fmt.Fprintf(&b, `<span class="actions"><button hx-delete="/tasks/%s" hx-target="#task-list" hx-swap="innerHTML" hx-confirm="Delete this task?">✕</button></span>`, t.ID)

		b.WriteString(`</div>`)
	}
	return b.String()
}

func checkedAttr(checked bool) string {
	if checked {
		return " checked"
	}
	return ""
}

func formatDeadline(d time.Time) string {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	deadline := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, now.Location())

	diff := deadline.Sub(today)
	switch {
	case diff < 0:
		return "overdue"
	case diff == 0:
		return "today"
	case diff <= 24*time.Hour:
		return "tomorrow"
	case diff <= 7*24*time.Hour:
		return d.Format("Mon")
	default:
		return d.Format("Jan 2")
	}
}

func deadlineClass(d time.Time) string {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	deadline := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, now.Location())

	diff := deadline.Sub(today)
	switch {
	case diff < 0:
		return "deadline deadline-overdue"
	case diff <= 24*time.Hour:
		return "deadline deadline-soon"
	default:
		return "deadline"
	}
}
