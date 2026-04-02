package main

import (
	"testing"
	"time"

	"github.com/emm5317/voicetask/db"
)

func TestBuildDashboardData_Empty(t *testing.T) {
	data := buildDashboardData(nil, []string{"personal"})
	if len(data.Groups) != 0 {
		t.Errorf("expected 0 groups for no tasks, got %d", len(data.Groups))
	}
	if data.TotalOpen != 0 {
		t.Errorf("TotalOpen = %d, want 0", data.TotalOpen)
	}
	if data.HasComplete {
		t.Error("HasComplete should be false with no tasks")
	}
}

func TestBuildDashboardData_GroupsByTag(t *testing.T) {
	tasks := []db.Task{
		{ID: "1", Title: "A", ProjectTag: "personal", Completed: false},
		{ID: "2", Title: "B", ProjectTag: "personal", Completed: true},
		{ID: "3", Title: "C", ProjectTag: "campbells", Completed: false},
	}
	tags := []string{"personal", "campbells"}
	data := buildDashboardData(tasks, tags)

	if len(data.Groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(data.Groups))
	}

	// Groups should follow tag order
	if data.Groups[0].Tag != "personal" {
		t.Errorf("first group tag = %q, want personal", data.Groups[0].Tag)
	}
	if data.Groups[1].Tag != "campbells" {
		t.Errorf("second group tag = %q, want campbells", data.Groups[1].Tag)
	}
}

func TestBuildDashboardData_Counts(t *testing.T) {
	tasks := []db.Task{
		{ID: "1", ProjectTag: "personal", Completed: false, Priority: "urgent"},
		{ID: "2", ProjectTag: "personal", Completed: false, Priority: "normal"},
		{ID: "3", ProjectTag: "personal", Completed: true},
	}
	data := buildDashboardData(tasks, []string{"personal"})

	if data.TotalOpen != 2 {
		t.Errorf("TotalOpen = %d, want 2", data.TotalOpen)
	}
	if data.UrgentCount != 1 {
		t.Errorf("UrgentCount = %d, want 1", data.UrgentCount)
	}
	if !data.HasComplete {
		t.Error("HasComplete should be true")
	}

	g := data.Groups[0]
	if g.OpenCount != 2 {
		t.Errorf("OpenCount = %d, want 2", g.OpenCount)
	}
	if g.DoneCount != 1 {
		t.Errorf("DoneCount = %d, want 1", g.DoneCount)
	}
	if g.Total != 3 {
		t.Errorf("Total = %d, want 3", g.Total)
	}
}

func TestBuildDashboardData_Progress(t *testing.T) {
	tasks := []db.Task{
		{ID: "1", ProjectTag: "p", Completed: true},
		{ID: "2", ProjectTag: "p", Completed: true},
		{ID: "3", ProjectTag: "p", Completed: false},
		{ID: "4", ProjectTag: "p", Completed: false},
	}
	data := buildDashboardData(tasks, []string{"p"})
	if data.Groups[0].Progress != 50 {
		t.Errorf("Progress = %d, want 50", data.Groups[0].Progress)
	}
}

func TestBuildDashboardData_ProgressRoundsDown(t *testing.T) {
	// 1 done out of 3 = 33.3% → should be 33 (integer division)
	tasks := []db.Task{
		{ID: "1", ProjectTag: "p", Completed: true},
		{ID: "2", ProjectTag: "p", Completed: false},
		{ID: "3", ProjectTag: "p", Completed: false},
	}
	data := buildDashboardData(tasks, []string{"p"})
	if data.Groups[0].Progress != 33 {
		t.Errorf("Progress = %d, want 33", data.Groups[0].Progress)
	}
}

func TestBuildDashboardData_UnconfiguredTag(t *testing.T) {
	// Task with a tag not in the configured tags list should still appear
	tasks := []db.Task{
		{ID: "1", ProjectTag: "surprise", Completed: false},
	}
	data := buildDashboardData(tasks, []string{"personal"})

	if len(data.Groups) != 1 {
		t.Fatalf("expected 1 group for unconfigured tag, got %d", len(data.Groups))
	}
	if data.Groups[0].Tag != "surprise" {
		t.Errorf("tag = %q, want surprise", data.Groups[0].Tag)
	}
	if data.TotalOpen != 1 {
		t.Errorf("TotalOpen = %d, want 1", data.TotalOpen)
	}
}

func TestBuildDashboardData_SkipsEmptyConfiguredTags(t *testing.T) {
	// Configured tag "campbells" has no tasks — should not produce a group
	tasks := []db.Task{
		{ID: "1", ProjectTag: "personal", Completed: false},
	}
	data := buildDashboardData(tasks, []string{"personal", "campbells"})
	if len(data.Groups) != 1 {
		t.Errorf("expected 1 group (campbells has no tasks), got %d", len(data.Groups))
	}
}

func TestBuildDashboardData_UrgentOnlyCountedWhenOpen(t *testing.T) {
	tasks := []db.Task{
		{ID: "1", ProjectTag: "p", Priority: "urgent", Completed: true},
		{ID: "2", ProjectTag: "p", Priority: "urgent", Completed: false},
	}
	data := buildDashboardData(tasks, []string{"p"})
	if data.UrgentCount != 1 {
		t.Errorf("UrgentCount = %d, want 1 (completed urgent should not count)", data.UrgentCount)
	}
}

func TestBuildDashboardData_ProjectMeta(t *testing.T) {
	tasks := []db.Task{
		{ID: "1", ProjectTag: "campbells", Completed: false},
	}
	data := buildDashboardData(tasks, []string{"campbells"})
	g := data.Groups[0]
	if g.Meta.Accent != "#c97b5a" {
		t.Errorf("Meta.Accent = %q, want #c97b5a", g.Meta.Accent)
	}
	if g.Meta.Label != "Campbell's" {
		t.Errorf("Meta.Label = %q, want Campbell's", g.Meta.Label)
	}
}

func TestBuildDashboardData_ProjectTagsPassedThrough(t *testing.T) {
	tags := []string{"a", "b", "c"}
	data := buildDashboardData(nil, tags)
	if len(data.ProjectTags) != 3 || data.ProjectTags[0] != "a" {
		t.Errorf("ProjectTags not passed through: %v", data.ProjectTags)
	}
}

func TestBuildDashboardData_DeadlinePreserved(t *testing.T) {
	dl := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
	tasks := []db.Task{
		{ID: "1", ProjectTag: "p", Deadline: &dl},
	}
	data := buildDashboardData(tasks, []string{"p"})
	if data.Groups[0].Tasks[0].Deadline == nil {
		t.Fatal("deadline should be preserved")
	}
	if !data.Groups[0].Tasks[0].Deadline.Equal(dl) {
		t.Errorf("deadline = %v, want %v", data.Groups[0].Tasks[0].Deadline, dl)
	}
}
