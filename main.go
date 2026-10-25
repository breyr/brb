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
	if len(args) == 0 || args[0] != "save" {
		return errors.New("usage: brb save [--project name | --no-project] <next step>")
	}

	return save(args[1:], out, r, st, now)
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
