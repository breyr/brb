package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/breyr/brb/gitlog"
	"github.com/breyr/brb/gitstate"
	"github.com/breyr/brb/inbox"
	"github.com/breyr/brb/runner"
	"github.com/breyr/brb/snapshot"
	"github.com/breyr/brb/store"
)

func main() {
	dir, err := store.DataDir()
	if err == nil {
		err = run(os.Args[1:], os.Stdout, runner.Exec{}, store.NewJSONL(dir), time.Now)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "brb:", err)
		os.Exit(1)
	}
}

func run(args []string, out io.Writer, r runner.Runner, st store.Store, now func() time.Time) error {
	if len(args) > 0 {
		switch args[0] {
		case "save":
			return save(args[1:], out, r, st, now)
		case "inbox":
			return showInbox(out, r)
		case "list":
			return list(args[1:], out, st, now)
		case "resume":
			return resume(args[1:], out, r, st, now)
		}
	}

	return errors.New("usage: brb save [--project name | --no-project] <next step> | brb list [-p project] | brb resume [-n commits | --standup] | brb inbox")
}

// shortcut: no limit or paging; add one if the stack gets long enough to scroll.
func list(args []string, out io.Writer, st store.Store, now func() time.Time) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	project := fs.String("p", "", "only this project")

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("list: %w", err)
	}

	all, err := st.List()
	if err != nil {
		return fmt.Errorf("list: %w", err)
	}

	n := 0
	for _, s := range all {
		if *project != "" && s.Project != *project {
			continue
		}

		n++
		fmt.Fprintln(out, snapshotLine(s, now()))
	}

	if n == 0 {
		fmt.Fprintln(out, "no snapshots")
	}

	return nil
}

// resume prints the latest snapshot for the current repo and the commits made
// since. Snapshots without a project are only reachable through list.
func resume(args []string, out io.Writer, r runner.Runner, st store.Store, now func() time.Time) error {
	fs := flag.NewFlagSet("resume", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	n := fs.Int("n", 10, "newest commits to show")
	standupFlag := fs.Bool("standup", false, "snapshots since the last working day, all projects")

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("resume: %w", err)
	}

	if *standupFlag {
		return standup(out, st, now().Local())
	}

	if *n < 1 {
		return errors.New("resume: -n must be at least 1")
	}

	g, err := gitstate.Read(r)
	if err != nil {
		return fmt.Errorf("resume: %w", err)
	}

	if g.Project == "" {
		return errors.New("resume: not in a git repo; use brb list")
	}

	all, err := st.List()
	if err != nil {
		return fmt.Errorf("resume: %w", err)
	}

	i := slices.IndexFunc(all, func(s snapshot.Snapshot) bool { return s.Project == g.Project })
	if i < 0 {
		return fmt.Errorf("resume: no snapshot for %s", g.Project)
	}

	s := all[i]

	cs, err := gitlog.Since(r, s.SHA, s.Time)
	if err != nil {
		return fmt.Errorf("resume: %w", err)
	}

	where := s.Project
	if s.Branch != "" {
		where += " (" + s.Branch + ")"
	}

	fmt.Fprintf(out, "%s, saved %s (%s)\nnext: %s\n", where, s.Time.Local().Format("2006-01-02 15:04"), ago(now(), s.Time), s.Note)

	// The summary is the status lines, then stat blocks after a blank line; only
	// the status lines are files.
	if sum := strings.TrimSpace(s.Diff.Summary); sum != "" {
		status, _, _ := strings.Cut(sum, "\n\n")
		first, _, _ := strings.Cut(status, "\n")
		fmt.Fprintf(out, "uncommitted then: %s", strings.TrimSpace(first))

		if more := strings.Count(status, "\n"); more > 0 {
			fmt.Fprintf(out, " (+%d more)", more)
		}

		fmt.Fprintln(out)
	}

	if len(cs) == 0 {
		fmt.Fprintln(out, "no commits since")

		return nil
	}

	fmt.Fprintln(out, "since then:")

	for _, c := range cs[:min(*n, len(cs))] {
		fmt.Fprintf(out, "  - %s\n", c.Subject)

		if c.Body != "" {
			fmt.Fprintf(out, "    %s\n", strings.ReplaceAll(c.Body, "\n", "\n    "))
		}
	}

	if len(cs) > *n {
		fmt.Fprintf(out, "  ...and %d more\n", len(cs)-*n)
	}

	return nil
}

func snapshotLine(s snapshot.Snapshot, now time.Time) string {
	return fmt.Sprintf("%s (%s)  %s", s.Time.Local().Format("2006-01-02 15:04"), ago(now, s.Time), describe(s))
}

// lastWorkingDay returns local midnight of the previous weekday: Friday for a
// Saturday, Sunday or Monday. shortcut: holidays are not known (out of scope).
func lastWorkingDay(now time.Time) time.Time {
	back := 1

	switch now.Weekday() {
	case time.Monday:
		back = 3
	case time.Sunday:
		back = 2
	}

	// time.Date, not Add, so a DST change cannot shift the cutoff off midnight.
	return time.Date(now.Year(), now.Month(), now.Day()-back, 0, 0, 0, 0, now.Location())
}

// standup lists snapshots from every project since the last working day. It
// reads no git: the notes are the record, and commits may span many branches.
func standup(out io.Writer, st store.Store, now time.Time) error {
	all, err := st.List()
	if err != nil {
		return fmt.Errorf("resume: %w", err)
	}

	since := lastWorkingDay(now)
	day := since.Format("Mon 2006-01-02")

	var lines []string
	for _, s := range all {
		if !s.Time.Before(since) {
			lines = append(lines, snapshotLine(s, now))
		}
	}

	if len(lines) == 0 {
		fmt.Fprintf(out, "no snapshots since %s\n", day)

		return nil
	}

	fmt.Fprintf(out, "standup since %s\n%s\n", day, strings.Join(lines, "\n"))

	return nil
}

// describe is "project (branch)  note", leaving out what the snapshot lacks.
func describe(s snapshot.Snapshot) string {
	where := s.Project
	if s.Branch != "" {
		where += " (" + s.Branch + ")"
	}

	if where == "" {
		return s.Note
	}

	return where + "  " + s.Note
}

func ago(now, t time.Time) string {
	switch d := now.Sub(t); {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

func showInbox(out io.Writer, r runner.Runner) error {
	prs, err := inbox.Review(r)
	if err != nil {
		return fmt.Errorf("inbox: %w", err)
	}

	if len(prs) == 0 {
		fmt.Fprintln(out, "no PRs awaiting your review")
	}

	for _, p := range prs {
		fmt.Fprintf(out, "%s#%d %s (@%s, %d comments) %s\n", p.Repo, p.Number, p.Title, p.Author, p.Comments, p.URL)
	}

	return nil
}

// shortcut: flags must come before the note (stdlib flag stops at the first
// non-flag argument); upgrade if that bites.
func save(args []string, out io.Writer, r runner.Runner, st store.Store, now func() time.Time) error {
	fs := flag.NewFlagSet("save", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	noProject := fs.Bool("no-project", false, "save without a project")
	project := fs.String("project", "", "project name (default: repo name)")

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("save: %w", err)
	}

	if *noProject && *project != "" {
		return errors.New("save: --no-project and --project conflict")
	}

	note := strings.Join(fs.Args(), " ")
	if note == "" {
		return errors.New("save: a next-step note is required")
	}

	g, err := gitstate.Read(r)
	if err != nil {
		return fmt.Errorf("save: %w", err)
	}

	switch {
	case *noProject:
		g.Project = ""
	case *project != "":
		g.Project = *project
	}

	s := snapshot.Snapshot{Time: now().UTC(), Project: g.Project, Branch: g.Branch, SHA: g.SHA, Diff: g.Diff, Note: note}
	if err := st.Append(s); err != nil {
		return fmt.Errorf("save: %w", err)
	}

	line := "saved:"
	if s.Project != "" {
		line += " " + s.Project
	}

	if s.Branch != "" {
		line += " (" + s.Branch + ")"
	}

	fmt.Fprintf(out, "%s %q\n", line, note)

	return nil
}
