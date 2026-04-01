package main

import (
	"context"
	_ "embed"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed db/migrations/001_create_tasks.sql
var migrationSQL string

// Task represents a single task in the database.
type Task struct {
	ID            string
	Title         string
	ProjectTag    string
	Priority      string
	Deadline      *time.Time
	RawTranscript *string
	Completed     bool
	CompletedAt   *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
	SortOrder     int
}

// NewPool creates a new pgx connection pool.
func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	config.MaxConns = 5

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}

	return pool, nil
}

// RunMigrations executes the embedded SQL migration.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, migrationSQL); err != nil {
		return fmt.Errorf("exec migration: %w", err)
	}
	return nil
}

// InsertTask creates a new task and populates its ID and timestamps.
func InsertTask(ctx context.Context, pool *pgxpool.Pool, t *Task) error {
	return pool.QueryRow(ctx,
		`INSERT INTO tasks (title, project_tag, priority, deadline, raw_transcript)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, created_at, updated_at`,
		t.Title, t.ProjectTag, t.Priority, t.Deadline, t.RawTranscript,
	).Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt)
}

// ListTasks returns all tasks ordered for display:
// incomplete first, then by priority (urgent > high > normal > low),
// then by sort_order, then newest first. Grouped by project_tag.
func ListTasks(ctx context.Context, pool *pgxpool.Pool) ([]Task, error) {
	rows, err := pool.Query(ctx,
		`SELECT id, title, project_tag, priority, deadline, raw_transcript,
		        completed, completed_at, created_at, updated_at, sort_order
		 FROM tasks
		 ORDER BY
		   project_tag ASC,
		   completed ASC,
		   CASE priority
		     WHEN 'urgent' THEN 0
		     WHEN 'high'   THEN 1
		     WHEN 'normal' THEN 2
		     WHEN 'low'    THEN 3
		   END ASC,
		   sort_order ASC,
		   created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("query tasks: %w", err)
	}
	defer rows.Close()

	return scanTasks(rows)
}

// ToggleTask flips the completed status and returns the updated task.
func ToggleTask(ctx context.Context, pool *pgxpool.Pool, id string) (*Task, error) {
	row := pool.QueryRow(ctx,
		`UPDATE tasks
		 SET completed = NOT completed,
		     completed_at = CASE WHEN NOT completed THEN NOW() ELSE NULL END,
		     updated_at = NOW()
		 WHERE id = $1
		 RETURNING id, title, project_tag, priority, deadline, raw_transcript,
		           completed, completed_at, created_at, updated_at, sort_order`,
		id)
	return scanTask(row)
}

// UpdateTask updates the title, project_tag, and priority of a task.
// Empty values are left unchanged via COALESCE/NULLIF.
func UpdateTask(ctx context.Context, pool *pgxpool.Pool, id, title, projectTag, priority string) (*Task, error) {
	row := pool.QueryRow(ctx,
		`UPDATE tasks
		 SET title = COALESCE(NULLIF($2, ''), title),
		     project_tag = COALESCE(NULLIF($3, ''), project_tag),
		     priority = COALESCE(NULLIF($4, ''), priority),
		     updated_at = NOW()
		 WHERE id = $1
		 RETURNING id, title, project_tag, priority, deadline, raw_transcript,
		           completed, completed_at, created_at, updated_at, sort_order`,
		id, title, projectTag, priority)
	return scanTask(row)
}

// DeleteTask removes a task by ID.
func DeleteTask(ctx context.Context, pool *pgxpool.Pool, id string) error {
	result, err := pool.Exec(ctx, `DELETE FROM tasks WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("task not found: %s", id)
	}
	return nil
}

// ClearCompleted deletes all completed tasks and returns the count removed.
func ClearCompleted(ctx context.Context, pool *pgxpool.Pool) (int64, error) {
	result, err := pool.Exec(ctx, `DELETE FROM tasks WHERE completed = TRUE`)
	if err != nil {
		return 0, fmt.Errorf("clear completed: %w", err)
	}
	return result.RowsAffected(), nil
}

// UpdateSortOrder updates the sort_order for a single task.
func UpdateSortOrder(ctx context.Context, pool *pgxpool.Pool, id string, sortOrder int) error {
	result, err := pool.Exec(ctx, `UPDATE tasks SET sort_order = $2, updated_at = NOW() WHERE id = $1`, id, sortOrder)
	if err != nil {
		return fmt.Errorf("update sort order: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("task not found: %s", id)
	}
	return nil
}

func scanTask(row pgx.Row) (*Task, error) {
	var t Task
	err := row.Scan(
		&t.ID, &t.Title, &t.ProjectTag, &t.Priority, &t.Deadline,
		&t.RawTranscript, &t.Completed, &t.CompletedAt,
		&t.CreatedAt, &t.UpdatedAt, &t.SortOrder,
	)
	if err != nil {
		return nil, fmt.Errorf("scan task: %w", err)
	}
	return &t, nil
}

func scanTasks(rows pgx.Rows) ([]Task, error) {
	var tasks []Task
	for rows.Next() {
		var t Task
		err := rows.Scan(
			&t.ID, &t.Title, &t.ProjectTag, &t.Priority, &t.Deadline,
			&t.RawTranscript, &t.Completed, &t.CompletedAt,
			&t.CreatedAt, &t.UpdatedAt, &t.SortOrder,
		)
		if err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}
