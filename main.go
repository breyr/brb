package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

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
		}
	}

	return errors.New("usage: brb save [--project name | --no-project] <next step> | brb list [-p project] | brb inbox")
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
		fmt.Fprintf(out, "%s (%s)  %s\n", s.Time.Local().Format("2006-01-02 15:04"), ago(now(), s.Time), describe(s))
	}

	if n == 0 {
		fmt.Fprintln(out, "no snapshots")
	}

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
