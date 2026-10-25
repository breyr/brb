# AGENTS.md

Guidance for AI coding agents working in this repo. Read this first.

> Status: DRAFT. Scaffolded (Go module, empty `main`); no features yet. Items
> marked (TBD) are decisions the owner has not finalized. Update this file as
> they land.

## Project

Name: **brb**. A short global command with subcommands (`brb save`,
`brb resume`, `brb list`, `brb inbox`).

A local-first CLI that fixes the cost of context switching for a developer who
alternates between an AI-agent coding session and PR reviews/messages.

### Phase 1: core

- `save`: snapshot the current thread of work (project, branch, uncommitted diff
  summary, and a one-line "next step" written by the user).
- `list`: recent snapshots as a stack, newest first. Filter by project.
- `resume`: print a short brief of where the user left off. Defaults to the
  latest snapshot for the current project, followed by the git-log reader below.
- `resume --standup`: everything since the last working day (Friday on a
  Monday), for a standup update.
- `inbox`: list PRs awaiting the user's review (via the `gh` CLI). The GitHub
  login comes from `git config brb.user` if set, otherwise `@me` (the account
  `gh` is signed in as). `git config user.name` is a display name, not a login,
  so it is not used.

**Projects:** a snapshot may belong to a project, but does not have to. The
project is an optional field on the snapshot, not a separate record. Inside a git
repo it defaults to the repo name; `--no-project` or a flag overrides it. Outside
a repo the snapshot is just a timestamp and a note (branch, SHA and diff are
empty). Branch is a separate field from project.

**Git-log reader:** each snapshot stores the `HEAD` SHA. `resume` shows commit
subjects and bodies since that SHA (`git log <sha>..HEAD`), so well-written
commit messages from an agent double as a record of what happened. Fallbacks:
`<base>..HEAD` when there is no SHA (detect base from `origin/HEAD`, then
`main`/`master`); the snapshot's timestamp if a rebase or squash removed the
SHA. `resume --standup` uses `git log --since=<last working day> --author=<user>`.
Read messages live from git and store only the SHA, never the commit text.
Commits with an empty body may be flagged, but the tool cannot enforce good
messages; that belongs in each repo's own agent guidance.

"Last agent prompt" capture is deferred: no agent exposes it in a stable way.
The git log and the user's one-line note are the sources of truth.

### Phase 2

- `--json` output on every command, so the TUI and a future VS Code extension
  can reuse the CLI. Not built in phase 1.
- Bookmarks: a label plus a target (`path:line`, URL, or free text), with an
  optional project, so they can also live outside any project (cross-repo
  references, runbooks, decision links). Store the path, line and HEAD SHA at
  creation; do not try to track line drift. `resume` shows only the current
  project's bookmarks; `marks` lists all, and `marks --done` clears finished
  ones. Marking done is an appended event, not an in-place edit. No tags.
- Questions log: track questions to architects/BAs per project (addressee, date
  asked, open/answered, answer, spec-impact flag), so spec updates can list every
  answered question that affects the spec. Needs a second entity in the store.
- A TUI over the same store.

### Later

- VS Code extension; it only wraps the CLI's `--json` output.

### Out of scope

Teams/Slack/chat integration, cloud sync, accounts.

## Why this exists (keep in mind when making tradeoffs)

The owner built this to reduce a real daily drain: switching between agent work
and reviews. Favor speed and low friction over features. Capturing state must
take seconds; reading it back must take seconds. If a change adds a step to
`brb save` or `brb resume`, it needs a strong reason.

## Stack

- Language: Go. Development is TDD: write the failing test first.
- Interact with git by shelling out to the `git` binary, not a Go git library.
- Interact with GitHub by shelling out to `gh`. Do not handle tokens directly.
- Storage: append-only JSONL, one file per month, under the user's data dir
  (never inside a repo). Filter in memory; expected volume is a few MB a year.
  State must be human-readable and diffable.
- All reads and writes go through a `Store` interface, so the backend can be
  swapped for SQLite (pure-Go `modernc.org/sqlite`, no CGO) if updating records
  in place, such as phase 2 question status, becomes painful. Do not switch
  without asking.
- Standard library first. Add a dependency only with a stated reason.

## Commands

```bash
go build ./...      # build
go test ./...       # run tests
go vet ./...        # static checks
gofmt -l .          # must print nothing
```

## Conventions

- Errors: return them with context (`fmt.Errorf("...: %w", err)`); print a short
  human message to stderr and exit non-zero. No panics in normal paths.
- Wrap all external commands (`git`, `gh`) behind a small interface so they can
  be faked in tests.
- Keep packages small and named for what they do (`snapshot`, `store`, `render`).
- Match surrounding code style. Don't reformat unrelated code.

## Testing

- Unit-test snapshot and render logic with fake git/gh runners.
- Integration tests may create temp git repos; never touch the user's real repos
  or config dir in tests.
- Fixtures and examples use invented data only. No real project names, people,
  or employer content anywhere in the repo.

## Boundaries (ask first)

- Anything that sends data off the machine.
- Adding dependencies, changing the storage format, or swapping the Store backend.
- Deleting or rewriting a user's saved snapshots.
- Never run destructive git commands (`reset --hard`, `clean`, force-push) on a
  user's repo. This tool reads git state; it does not change it.

## Working style

- Prefer small, reviewable changes. Explain the why in commit messages.
- When requirements are unclear, ask the owner rather than guessing.
- The owner makes the decisions; the agent implements. The goal is ownership,
  not hand-writing code. For each issue:
  1. **Propose first.** Before implementing, show the types, function
     signatures and test names, then stop and wait for approval or changes.
  2. **Tests are the spec.** Write the failing tests first and show them. The
     owner reads every test case; add the edge cases they name.
  3. **Name the risk.** In the PR description, list the two riskiest lines or
     assumptions in the change.
  4. **Leave acceptance to the owner.** Say how to try it in a real repo; do not
     claim it works because the tests pass.
- One issue per PR, closing it with `Closes #N`. The owner writes the "What and
  why" in their own words.
- Explain non-obvious choices briefly rather than just dumping code.
- When a decision changes, update this file in the same PR.
