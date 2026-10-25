package gitstate

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/breyr/brb/runner"
)

var errExit = errors.New("exit status 128")

// replies scripts a healthy repo; tests override single entries.
func replies(overrides map[string]runner.Reply) *runner.FakeRunner {
	m := map[string]runner.Reply{
		"git rev-parse --show-toplevel": {Out: "/src/demo"},
		"git branch --show-current":     {Out: "main"},
		"git rev-parse HEAD":            {Out: "abc123"},
		"git status --short":            {},
		"git diff --stat":               {},
		"git diff --cached --stat":      {},
	}
	for k, v := range overrides {
		m[k] = v
	}

	return &runner.FakeRunner{Replies: m}
}

func TestReadOutsideRepo(t *testing.T) {
	f := replies(map[string]runner.Reply{
		"git rev-parse --show-toplevel": {Err: errExit},
	})

	got, err := Read(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != (State{}) {
		t.Errorf("got %+v, want empty State", got)
	}
}

func TestReadGitNotInstalled(t *testing.T) {
	f := replies(map[string]runner.Reply{
		"git rev-parse --show-toplevel": {Err: exec.ErrNotFound},
	})

	got, err := Read(f)
	if err == nil || !strings.Contains(err.Error(), "git is not installed") {
		t.Fatalf("err = %v, want 'git is not installed'", err)
	}

	if got != (State{}) {
		t.Errorf("got %+v, want empty State", got)
	}
}

func TestReadCleanRepo(t *testing.T) {
	got, err := Read(replies(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := State{Project: "demo", Branch: "main", SHA: "abc123"}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestReadDirtyRepo(t *testing.T) {
	f := replies(map[string]runner.Reply{
		"git status --short": {Out: " M a.go\n?? b.go"},
		"git diff --stat":    {Out: " a.go | 2 +-"},
	})

	got, err := Read(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := " M a.go\n?? b.go\n\n a.go | 2 +-"
	if got.Diff.Summary != want {
		t.Errorf("summary = %q, want %q", got.Diff.Summary, want)
	}
}

func TestReadStagedChanges(t *testing.T) {
	f := replies(map[string]runner.Reply{
		"git status --short":       {Out: "M  a.go"},
		"git diff --cached --stat": {Out: " a.go | 3 ++-"},
	})

	got, err := Read(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "M  a.go\n\n a.go | 3 ++-"
	if got.Diff.Summary != want {
		t.Errorf("summary = %q, want %q", got.Diff.Summary, want)
	}
}

func TestReadDetachedHead(t *testing.T) {
	f := replies(map[string]runner.Reply{
		"git branch --show-current": {Out: ""},
	})

	got, err := Read(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := State{Project: "demo", SHA: "abc123"}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestReadNoCommits(t *testing.T) {
	f := replies(map[string]runner.Reply{
		"git rev-parse HEAD": {Err: errExit},
	})

	got, err := Read(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.SHA != "" || got.Project != "demo" {
		t.Errorf("got %+v, want empty SHA and project demo", got)
	}
}

func TestReadProjectFromWindowsPath(t *testing.T) {
	f := replies(map[string]runner.Reply{
		"git rev-parse --show-toplevel": {Out: "C:/src/demo"},
	})

	got, err := Read(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Project != "demo" {
		t.Errorf("project = %q, want demo", got.Project)
	}
}

func TestReadUnexpectedError(t *testing.T) {
	f := replies(map[string]runner.Reply{
		"git status --short": {Err: errExit},
	})

	if _, err := Read(f); !errors.Is(err, errExit) {
		t.Errorf("err = %v, want it to wrap errExit", err)
	}
}

func TestReadIntegration(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}

	// Never read the developer's real git config.
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)

	dir := filepath.Join(t.TempDir(), "demo")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	git := func(args ...string) {
		t.Helper()

		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(name string) {
		t.Helper()

		if err := os.WriteFile(filepath.Join(dir, name), []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	git("init", "-b", "trunk")
	write("a.txt")
	git("add", "a.txt")
	git("-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-m", "init")
	write("staged.txt")
	git("add", "staged.txt")
	write("untracked.txt")

	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(old) })

	got, err := Read(runner.Exec{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Project != "demo" || got.Branch != "trunk" || len(got.SHA) < 7 {
		t.Errorf("got %+v, want project demo, branch trunk, a SHA", got)
	}

	for _, want := range []string{"staged.txt", "untracked.txt"} {
		if !strings.Contains(got.Diff.Summary, want) {
			t.Errorf("summary %q missing %q", got.Diff.Summary, want)
		}
	}
}
