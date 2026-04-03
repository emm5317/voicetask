package main

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/emm5317/voicetask/db"
)

func testPool(t *testing.T) (*pgxpool.Pool, *db.Queries) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}

	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://voicetask:voicetask@localhost:5432/voicetask_test?sslmode=disable"
	}

	ctx := context.Background()
	pool, err := NewPool(ctx, dbURL, 5)
	if err != nil {
		t.Skipf("skipping DB test — cannot connect: %v", err)
	}

	if err := RunMigrations(ctx, pool); err != nil {
		t.Fatalf("migrations: %v", err)
	}

	// Truncate between tests
	if _, err := pool.Exec(ctx, "DELETE FROM tasks"); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	return pool, db.New(pool)
}

func TestInsertAndListTasks(t *testing.T) {
	_, queries := testPool(t)
	ctx := context.Background()

	// Insert three tasks with different priorities and tags
	params := []db.InsertTaskParams{
		{Title: "Urgent legal task", ProjectTag: "campbells", Priority: "urgent"},
		{Title: "Normal dev task", ProjectTag: "clientsite", Priority: "normal"},
		{Title: "Low personal task", ProjectTag: "personal", Priority: "low"},
	}
	for i, p := range params {
		task, err := queries.InsertTask(ctx, p)
		if err != nil {
			t.Fatalf("insert task %d: %v", i, err)
		}
		if task.ID == "" {
			t.Fatalf("task %d: expected ID to be set", i)
		}
		if task.CreatedAt.IsZero() {
			t.Fatalf("task %d: expected created_at to be set", i)
		}
	}

	// List and verify order: grouped by project_tag, then by priority within group
	listed, err := queries.ListTasks(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(listed) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(listed))
	}

	// campbells < clientsite < personal (alphabetical)
	if listed[0].ProjectTag != "campbells" {
		t.Errorf("first task tag: got %s, want campbells", listed[0].ProjectTag)
	}
	if listed[1].ProjectTag != "clientsite" {
		t.Errorf("second task tag: got %s, want clientsite", listed[1].ProjectTag)
	}
	if listed[2].ProjectTag != "personal" {
		t.Errorf("third task tag: got %s, want personal", listed[2].ProjectTag)
	}
}

func TestToggleTask(t *testing.T) {
	_, queries := testPool(t)
	ctx := context.Background()

	task, err := queries.InsertTask(ctx, db.InsertTaskParams{Title: "Toggle me", ProjectTag: "personal", Priority: "normal"})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	// Toggle to completed
	toggled, err := queries.ToggleTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("toggle: %v", err)
	}
	if !toggled.Completed {
		t.Error("expected completed=true after first toggle")
	}
	if toggled.CompletedAt == nil {
		t.Error("expected completed_at to be set")
	}

	// Toggle back to incomplete
	toggled, err = queries.ToggleTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("toggle back: %v", err)
	}
	if toggled.Completed {
		t.Error("expected completed=false after second toggle")
	}
	if toggled.CompletedAt != nil {
		t.Error("expected completed_at to be nil")
	}
}

func TestUpdateTask(t *testing.T) {
	_, queries := testPool(t)
	ctx := context.Background()

	task, err := queries.InsertTask(ctx, db.InsertTaskParams{Title: "Original title", ProjectTag: "personal", Priority: "normal"})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	// Update only title (empty strings leave other fields unchanged)
	updated, err := queries.UpdateTask(ctx, db.UpdateTaskParams{ID: task.ID, Title: "New title", ProjectTag: "", Priority: ""})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Title != "New title" {
		t.Errorf("title: got %q, want %q", updated.Title, "New title")
	}
	if updated.ProjectTag != "personal" {
		t.Errorf("project_tag should be unchanged: got %q", updated.ProjectTag)
	}
	if updated.Priority != "normal" {
		t.Errorf("priority should be unchanged: got %q", updated.Priority)
	}
}

func TestDeleteTask(t *testing.T) {
	_, queries := testPool(t)
	ctx := context.Background()

	task, err := queries.InsertTask(ctx, db.InsertTaskParams{Title: "Delete me", ProjectTag: "personal", Priority: "normal"})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	rows, err := queries.DeleteTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if rows != 1 {
		t.Errorf("expected 1 row affected, got %d", rows)
	}

	tasks, err := queries.ListTasks(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks after delete, got %d", len(tasks))
	}

	// Deleting again should return 0 rows
	rows, err = queries.DeleteTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("delete again: %v", err)
	}
	if rows != 0 {
		t.Errorf("expected 0 rows on second delete, got %d", rows)
	}
}

func TestClearCompleted(t *testing.T) {
	_, queries := testPool(t)
	ctx := context.Background()

	// Insert two tasks, complete one
	t1, err := queries.InsertTask(ctx, db.InsertTaskParams{Title: "Keep me", ProjectTag: "personal", Priority: "normal"})
	if err != nil {
		t.Fatalf("insert t1: %v", err)
	}
	t2, err := queries.InsertTask(ctx, db.InsertTaskParams{Title: "Clear me", ProjectTag: "personal", Priority: "normal"})
	if err != nil {
		t.Fatalf("insert t2: %v", err)
	}
	_ = t1
	if _, err := queries.ToggleTask(ctx, t2.ID); err != nil {
		t.Fatalf("toggle t2: %v", err)
	}

	cleared, err := queries.ClearCompleted(ctx)
	if err != nil {
		t.Fatalf("clear: %v", err)
	}
	if cleared != 1 {
		t.Errorf("expected 1 cleared, got %d", cleared)
	}

	tasks, err := queries.ListTasks(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(tasks) != 1 {
		t.Errorf("expected 1 remaining task, got %d", len(tasks))
	}
	if tasks[0].Title != "Keep me" {
		t.Errorf("remaining task: got %q, want %q", tasks[0].Title, "Keep me")
	}
}

func TestInsertTaskWithDeadline(t *testing.T) {
	_, queries := testPool(t)
	ctx := context.Background()

	deadline := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)
	transcript := "draft motion by april 15"
	task, err := queries.InsertTask(ctx, db.InsertTaskParams{
		Title:         "Draft motion to compel",
		ProjectTag:    "campbells",
		Priority:      "urgent",
		Deadline:      &deadline,
		RawTranscript: &transcript,
	})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	tasks, err := queries.ListTasks(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].Deadline == nil {
		t.Fatal("expected deadline to be set")
	}
	if !tasks[0].Deadline.Equal(deadline) {
		t.Errorf("deadline: got %v, want %v", tasks[0].Deadline, deadline)
	}
	if tasks[0].RawTranscript == nil || *tasks[0].RawTranscript != transcript {
		t.Errorf("raw_transcript: got %v, want %q", tasks[0].RawTranscript, transcript)
	}
	_ = task
}
