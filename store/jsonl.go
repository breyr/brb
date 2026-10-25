package store

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/breyr/brb/snapshot"
)

// JSONL stores snapshots as append-only JSON lines, one file per UTC month
// (e.g. 2026-10.jsonl). Files are private to the user (0700 dir, 0600 files).
type JSONL struct {
	dir string
}

var _ Store = (*JSONL)(nil)

func NewJSONL(dir string) *JSONL { return &JSONL{dir: dir} }

// DataDir returns $BRB_DATA_DIR if set and non-empty, else the per-OS default:
// ~/.local/share/brb, or %LocalAppData%\brb on Windows.
func DataDir() (string, error) {
	if d := os.Getenv("BRB_DATA_DIR"); d != "" {
		return d, nil
	}

	if runtime.GOOS == "windows" {
		base := os.Getenv("LOCALAPPDATA")
		if base == "" {
			return "", errors.New("find data dir: %LocalAppData% is not set")
		}

		return filepath.Join(base, "brb"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find data dir: %w", err)
	}

	return filepath.Join(home, ".local", "share", "brb"), nil
}

func (j *JSONL) Append(s snapshot.Snapshot) error {
	s.Time = s.Time.UTC()
	line, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("encode snapshot: %w", err)
	}

	line = append(line, '\n')

	if err := os.MkdirAll(j.dir, 0o700); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	path := filepath.Join(j.dir, s.Time.Format("2006-01")+".jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}

	// One Write call per snapshot so a line is never split.
	// shortcut: no locking, so parallel writers are untested; move to the SQLite Store if that matters.
	if _, err := f.Write(line); err != nil {
		f.Close()
		return fmt.Errorf("write %s: %w", path, err)
	}

	if err := f.Close(); err != nil {
		return fmt.Errorf("close %s: %w", path, err)
	}

	return nil
}

func (j *JSONL) List() ([]snapshot.Snapshot, error) {
	entries, err := os.ReadDir(j.dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("read data dir: %w", err)
	}

	// ReadDir sorts by name, so this is oldest month first, oldest append first.
	var all []snapshot.Snapshot
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}

		snaps, err := readFile(filepath.Join(j.dir, e.Name()))
		if err != nil {
			return nil, err
		}

		all = append(all, snaps...)
	}

	// Reverse so later appends come first, then a stable sort keeps that order
	// for equal timestamps.
	for i, k := 0, len(all)-1; i < k; i, k = i+1, k-1 {
		all[i], all[k] = all[k], all[i]
	}
	sort.SliceStable(all, func(a, b int) bool { return all[a].Time.After(all[b].Time) })
	return all, nil
}

// readFile reads one month file. bufio.Reader has no line-length limit,
// unlike bufio.Scanner's default 64KB.
func readFile(path string) ([]snapshot.Snapshot, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}

	defer f.Close()

	var out []snapshot.Snapshot
	r := bufio.NewReader(f)
	for n := 1; ; n++ {
		raw, err := r.ReadBytes('\n')
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}

		// TrimSpace also drops the \r of Windows line endings.
		if line := bytes.TrimSpace(raw); len(line) > 0 {
			var s snapshot.Snapshot
			if jerr := json.Unmarshal(line, &s); jerr != nil {
				return nil, fmt.Errorf("%s:%d: %w", path, n, jerr)
			}

			out = append(out, s)
		}

		if err == io.EOF {
			return out, nil
		}
	}
}
