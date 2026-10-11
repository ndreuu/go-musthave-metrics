package config

import (
	"testing"
	"time"
)

func TestMergePreservesPresence(t *testing.T) {
	type source struct {
		Text    *string
		Number  *int
		Enabled *bool
		Absent  *string
	}
	defaults := source{Pointer("default"), Pointer(10), Pointer(true), Pointer("retained")}
	zeroes := source{Text: Pointer(""), Number: Pointer(0), Enabled: Pointer(false)}
	got, err := Merge(defaults, zeroes, source{})
	if err != nil {
		t.Fatal(err)
	}
	if *got.Text != "" || *got.Number != 0 || *got.Enabled || *got.Absent != "retained" {
		t.Fatalf("merged values = %q, %d, %v, %q", *got.Text, *got.Number, *got.Enabled, *got.Absent)
	}
	if *defaults.Text != "default" || *defaults.Number != 10 || !*defaults.Enabled {
		t.Fatal("merge modified the defaults source")
	}
}

func TestDuration(t *testing.T) {
	tests := []struct {
		input string
		want  time.Duration
		valid bool
	}{
		{"10s", 10 * time.Second, true}, {"1m", time.Minute, true},
		{"3", 3 * time.Second, true}, {"1500ms", 1500 * time.Millisecond, true},
		{"0", 0, true}, {"0s", 0, true}, {"-2", -2 * time.Second, true},
		{"abc", 0, false}, {"", 0, false}, {"9223372036854775807", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := Duration(tt.input)
			if (err == nil) != tt.valid || got != tt.want {
				t.Fatalf("Duration(%q) = %v, %v; want %v, valid=%v", tt.input, got, err, tt.want, tt.valid)
			}
		})
	}
}
