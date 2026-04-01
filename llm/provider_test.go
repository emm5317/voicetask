package llm

import (
	"strings"
	"testing"
	"time"
)

var testTags = []string{"campbells", "personal", "sedalia", "bofa", "gritton", "diment", "constellation", "national life", "cinfin"}

func TestCleanJSON_Plain(t *testing.T) {
	input := `{"tasks":[{"title":"test","project_tag":"personal","priority":"normal"}]}`
	got := cleanJSON(input)
	if got != input {
		t.Errorf("cleanJSON should not modify plain JSON:\ngot:  %s\nwant: %s", got, input)
	}
}

func TestCleanJSON_MarkdownFencing(t *testing.T) {
	input := "```json\n{\"tasks\":[{\"title\":\"test\"}]}\n```"
	want := `{"tasks":[{"title":"test"}]}`
	got := cleanJSON(input)
	if got != want {
		t.Errorf("cleanJSON with markdown fencing:\ngot:  %s\nwant: %s", got, want)
	}
}

func TestCleanJSON_TripleBackticksNoLang(t *testing.T) {
	input := "```\n{\"tasks\":[]}\n```"
	want := `{"tasks":[]}`
	got := cleanJSON(input)
	if got != want {
		t.Errorf("cleanJSON with ``` no lang:\ngot:  %s\nwant: %s", got, want)
	}
}

func TestParseTasksResponse_ValidSingleTask(t *testing.T) {
	body := `{"tasks":[{"title":"Draft motion to compel","project_tag":"campbells","priority":"urgent","deadline":"2026-04-15"}]}`
	tasks := ParseTasksResponse([]byte(body), "draft the motion thing", testTags)
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].Title != "Draft motion to compel" {
		t.Errorf("title: got %q", tasks[0].Title)
	}
	if tasks[0].ProjectTag != "campbells" {
		t.Errorf("tag: got %q", tasks[0].ProjectTag)
	}
	if tasks[0].Priority != "urgent" {
		t.Errorf("priority: got %q", tasks[0].Priority)
	}
	if tasks[0].Deadline != "2026-04-15" {
		t.Errorf("deadline: got %q", tasks[0].Deadline)
	}
}

func TestParseTasksResponse_MultiTask(t *testing.T) {
	body := `{"tasks":[
		{"title":"Update Clientsite docs","project_tag":"personal","priority":"normal"},
		{"title":"Call Kayla about demo","project_tag":"personal","priority":"low"},
		{"title":"Pick up dry cleaning","project_tag":"personal","priority":"low"}
	]}`
	tasks := ParseTasksResponse([]byte(body), "update docs and call kayla and pick up dry cleaning", testTags)
	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(tasks))
	}
}

func TestParseTasksResponse_MarkdownWrapped(t *testing.T) {
	body := "```json\n{\"tasks\":[{\"title\":\"Test task\",\"project_tag\":\"personal\",\"priority\":\"normal\"}]}\n```"
	tasks := ParseTasksResponse([]byte(body), "test task", testTags)
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].Title != "Test task" {
		t.Errorf("title: got %q", tasks[0].Title)
	}
}

func TestParseTasksResponse_MalformedJSON(t *testing.T) {
	body := `this is not json at all`
	tasks := ParseTasksResponse([]byte(body), "original transcript", testTags)
	if len(tasks) != 1 {
		t.Fatalf("expected 1 fallback task, got %d", len(tasks))
	}
	if tasks[0].Title != "original transcript" {
		t.Errorf("fallback title: got %q", tasks[0].Title)
	}
	if tasks[0].ProjectTag != "personal" {
		t.Errorf("fallback tag: got %q", tasks[0].ProjectTag)
	}
	if tasks[0].Priority != "normal" {
		t.Errorf("fallback priority: got %q", tasks[0].Priority)
	}
}

func TestParseTasksResponse_EmptyTasksArray(t *testing.T) {
	body := `{"tasks":[]}`
	tasks := ParseTasksResponse([]byte(body), "something", testTags)
	if len(tasks) != 1 {
		t.Fatalf("expected 1 fallback task, got %d", len(tasks))
	}
	if tasks[0].Title != "something" {
		t.Errorf("fallback title: got %q", tasks[0].Title)
	}
}

func TestParseTasksResponse_InvalidPriority(t *testing.T) {
	body := `{"tasks":[{"title":"A task","project_tag":"personal","priority":"critical"}]}`
	tasks := ParseTasksResponse([]byte(body), "a task", testTags)
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].Priority != "normal" {
		t.Errorf("invalid priority should normalize to 'normal': got %q", tasks[0].Priority)
	}
}

func TestParseTasksResponse_UnknownTag(t *testing.T) {
	body := `{"tasks":[{"title":"A task","project_tag":"unknown_project","priority":"normal"}]}`
	tasks := ParseTasksResponse([]byte(body), "a task", testTags)
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].ProjectTag != "personal" {
		t.Errorf("unknown tag should normalize to 'personal': got %q", tasks[0].ProjectTag)
	}
}

func TestParseTasksResponse_EmptyTitle(t *testing.T) {
	body := `{"tasks":[{"title":"","project_tag":"campbells","priority":"high"}]}`
	tasks := ParseTasksResponse([]byte(body), "the original input", testTags)
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].Title != "the original input" {
		t.Errorf("empty title should use raw transcript: got %q", tasks[0].Title)
	}
}

func TestParseTasksResponse_CaseInsensitiveTag(t *testing.T) {
	body := `{"tasks":[{"title":"BofA review","project_tag":"BofA","priority":"normal"}]}`
	tasks := ParseTasksResponse([]byte(body), "bofa review", testTags)
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].ProjectTag != "bofa" {
		t.Errorf("tag should match case-insensitively: got %q", tasks[0].ProjectTag)
	}
}

func TestSystemPrompt_ContainsTagsAndDate(t *testing.T) {
	tags := []string{"campbells", "personal"}
	today := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	prompt := SystemPrompt(today, tags)
	if !strings.Contains(prompt, "campbells") {
		t.Error("prompt should contain tag 'campbells'")
	}
	if !strings.Contains(prompt, "personal") {
		t.Error("prompt should contain tag 'personal'")
	}
	if !strings.Contains(prompt, "2026-04-01") {
		t.Error("prompt should contain today's date")
	}
}
