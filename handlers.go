package main

import (
	"log/slog"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/emm5317/voicetask/llm"
)

// HandleDashboard serves the main page.
func (a *App) HandleDashboard(c *fiber.Ctx) error {
	tasks, err := ListTasks(c.UserContext(), a.pool)
	if err != nil {
		slog.Error("list tasks", "err", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to load tasks")
	}
	html, err := a.renderer.RenderDashboard(tasks, a.cfg.ProjectTags)
	if err != nil {
		slog.Error("render dashboard", "err", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to render page")
	}
	return c.Type("html").SendString(html)
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
	return a.renderTaskList(c)
}

// HandleUpdateTask handles PATCH /tasks/:id for toggling or editing tasks.
func (a *App) HandleUpdateTask(c *fiber.Ctx) error {
	id := c.Params("id")
	action := c.FormValue("action")

	switch action {
	case "toggle":
		if _, err := ToggleTask(c.UserContext(), a.pool, id); err != nil {
			slog.Error("toggle task", "id", id, "err", err)
			return c.Status(fiber.StatusInternalServerError).SendString("Failed to toggle task")
		}
		slog.Info("task toggled", "id", id)

	case "edit":
		title := c.FormValue("title")
		tag := c.FormValue("project_tag")
		priority := c.FormValue("priority")
		if _, err := UpdateTask(c.UserContext(), a.pool, id, title, tag, priority); err != nil {
			slog.Error("update task", "id", id, "err", err)
			return c.Status(fiber.StatusInternalServerError).SendString("Failed to update task")
		}
		slog.Info("task updated", "id", id)

	default:
		return c.Status(fiber.StatusBadRequest).SendString("Invalid action")
	}

	a.hub.Broadcast("tasks-updated", "reload")
	return a.renderTaskList(c)
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
	return a.renderTaskList(c)
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
	return a.renderTaskList(c)
}

// renderTaskList is a helper that re-queries tasks and returns the HTML partial.
func (a *App) renderTaskList(c *fiber.Ctx) error {
	tasks, err := ListTasks(c.UserContext(), a.pool)
	if err != nil {
		slog.Error("list tasks", "err", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to load tasks")
	}
	html, err := a.renderer.RenderTaskList(tasks, a.cfg.ProjectTags)
	if err != nil {
		slog.Error("render task list", "err", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to render tasks")
	}
	return c.Type("html").SendString(html)
}
