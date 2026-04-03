package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"log/slog"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/emm5317/voicetask/db"
	"github.com/emm5317/voicetask/templates/components"
	"github.com/emm5317/voicetask/templates/model"
)

// decimalTimeRe matches a decimal number at the end of a string (e.g., ".2", "1.1", "0.5").
// Used to detect when a user dictates billable hours inline in a voice note description.
var decimalTimeRe = regexp.MustCompile(`(?:^|\s)(\d*\.\d+)\s*$`)

// extractDecimalTime checks if the description ends with a decimal time value.
// Returns the cleaned description (number stripped), parsed hours, and whether a match was found.
func extractDecimalTime(desc string) (cleaned string, hours float64, ok bool) {
	match := decimalTimeRe.FindStringSubmatchIndex(desc)
	if match == nil {
		return desc, 0, false
	}
	hoursStr := desc[match[2]:match[3]]
	h, err := strconv.ParseFloat(hoursStr, 64)
	if err != nil || h <= 0 || h > 24.0 {
		return desc, 0, false
	}
	// Strip the decimal time from the description
	cleaned = strings.TrimSpace(desc[:match[0]])
	if match[0] > 0 && desc[match[0]] == ' ' {
		cleaned = strings.TrimSpace(desc[:match[0]])
	}
	if cleaned == "" {
		return desc, 0, false
	}
	// Round to nearest tenth to avoid float precision issues
	h = math.Round(h*10) / 10
	return cleaned, h, true
}

// populateTimeData fills DashboardData with time tracking info for the given date.
func (a *App) populateTimeData(ctx context.Context, data *DashboardData, viewDate time.Time) {
	loc := viewDate.Location()
	dayStart := time.Date(viewDate.Year(), viewDate.Month(), viewDate.Day(), 0, 0, 0, 0, loc)
	dayEnd := dayStart.Add(24 * time.Hour)
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)

	data.ViewDate = dayStart.Format("2006-01-02")
	switch {
	case dayStart.Equal(today):
		data.ViewDateLabel = "Today"
	case dayStart.Equal(today.Add(-24 * time.Hour)):
		data.ViewDateLabel = "Yesterday"
	default:
		data.ViewDateLabel = dayStart.Format("Mon, Jan 2")
	}

	// Active timer
	active, err := a.queries.GetActiveTimer(ctx)
	if err == nil {
		data.Timer = TimerState{
			Active:    true,
			EntryID:   active.ID,
			Matter:    active.Matter,
			Meta:      getProjectMeta(active.Matter),
			StartTime: active.StartTime,
		}
	}

	// Per-matter totals for the day
	sums, err := a.queries.SumDurationByMatter(ctx, dayStart, dayEnd)
	if err != nil {
		slog.Error("sum duration by matter", "err", err)
	}
	sumMap := make(map[string]db.SumDurationByMatterRow)
	for _, s := range sums {
		sumMap[strings.ToLower(s.Matter)] = s
	}

	var dayTotal float64
	for _, tag := range a.cfg.ProjectTags {
		s := sumMap[strings.ToLower(tag)]
		ms := MatterSummary{
			Matter:      tag,
			Meta:        getProjectMeta(tag),
			TotalSecs:   s.TotalSecs,
			BillableHrs: s.TotalBillable,
			Formatted:   fmt.Sprintf("%.1f hrs", s.TotalBillable),
			IsActive:    data.Timer.Active && strings.EqualFold(data.Timer.Matter, tag),
		}
		data.MatterTotals = append(data.MatterTotals, ms)
		dayTotal += s.TotalBillable
		delete(sumMap, strings.ToLower(tag))
	}
	// Include matters not in configured tags
	for _, s := range sumMap {
		ms := MatterSummary{
			Matter:      s.Matter,
			Meta:        getProjectMeta(s.Matter),
			TotalSecs:   s.TotalSecs,
			BillableHrs: s.TotalBillable,
			Formatted:   fmt.Sprintf("%.1f hrs", s.TotalBillable),
			IsActive:    data.Timer.Active && strings.EqualFold(data.Timer.Matter, s.Matter),
		}
		data.MatterTotals = append(data.MatterTotals, ms)
		dayTotal += s.TotalBillable
	}

	data.DayTotalHrs = dayTotal
	data.DayTotal = fmt.Sprintf("%.1f hrs", dayTotal)

	// Recent entries for the day
	entries, err := a.queries.ListTimeEntriesByDate(ctx, dayStart, dayEnd)
	if err != nil {
		slog.Error("list time entries", "err", err)
	}
	data.RecentEntries = entries
}

// HandleSwitchMatter stops any active timer and starts a new one for the given matter.
func (a *App) HandleSwitchMatter(c *fiber.Ctx) error {
	matter := c.Params("matter")
	ctx := c.UserContext()

	// Check if already timing this matter — if so, just stop it (toggle behavior)
	active, err := a.queries.GetActiveTimer(ctx)
	if err == nil && strings.EqualFold(active.Matter, matter) {
		if _, err := a.queries.StopTimer(ctx, active.ID); err != nil {
			slog.Error("stop timer", "err", err)
		}
		slog.Info("timer stopped (toggle)", "matter", matter)
		a.hub.Broadcast("timer-updated", "reload")
		return a.renderTimePanel(c)
	}

	// Stop all running timers
	if _, err := a.queries.StopAllTimers(ctx); err != nil {
		slog.Error("stop all timers", "err", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to stop timers")
	}

	// If the last stopped entry is for this matter, resume it instead of starting fresh
	last, lastErr := a.queries.GetLastStoppedEntryByMatter(ctx, matter)
	if lastErr == nil {
		entry, err := a.queries.ResumeTimer(ctx, last.ID)
		if err != nil {
			slog.Error("resume timer", "err", err, "matter", matter)
			return c.Status(fiber.StatusInternalServerError).SendString("Failed to resume timer")
		}
		slog.Info("timer resumed (toggle)", "id", entry.ID, "matter", matter)
		a.hub.Broadcast("timer-updated", "reload")
		return a.renderTimePanel(c)
	}

	// No previous entry for this matter — start new
	entry, err := a.queries.StartTimer(ctx, db.StartTimerParams{Matter: matter})
	if err != nil {
		slog.Error("start timer", "err", err, "matter", matter)
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to start timer")
	}

	slog.Info("timer started", "id", entry.ID, "matter", matter)
	a.hub.Broadcast("timer-updated", "reload")
	return a.renderTimePanel(c)
}

// HandleStopTimer stops the currently active timer.
func (a *App) HandleStopTimer(c *fiber.Ctx) error {
	ctx := c.UserContext()
	active, err := a.queries.GetActiveTimer(ctx)
	if err != nil {
		slog.Info("no active timer to stop")
		return a.renderTimePanel(c)
	}

	if _, err := a.queries.StopTimer(ctx, active.ID); err != nil {
		slog.Error("stop timer", "err", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to stop timer")
	}

	slog.Info("timer stopped", "id", active.ID, "matter", active.Matter)
	a.hub.Broadcast("timer-updated", "reload")
	return a.renderTimePanel(c)
}

// HandleResumeLast stops any active timer and re-starts the most recently stopped matter.
func (a *App) HandleResumeLast(c *fiber.Ctx) error {
	ctx := c.UserContext()
	last, err := a.queries.GetLastStoppedEntry(ctx)
	if err != nil {
		slog.Info("no previous entry to resume")
		return a.renderTimePanel(c)
	}

	if _, err := a.queries.StopAllTimers(ctx); err != nil {
		slog.Error("stop all timers", "err", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to stop timers")
	}
	entry, err := a.queries.StartTimer(ctx, db.StartTimerParams{Matter: last.Matter})
	if err != nil {
		slog.Error("resume last", "err", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to resume timer")
	}

	slog.Info("timer resumed", "id", entry.ID, "matter", last.Matter)
	a.hub.Broadcast("timer-updated", "reload")
	return a.renderTimePanel(c)
}

// HandleCreateManualEntry creates a time entry with explicit start/end times.
func (a *App) HandleCreateManualEntry(c *fiber.Ctx) error {
	matter := c.FormValue("matter")
	desc := c.FormValue("description")
	startStr := c.FormValue("start_time")
	endStr := c.FormValue("end_time")
	dateStr := c.FormValue("date")

	if matter == "" || startStr == "" || endStr == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Matter, start time, and end time are required")
	}

	// Parse date + time
	if dateStr == "" {
		dateStr = time.Now().Format("2006-01-02")
	}
	startTime, err := time.ParseInLocation("2006-01-02 15:04", dateStr+" "+startStr, time.Now().Location())
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid start time format")
	}
	endTime, err := time.ParseInLocation("2006-01-02 15:04", dateStr+" "+endStr, time.Now().Location())
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid end time format")
	}
	if !endTime.After(startTime) {
		return c.Status(fiber.StatusBadRequest).SendString("End time must be after start time")
	}

	durationSecs := int(endTime.Sub(startTime).Seconds())
	entry, err := a.queries.InsertManualEntry(c.UserContext(), db.InsertManualEntryParams{
		Matter:       matter,
		Description:  desc,
		StartTime:    startTime,
		EndTime:      endTime,
		DurationSecs: durationSecs,
	})
	if err != nil {
		slog.Error("insert manual entry", "err", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to create entry")
	}

	slog.Info("manual entry created", "id", entry.ID, "matter", matter, "hours", entry.BillableHours)
	a.hub.Broadcast("timer-updated", "reload")
	return a.renderTimePanel(c)
}

// HandleUpdateTimeEntry handles PATCH /time/:id for editing description or times.
func (a *App) HandleUpdateTimeEntry(c *fiber.Ctx) error {
	id := c.Params("id")
	action := c.FormValue("action")

	switch action {
	case "description":
		desc := c.FormValue("description")
		transcript := c.FormValue("raw_transcript")
		var tp *string
		if transcript != "" {
			tp = &transcript
		}

		cleanedDesc, hours, hasDecimalTime := extractDecimalTime(desc)
		if hasDecimalTime {
			entry, err := a.queries.GetTimeEntry(c.UserContext(), id)
			if err != nil {
				slog.Error("get time entry for decimal time", "id", id, "err", err)
				return c.Status(fiber.StatusInternalServerError).SendString("Failed to update")
			}
			durationSecs := int(hours * 3600)
			endTime := entry.StartTime.Add(time.Duration(durationSecs) * time.Second)
			if _, err := a.queries.UpdateTimeEntryWithDuration(c.UserContext(), db.UpdateTimeEntryWithDurationParams{
				ID:            id,
				Description:   cleanedDesc,
				RawTranscript: tp,
				EndTime:       endTime,
				DurationSecs:  durationSecs,
				BillableHours: hours,
			}); err != nil {
				slog.Error("update time entry with decimal time", "id", id, "err", err)
				return c.Status(fiber.StatusInternalServerError).SendString("Failed to update")
			}
			slog.Info("time entry updated with decimal time", "id", id, "hours", hours, "desc", cleanedDesc)
		} else {
			if _, err := a.queries.UpdateTimeEntryDescription(c.UserContext(), db.UpdateTimeEntryDescriptionParams{
				ID:            id,
				Description:   desc,
				RawTranscript: tp,
			}); err != nil {
				slog.Error("update time entry description", "id", id, "err", err)
				return c.Status(fiber.StatusInternalServerError).SendString("Failed to update")
			}
			slog.Info("time entry description updated", "id", id)
		}

	case "edit":
		desc := c.FormValue("description")
		startStr := c.FormValue("start_time")
		endStr := c.FormValue("end_time")
		dateStr := c.FormValue("date")

		if startStr == "" || endStr == "" {
			return c.Status(fiber.StatusBadRequest).SendString("Start and end times required")
		}
		if dateStr == "" {
			dateStr = time.Now().Format("2006-01-02")
		}

		startTime, err := time.ParseInLocation("2006-01-02 15:04", dateStr+" "+startStr, time.Now().Location())
		if err != nil {
			return c.Status(fiber.StatusBadRequest).SendString("Invalid start time")
		}
		endTime, err := time.ParseInLocation("2006-01-02 15:04", dateStr+" "+endStr, time.Now().Location())
		if err != nil {
			return c.Status(fiber.StatusBadRequest).SendString("Invalid end time")
		}
		if !endTime.After(startTime) {
			return c.Status(fiber.StatusBadRequest).SendString("End must be after start")
		}

		durationSecs := int(endTime.Sub(startTime).Seconds())
		if _, err := a.queries.UpdateTimeEntry(c.UserContext(), db.UpdateTimeEntryParams{
			ID:           id,
			Description:  desc,
			StartTime:    startTime,
			EndTime:      endTime,
			DurationSecs: durationSecs,
		}); err != nil {
			slog.Error("update time entry", "id", id, "err", err)
			return c.Status(fiber.StatusInternalServerError).SendString("Failed to update")
		}
		slog.Info("time entry updated", "id", id)

	default:
		return c.Status(fiber.StatusBadRequest).SendString("Invalid action")
	}

	a.hub.Broadcast("timer-updated", "reload")
	return a.renderTimePanel(c)
}

// HandleDeleteTimeEntry handles DELETE /time/:id.
func (a *App) HandleDeleteTimeEntry(c *fiber.Ctx) error {
	id := c.Params("id")
	rows, err := a.queries.DeleteTimeEntry(c.UserContext(), id)
	if err != nil {
		slog.Error("delete time entry", "id", id, "err", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to delete")
	}
	if rows == 0 {
		return c.Status(fiber.StatusNotFound).SendString("Entry not found")
	}
	slog.Info("time entry deleted", "id", id)
	a.hub.Broadcast("timer-updated", "reload")
	return a.renderTimePanel(c)
}

// HandleTimeList returns the time panel partial (used by SSE refresh).
func (a *App) HandleTimeList(c *fiber.Ctx) error {
	return a.renderTimePanel(c)
}

// HandleTimeEntries returns the time panel for a specific date.
func (a *App) HandleTimeEntries(c *fiber.Ctx) error {
	return a.renderTimePanel(c)
}

// HandleWeeklySummary renders the weekly summary view.
func (a *App) HandleWeeklySummary(c *fiber.Ctx) error {
	weekStr := c.Query("week", time.Now().Format("2006-01-02"))
	weekDate, err := time.Parse("2006-01-02", weekStr)
	if err != nil {
		weekDate = time.Now()
	}

	// Find Monday of that week
	offset := int(weekDate.Weekday()) - 1 // Monday = 0
	if offset < 0 {
		offset = 6 // Sunday
	}
	monday := weekDate.AddDate(0, 0, -offset)
	loc := time.Now().Location()
	weekStart := time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, loc)
	weekEnd := weekStart.AddDate(0, 0, 7)

	rows, err := a.queries.WeeklySummary(c.UserContext(), weekStart, weekEnd)
	if err != nil {
		slog.Error("weekly summary", "err", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to load summary")
	}

	// Build grid: matter → [Mon..Fri] hours
	type dayHours [5]float64
	grid := make(map[string]*dayHours)
	var dailyTotals dayHours
	matters := []string{}

	for _, row := range rows {
		dayIdx := int(row.EntryDate.Weekday()) - 1 // Monday=0
		if dayIdx < 0 || dayIdx > 4 {
			continue // skip weekends
		}
		key := strings.ToLower(row.Matter)
		if _, ok := grid[key]; !ok {
			grid[key] = &dayHours{}
			matters = append(matters, row.Matter)
		}
		grid[key][dayIdx] += row.DailyBillable
		dailyTotals[dayIdx] += row.DailyBillable
	}

	// Build display rows
	var displayRows []model.WeeklySummaryRow
	for _, m := range matters {
		key := strings.ToLower(m)
		dh := grid[key]
		var total float64
		var days [5]string
		for i := 0; i < 5; i++ {
			if dh[i] > 0 {
				days[i] = fmt.Sprintf("%.1f", dh[i])
			} else {
				days[i] = "-"
			}
			total += dh[i]
		}
		displayRows = append(displayRows, model.WeeklySummaryRow{
			Meta:  getProjectMeta(m),
			Days:  days[:],
			Total: fmt.Sprintf("%.1f", total),
		})
	}

	var totalDays [5]string
	var grandTotal float64
	for i := 0; i < 5; i++ {
		if dailyTotals[i] > 0 {
			totalDays[i] = fmt.Sprintf("%.1f", dailyTotals[i])
		} else {
			totalDays[i] = "-"
		}
		grandTotal += dailyTotals[i]
	}

	// Day labels
	var dayLabels [5]string
	for i := 0; i < 5; i++ {
		dayLabels[i] = weekStart.AddDate(0, 0, i).Format("Mon 1/2")
	}

	templateData := model.WeeklySummaryData{
		WeekLabel:  fmt.Sprintf("Week of %s – %s", weekStart.Format("Jan 2"), weekStart.AddDate(0, 0, 4).Format("Jan 2")),
		PrevWeek:   weekStart.AddDate(0, 0, -7).Format("2006-01-02"),
		NextWeek:   weekStart.AddDate(0, 0, 7).Format("2006-01-02"),
		DayLabels:  dayLabels[:],
		Rows:       displayRows,
		TotalDays:  totalDays[:],
		GrandTotal: fmt.Sprintf("%.1f", grandTotal),
	}

	c.Set("Content-Type", "text/html")
	return components.WeeklySummary(templateData).Render(c.UserContext(), c.Response().BodyWriter())
}

// HandleTimeExportCSV exports time entries as CSV.
func (a *App) HandleTimeExportCSV(c *fiber.Ctx) error {
	fromStr := c.Query("from", time.Now().Format("2006-01-02"))
	toStr := c.Query("to", time.Now().Format("2006-01-02"))

	loc := time.Now().Location()
	from, _ := time.ParseInLocation("2006-01-02", fromStr, loc)
	to, _ := time.ParseInLocation("2006-01-02", toStr, loc)
	toEnd := to.Add(24 * time.Hour)

	entries, err := a.queries.ListTimeEntriesByDate(c.UserContext(), from, toEnd)
	if err != nil {
		slog.Error("export time csv", "err", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Export failed")
	}

	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=time-entries-%s.csv", fromStr))
	c.Set("Content-Type", "text/csv")

	w := csv.NewWriter(c.Response().BodyWriter())
	if err := w.Write([]string{"Date", "Matter", "Start", "End", "Duration", "Billable Hours", "Description"}); err != nil {
		return fmt.Errorf("csv header: %w", err)
	}
	for _, e := range entries {
		endStr := ""
		durStr := ""
		if e.EndTime != nil {
			endStr = e.EndTime.Local().Format("3:04 PM")
			h := e.DurationSecs / 3600
			m := (e.DurationSecs % 3600) / 60
			durStr = fmt.Sprintf("%d:%02d", h, m)
		} else {
			endStr = "(running)"
			durStr = "(running)"
		}
		if err := w.Write([]string{
			e.StartTime.Local().Format("2006-01-02"),
			e.Matter,
			e.StartTime.Local().Format("3:04 PM"),
			endStr,
			durStr,
			fmt.Sprintf("%.1f", e.BillableHours),
			e.Description,
		}); err != nil {
			return fmt.Errorf("csv row: %w", err)
		}
	}
	w.Flush()
	return nil
}

// HandleTimeExportEmail sends a time report via email.
func (a *App) HandleTimeExportEmail(c *fiber.Ctx) error {
	if a.cfg.SMTPHost == "" || a.cfg.EmailTo == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Email not configured")
	}

	fromStr := c.FormValue("from", time.Now().Format("2006-01-02"))
	toStr := c.FormValue("to", time.Now().Format("2006-01-02"))

	loc := time.Now().Location()
	from, _ := time.ParseInLocation("2006-01-02", fromStr, loc)
	to, _ := time.ParseInLocation("2006-01-02", toStr, loc)
	toEnd := to.Add(24 * time.Hour)

	ctx := c.UserContext()
	sums, err := a.queries.SumDurationByMatter(ctx, from, toEnd)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to load data")
	}
	entries, err := a.queries.ListTimeEntriesByDate(ctx, from, toEnd)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to load data")
	}

	body := buildTimeReportHTML(sums, entries, fromStr, toStr)
	subject := fmt.Sprintf("VoiceTask — Time Report %s to %s", fromStr, toStr)

	if err := a.sendEmail(subject, body); err != nil {
		slog.Error("send time report", "err", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to send email")
	}

	slog.Info("time report sent", "from", fromStr, "to", toStr)
	return a.renderTimePanel(c)
}

// renderTimePanel is a helper that fetches time data and renders the panel partial.
func (a *App) renderTimePanel(c *fiber.Ctx) error {
	dateStr := c.Query("date", c.FormValue("date", time.Now().Format("2006-01-02")))
	viewDate, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		viewDate = time.Now()
	}

	data := DashboardData{ProjectTags: a.cfg.ProjectTags}
	a.populateTimeData(c.UserContext(), &data, viewDate)

	c.Set("Content-Type", "text/html")
	return components.TimePanel(data).Render(c.UserContext(), c.Response().BodyWriter())
}

func buildTimeReportHTML(sums []db.SumDurationByMatterRow, entries []db.TimeEntry, from, to string) string {
	var b strings.Builder
	b.WriteString(`<div style="font-family:system-ui,sans-serif;max-width:500px;margin:0 auto;padding:20px;color:#333;">`)
	b.WriteString(`<h2 style="color:#b87040;margin:0 0 4px;font-size:18px;">Time Report</h2>`)
	b.WriteString(fmt.Sprintf(`<p style="font-size:13px;color:#999;margin:0 0 20px;">%s to %s</p>`, from, to))

	// Summary table
	b.WriteString(`<table style="width:100%;border-collapse:collapse;margin-bottom:24px;">`)
	b.WriteString(`<tr style="border-bottom:2px solid #eee;"><th style="text-align:left;padding:6px 0;font-size:13px;color:#666;">Matter</th><th style="text-align:right;padding:6px 0;font-size:13px;color:#666;">Hours</th><th style="text-align:right;padding:6px 0;font-size:13px;color:#666;">Entries</th></tr>`)

	var grandTotal float64
	for _, s := range sums {
		meta := getProjectMeta(s.Matter)
		b.WriteString(fmt.Sprintf(`<tr style="border-bottom:1px solid #f0f0f0;"><td style="padding:6px 0;font-size:14px;"><span style="color:%s;">&#9632;</span> %s</td><td style="text-align:right;padding:6px 0;font-size:14px;font-weight:600;">%.1f</td><td style="text-align:right;padding:6px 0;font-size:13px;color:#999;">%d</td></tr>`,
			meta.Accent, meta.Label, s.TotalBillable, s.EntryCount))
		grandTotal += s.TotalBillable
	}
	b.WriteString(fmt.Sprintf(`<tr style="border-top:2px solid #ddd;"><td style="padding:8px 0;font-size:14px;font-weight:700;">Total</td><td style="text-align:right;padding:8px 0;font-size:14px;font-weight:700;">%.1f hrs</td><td></td></tr>`, grandTotal))
	b.WriteString(`</table>`)

	// Detail entries
	if len(entries) > 0 {
		b.WriteString(`<h3 style="font-size:13px;text-transform:uppercase;letter-spacing:0.05em;color:#999;margin:0 0 8px;">Detail</h3>`)
		for _, e := range entries {
			meta := getProjectMeta(e.Matter)
			endStr := "(running)"
			if e.EndTime != nil {
				endStr = e.EndTime.Local().Format("3:04 PM")
			}
			desc := e.Description
			if desc == "" {
				desc = "(no description)"
			}
			b.WriteString(fmt.Sprintf(`<p style="margin:4px 0;font-size:13px;"><span style="color:%s;">&#9632;</span> %s — %s to %s (%.1f hrs)<br><span style="color:#999;font-size:12px;">%s</span></p>`,
				meta.Accent, meta.Label, e.StartTime.Local().Format("3:04 PM"), endStr, e.BillableHours, desc))
		}
	}

	b.WriteString(`<hr style="border:none;border-top:1px solid #eee;margin:20px 0;">`)
	b.WriteString(fmt.Sprintf(`<p style="font-size:12px;color:#999;">Generated at %s</p>`, time.Now().Format("3:04 PM, Mon Jan 2")))
	b.WriteString(`</div>`)

	return b.String()
}

// crashRecovery stops any timers left running from a previous crash.
func (a *App) crashRecovery(ctx context.Context) {
	count, err := a.queries.StopAllTimers(ctx)
	if err != nil {
		slog.Error("crash recovery: stop dangling timers", "err", err)
		return
	}
	if count > 0 {
		slog.Warn("crash recovery: stopped dangling timers", "count", count)
	}
}
