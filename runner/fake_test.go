package runner

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

var (
	_ Runner = (*Exec)(nil)
	_ Runner = (*FakeRunner)(nil)
)

func TestFakeRunner_ReturnsScriptedOutput(t *testing.T) {
	f := &FakeRunner{Replies: map[string]Reply{
		"git rev-parse HEAD": {Out: "abc123"},
	}}

	out, err := f.Run("git", "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if out != "abc123" {
		t.Errorf("out = %q, want %q", out, "abc123")
	}
}

func TestFakeRunner_ReturnsScriptedError(t *testing.T) {
	boom := errors.New("boom")
	f := &FakeRunner{Replies: map[string]Reply{
		"gh pr list": {Err: boom},
	}}

	_, err := f.Run("gh", "pr", "list")
	if !errors.Is(err, boom) {
		t.Errorf("err = %v, want %v", err, boom)
	}
}

func TestFakeRunner_RecordsCalls(t *testing.T) {
	f := &FakeRunner{Replies: map[string]Reply{
		"git status --short": {},
		"git branch":         {},
	}}

	f.Run("git", "status", "--short")
	f.Run("git", "branch")

	want := [][]string{
		{"git", "status", "--short"},
		{"git", "branch"},
	}
	if !reflect.DeepEqual(f.Calls, want) {
		t.Errorf("Calls = %v, want %v", f.Calls, want)
	}
}

func TestFakeRunner_UnscriptedCallErrors(t *testing.T) {
	f := &FakeRunner{}

	_, err := f.Run("git", "log")
	if err == nil || !strings.Contains(err.Error(), "git log") {
		t.Errorf("err = %v, want error naming %q", err, "git log")
	}

	// An unscripted call is still recorded, so a test can see what was attempted.
	if len(f.Calls) != 1 {
		t.Errorf("Calls = %v, want 1 recorded call", f.Calls)
	}
}
