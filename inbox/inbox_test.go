package inbox

import (
	"errors"
	"os/exec"
	"reflect"
	"testing"

	"github.com/breyr/brb/runner"
)

const ghFlags = " --state=open --json number,title,url,repository,author,commentsCount"

const sample = `[{"author":{"login":"sam"},"commentsCount":3,"number":12,"repository":{"nameWithOwner":"acme/widgets"},"title":"Fix it","url":"https://github.com/acme/widgets/pull/12"}]`

func fake(user runner.Reply, login string, gh runner.Reply) *runner.FakeRunner {
	return &runner.FakeRunner{Replies: map[string]runner.Reply{
		"git config brb.user": user,
		"gh search prs --review-requested=" + login + ghFlags: gh,
	}}
}

func TestReview_configUserSet_usesIt(t *testing.T) {
	r := fake(runner.Reply{Out: "pat"}, "pat", runner.Reply{Out: "[]"})
	if _, err := Review(r); err != nil {
		t.Fatal(err)
	}
}

func TestReview_configUnset_usesAtMe(t *testing.T) {
	r := fake(runner.Reply{Err: errors.New("exit status 1")}, "@me", runner.Reply{Out: "[]"})
	if _, err := Review(r); err != nil {
		t.Fatal(err)
	}
}

func TestReview_parsesPRs(t *testing.T) {
	r := fake(runner.Reply{Err: errors.New("x")}, "@me", runner.Reply{Out: sample})

	got, err := Review(r)
	if err != nil {
		t.Fatal(err)
	}

	want := []PR{{Repo: "acme/widgets", Number: 12, Title: "Fix it", Author: "sam", URL: "https://github.com/acme/widgets/pull/12", Comments: 3}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestReview_empty_returnsNone(t *testing.T) {
	r := fake(runner.Reply{Err: errors.New("x")}, "@me", runner.Reply{Out: "[]"})

	got, err := Review(r)
	if err != nil || len(got) != 0 {
		t.Fatalf("got %+v, err %v", got, err)
	}
}

func TestReview_badJSON_errors(t *testing.T) {
	r := fake(runner.Reply{Err: errors.New("x")}, "@me", runner.Reply{Out: "not json"})
	if _, err := Review(r); err == nil {
		t.Fatal("want error")
	}
}

func TestReview_ghFails_errors(t *testing.T) {
	r := fake(runner.Reply{Err: errors.New("x")}, "@me", runner.Reply{Err: errors.New("auth required")})
	if _, err := Review(r); err == nil {
		t.Fatal("want error")
	}
}

func TestReview_ghNotInstalled_errors(t *testing.T) {
	r := fake(runner.Reply{Err: errors.New("x")}, "@me", runner.Reply{Err: exec.ErrNotFound})
	if _, err := Review(r); err == nil || err.Error() != "gh is not installed" {
		t.Fatalf("err = %v", err)
	}
}
