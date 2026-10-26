package main

import (
	"bytes"
	"errors"
	"os/exec"
	"testing"
	"time"

	"github.com/breyr/brb/runner"
	"github.com/breyr/brb/snapshot"
)

type memStore struct{ got []snapshot.Snapshot }

func (m *memStore) Append(s snapshot.Snapshot) error   { m.got = append(m.got, s); return nil }
func (m *memStore) List() ([]snapshot.Snapshot, error) { return m.got, nil }

var fixedNow = func() time.Time {
	return time.Date(2026, 1, 2, 3, 4, 5, 0, time.FixedZone("x", 3600))
}

func repoRunner() *runner.FakeRunner {
	return &runner.FakeRunner{Replies: map[string]runner.Reply{
		"git rev-parse --show-toplevel": {Out: "/src/widgets"},
		"git branch --show-current":     {Out: "feat"},
		"git rev-parse HEAD":            {Out: "abc123"},
		"git status --short":            {Out: " M a.go"},
		"git diff --stat":               {},
		"git diff --cached --stat":      {},
	}}
}

func notRepoRunner() *runner.FakeRunner {
	return &runner.FakeRunner{Replies: map[string]runner.Reply{
		"git rev-parse --show-toplevel": {Err: errors.New("not a git repository")},
	}}
}

func doRun(t *testing.T, r runner.Runner, args ...string) (*memStore, string, error) {
	t.Helper()
	st := &memStore{}
	var out bytes.Buffer
	err := run(args, &out, r, st, fixedNow)
	return st, out.String(), err
}

func TestSave_inRepo_usesRepoNameBranchSHADiff(t *testing.T) {
	st, _, err := doRun(t, repoRunner(), "save", "wire up flags")
	if err != nil {
		t.Fatal(err)
	}

	want := snapshot.Snapshot{
		Time:    time.Date(2026, 1, 2, 2, 4, 5, 0, time.UTC),
		Project: "widgets", Branch: "feat", SHA: "abc123",
		Diff: snapshot.Diff{Summary: " M a.go"}, Note: "wire up flags",
	}
	if len(st.got) != 1 || st.got[0] != want {
		t.Fatalf("got %+v, want %+v", st.got, want)
	}
}

func TestSave_outsideRepo_hasOnlyTimeAndNote(t *testing.T) {
	st, out, err := doRun(t, notRepoRunner(), "save", "call Sam")
	if err != nil {
		t.Fatal(err)
	}

	want := snapshot.Snapshot{Time: time.Date(2026, 1, 2, 2, 4, 5, 0, time.UTC), Note: "call Sam"}
	if len(st.got) != 1 || st.got[0] != want {
		t.Fatalf("got %+v, want %+v", st.got, want)
	}

	if out != "saved: \"call Sam\"\n" {
		t.Fatalf("out = %q", out)
	}
}

func TestSave_noProjectFlag_clearsProject(t *testing.T) {
	st, _, err := doRun(t, repoRunner(), "save", "--no-project", "x")
	if err != nil {
		t.Fatal(err)
	}

	if st.got[0].Project != "" || st.got[0].Branch != "feat" {
		t.Fatalf("got %+v", st.got[0])
	}
}

func TestSave_projectFlag_overridesRepoName(t *testing.T) {
	st, _, err := doRun(t, repoRunner(), "save", "--project", "other", "x")
	if err != nil {
		t.Fatal(err)
	}

	if st.got[0].Project != "other" {
		t.Fatalf("got %+v", st.got[0])
	}
}

func TestSave_bothProjectFlags_errors(t *testing.T) {
	st, _, err := doRun(t, repoRunner(), "save", "--no-project", "--project", "o", "x")
	if err == nil || len(st.got) != 0 {
		t.Fatalf("err = %v, stored = %d", err, len(st.got))
	}
}

func TestSave_emptyNote_errors_andStoresNothing(t *testing.T) {
	st, _, err := doRun(t, repoRunner(), "save")
	if err == nil || len(st.got) != 0 {
		t.Fatalf("err = %v, stored = %d", err, len(st.got))
	}
}

func TestSave_multiWordNote_joined(t *testing.T) {
	st, _, err := doRun(t, repoRunner(), "save", "fix", "the", "bug")
	if err != nil {
		t.Fatal(err)
	}

	if st.got[0].Note != "fix the bug" {
		t.Fatalf("note = %q", st.got[0].Note)
	}
}

func TestSave_gitNotInstalled_errors(t *testing.T) {
	r := &runner.FakeRunner{Replies: map[string]runner.Reply{
		"git rev-parse --show-toplevel": {Err: exec.ErrNotFound},
	}}

	st, _, err := doRun(t, r, "save", "x")
	if err == nil || len(st.got) != 0 {
		t.Fatalf("err = %v, stored = %d", err, len(st.got))
	}
}

func TestSave_gitFailureMidRead_errors_andStoresNothing(t *testing.T) {
	r := repoRunner()
	r.Replies["git status --short"] = runner.Reply{Err: errors.New("boom")}

	st, _, err := doRun(t, r, "save", "x")
	if err == nil || len(st.got) != 0 {
		t.Fatalf("err = %v, stored = %d", err, len(st.got))
	}
}

func TestRun_unknownSubcommand_errors(t *testing.T) {
	for _, args := range [][]string{{"nope"}, {}} {
		if _, _, err := doRun(t, repoRunner(), args...); err == nil {
			t.Fatalf("args %v: want error", args)
		}
	}
}

func TestSave_confirmationLine(t *testing.T) {
	_, out, err := doRun(t, repoRunner(), "save", "wire up flags")
	if err != nil {
		t.Fatal(err)
	}

	if want := "saved: widgets (feat) \"wire up flags\"\n"; out != want {
		t.Fatalf("out = %q, want %q", out, want)
	}
}

const inboxCmd = "gh search prs --review-requested=@me --state=open --json number,title,url,repository,author,commentsCount"

func inboxRunner(ghOut string) *runner.FakeRunner {
	return &runner.FakeRunner{Replies: map[string]runner.Reply{
		"git config brb.user": {Err: errors.New("exit status 1")},
		inboxCmd:              {Out: ghOut},
	}}
}

func TestRun_inbox_printsLines(t *testing.T) {
	r := inboxRunner(`[{"author":{"login":"sam"},"commentsCount":3,"number":12,"repository":{"nameWithOwner":"acme/widgets"},"title":"Fix it","url":"https://x/12"}]`)

	_, out, err := doRun(t, r, "inbox")
	if err != nil {
		t.Fatal(err)
	}

	if want := "acme/widgets#12 Fix it (@sam, 3 comments) https://x/12\n"; out != want {
		t.Fatalf("out = %q, want %q", out, want)
	}
}

func TestRun_inbox_empty(t *testing.T) {
	_, out, err := doRun(t, inboxRunner("[]"), "inbox")
	if err != nil {
		t.Fatal(err)
	}

	if want := "no PRs awaiting your review\n"; out != want {
		t.Fatalf("out = %q, want %q", out, want)
	}
}

func useUTC(t *testing.T) {
	t.Helper()

	old := time.Local
	time.Local = time.UTC
	t.Cleanup(func() { time.Local = old })
}

func snaps(ss ...snapshot.Snapshot) *memStore { return &memStore{got: ss} }

var (
	nowUTC = time.Date(2026, 1, 2, 2, 4, 5, 0, time.UTC) // fixedNow in UTC
	snapA  = snapshot.Snapshot{Time: nowUTC.Add(-2 * time.Hour), Project: "widgets", Branch: "feat", Note: "a"}
	snapB  = snapshot.Snapshot{Time: nowUTC.Add(-26 * time.Hour), Project: "other", Branch: "main", Note: "b"}
	snapC  = snapshot.Snapshot{Time: nowUTC.Add(-30 * time.Second), Note: "c"}
)

func runList(t *testing.T, st *memStore, args ...string) (string, error) {
	t.Helper()
	useUTC(t)

	var out bytes.Buffer
	err := run(append([]string{"list"}, args...), &out, repoRunner(), st, fixedNow)

	return out.String(), err
}

func TestList_newestFirst(t *testing.T) {
	out, err := runList(t, snaps(snapA, snapB))
	if err != nil {
		t.Fatal(err)
	}

	want := "2026-01-02 00:04 (2h ago)  widgets (feat)  a\n2026-01-01 00:04 (1d ago)  other (main)  b\n"
	if out != want {
		t.Fatalf("out = %q, want %q", out, want)
	}
}

func TestList_filterByProject(t *testing.T) {
	out, err := runList(t, snaps(snapA, snapB), "-p", "other")
	if err != nil {
		t.Fatal(err)
	}

	if want := "2026-01-01 00:04 (1d ago)  other (main)  b\n"; out != want {
		t.Fatalf("out = %q, want %q", out, want)
	}
}

func TestList_filterNoMatch_printsNone(t *testing.T) {
	out, err := runList(t, snaps(snapA), "-p", "nope")
	if err != nil || out != "no snapshots\n" {
		t.Fatalf("out = %q, err %v", out, err)
	}
}

func TestList_noSnapshots(t *testing.T) {
	out, err := runList(t, snaps())
	if err != nil || out != "no snapshots\n" {
		t.Fatalf("out = %q, err %v", out, err)
	}
}

func TestList_outsideRepoSnapshot_omitsProjectAndBranch(t *testing.T) {
	out, err := runList(t, snaps(snapC))
	if err != nil {
		t.Fatal(err)
	}

	if want := "2026-01-02 02:03 (just now)  c\n"; out != want {
		t.Fatalf("out = %q, want %q", out, want)
	}
}

func TestList_badFlag_errors(t *testing.T) {
	if _, err := runList(t, snaps(snapA), "--nope"); err == nil {
		t.Fatal("want error")
	}
}

func TestAgo(t *testing.T) {
	for d, want := range map[time.Duration]string{
		30 * time.Second: "just now", 5 * time.Minute: "5m ago", 3 * time.Hour: "3h ago",
		48 * time.Hour: "2d ago", -time.Minute: "just now",
	} {
		if got := ago(nowUTC, nowUTC.Add(-d)); got != want {
			t.Errorf("%v: got %q, want %q", d, got, want)
		}
	}
}
