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
    deadline = COALESCE(sqlc.arg(deadline), deadline),
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

-- Time entry queries

-- name: StartTimer :one
INSERT INTO time_entries (matter, description)
VALUES ($1, $2)
RETURNING id, matter, description, raw_transcript, start_time, end_time,
          duration_secs, billable_hours::float8 AS billable_hours, created_at, updated_at;

-- name: StopTimer :one
UPDATE time_entries
SET end_time = NOW(),
    duration_secs = EXTRACT(EPOCH FROM (NOW() - start_time))::INTEGER,
    billable_hours = CASE
        WHEN EXTRACT(EPOCH FROM (NOW() - start_time))::INTEGER < 1 THEN 0.0
        ELSE CEIL(EXTRACT(EPOCH FROM (NOW() - start_time)) / 360.0) / 10.0
    END,
    updated_at = NOW()
WHERE id = $1 AND end_time IS NULL
RETURNING id, matter, description, raw_transcript, start_time, end_time,
          duration_secs, billable_hours::float8 AS billable_hours, created_at, updated_at;

-- name: StopAllTimers :execrows
UPDATE time_entries
SET end_time = NOW(),
    duration_secs = EXTRACT(EPOCH FROM (NOW() - start_time))::INTEGER,
    billable_hours = CASE
        WHEN EXTRACT(EPOCH FROM (NOW() - start_time))::INTEGER < 1 THEN 0.0
        ELSE CEIL(EXTRACT(EPOCH FROM (NOW() - start_time)) / 360.0) / 10.0
    END,
    updated_at = NOW()
WHERE end_time IS NULL;

-- name: GetActiveTimer :one
SELECT id, matter, description, raw_transcript, start_time, end_time,
       duration_secs, billable_hours::float8 AS billable_hours, created_at, updated_at
FROM time_entries
WHERE end_time IS NULL
LIMIT 1;

-- name: GetLastStoppedEntry :one
SELECT id, matter, description, raw_transcript, start_time, end_time,
       duration_secs, billable_hours::float8 AS billable_hours, created_at, updated_at
FROM time_entries
WHERE end_time IS NOT NULL
ORDER BY end_time DESC
LIMIT 1;

-- name: GetTimeEntry :one
SELECT id, matter, description, raw_transcript, start_time, end_time,
       duration_secs, billable_hours::float8 AS billable_hours, created_at, updated_at
FROM time_entries
WHERE id = $1;

-- name: UpdateTimeEntryDescription :one
UPDATE time_entries
SET description = $2,
    raw_transcript = $3,
    updated_at = NOW()
WHERE id = $1
RETURNING id, matter, description, raw_transcript, start_time, end_time,
          duration_secs, billable_hours::float8 AS billable_hours, created_at, updated_at;

-- name: UpdateTimeEntryWithDuration :one
UPDATE time_entries
SET description = $2,
    raw_transcript = $3,
    end_time = $4,
    duration_secs = $5,
    billable_hours = $6,
    updated_at = NOW()
WHERE id = $1
RETURNING id, matter, description, raw_transcript, start_time, end_time,
          duration_secs, billable_hours::float8 AS billable_hours, created_at, updated_at;

-- name: DeleteTimeEntry :execrows
DELETE FROM time_entries WHERE id = $1;

-- name: ListTimeEntriesByDate :many
SELECT id, matter, description, raw_transcript, start_time, end_time,
       duration_secs, billable_hours::float8 AS billable_hours, created_at, updated_at
FROM time_entries
WHERE start_time >= $1 AND start_time < $2
ORDER BY start_time DESC;

-- name: SumDurationByMatter :many
SELECT matter,
       COALESCE(SUM(duration_secs), 0)::INTEGER AS total_secs,
       COALESCE(SUM(billable_hours), 0.0)::float8 AS total_billable,
       COUNT(*)::INTEGER AS entry_count
FROM time_entries
WHERE start_time >= $1 AND start_time < $2
  AND end_time IS NOT NULL
GROUP BY matter
ORDER BY total_billable DESC;

-- name: ListTimeEntriesByMatter :many
SELECT id, matter, description, raw_transcript, start_time, end_time,
       duration_secs, billable_hours::float8 AS billable_hours, created_at, updated_at
FROM time_entries
WHERE matter = $1 AND start_time >= $2 AND start_time < $3
ORDER BY start_time DESC;

-- name: InsertManualEntry :one
INSERT INTO time_entries (matter, description, start_time, end_time, duration_secs, billable_hours)
VALUES ($1, $2, $3, $4, $5,
    CASE
        WHEN $5 < 1 THEN 0.0
        ELSE CEIL($5::float8 / 360.0) / 10.0
    END)
RETURNING id, matter, description, raw_transcript, start_time, end_time,
          duration_secs, billable_hours::float8 AS billable_hours, created_at, updated_at;

-- name: UpdateTimeEntry :one
UPDATE time_entries
SET description = $2,
    start_time = $3,
    end_time = $4,
    duration_secs = $5,
    billable_hours = CASE
        WHEN $5 < 1 THEN 0.0
        ELSE CEIL($5::float8 / 360.0) / 10.0
    END,
    updated_at = NOW()
WHERE id = $1
RETURNING id, matter, description, raw_transcript, start_time, end_time,
          duration_secs, billable_hours::float8 AS billable_hours, created_at, updated_at;

-- name: WeeklySummary :many
SELECT matter,
       start_time::date AS entry_date,
       COALESCE(SUM(billable_hours), 0.0)::float8 AS daily_billable
FROM time_entries
WHERE start_time >= $1 AND start_time < $2
  AND end_time IS NOT NULL
GROUP BY matter, start_time::date
ORDER BY matter, entry_date;
