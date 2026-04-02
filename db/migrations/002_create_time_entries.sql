CREATE TABLE IF NOT EXISTS time_entries (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    matter          TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    raw_transcript  TEXT,
    start_time      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    end_time        TIMESTAMPTZ,
    duration_secs   INTEGER NOT NULL DEFAULT 0,
    billable_hours  NUMERIC(5,1) NOT NULL DEFAULT 0.0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_time_entries_matter ON time_entries(matter);
CREATE INDEX IF NOT EXISTS idx_time_entries_start_time ON time_entries(start_time DESC);
CREATE INDEX IF NOT EXISTS idx_time_entries_active ON time_entries(end_time) WHERE end_time IS NULL;
