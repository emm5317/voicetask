package main

import (
	"testing"
)

func TestExtractDecimalTime(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantCleaned string
		wantHours   float64
		wantOk      bool
	}{
		{
			name:        "simple tenths",
			input:       "call with Bob .2",
			wantCleaned: "call with Bob",
			wantHours:   0.2,
			wantOk:      true,
		},
		{
			name:        "whole and tenths",
			input:       "reviewed documents 1.1",
			wantCleaned: "reviewed documents",
			wantHours:   1.1,
			wantOk:      true,
		},
		{
			name:        "zero prefix decimal",
			input:       "meeting 0.5",
			wantCleaned: "meeting",
			wantHours:   0.5,
			wantOk:      true,
		},
		{
			name:        "trailing whitespace",
			input:       "phone call .3  ",
			wantCleaned: "phone call",
			wantHours:   0.3,
			wantOk:      true,
		},
		{
			name:        "one tenth",
			input:       "quick question .1",
			wantCleaned: "quick question",
			wantHours:   0.1,
			wantOk:      true,
		},
		{
			name:        "larger value",
			input:       "all day meeting 8.0",
			wantCleaned: "all day meeting",
			wantHours:   8.0,
			wantOk:      true,
		},
		{
			name:        "not at end - no match",
			input:       "version 3.0 released",
			wantCleaned: "version 3.0 released",
			wantHours:   0,
			wantOk:      false,
		},
		{
			name:        "no decimal point - no match",
			input:       "item 2",
			wantCleaned: "item 2",
			wantHours:   0,
			wantOk:      false,
		},
		{
			name:        "only decimal - no match (empty desc)",
			input:       ".5",
			wantCleaned: ".5",
			wantHours:   0,
			wantOk:      false,
		},
		{
			name:        "exceeds 24 hours - no match",
			input:       "something 25.0",
			wantCleaned: "something 25.0",
			wantHours:   0,
			wantOk:      false,
		},
		{
			name:        "zero value - no match",
			input:       "something 0.0",
			wantCleaned: "something 0.0",
			wantHours:   0,
			wantOk:      false,
		},
		{
			name:        "negative lookalike - still matches trailing decimal",
			input:       "drafted motion for summary judgment .4",
			wantCleaned: "drafted motion for summary judgment",
			wantHours:   0.4,
			wantOk:      true,
		},
		{
			name:        "multiple decimals - only trailing matched",
			input:       "reviewed section 3.2 of contract .5",
			wantCleaned: "reviewed section 3.2 of contract",
			wantHours:   0.5,
			wantOk:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleaned, hours, ok := extractDecimalTime(tt.input)
			if ok != tt.wantOk {
				t.Errorf("ok = %v, want %v", ok, tt.wantOk)
			}
			if cleaned != tt.wantCleaned {
				t.Errorf("cleaned = %q, want %q", cleaned, tt.wantCleaned)
			}
			if hours != tt.wantHours {
				t.Errorf("hours = %v, want %v", hours, tt.wantHours)
			}
		})
	}
}
