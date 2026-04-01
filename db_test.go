package main

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}

	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://voicetask:voicetask@localhost:5432/voicetask_test?sslmode=disable"
	}

	ctx := context.Background()
	pool, err := NewPool(ctx, dbURL)
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

	return pool
}

func TestInsertAndListTasks(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	ctx := context.Background()

	// Insert three tasks with different priorities and tags
	tasks := []Task{
		{Title: "Urgent legal task", ProjectTag: "campbells", Priority: "urgent"},
		{Title: "Normal dev task", ProjectTag: "clientsite", Priority: "normal"},
		{Title: "Low personal task", ProjectTag: "personal", Priority: "low"},
	}
	for i := range tasks {
		if err := InsertTask(ctx, pool, &tasks[i]); err != nil {
			t.Fatalf("insert task %d: %v", i, err)
		}
		if tasks[i].ID == "" {
			t.Fatalf("task %d: expected ID to be set", i)
		}
		if tasks[i].CreatedAt.IsZero() {
			t.Fatalf("task %d: expected created_at to be set", i)
		}
	}

	// List and verify order: grouped by project_tag, then by priority within group
	listed, err := ListTasks(ctx, pool)
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
	pool := testPool(t)
	defer pool.Close()
	ctx := context.Background()

	task := Task{Title: "Toggle me", ProjectTag: "personal", Priority: "normal"}
	if err := InsertTask(ctx, pool, &task); err != nil {
		t.Fatalf("insert: %v", err)
	}

	// Toggle to completed
	toggled, err := ToggleTask(ctx, pool, task.ID)
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
	toggled, err = ToggleTask(ctx, pool, task.ID)
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
	pool := testPool(t)
	defer pool.Close()
	ctx := context.Background()

	task := Task{Title: "Original title", ProjectTag: "personal", Priority: "normal"}
	if err := InsertTask(ctx, pool, &task); err != nil {
		t.Fatalf("insert: %v", err)
	}

	// Update only title (empty strings leave other fields unchanged)
	updated, err := UpdateTask(ctx, pool, task.ID, "New title", "", "")
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
	pool := testPool(t)
	defer pool.Close()
	ctx := context.Background()

	task := Task{Title: "Delete me", ProjectTag: "personal", Priority: "normal"}
	if err := InsertTask(ctx, pool, &task); err != nil {
		t.Fatalf("insert: %v", err)
	}

	if err := DeleteTask(ctx, pool, task.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	tasks, err := ListTasks(ctx, pool)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks after delete, got %d", len(tasks))
	}

	// Deleting again should return not found
	if err := DeleteTask(ctx, pool, task.ID); err == nil {
		t.Error("expected error deleting non-existent task")
	}
}

func TestClearCompleted(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	ctx := context.Background()

	// Insert two tasks, complete one
	t1 := Task{Title: "Keep me", ProjectTag: "personal", Priority: "normal"}
	t2 := Task{Title: "Clear me", ProjectTag: "personal", Priority: "normal"}
	if err := InsertTask(ctx, pool, &t1); err != nil {
		t.Fatalf("insert t1: %v", err)
	}
	if err := InsertTask(ctx, pool, &t2); err != nil {
		t.Fatalf("insert t2: %v", err)
	}
	if _, err := ToggleTask(ctx, pool, t2.ID); err != nil {
		t.Fatalf("toggle t2: %v", err)
	}

	cleared, err := ClearCompleted(ctx, pool)
	if err != nil {
		t.Fatalf("clear: %v", err)
	}
	if cleared != 1 {
		t.Errorf("expected 1 cleared, got %d", cleared)
	}

	tasks, err := ListTasks(ctx, pool)
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
	pool := testPool(t)
	defer pool.Close()
	ctx := context.Background()

	deadline := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)
	transcript := "draft motion by april 15"
	task := Task{
		Title:         "Draft motion to compel",
		ProjectTag:    "campbells",
		Priority:      "urgent",
		Deadline:      &deadline,
		RawTranscript: &transcript,
	}
	if err := InsertTask(ctx, pool, &task); err != nil {
		t.Fatalf("insert: %v", err)
	}

	tasks, err := ListTasks(ctx, pool)
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
}
