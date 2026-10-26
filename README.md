# brb

A local-first CLI that makes switching between agent coding sessions and PR
reviews cheap: save where you were in seconds, read it back in seconds.

Status: early. `save`, `list`, `resume` and `inbox` work. See
[AGENTS.md](AGENTS.md) for the plan.

## Install

Needs Go, `git`, and (for `inbox`) the [`gh`](https://cli.github.com) CLI signed in.

```bash
go install .
```

This puts `brb` in `$(go env GOPATH)/bin`; make sure that is on your `PATH`.
Tagged releases (`v*`) also publish prebuilt binaries for macOS, Linux and
Windows on the GitHub Releases page. `brb version` prints which one you have.

## Commands

`brb help` (or `-h` on any command) lists them.

### `brb save [--project name | --no-project] <next step>`

Saves a snapshot of where you are: the repo name, branch, `HEAD` SHA, a summary
of uncommitted changes, and your one-line note.

```bash
brb save "wire up the --project flag"
```

- Inside a git repo the project defaults to the repo name. `--project` overrides
  it and `--no-project` clears it.
- Outside a repo only the time and your note are saved.
- Put flags before the note. Multi-word notes need no quotes.

### `brb list [-p project]`

Shows your snapshots, newest first, with absolute and relative times:

```
2026-10-25 14:03 (2h ago)  brb (main)  wire up the --project flag
```

`-p` shows one project only. Snapshots saved outside a repo or with
`--no-project` appear here and nowhere else.

### `brb resume [-n commits]`

Reads back where you left off in the current repo: your latest snapshot for it,
then the commits made since (subjects and bodies, read live from `git`).

```
brb (main), saved 2026-10-25 14:03 (2h ago)
next: wire up the --project flag
uncommitted then: M main.go (+2 more)
since then:
  - Add brb list
    Newest-first stack of snapshots.
```

Shows the newest 10 commits; `-n` changes that. It only looks at snapshots
saved with a project, so it does not work outside a repo or after
`--no-project`; use `brb list` for those.

### `brb resume --standup`

Lists every snapshot from all projects since the last working day (Friday on a
Saturday, Sunday or Monday), newest first, for a standup update. It works
anywhere and does not read git, so it shows your notes, not your commits.

```
standup since Fri 2026-10-23
2026-10-25 14:03 (2h ago)  brb (main)  wire up the --project flag
```

### `brb inbox`

Lists open PRs waiting for your review, with author and comment count:

```
acme/widgets#12 Fix it (@sam, 3 comments) https://github.com/acme/widgets/pull/12
```

## Configuration

- `git config --global brb.user <github-login>` sets whose review requests
  `inbox` lists. Without it, `inbox` uses the account `gh` is signed in as.
  `user.name` is a display name, so it is not used.
- `BRB_DATA_DIR` overrides where snapshots are stored.

## Where your data lives

One JSONL file per month, for example `2026-10.jsonl`, in `~/.local/share/brb/`
(`%LocalAppData%\brb` on Windows). The files are plain text, private to you
(mode `0600`), and never inside a repo. Nothing leaves your machine except the
`gh` call that `inbox` makes.
