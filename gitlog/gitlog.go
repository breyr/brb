// Package gitlog reads commit messages live from git; nothing is stored.
package gitlog

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/breyr/brb/runner"
)

// Commit is one commit's message.
type Commit struct{ Subject, Body string }

// Since returns the commits made since a snapshot, newest first. It uses
// sha..HEAD, else base..HEAD when sha is empty, else commits newer than t
// (the SHA is gone after a rebase or squash, or no base branch exists).
func Since(r runner.Runner, sha string, t time.Time) ([]Commit, error) {
	if _, err := r.Run("git", "rev-parse", "--verify", "--quiet", "HEAD"); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, errors.New("git is not installed")
		}

		return nil, nil // no commits yet
	}

	args := []string{"log", "--format=%s%x1f%b%x1e"}

	// shortcut: any failure of the ancestor check reads as "SHA gone"; git
	// has no distinct exit code for a transient error, widen to the timestamp.
	switch base := rangeStart(r, sha); base {
	case "":
		args = append(args, "--since="+t.UTC().Format(time.RFC3339))
	default:
		args = append(args, base+"..HEAD")
	}

	out, err := r.Run("git", args...)
	if err != nil {
		return nil, fmt.Errorf("read git log: %w", err)
	}

	var commits []Commit
	for _, rec := range strings.Split(out, "\x1e") {
		subject, body, ok := strings.Cut(rec, "\x1f")
		if !ok {
			continue
		}

		commits = append(commits, Commit{strings.TrimSpace(subject), strings.TrimSpace(body)})
	}

	return commits, nil
}

// rangeStart returns the left side of the range, or "" to use the timestamp.
func rangeStart(r runner.Runner, sha string) string {
	if sha != "" {
		if _, err := r.Run("git", "merge-base", "--is-ancestor", sha, "HEAD"); err != nil {
			return ""
		}

		return sha
	}

	if ref, err := r.Run("git", "symbolic-ref", "--short", "refs/remotes/origin/HEAD"); err == nil {
		return ref
	}

	for _, b := range []string{"main", "master"} {
		if _, err := r.Run("git", "rev-parse", "--verify", "--quiet", b); err == nil {
			return b
		}
	}

	return ""
}
