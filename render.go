package main

import (
	"strings"

	"github.com/emm5317/voicetask/db"
	"github.com/emm5317/voicetask/templates/components"
	"github.com/emm5317/voicetask/templates/model"
)

// Type aliases so existing handler code can refer to these without prefix.
type DashboardData = model.DashboardData
type TaskGroup = model.TaskGroup
type ProjectMeta = model.ProjectMeta
type TimerState = model.TimerState
type MatterSummary = model.MatterSummary
type DeadlineInfo = model.DeadlineInfo

// Renderer is kept as a struct for API compatibility with tests that create &App{renderer: NewRenderer()}.
// With Templ, it holds no state.
type Renderer struct{}

// NewRenderer returns a Renderer. With Templ, no template parsing is needed.
func NewRenderer() *Renderer {
	return &Renderer{}
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

// getProjectMeta delegates to the components package which owns the color map.
func getProjectMeta(tag string) ProjectMeta {
	return components.GetProjectMeta(tag)
}
