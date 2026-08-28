package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunArchiveRoutesKnownCommands(t *testing.T) {
	for _, cmd := range []string{"list", "restore", "purge"} {
		t.Run(cmd, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if err := RunArchive(nil, []string{cmd}, &stdout, &stderr); err != nil {
				t.Fatalf("RunArchive(%q) error: %v", cmd, err)
			}
			if !strings.Contains(stdout.String(), cmd) {
				t.Errorf("stdout = %q, want it to mention %q", stdout.String(), cmd)
			}
		})
	}
}

func TestRunArchiveRejectsMissingAndUnknownCommands(t *testing.T) {
	tests := map[string][]string{
		"no command":      {},
		"unknown command": {"bogus"},
	}
	for name, args := range tests {
		t.Run(name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if err := RunArchive(nil, args, &stdout, &stderr); err == nil {
				t.Fatalf("RunArchive(%q) = nil, want error", args)
			}
			if !strings.Contains(stderr.String(), "usage:") {
				t.Errorf("stderr = %q, want usage text", stderr.String())
			}
		})
	}
}
