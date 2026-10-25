// Package runner runs external commands (git, gh) behind an interface so tests
// can fake them.
package runner

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// Runner runs an external command and returns its stdout.
type Runner interface {
	Run(name string, args ...string) (string, error)
}

// Exec is the os/exec implementation. On a non-zero exit the error wraps
// *exec.ExitError and includes trimmed stderr.
type Exec struct{}

func (Exec) Run(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("run %s: %w: %s", name, err, strings.TrimSpace(stderr.String()))
	}

	// Only the trailing newline goes; leading spaces can be meaningful (git status).
	return strings.TrimRight(string(out), "\r\n"), nil
}

// Reply is a scripted result for FakeRunner.
type Reply struct {
	Out string
	Err error
}

// FakeRunner returns scripted replies and records every call. Replies are keyed
// by the full command line: name and args joined by single spaces.
type FakeRunner struct {
	Replies map[string]Reply
	Calls   [][]string
}

func (f *FakeRunner) Run(name string, args ...string) (string, error) {
	f.Calls = append(f.Calls, append([]string{name}, args...))

	key := strings.Join(append([]string{name}, args...), " ")
	r, ok := f.Replies[key]
	if !ok {
		return "", fmt.Errorf("fake runner: unscripted command %q", key)
	}

	return r.Out, r.Err
}
