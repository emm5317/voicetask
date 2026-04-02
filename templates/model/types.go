package model

import (
	"time"

	"github.com/emm5317/voicetask/db"
)

// ProjectMeta holds display metadata for a project tag.
type ProjectMeta struct {
	Accent string
	Label  string
}

// DeadlineInfo holds formatted deadline display data.
type DeadlineInfo struct {
	Text  string
	Color string
	Hot   bool
}

// TaskGroup holds a project tag and its tasks for template rendering.
type TaskGroup struct {
	Tag       string
	Meta      ProjectMeta
	Tasks     []db.Task
	OpenCount int
	DoneCount int
	Total     int
	Progress  int // percentage 0-100
}

// TimerState holds the current active timer info for template rendering.
type TimerState struct {
	Active    bool
	EntryID   string
	Matter    string
	Meta      ProjectMeta
	StartTime time.Time
}

// MatterSummary holds per-matter time totals for the day.
type MatterSummary struct {
	Matter      string
	Meta        ProjectMeta
	TotalSecs   int
	BillableHrs float64
	Formatted   string // e.g. "1.5 hrs"
	IsActive    bool
}

// DashboardData holds all data needed to render the dashboard.
type DashboardData struct {
	Groups      []TaskGroup
	TotalOpen   int
	UrgentCount int
	HasComplete bool
	ProjectTags []string
	// Time tracking fields
	Timer         TimerState
	MatterTotals  []MatterSummary
	DayTotalHrs   float64
	DayTotal      string // formatted e.g. "3.5 hrs"
	RecentEntries []db.TimeEntry
	ViewDate      string // YYYY-MM-DD for date navigation
	ViewDateLabel string // "Today", "Yesterday", or "Mon, Jan 2"
}

// WeeklySummaryRow holds a single matter's weekly breakdown.
type WeeklySummaryRow struct {
	Meta  ProjectMeta
	Days  []string
	Total string
}

// WeeklySummaryData holds all data for the weekly summary view.
type WeeklySummaryData struct {
	PrevWeek   string
	NextWeek   string
	WeekLabel  string
	DayLabels  []string
	Rows       []WeeklySummaryRow
	TotalDays  []string
	GrandTotal string
}

// LoginData holds login page data.
type LoginData struct {
	Error string
}
