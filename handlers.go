package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/emm5317/voicetask/db"
	"github.com/emm5317/voicetask/llm"
)

const maxInputLength = 2000

// HandleDashboard serves the main page.
func (a *App) HandleDashboard(c *fiber.Ctx) error {
	tasks, err := a.queries.ListTasks(c.UserContext())
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
	if len(input) > maxInputLength {
		input = input[:maxInputLength]
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
		params := db.InsertTaskParams{
			Title:         et.Title,
			ProjectTag:    et.ProjectTag,
			Priority:      et.Priority,
			RawTranscript: &input,
		}

		if et.Deadline != "" {
			if d, err := time.Parse("2006-01-02", et.Deadline); err == nil {
				params.Deadline = &d
			} else {
				slog.Warn("invalid deadline from LLM", "deadline", et.Deadline, "err", err)
			}
		}

		task, err := a.queries.InsertTask(c.UserContext(), params)
		if err != nil {
			slog.Error("insert task", "err", err, "title", params.Title)
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
		if _, err := a.queries.ToggleTask(c.UserContext(), id); err != nil {
			slog.Error("toggle task", "id", id, "err", err)
			return c.Status(fiber.StatusInternalServerError).SendString("Failed to toggle task")
		}
		slog.Info("task toggled", "id", id)

	case "edit":
		params := db.UpdateTaskParams{
			ID:         id,
			Title:      c.FormValue("title"),
			ProjectTag: c.FormValue("project_tag"),
			Priority:   c.FormValue("priority"),
		}
		if _, err := a.queries.UpdateTask(c.UserContext(), params); err != nil {
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
	rows, err := a.queries.DeleteTask(c.UserContext(), id)
	if err != nil {
		slog.Error("delete task", "id", id, "err", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to delete task")
	}
	if rows == 0 {
		return c.Status(fiber.StatusNotFound).SendString("Task not found")
	}
	slog.Info("task deleted", "id", id)

	a.hub.Broadcast("tasks-updated", "reload")
	return a.renderTaskList(c)
}

// HandleClearCompleted handles POST /tasks/clear.
func (a *App) HandleClearCompleted(c *fiber.Ctx) error {
	count, err := a.queries.ClearCompleted(c.UserContext())
	if err != nil {
		slog.Error("clear completed", "err", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to clear tasks")
	}
	slog.Info("cleared completed tasks", "count", count)

	a.hub.Broadcast("tasks-updated", "reload")
	return a.renderTaskList(c)
}

// HandleReorderTasks accepts a JSON array of {id, sort_order} pairs and updates sort orders.
func (a *App) HandleReorderTasks(c *fiber.Ctx) error {
	type reorderItem struct {
		ID        string `json:"id"`
		SortOrder int    `json:"sort_order"`
	}

	var items []reorderItem
	if err := json.Unmarshal(c.Body(), &items); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid JSON")
	}

	for _, item := range items {
		_, err := a.queries.UpdateSortOrder(c.UserContext(), db.UpdateSortOrderParams{
			ID:        item.ID,
			SortOrder: int32(item.SortOrder),
		})
		if err != nil {
			slog.Error("reorder task", "id", item.ID, "err", err)
		}
	}

	slog.Info("tasks reordered", "count", len(items))
	a.hub.Broadcast("tasks-updated", "reload")
	return c.SendStatus(fiber.StatusOK)
}

// HandleTaskList returns just the task list partial (used by SSE refresh).
func (a *App) HandleTaskList(c *fiber.Ctx) error {
	return a.renderTaskList(c)
}

// renderTaskList is a helper that re-queries tasks and returns the HTML partial.
func (a *App) renderTaskList(c *fiber.Ctx) error {
	tasks, err := a.queries.ListTasks(c.UserContext())
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

// HandleExportCSV streams all tasks as a CSV download.
func (a *App) HandleExportCSV(c *fiber.Ctx) error {
	tasks, err := a.queries.ListTasks(c.UserContext())
	if err != nil {
		slog.Error("export csv", "err", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Export failed")
	}

	c.Set("Content-Disposition", "attachment; filename=voicetask-export.csv")
	c.Set("Content-Type", "text/csv")

	w := csv.NewWriter(c.Response().BodyWriter())
	w.Write([]string{"ID", "Title", "Project", "Priority", "Deadline", "Completed", "Created"})
	for _, t := range tasks {
		deadline := ""
		if t.Deadline != nil {
			deadline = t.Deadline.Format("2006-01-02")
		}
		completed := "no"
		if t.Completed {
			completed = "yes"
		}
		w.Write([]string{t.ID, t.Title, t.ProjectTag, t.Priority, deadline, completed, t.CreatedAt.Format("2006-01-02 15:04")})
	}
	w.Flush()
	return nil
}

// HandleExportJSON streams all tasks as a JSON download.
func (a *App) HandleExportJSON(c *fiber.Ctx) error {
	tasks, err := a.queries.ListTasks(c.UserContext())
	if err != nil {
		slog.Error("export json", "err", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Export failed")
	}

	c.Set("Content-Disposition", "attachment; filename=voicetask-export.json")
	c.Set("Content-Type", "application/json")

	type exportTask struct {
		ID            string  `json:"id"`
		Title         string  `json:"title"`
		ProjectTag    string  `json:"project_tag"`
		Priority      string  `json:"priority"`
		Deadline      *string `json:"deadline,omitempty"`
		RawTranscript *string `json:"raw_transcript,omitempty"`
		Completed     bool    `json:"completed"`
		CreatedAt     string  `json:"created_at"`
	}

	export := make([]exportTask, len(tasks))
	for i, t := range tasks {
		var deadline *string
		if t.Deadline != nil {
			d := t.Deadline.Format("2006-01-02")
			deadline = &d
		}
		export[i] = exportTask{
			ID: t.ID, Title: t.Title, ProjectTag: t.ProjectTag,
			Priority: t.Priority, Deadline: deadline, RawTranscript: t.RawTranscript,
			Completed: t.Completed, CreatedAt: t.CreatedAt.Format(time.RFC3339),
		}
	}

	out, err := json.MarshalIndent(map[string]any{"tasks": export, "exported_at": time.Now().Format(time.RFC3339)}, "", "  ")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("marshal: %v", err))
	}
	return c.Send(out)
}
