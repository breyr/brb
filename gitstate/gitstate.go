// Package gitstate reads the current directory's git state.
package gitstate

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/breyr/brb/runner"
	"github.com/breyr/brb/snapshot"
)

// State is what git says about the current directory. All fields are empty
// outside a repo.
type State struct {
	Project string // base name of the repo toplevel
	Branch  string // empty on a detached HEAD
	SHA     string // empty in a repo with no commits
	Diff    snapshot.Diff
}

// Read returns the git state. Outside a repo it returns State{} and a nil
// error; a missing git binary is an error.
func Read(r runner.Runner) (State, error) {
	top, err := r.Run("git", "rev-parse", "--show-toplevel")
	if errors.Is(err, exec.ErrNotFound) {
		return State{}, errors.New("git is not installed")
	}

	// shortcut: every other failure (dubious ownership, corrupt repo) also reads
	// as "no repo"; git has no distinct exit code, see AGENTS.md Known issues.
	if err != nil {
		return State{}, nil
	}

	s := State{Project: filepath.Base(top)}

	if s.Branch, err = r.Run("git", "branch", "--show-current"); err != nil {
		return State{}, fmt.Errorf("read branch: %w", err)
	}

	// Fails in a repo with no commits; that is not an error.
	s.SHA, _ = r.Run("git", "rev-parse", "HEAD")

	var parts []string
	for _, args := range [][]string{
		{"status", "--short"},
		{"diff", "--stat"},
		{"diff", "--cached", "--stat"},
	} {
		out, err := r.Run("git", args...)
		if err != nil {
			return State{}, fmt.Errorf("git %s: %w", args[0], err)
		}

		if out != "" {
			parts = append(parts, out)
		}
	}

	s.Diff.Summary = strings.Join(parts, "\n\n")

	return s, nil
}
