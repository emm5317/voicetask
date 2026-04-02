package main

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/emm5317/voicetask/db"
)

//go:embed templates/*
var templateFS embed.FS

// ProjectMeta holds display metadata for a project tag.
type ProjectMeta struct {
	Accent string
	Label  string
}

var projectMeta = map[string]ProjectMeta{
	"campbells":     {Accent: "#c97b5a", Label: "Campbell's"},
	"personal":      {Accent: "#9b8aad", Label: "Personal"},
	"sedalia":       {Accent: "#5a9cc9", Label: "Sedalia"},
	"bofa":          {Accent: "#5ac9b3", Label: "BofA"},
	"gritton":       {Accent: "#7bc95a", Label: "Gritton"},
	"diment":        {Accent: "#c9b35a", Label: "Diment"},
	"constellation": {Accent: "#5a7bc9", Label: "Constellation"},
	"national life": {Accent: "#c95a7b", Label: "National Life"},
	"cinfin":        {Accent: "#8a5ac9", Label: "CinFin"},
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

// DashboardData holds all data needed to render the dashboard.
type DashboardData struct {
	Groups      []TaskGroup
	TotalOpen   int
	UrgentCount int
	HasComplete bool
	ProjectTags []string
}

// Renderer holds parsed templates.
type Renderer struct {
	templates *template.Template
}

// NewRenderer parses all embedded templates.
func NewRenderer() *Renderer {
	funcMap := template.FuncMap{
		"upper":          strings.ToUpper,
		"formatDeadline": fmtDeadline,
		"projectAccent":  func(tag string) string { return getProjectMeta(tag).Accent },
		"projectLabel":   func(tag string) string { return getProjectMeta(tag).Label },
		"priorityColor":  priorityColor,
		"priorityLabel":  priorityLabel,
		"seq":          func(n int) []int { s := make([]int, n); for i := range s { s[i] = i }; return s },
		"fmtDateInput": func(d *time.Time) string { if d == nil { return "" }; return d.Format("2006-01-02") },
	}

	tmpl := template.Must(
		template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/*.html", "templates/partials/*.html"),
	)

	return &Renderer{templates: tmpl}
}

// RenderDashboard renders the full dashboard page.
func (r *Renderer) RenderDashboard(tasks []db.Task, tags []string) (string, error) {
	data := buildDashboardData(tasks, tags)
	var buf bytes.Buffer
	if err := r.templates.ExecuteTemplate(&buf, "layout.html", data); err != nil {
		return "", fmt.Errorf("render dashboard: %w", err)
	}
	return buf.String(), nil
}

// RenderTaskList renders only the task list partial (for HTMX swaps).
func (r *Renderer) RenderTaskList(tasks []db.Task, tags []string) (string, error) {
	data := buildDashboardData(tasks, tags)
	var buf bytes.Buffer
	if err := r.templates.ExecuteTemplate(&buf, "tasklist.html", data); err != nil {
		return "", fmt.Errorf("render task list: %w", err)
	}
	return buf.String(), nil
}

// RenderLogin renders the login page.
func (r *Renderer) RenderLogin(errMsg string) (string, error) {
	var buf bytes.Buffer
	if err := r.templates.ExecuteTemplate(&buf, "login.html", map[string]string{"Error": errMsg}); err != nil {
		return "", fmt.Errorf("render login: %w", err)
	}
	return buf.String(), nil
}

func buildDashboardData(tasks []db.Task, tags []string) DashboardData {
	grouped := make(map[string][]db.Task)
	for _, t := range tasks {
		grouped[t.ProjectTag] = append(grouped[t.ProjectTag], t)
	}

	var groups []TaskGroup
	totalOpen := 0
	urgentCount := 0
	hasComplete := false

	for _, tag := range tags {
		tagTasks, ok := grouped[tag]
		if !ok || len(tagTasks) == 0 {
			continue
		}

		openCount := 0
		doneCount := 0
		for _, t := range tagTasks {
			if t.Completed {
				doneCount++
				hasComplete = true
			} else {
				openCount++
				if t.Priority == "urgent" {
					urgentCount++
				}
			}
		}
		totalOpen += openCount

		progress := 0
		if len(tagTasks) > 0 {
			progress = (doneCount * 100) / len(tagTasks)
		}

		groups = append(groups, TaskGroup{
			Tag:       tag,
			Meta:      getProjectMeta(tag),
			Tasks:     tagTasks,
			OpenCount: openCount,
			DoneCount: doneCount,
			Total:     len(tagTasks),
			Progress:  progress,
		})
	}

	// Also include tags not in the configured list
	for tag, tagTasks := range grouped {
		found := false
		for _, t := range tags {
			if strings.EqualFold(t, tag) {
				found = true
				break
			}
		}
		if !found {
			openCount := 0
			doneCount := 0
			for _, t := range tagTasks {
				if t.Completed {
					doneCount++
					hasComplete = true
				} else {
					openCount++
				}
			}
			totalOpen += openCount
			progress := 0
			if len(tagTasks) > 0 {
				progress = (doneCount * 100) / len(tagTasks)
			}
			groups = append(groups, TaskGroup{
				Tag: tag, Meta: getProjectMeta(tag), Tasks: tagTasks,
				OpenCount: openCount, DoneCount: doneCount, Total: len(tagTasks), Progress: progress,
			})
		}
	}

	return DashboardData{
		Groups:      groups,
		TotalOpen:   totalOpen,
		UrgentCount: urgentCount,
		HasComplete: hasComplete,
		ProjectTags: tags,
	}
}

func getProjectMeta(tag string) ProjectMeta {
	if m, ok := projectMeta[strings.ToLower(tag)]; ok {
		return m
	}
	return ProjectMeta{Accent: "#6a6760", Label: tag}
}

func fmtDeadline(d *time.Time) *DeadlineInfo {
	if d == nil {
		return nil
	}
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	deadline := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, now.Location())
	diff := deadline.Sub(today)
	days := int(diff.Hours() / 24)

	switch {
	case days < 0:
		return &DeadlineInfo{Text: fmt.Sprintf("%dd overdue", -days), Color: "#e8655c", Hot: true}
	case days == 0:
		return &DeadlineInfo{Text: "Today", Color: "#d4975c", Hot: true}
	case days == 1:
		return &DeadlineInfo{Text: "Tomorrow", Color: "#bfa260", Hot: false}
	default:
		return &DeadlineInfo{Text: d.Format("Mon, Jan 2"), Color: "#7a7770", Hot: false}
	}
}

func priorityColor(p string) string {
	switch p {
	case "urgent":
		return "#e8655c"
	case "high":
		return "#d4975c"
	case "low":
		return "#4a4840"
	default:
		return "#6a6760"
	}
}

func priorityLabel(p string) string {
	switch p {
	case "urgent":
		return "URGENT"
	case "high":
		return "HIGH"
	case "low":
		return "LOW"
	default:
		return ""
	}
}
