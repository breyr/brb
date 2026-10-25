// Package snapshot defines the record brb saves for a thread of work.
package snapshot

import "time"

// Diff describes uncommitted changes. Summary is the only field for now; a
// struct leaves room to add fields without changing the Snapshot shape.
type Diff struct {
	Summary string `json:"summary"`
}

// Snapshot is one saved thread of work. Project is optional, and Branch, SHA
// and Diff are empty outside a git repo. Time is always UTC.
type Snapshot struct {
	Time    time.Time `json:"time"`
	Project string    `json:"project,omitempty"`
	Branch  string    `json:"branch"`
	SHA     string    `json:"sha"`
	Diff    Diff      `json:"diff"`
	Note    string    `json:"note"`
}
