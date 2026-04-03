// Code generated manually (sqlc pattern). Edit with care.

package db

import (
	"context"
	"time"
)

// TimeEntry scan helper
func scanTimeEntry(row interface{ Scan(...any) error }) (TimeEntry, error) {
	var i TimeEntry
	err := row.Scan(
		&i.ID, &i.Matter, &i.Description, &i.RawTranscript,
		&i.StartTime, &i.EndTime, &i.DurationSecs, &i.BillableHours,
		&i.CreatedAt, &i.UpdatedAt,
	)
	return i, err
}

const startTimer = `
INSERT INTO time_entries (matter, description)
VALUES ($1, $2)
RETURNING id, matter, description, raw_transcript, start_time, end_time,
          duration_secs, billable_hours::float8 AS billable_hours, created_at, updated_at
`

type StartTimerParams struct {
	Matter      string `json:"matter"`
	Description string `json:"description"`
}

func (q *Queries) StartTimer(ctx context.Context, arg StartTimerParams) (TimeEntry, error) {
	row := q.db.QueryRow(ctx, startTimer, arg.Matter, arg.Description)
	return scanTimeEntry(row)
}

const stopTimer = `
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
          duration_secs, billable_hours::float8 AS billable_hours, created_at, updated_at
`

func (q *Queries) StopTimer(ctx context.Context, id string) (TimeEntry, error) {
	row := q.db.QueryRow(ctx, stopTimer, id)
	return scanTimeEntry(row)
}

const stopAllTimers = `
UPDATE time_entries
SET end_time = NOW(),
    duration_secs = EXTRACT(EPOCH FROM (NOW() - start_time))::INTEGER,
    billable_hours = CASE
        WHEN EXTRACT(EPOCH FROM (NOW() - start_time))::INTEGER < 1 THEN 0.0
        ELSE CEIL(EXTRACT(EPOCH FROM (NOW() - start_time)) / 360.0) / 10.0
    END,
    updated_at = NOW()
WHERE end_time IS NULL
`

func (q *Queries) StopAllTimers(ctx context.Context) (int64, error) {
	result, err := q.db.Exec(ctx, stopAllTimers)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

const getActiveTimer = `
SELECT id, matter, description, raw_transcript, start_time, end_time,
       duration_secs, billable_hours::float8 AS billable_hours, created_at, updated_at
FROM time_entries
WHERE end_time IS NULL
LIMIT 1
`

func (q *Queries) GetActiveTimer(ctx context.Context) (TimeEntry, error) {
	row := q.db.QueryRow(ctx, getActiveTimer)
	return scanTimeEntry(row)
}

const resumeTimer = `
UPDATE time_entries
SET start_time = NOW() - (duration_secs || ' seconds')::INTERVAL,
    end_time = NULL,
    duration_secs = 0,
    billable_hours = 0,
    updated_at = NOW()
WHERE id = $1 AND end_time IS NOT NULL
RETURNING id, matter, description, raw_transcript, start_time, end_time,
          duration_secs, billable_hours::float8 AS billable_hours, created_at, updated_at
`

func (q *Queries) ResumeTimer(ctx context.Context, id string) (TimeEntry, error) {
	row := q.db.QueryRow(ctx, resumeTimer, id)
	return scanTimeEntry(row)
}

const getLastStoppedEntryByMatter = `
SELECT id, matter, description, raw_transcript, start_time, end_time,
       duration_secs, billable_hours::float8 AS billable_hours, created_at, updated_at
FROM time_entries
WHERE end_time IS NOT NULL AND LOWER(matter) = LOWER($1)
ORDER BY end_time DESC
LIMIT 1
`

func (q *Queries) GetLastStoppedEntryByMatter(ctx context.Context, matter string) (TimeEntry, error) {
	row := q.db.QueryRow(ctx, getLastStoppedEntryByMatter, matter)
	return scanTimeEntry(row)
}

const getLastStoppedEntry = `
SELECT id, matter, description, raw_transcript, start_time, end_time,
       duration_secs, billable_hours::float8 AS billable_hours, created_at, updated_at
FROM time_entries
WHERE end_time IS NOT NULL
ORDER BY end_time DESC
LIMIT 1
`

func (q *Queries) GetLastStoppedEntry(ctx context.Context) (TimeEntry, error) {
	row := q.db.QueryRow(ctx, getLastStoppedEntry)
	return scanTimeEntry(row)
}

const getTimeEntry = `
SELECT id, matter, description, raw_transcript, start_time, end_time,
       duration_secs, billable_hours::float8 AS billable_hours, created_at, updated_at
FROM time_entries
WHERE id = $1
`

func (q *Queries) GetTimeEntry(ctx context.Context, id string) (TimeEntry, error) {
	row := q.db.QueryRow(ctx, getTimeEntry, id)
	return scanTimeEntry(row)
}

const updateTimeEntryDescription = `
UPDATE time_entries
SET description = $2,
    raw_transcript = $3,
    updated_at = NOW()
WHERE id = $1
RETURNING id, matter, description, raw_transcript, start_time, end_time,
          duration_secs, billable_hours::float8 AS billable_hours, created_at, updated_at
`

type UpdateTimeEntryDescriptionParams struct {
	ID            string  `json:"id"`
	Description   string  `json:"description"`
	RawTranscript *string `json:"raw_transcript"`
}

func (q *Queries) UpdateTimeEntryDescription(ctx context.Context, arg UpdateTimeEntryDescriptionParams) (TimeEntry, error) {
	row := q.db.QueryRow(ctx, updateTimeEntryDescription, arg.ID, arg.Description, arg.RawTranscript)
	return scanTimeEntry(row)
}

const updateTimeEntryWithDuration = `
UPDATE time_entries
SET description = $2,
    raw_transcript = $3,
    end_time = $4,
    duration_secs = $5,
    billable_hours = $6,
    updated_at = NOW()
WHERE id = $1
RETURNING id, matter, description, raw_transcript, start_time, end_time,
          duration_secs, billable_hours::float8 AS billable_hours, created_at, updated_at
`

type UpdateTimeEntryWithDurationParams struct {
	ID            string    `json:"id"`
	Description   string    `json:"description"`
	RawTranscript *string   `json:"raw_transcript"`
	EndTime       time.Time `json:"end_time"`
	DurationSecs  int       `json:"duration_secs"`
	BillableHours float64   `json:"billable_hours"`
}

func (q *Queries) UpdateTimeEntryWithDuration(ctx context.Context, arg UpdateTimeEntryWithDurationParams) (TimeEntry, error) {
	row := q.db.QueryRow(ctx, updateTimeEntryWithDuration,
		arg.ID, arg.Description, arg.RawTranscript, arg.EndTime, arg.DurationSecs, arg.BillableHours,
	)
	return scanTimeEntry(row)
}

const deleteTimeEntry = `
DELETE FROM time_entries WHERE id = $1
`

func (q *Queries) DeleteTimeEntry(ctx context.Context, id string) (int64, error) {
	result, err := q.db.Exec(ctx, deleteTimeEntry, id)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

const listTimeEntriesByDate = `
SELECT id, matter, description, raw_transcript, start_time, end_time,
       duration_secs, billable_hours::float8 AS billable_hours, created_at, updated_at
FROM time_entries
WHERE start_time >= $1 AND start_time < $2
ORDER BY start_time DESC
`

func (q *Queries) ListTimeEntriesByDate(ctx context.Context, start, end time.Time) ([]TimeEntry, error) {
	rows, err := q.db.Query(ctx, listTimeEntriesByDate, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []TimeEntry
	for rows.Next() {
		var i TimeEntry
		if err := rows.Scan(
			&i.ID, &i.Matter, &i.Description, &i.RawTranscript,
			&i.StartTime, &i.EndTime, &i.DurationSecs, &i.BillableHours,
			&i.CreatedAt, &i.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

type SumDurationByMatterRow struct {
	Matter        string  `json:"matter"`
	TotalSecs     int     `json:"total_secs"`
	TotalBillable float64 `json:"total_billable"`
	EntryCount    int     `json:"entry_count"`
}

const sumDurationByMatter = `
SELECT matter,
       COALESCE(SUM(duration_secs), 0)::INTEGER AS total_secs,
       COALESCE(SUM(billable_hours), 0.0)::float8 AS total_billable,
       COUNT(*)::INTEGER AS entry_count
FROM time_entries
WHERE start_time >= $1 AND start_time < $2
  AND end_time IS NOT NULL
GROUP BY matter
ORDER BY total_billable DESC
`

func (q *Queries) SumDurationByMatter(ctx context.Context, start, end time.Time) ([]SumDurationByMatterRow, error) {
	rows, err := q.db.Query(ctx, sumDurationByMatter, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []SumDurationByMatterRow
	for rows.Next() {
		var i SumDurationByMatterRow
		if err := rows.Scan(&i.Matter, &i.TotalSecs, &i.TotalBillable, &i.EntryCount); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

const listTimeEntriesByMatter = `
SELECT id, matter, description, raw_transcript, start_time, end_time,
       duration_secs, billable_hours::float8 AS billable_hours, created_at, updated_at
FROM time_entries
WHERE matter = $1 AND start_time >= $2 AND start_time < $3
ORDER BY start_time DESC
`

func (q *Queries) ListTimeEntriesByMatter(ctx context.Context, matter string, start, end time.Time) ([]TimeEntry, error) {
	rows, err := q.db.Query(ctx, listTimeEntriesByMatter, matter, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []TimeEntry
	for rows.Next() {
		var i TimeEntry
		if err := rows.Scan(
			&i.ID, &i.Matter, &i.Description, &i.RawTranscript,
			&i.StartTime, &i.EndTime, &i.DurationSecs, &i.BillableHours,
			&i.CreatedAt, &i.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

const insertManualEntry = `
INSERT INTO time_entries (matter, description, start_time, end_time, duration_secs, billable_hours)
VALUES ($1, $2, $3, $4, $5,
    CASE
        WHEN $5 < 1 THEN 0.0
        ELSE CEIL($5::float8 / 360.0) / 10.0
    END)
RETURNING id, matter, description, raw_transcript, start_time, end_time,
          duration_secs, billable_hours::float8 AS billable_hours, created_at, updated_at
`

type InsertManualEntryParams struct {
	Matter       string    `json:"matter"`
	Description  string    `json:"description"`
	StartTime    time.Time `json:"start_time"`
	EndTime      time.Time `json:"end_time"`
	DurationSecs int       `json:"duration_secs"`
}

func (q *Queries) InsertManualEntry(ctx context.Context, arg InsertManualEntryParams) (TimeEntry, error) {
	row := q.db.QueryRow(ctx, insertManualEntry,
		arg.Matter, arg.Description, arg.StartTime, arg.EndTime, arg.DurationSecs,
	)
	return scanTimeEntry(row)
}

const updateTimeEntry = `
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
          duration_secs, billable_hours::float8 AS billable_hours, created_at, updated_at
`

type UpdateTimeEntryParams struct {
	ID           string    `json:"id"`
	Description  string    `json:"description"`
	StartTime    time.Time `json:"start_time"`
	EndTime      time.Time `json:"end_time"`
	DurationSecs int       `json:"duration_secs"`
}

func (q *Queries) UpdateTimeEntry(ctx context.Context, arg UpdateTimeEntryParams) (TimeEntry, error) {
	row := q.db.QueryRow(ctx, updateTimeEntry,
		arg.ID, arg.Description, arg.StartTime, arg.EndTime, arg.DurationSecs,
	)
	return scanTimeEntry(row)
}

type WeeklySummaryRow struct {
	Matter        string    `json:"matter"`
	EntryDate     time.Time `json:"entry_date"`
	DailyBillable float64   `json:"daily_billable"`
}

const weeklySummary = `
SELECT matter,
       start_time::date AS entry_date,
       COALESCE(SUM(billable_hours), 0.0)::float8 AS daily_billable
FROM time_entries
WHERE start_time >= $1 AND start_time < $2
  AND end_time IS NOT NULL
GROUP BY matter, start_time::date
ORDER BY matter, entry_date
`

func (q *Queries) WeeklySummary(ctx context.Context, start, end time.Time) ([]WeeklySummaryRow, error) {
	rows, err := q.db.Query(ctx, weeklySummary, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []WeeklySummaryRow
	for rows.Next() {
		var i WeeklySummaryRow
		if err := rows.Scan(&i.Matter, &i.EntryDate, &i.DailyBillable); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}
