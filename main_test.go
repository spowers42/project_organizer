package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestGreetingMentionsProgramName(t *testing.T) {
	if got := greeting(); !strings.Contains(got, "project_organizer") {
		t.Errorf("greeting() = %q, want it to mention %q", got, "project_organizer")
	}
}

func TestRunWritesGreetingLine(t *testing.T) {
	var buf bytes.Buffer
	if err := run(&buf); err != nil {
		t.Fatalf("run() returned error: %v", err)
	}

	want := greeting() + "\n"
	if got := buf.String(); got != want {
		t.Errorf("run() wrote %q, want %q", got, want)
	}
}
