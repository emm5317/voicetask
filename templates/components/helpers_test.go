package components

import (
	"testing"
	"time"
)

func TestGetProjectMeta_KnownTag(t *testing.T) {
	m := GetProjectMeta("campbells")
	if m.Accent != "#c97b5a" {
		t.Errorf("accent = %q, want #c97b5a", m.Accent)
	}
	if m.Label != "Campbell's" {
		t.Errorf("label = %q, want Campbell's", m.Label)
	}
}

func TestGetProjectMeta_CaseInsensitive(t *testing.T) {
	m := GetProjectMeta("BofA")
	if m.Label != "BofA" {
		t.Errorf("label = %q, want BofA", m.Label)
	}
}

func TestGetProjectMeta_Unknown(t *testing.T) {
	m := GetProjectMeta("newclient")
	if m.Accent != "#6a6760" {
		t.Errorf("unknown accent = %q, want #6a6760", m.Accent)
	}
	if m.Label != "newclient" {
		t.Errorf("unknown label = %q, want raw tag echoed back", m.Label)
	}
}

func TestProjectAccent(t *testing.T) {
	if a := ProjectAccent("personal"); a != "#9b8aad" {
		t.Errorf("accent = %q, want #9b8aad", a)
	}
	if a := ProjectAccent("nonexistent"); a != "#6a6760" {
		t.Errorf("fallback accent = %q, want #6a6760", a)
	}
}

func TestProjectLabel(t *testing.T) {
	if l := ProjectLabel("sedalia"); l != "Sedalia" {
		t.Errorf("label = %q, want Sedalia", l)
	}
	if l := ProjectLabel("xyz"); l != "xyz" {
		t.Errorf("fallback label = %q, want xyz", l)
	}
}

func TestFormatDeadline_Nil(t *testing.T) {
	if d := FormatDeadline(nil); d != nil {
		t.Errorf("expected nil for nil deadline, got %+v", d)
	}
}

func TestFormatDeadline_Overdue(t *testing.T) {
	yesterday := time.Now().AddDate(0, 0, -3)
	d := FormatDeadline(&yesterday)
	if d == nil {
		t.Fatal("expected non-nil DeadlineInfo")
	}
	if d.Text != "3d overdue" {
		t.Errorf("text = %q, want 3d overdue", d.Text)
	}
	if d.Color != "#e8655c" {
		t.Errorf("color = %q, want #e8655c", d.Color)
	}
	if !d.Hot {
		t.Error("overdue should be hot")
	}
}

func TestFormatDeadline_Today(t *testing.T) {
	now := time.Now()
	d := FormatDeadline(&now)
	if d == nil {
		t.Fatal("expected non-nil")
	}
	if d.Text != "Today" {
		t.Errorf("text = %q, want Today", d.Text)
	}
	if !d.Hot {
		t.Error("today should be hot")
	}
}

func TestFormatDeadline_Tomorrow(t *testing.T) {
	tom := time.Now().AddDate(0, 0, 1)
	d := FormatDeadline(&tom)
	if d == nil {
		t.Fatal("expected non-nil")
	}
	if d.Text != "Tomorrow" {
		t.Errorf("text = %q, want Tomorrow", d.Text)
	}
	if d.Hot {
		t.Error("tomorrow should not be hot")
	}
}

func TestFormatDeadline_FutureDateFormat(t *testing.T) {
	future := time.Now().AddDate(0, 0, 10)
	d := FormatDeadline(&future)
	if d == nil {
		t.Fatal("expected non-nil")
	}
	// Should be formatted as "Mon, Jan 2" not a relative string
	if d.Text == "Today" || d.Text == "Tomorrow" {
		t.Errorf("future date should show formatted date, got %q", d.Text)
	}
	if d.Color != "#7a7770" {
		t.Errorf("future color = %q, want #7a7770", d.Color)
	}
}

func TestPriorityColor(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"urgent", "#e8655c"},
		{"high", "#d4975c"},
		{"low", "#4a4840"},
		{"normal", "#6a6760"},
		{"", "#6a6760"},
		{"unknown", "#6a6760"},
	}
	for _, tt := range tests {
		if got := PriorityColor(tt.input); got != tt.want {
			t.Errorf("PriorityColor(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestPriorityLabel(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"urgent", "URGENT"},
		{"high", "HIGH"},
		{"low", "LOW"},
		{"normal", ""},
		{"", ""},
	}
	for _, tt := range tests {
		if got := PriorityLabel(tt.input); got != tt.want {
			t.Errorf("PriorityLabel(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFmtDateInput_Pointer(t *testing.T) {
	ts := time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC)
	got := FmtDateInput(&ts)
	if got != "2025-03-15" {
		t.Errorf("FmtDateInput(*time.Time) = %q, want 2025-03-15", got)
	}
}

func TestFmtDateInput_Value(t *testing.T) {
	ts := time.Date(2025, 12, 1, 14, 30, 0, 0, time.UTC)
	got := FmtDateInput(ts)
	if got != "2025-12-01" {
		t.Errorf("FmtDateInput(time.Time) = %q, want 2025-12-01", got)
	}
}

func TestFmtDateInput_NilPointer(t *testing.T) {
	got := FmtDateInput((*time.Time)(nil))
	if got != "" {
		t.Errorf("FmtDateInput(nil) = %q, want empty", got)
	}
}

func TestFmtDateInput_UnsupportedType(t *testing.T) {
	got := FmtDateInput("not a time")
	if got != "" {
		t.Errorf("FmtDateInput(string) = %q, want empty", got)
	}
}

func TestFmtBillable(t *testing.T) {
	tests := []struct {
		input float64
		want  string
	}{
		{0.0, "0.0 hrs"},
		{1.5, "1.5 hrs"},
		{10.25, "10.2 hrs"},
		{0.1, "0.1 hrs"},
	}
	for _, tt := range tests {
		if got := FmtBillable(tt.input); got != tt.want {
			t.Errorf("FmtBillable(%v) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFmtTimeOnly(t *testing.T) {
	ts := time.Date(2025, 1, 1, 14, 5, 0, 0, time.Local)
	got := FmtTimeOnly(ts)
	if got != "2:05 PM" {
		t.Errorf("FmtTimeOnly = %q, want 2:05 PM", got)
	}
}

func TestIsoTime(t *testing.T) {
	ts := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
	got := IsoTime(ts)
	if got != "2025-06-15T10:30:00Z" {
		t.Errorf("IsoTime = %q, want 2025-06-15T10:30:00Z", got)
	}
}
