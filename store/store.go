// Package store persists snapshots.
package store

import "github.com/breyr/brb/snapshot"

// Store is the only way the rest of brb reads or writes snapshots.
type Store interface {
	// Append saves a snapshot.
	Append(s snapshot.Snapshot) error
	// List returns every snapshot, newest first.
	List() ([]snapshot.Snapshot, error)
}
