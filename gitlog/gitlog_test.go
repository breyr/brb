package gitlog

import (
	"errors"
	"os/exec"
	"reflect"
	"testing"
	"time"

	"github.com/breyr/brb/runner"
)

const format = "--format=%s%x1f%b%x1e"

var when = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

const since = "--since=2026-01-02T03:04:05Z"

var errGit = errors.New("exit status 1")

func fake(replies map[string]runner.Reply) *runner.FakeRunner {
	if _, ok := replies["git rev-parse --verify --quiet HEAD"]; !ok {
		replies["git rev-parse --verify --quiet HEAD"] = runner.Reply{Out: "head1"}
	}

	return &runner.FakeRunner{Replies: replies}
}

func logOut(entries ...string) string {
	var s string
	for _, e := range entries {
		s += e + "\x1e\n"
	}

	return s
}

func TestSince_shaIsAncestor_usesShaRange(t *testing.T) {
	r := fake(map[string]runner.Reply{
		"git merge-base --is-ancestor abc HEAD": {},
		"git log " + format + " abc..HEAD":      {Out: logOut("one\x1fbody one", "two\x1f")},
	})

	got, err := Since(r, "abc", when)
	if err != nil {
		t.Fatal(err)
	}

	want := []Commit{{"one", "body one"}, {"two", ""}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestSince_shaGone_fallsBackToTimestamp(t *testing.T) {
	r := fake(map[string]runner.Reply{
		"git merge-base --is-ancestor abc HEAD": {Err: errGit},
		"git log " + format + " " + since:       {Out: logOut("x\x1f")},
	})

	got, err := Since(r, "abc", when)
	if err != nil || len(got) != 1 {
		t.Fatalf("got %+v, err %v", got, err)
	}
}

func TestSince_noSHA_usesOriginHEAD(t *testing.T) {
	r := fake(map[string]runner.Reply{
		"git symbolic-ref --short refs/remotes/origin/HEAD": {Out: "origin/trunk"},
		"git log " + format + " origin/trunk..HEAD":         {Out: logOut("x\x1f")},
	})

	got, err := Since(r, "", when)
	if err != nil || len(got) != 1 {
		t.Fatalf("got %+v, err %v", got, err)
	}
}

func TestSince_noSHA_noOrigin_usesMain(t *testing.T) {
	r := fake(map[string]runner.Reply{
		"git symbolic-ref --short refs/remotes/origin/HEAD": {Err: errGit},
		"git rev-parse --verify --quiet main":               {Out: "m1"},
		"git log " + format + " main..HEAD":                 {Out: logOut("x\x1f")},
	})

	got, err := Since(r, "", when)
	if err != nil || len(got) != 1 {
		t.Fatalf("got %+v, err %v", got, err)
	}
}

func TestSince_noSHA_noOrigin_usesMasterWhenNoMain(t *testing.T) {
	r := fake(map[string]runner.Reply{
		"git symbolic-ref --short refs/remotes/origin/HEAD": {Err: errGit},
		"git rev-parse --verify --quiet main":               {Err: errGit},
		"git rev-parse --verify --quiet master":             {Out: "m1"},
		"git log " + format + " master..HEAD":               {Out: logOut("x\x1f")},
	})

	got, err := Since(r, "", when)
	if err != nil || len(got) != 1 {
		t.Fatalf("got %+v, err %v", got, err)
	}
}

func TestSince_noSHA_noBase_fallsBackToTimestamp(t *testing.T) {
	r := fake(map[string]runner.Reply{
		"git symbolic-ref --short refs/remotes/origin/HEAD": {Err: errGit},
		"git rev-parse --verify --quiet main":               {Err: errGit},
		"git rev-parse --verify --quiet master":             {Err: errGit},
		"git log " + format + " " + since:                   {Out: logOut("x\x1f")},
	})

	got, err := Since(r, "", when)
	if err != nil || len(got) != 1 {
		t.Fatalf("got %+v, err %v", got, err)
	}
}

func TestSince_parsesSubjectAndMultiLineBody(t *testing.T) {
	r := fake(map[string]runner.Reply{
		"git merge-base --is-ancestor abc HEAD": {},
		"git log " + format + " abc..HEAD":      {Out: logOut("fix it\x1fline1\n\nline3\n", "next\x1fb")},
	})

	got, err := Since(r, "abc", when)
	if err != nil {
		t.Fatal(err)
	}

	want := []Commit{{"fix it", "line1\n\nline3"}, {"next", "b"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestSince_noCommits_returnsEmpty(t *testing.T) {
	r := fake(map[string]runner.Reply{
		"git merge-base --is-ancestor abc HEAD": {},
		"git log " + format + " abc..HEAD":      {},
	})

	got, err := Since(r, "abc", when)
	if err != nil || len(got) != 0 {
		t.Fatalf("got %+v, err %v", got, err)
	}
}

func TestSince_unbornHEAD_returnsEmpty(t *testing.T) {
	r := fake(map[string]runner.Reply{
		"git rev-parse --verify --quiet HEAD": {Err: errGit},
	})

	got, err := Since(r, "", when)
	if err != nil || len(got) != 0 {
		t.Fatalf("got %+v, err %v", got, err)
	}
}

func TestSince_gitNotInstalled_errors(t *testing.T) {
	r := fake(map[string]runner.Reply{
		"git rev-parse --verify --quiet HEAD": {Err: exec.ErrNotFound},
	})

	if _, err := Since(r, "abc", when); err == nil {
		t.Fatal("want error")
	}
}

func TestSince_logFails_errors(t *testing.T) {
	r := fake(map[string]runner.Reply{
		"git merge-base --is-ancestor abc HEAD": {},
		"git log " + format + " abc..HEAD":      {Err: errGit},
	})

	if _, err := Since(r, "abc", when); err == nil {
		t.Fatal("want error")
	}
}
