package main

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/emm5317/voicetask/db"
	"github.com/emm5317/voicetask/llm"
)

// mockLLMProvider returns predefined tasks without calling any API.
type mockLLMProvider struct {
	tasks []llm.ExtractedTask
	err   error
}

func (m *mockLLMProvider) ExtractTasks(_ context.Context, _ string) ([]llm.ExtractedTask, error) {
	return m.tasks, m.err
}

func handlerTestApp(t *testing.T) (*App, *fiber.App) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping handler test in short mode")
	}

	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://voicetask:voicetask@localhost:5432/voicetask_test?sslmode=disable"
	}

	ctx := context.Background()
	pool, err := NewPool(ctx, dbURL, 5)
	if err != nil {
		t.Skipf("skipping handler test — cannot connect to DB: %v", err)
	}

	if err := RunMigrations(ctx, pool); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	// Clean slate
	if _, err := pool.Exec(ctx, "DELETE FROM tasks"); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	cfg := &Config{
		PassphraseHash: "", // bypass auth for handler tests
		ProjectTags:    []string{"campbells", "personal", "sedalia"},
	}

	app := &App{
		cfg:      cfg,
		pool:     pool,
		queries:  db.New(pool),
		hub:      NewSSEHub(),
		renderer: NewRenderer(),
		llm: &mockLLMProvider{
			tasks: []llm.ExtractedTask{
				{Title: "Draft motion to compel", ProjectTag: "campbells", Priority: "urgent", Deadline: "2026-04-15"},
			},
		},
	}

	server := fiber.New()
	server.Get("/", app.HandleDashboard)
	server.Get("/tasks/list", app.HandleTaskList)
	server.Post("/tasks", app.HandleCreateTask)
	server.Patch("/tasks/:id", app.HandleUpdateTask)
	server.Delete("/tasks/:id", app.HandleDeleteTask)
	server.Post("/tasks/clear", app.HandleClearCompleted)

	t.Cleanup(func() { pool.Close() })

	return app, server
}

func TestHandleCreateTask(t *testing.T) {
	_, server := handlerTestApp(t)

	req, _ := http.NewRequest("POST", "/tasks", strings.NewReader("input=draft+motion+to+compel"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := server.Test(req, -1)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	html := string(body)

	if !strings.Contains(html, "Draft motion to compel") {
		t.Error("response should contain the task title from mock LLM")
	}
	if !strings.Contains(html, "Campbell") {
		t.Error("response should contain the project tag header")
	}
	if !strings.Contains(html, "URGENT") {
		t.Error("response should contain the priority badge")
	}
}

func TestHandleCreateTask_EmptyInput(t *testing.T) {
	_, server := handlerTestApp(t)

	req, _ := http.NewRequest("POST", "/tasks", strings.NewReader("input="))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := server.Test(req, -1)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != 400 {
		t.Errorf("expected 400 for empty input, got %d", resp.StatusCode)
	}
}

func TestHandleToggleTask(t *testing.T) {
	app, server := handlerTestApp(t)

	// Create a task directly in DB
	task, err := app.queries.InsertTask(context.Background(), db.InsertTaskParams{Title: "Toggle test", ProjectTag: "personal", Priority: "normal"})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	// Toggle it
	req, _ := http.NewRequest("PATCH", "/tasks/"+task.ID, strings.NewReader("action=toggle"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := server.Test(req, -1)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	html := string(body)

	// Completed tasks get line-through style
	if !strings.Contains(html, "line-through") {
		t.Error("toggled task should have line-through style")
	}
}

func TestHandleDeleteTask(t *testing.T) {
	app, server := handlerTestApp(t)

	// Create a task
	task, err := app.queries.InsertTask(context.Background(), db.InsertTaskParams{Title: "Delete me", ProjectTag: "personal", Priority: "normal"})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	// Delete it
	req, _ := http.NewRequest("DELETE", "/tasks/"+task.ID, nil)
	resp, err := server.Test(req, -1)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if strings.Contains(string(body), "Delete me") {
		t.Error("deleted task should not appear in response")
	}
}

func TestHandleClearCompleted(t *testing.T) {
	app, server := handlerTestApp(t)
	ctx := context.Background()

	// Create two tasks, complete one
	if _, err := app.queries.InsertTask(ctx, db.InsertTaskParams{Title: "Keep this", ProjectTag: "personal", Priority: "normal"}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	t2, err := app.queries.InsertTask(ctx, db.InsertTaskParams{Title: "Clear this", ProjectTag: "personal", Priority: "normal"})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if _, err := app.queries.ToggleTask(ctx, t2.ID); err != nil {
		t.Fatalf("toggle: %v", err)
	}

	// Clear completed
	req, _ := http.NewRequest("POST", "/tasks/clear", nil)
	resp, err := server.Test(req, -1)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	html := string(body)
	if !strings.Contains(html, "Keep this") {
		t.Error("uncompleted task should remain")
	}
	if strings.Contains(html, "Clear this") {
		t.Error("completed task should be cleared")
	}
}

func TestHandleDashboard(t *testing.T) {
	_, server := handlerTestApp(t)

	req, _ := http.NewRequest("GET", "/", nil)
	resp, err := server.Test(req, -1)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	html := string(body)

	if !strings.Contains(html, "VoiceTask") {
		t.Error("dashboard should contain app name")
	}
	if !strings.Contains(html, "voiceCapture") {
		t.Error("dashboard should contain voice capture Alpine component")
	}
	if !strings.Contains(html, "tasks/stream") {
		t.Error("dashboard should contain SSE connection")
	}
}

func TestHandleTaskList(t *testing.T) {
	app, server := handlerTestApp(t)
	ctx := context.Background()

	if _, err := app.queries.InsertTask(ctx, db.InsertTaskParams{Title: "List test task", ProjectTag: "personal", Priority: "high"}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	req, _ := http.NewRequest("GET", "/tasks/list", nil)
	resp, err := server.Test(req, -1)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	html := string(body)

	if !strings.Contains(html, "List test task") {
		t.Error("task list should contain the task")
	}
	if !strings.Contains(html, "task-list") {
		t.Error("task list should contain #task-list div")
	}
	// Should NOT contain full page layout
	if strings.Contains(html, "<html") {
		t.Error("task list partial should not contain full HTML document")
	}
}

func TestHandleCreateTask_NilLLM(t *testing.T) {
	app, server := handlerTestApp(t)
	app.llm = nil // simulate no LLM configured

	req, _ := http.NewRequest("POST", "/tasks", strings.NewReader("input=raw+task+text"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := server.Test(req, -1)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "raw task text") {
		t.Error("with nil LLM, raw input should become task title")
	}
}
