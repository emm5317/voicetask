package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

// Provider is the interface for LLM-based task extraction.
type Provider interface {
	ExtractTasks(ctx context.Context, transcript string) ([]ExtractedTask, error)
}

// ExtractedTask represents a task parsed from an LLM response.
type ExtractedTask struct {
	Title      string `json:"title"`
	ProjectTag string `json:"project_tag"`
	Priority   string `json:"priority"`
	Deadline   string `json:"deadline,omitempty"`
}

type tasksResponse struct {
	Tasks []ExtractedTask `json:"tasks"`
}

var validPriorities = map[string]bool{
	"urgent": true,
	"high":   true,
	"normal": true,
	"low":    true,
}

// SystemPrompt builds the system prompt for task extraction.
func SystemPrompt(today time.Time, tags []string) string {
	return fmt.Sprintf(`You are a task extraction assistant. You receive raw voice transcripts and extract structured action items. Return ONLY valid JSON — no markdown, no commentary, no backticks.

Known projects: %s.
Infer the project tag from context. Default to "personal" if unclear.

Infer priority:
- "urgent" for time-sensitive legal deadlines or explicit urgency
- "high" for this-week items
- "normal" for everything else
- "low" for someday/maybe

Parse any deadline language relative to today's date: %s

If the transcript contains multiple tasks, return multiple items.
If the transcript is a single thought, return one item with a clean, actionable title.

Respond with this exact JSON structure:
{"tasks":[{"title":"...","project_tag":"...","priority":"...","deadline":"2025-04-15"}]}`,
		strings.Join(tags, ", "),
		today.Format("2006-01-02"))
}

// cleanJSON strips markdown code fencing from LLM responses.
func cleanJSON(raw string) string {
	s := strings.TrimSpace(raw)

	// Strip ```json ... ``` or ``` ... ```
	if strings.HasPrefix(s, "```") {
		// Remove opening fence (with optional language tag)
		if idx := strings.Index(s, "\n"); idx != -1 {
			s = s[idx+1:]
		}
		// Remove closing fence
		if idx := strings.LastIndex(s, "```"); idx != -1 {
			s = s[:idx]
		}
		s = strings.TrimSpace(s)
	}

	return s
}

// ParseTasksResponse parses an LLM JSON response into extracted tasks.
// On any failure, returns a single fallback task with the raw transcript as title.
func ParseTasksResponse(body []byte, rawTranscript string, knownTags []string) []ExtractedTask {
	cleaned := cleanJSON(string(body))

	var resp tasksResponse
	if err := json.Unmarshal([]byte(cleaned), &resp); err != nil {
		log.Printf("LLM parse error: %v\nRaw response: %s", err, string(body))
		return fallbackTask(rawTranscript)
	}

	if len(resp.Tasks) == 0 {
		log.Printf("LLM returned empty tasks array\nRaw response: %s", string(body))
		return fallbackTask(rawTranscript)
	}

	tagSet := make(map[string]bool, len(knownTags))
	for _, t := range knownTags {
		tagSet[strings.ToLower(t)] = true
	}

	for i := range resp.Tasks {
		resp.Tasks[i] = validateTask(resp.Tasks[i], rawTranscript, tagSet)
	}

	return resp.Tasks
}

func validateTask(t ExtractedTask, rawTranscript string, knownTags map[string]bool) ExtractedTask {
	if strings.TrimSpace(t.Title) == "" {
		t.Title = rawTranscript
	}

	if !validPriorities[t.Priority] {
		t.Priority = "normal"
	}

	tag := strings.ToLower(strings.TrimSpace(t.ProjectTag))
	if !knownTags[tag] {
		t.ProjectTag = "personal"
	} else {
		t.ProjectTag = tag
	}

	return t
}

func fallbackTask(rawTranscript string) []ExtractedTask {
	return []ExtractedTask{{
		Title:      rawTranscript,
		ProjectTag: "personal",
		Priority:   "normal",
	}}
}
