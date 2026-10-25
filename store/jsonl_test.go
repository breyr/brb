package store

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/breyr/brb/snapshot"
)

func at(y int, m time.Month, d, h, min int) time.Time {
	return time.Date(y, m, d, h, min, 0, 0, time.UTC)
}

func snap(t time.Time, note string) snapshot.Snapshot {
	return snapshot.Snapshot{Time: t, Note: note}
}

func newStore(t *testing.T) (*JSONL, string) {
	t.Helper()
	dir := t.TempDir()
	return NewJSONL(dir), dir
}

func mustAppend(t *testing.T, s Store, snaps ...snapshot.Snapshot) {
	t.Helper()
	for _, sn := range snaps {
		if err := s.Append(sn); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}
}

func notes(snaps []snapshot.Snapshot) []string {
	var out []string
	for _, s := range snaps {
		out = append(out, s.Note)
	}
	return out
}

func TestSavedSnapshotComesBackUnchanged(t *testing.T) {
	// Arrange
	st, _ := newStore(t)
	inRepo := snapshot.Snapshot{
		Time:    at(2026, time.October, 5, 9, 30),
		Project: "demo",
		Branch:  "feature/x",
		SHA:     "0f9d81e0f9d81e0f9d81e0f9d81e0f9d81e0f9d8",
		Diff:    snapshot.Diff{Summary: "2 files changed"},
		Note:    "wire up the store",
	}
	// Outside a repo: no project, branch, SHA or diff.
	outside := snapshot.Snapshot{Time: at(2026, time.October, 6, 9, 30), Note: "call the vendor"}
	mustAppend(t, st, inRepo, outside)

	// Act
	got, err := st.List()

	// Assert
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	want := []snapshot.Snapshot{outside, inRepo}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip mismatch\n got: %+v\nwant: %+v", got, want)
	}
}

func TestListReturnsNewestSnapshotFirst(t *testing.T) {
	// Arrange
	st, _ := newStore(t)
	mustAppend(t, st,
		snap(at(2026, time.October, 2, 0, 0), "middle"),
		snap(at(2026, time.October, 3, 0, 0), "newest"),
		snap(at(2026, time.October, 1, 0, 0), "oldest"),
	)

	// Act
	got, err := st.List()

	// Assert
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	want := []string{"newest", "middle", "oldest"}
	if !reflect.DeepEqual(notes(got), want) {
		t.Fatalf("got %v, want %v", notes(got), want)
	}
}

func TestListOrdersCorrectlyAcrossTwoMonths(t *testing.T) {
	// Arrange
	st, dir := newStore(t)
	mustAppend(t, st,
		snap(at(2026, time.September, 30, 23, 59), "sept"),
		snap(at(2026, time.October, 1, 0, 1), "oct"),
	)

	// Act
	got, err := st.List()

	// Assert
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	for _, name := range []string{"2026-09.jsonl", "2026-10.jsonl"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("expected month file %s: %v", name, err)
		}
	}
	want := []string{"oct", "sept"}
	if !reflect.DeepEqual(notes(got), want) {
		t.Fatalf("got %v, want %v", notes(got), want)
	}
}

func TestMonthFileIsChosenInUTCNotLocalTime(t *testing.T) {
	// Arrange
	st, dir := newStore(t)
	// 2026-10-01 02:00 at +05:00 is 2026-09-30 21:00 UTC.
	local := time.Date(2026, time.October, 1, 2, 0, 0, 0, time.FixedZone("plus5", 5*3600))

	// Act
	mustAppend(t, st, snap(local, "x"))
	got, err := st.List()

	// Assert
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "2026-09.jsonl")); err != nil {
		t.Errorf("expected snapshot in the UTC month file 2026-09.jsonl: %v", err)
	}

	if len(got) != 1 || !got[0].Time.Equal(local) || got[0].Time.Location() != time.UTC {
		t.Fatalf("want one snapshot at the same instant in UTC, got %+v", got)
	}
}

func TestListPutsLaterSaveFirstWhenTimesAreEqual(t *testing.T) {
	// Arrange
	st, _ := newStore(t)
	ts := at(2026, time.October, 5, 9, 0)
	mustAppend(t, st, snap(ts, "first"), snap(ts, "second"))

	// Act
	got, err := st.List()

	// Assert
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	want := []string{"second", "first"}
	if !reflect.DeepEqual(notes(got), want) {
		t.Fatalf("got %v, want %v", notes(got), want)
	}
}

func TestListWithNoDataDirReturnsNothing(t *testing.T) {
	// Arrange
	st := NewJSONL(filepath.Join(t.TempDir(), "does-not-exist"))

	// Act
	got, err := st.List()

	// Assert
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("want empty, got %+v", got)
	}
}

func TestListWithEmptyDataDirReturnsNothing(t *testing.T) {
	// Arrange
	st, _ := newStore(t)

	// Act
	got, err := st.List()

	// Assert
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("want empty, got %+v", got)
	}
}

func TestAppendCreatesMissingDataDir(t *testing.T) {
	// Arrange
	dir := filepath.Join(t.TempDir(), "a", "b")
	st := NewJSONL(dir)

	// Act
	err := st.Append(snap(at(2026, time.October, 5, 9, 0), "x"))

	// Assert
	if err != nil {
		t.Fatalf("Append: %v", err)
	}

	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("data dir was not created: %v", err)
	}
}

func TestAppendMakesDirAndFileOwnerOnly(t *testing.T) {
	// Arrange
	if runtime.GOOS == "windows" {
		t.Skip("Go ignores Unix mode bits on Windows; access is by ACL")
	}

	dir := filepath.Join(t.TempDir(), "brb")
	st := NewJSONL(dir)

	// Act
	mustAppend(t, st, snap(at(2026, time.October, 5, 9, 0), "x"))

	// Assert
	di, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}

	if di.Mode().Perm() != 0o700 {
		t.Errorf("dir mode = %o, want 700", di.Mode().Perm())
	}

	fi, err := os.Stat(filepath.Join(dir, "2026-10.jsonl"))
	if err != nil {
		t.Fatal(err)
	}

	if fi.Mode().Perm() != 0o600 {
		t.Errorf("file mode = %o, want 600", fi.Mode().Perm())
	}
}

func TestListFailsOnCorruptLineAndNamesFileAndLine(t *testing.T) {
	// Arrange
	st, dir := newStore(t)
	good := `{"time":"2026-10-05T09:00:00Z","branch":"","sha":"","diff":{"summary":""},"note":"ok"}`
	body := good + "\n" + "{not json\n"
	if err := os.WriteFile(filepath.Join(dir, "2026-10.jsonl"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	// Act
	_, err := st.List()

	// Assert
	if err == nil {
		t.Fatal("want error for corrupt line, got nil")
	}

	if !strings.Contains(err.Error(), "2026-10.jsonl") || !strings.Contains(err.Error(), ":2") {
		t.Fatalf("error should name file and line 2, got: %v", err)
	}
}

func TestListSkipsFilesThatAreNotJSONL(t *testing.T) {
	// Arrange
	st, dir := newStore(t)
	mustAppend(t, st, snap(at(2026, time.October, 5, 9, 0), "x"))
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Act
	got, err := st.List()

	// Assert
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("want 1 snapshot, got %d", len(got))
	}
}

func TestListReadsWindowsLineEndings(t *testing.T) {
	// Arrange
	st, dir := newStore(t)
	l1 := `{"time":"2026-10-05T09:00:00Z","branch":"","sha":"","diff":{"summary":""},"note":"one"}`
	l2 := `{"time":"2026-10-06T09:00:00Z","branch":"","sha":"","diff":{"summary":""},"note":"two"}`
	if err := os.WriteFile(filepath.Join(dir, "2026-10.jsonl"), []byte(l1+"\r\n"+l2+"\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Act
	got, err := st.List()

	// Assert
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	want := []string{"two", "one"}
	if !reflect.DeepEqual(notes(got), want) {
		t.Fatalf("got %v, want %v", notes(got), want)
	}
}

func TestVeryLargeSnapshotStillRoundTrips(t *testing.T) {
	// Arrange
	st, _ := newStore(t)
	big := snapshot.Snapshot{
		Time: at(2026, time.October, 5, 9, 0),
		Diff: snapshot.Diff{Summary: strings.Repeat("x", 1<<20)},
	}
	mustAppend(t, st, big)

	// Act
	got, err := st.List()

	// Assert
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(got) != 1 || got[0].Diff.Summary != big.Diff.Summary {
		t.Fatal("large snapshot did not round trip")
	}
}

func TestDataDirUsesEnvVarWhenSet(t *testing.T) {
	// Arrange
	want := t.TempDir()
	t.Setenv("BRB_DATA_DIR", want)

	// Act
	got, err := DataDir()

	// Assert
	if err != nil {
		t.Fatal(err)
	}

	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestDataDirUsesOSDefaultWhenEnvVarUnsetOrEmpty(t *testing.T) {
	// Arrange
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("LOCALAPPDATA", filepath.Join(home, "AppData", "Local"))
	t.Setenv("BRB_DATA_DIR", "") // set-but-empty must behave like unset
	want := filepath.Join(home, ".local", "share", "brb")
	if runtime.GOOS == "windows" {
		want = filepath.Join(home, "AppData", "Local", "brb")
	}

	// Act
	got, err := DataDir()

	// Assert
	if err != nil {
		t.Fatal(err)
	}

	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
