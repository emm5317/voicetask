package components

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/emm5317/voicetask/templates/model"
)

var projectMeta = map[string]model.ProjectMeta{
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

func GetProjectMeta(tag string) model.ProjectMeta {
	if m, ok := projectMeta[strings.ToLower(tag)]; ok {
		return m
	}
	return model.ProjectMeta{Accent: "#6a6760", Label: tag}
}

func ProjectAccent(tag string) string {
	return GetProjectMeta(tag).Accent
}

func ProjectLabel(tag string) string {
	return GetProjectMeta(tag).Label
}

func FormatDeadline(d *time.Time) *model.DeadlineInfo {
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
		return &model.DeadlineInfo{Text: fmt.Sprintf("%dd overdue", -days), Color: "#e8655c", Hot: true}
	case days == 0:
		return &model.DeadlineInfo{Text: "Today", Color: "#d4975c", Hot: true}
	case days == 1:
		return &model.DeadlineInfo{Text: "Tomorrow", Color: "#bfa260", Hot: false}
	default:
		return &model.DeadlineInfo{Text: d.Format("Mon, Jan 2"), Color: "#7a7770", Hot: false}
	}
}

func PriorityColor(p string) string {
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

func PriorityLabel(p string) string {
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

func FmtDateInput(d interface{}) string {
	switch v := d.(type) {
	case *time.Time:
		if v == nil {
			return ""
		}
		return v.Format("2006-01-02")
	case time.Time:
		return v.Format("2006-01-02")
	default:
		return ""
	}
}

func FmtBillable(h float64) string {
	return fmt.Sprintf("%.1f hrs", h)
}

func FmtTimeOnly(t time.Time) string {
	return t.Local().Format("3:04 PM")
}

func IsoTime(t time.Time) string {
	return t.Format(time.RFC3339)
}

func intStr(n int) string {
	return strconv.Itoa(n)
}

func percentStr(n int) string {
	return strconv.Itoa(n) + "%"
}
