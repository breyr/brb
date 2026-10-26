# brb

A local-first CLI that makes switching between agent coding sessions and PR
reviews cheap: save where you were in seconds, read it back in seconds.

Status: early. `save` and `inbox` work; `list` and `resume` are next. See
[AGENTS.md](AGENTS.md) for the plan.

## Install

Needs Go, `git`, and (for `inbox`) the [`gh`](https://cli.github.com) CLI signed in.

```bash
go install .
```

This puts `brb` in `$(go env GOPATH)/bin`; make sure that is on your `PATH`.

## Commands

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
