-- name: InsertTask :one
INSERT INTO tasks (title, project_tag, priority, deadline, raw_transcript)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, title, project_tag, priority, deadline, raw_transcript,
          completed, completed_at, created_at, updated_at, sort_order;

-- name: ListTasks :many
SELECT id, title, project_tag, priority, deadline, raw_transcript,
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
  created_at DESC;

-- name: ToggleTask :one
UPDATE tasks
SET completed = NOT completed,
    completed_at = CASE WHEN NOT completed THEN NOW() ELSE NULL END,
    updated_at = NOW()
WHERE id = $1
RETURNING id, title, project_tag, priority, deadline, raw_transcript,
          completed, completed_at, created_at, updated_at, sort_order;

-- name: UpdateTask :one
UPDATE tasks
SET title = COALESCE(NULLIF(sqlc.arg(title)::text, ''), title),
    project_tag = COALESCE(NULLIF(sqlc.arg(project_tag)::text, ''), project_tag),
    priority = COALESCE(NULLIF(sqlc.arg(priority)::text, ''), priority),
    updated_at = NOW()
WHERE id = sqlc.arg(id)
RETURNING id, title, project_tag, priority, deadline, raw_transcript,
          completed, completed_at, created_at, updated_at, sort_order;

-- name: DeleteTask :execrows
DELETE FROM tasks WHERE id = $1;

-- name: ClearCompleted :execrows
DELETE FROM tasks WHERE completed = TRUE;

-- name: UpdateSortOrder :execrows
UPDATE tasks SET sort_order = $2, updated_at = NOW() WHERE id = $1;
