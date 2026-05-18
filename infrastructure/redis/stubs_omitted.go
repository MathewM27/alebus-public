// Package-level identifiers that were defined in files omitted from this
// public release but are still referenced by code that ships. Kept minimal so
// the surrounding plumbing compiles; the original runtime semantics are
// intentionally not reproduced here.
package redis

import "strings"

// fieldUpdatedAt is the bus-state hash field name persisted by the (omitted)
// live bus writer. The metrics collector uses it to classify online/offline buses.
const fieldUpdatedAt = "updated_at"

// splitLines splits Redis INFO-style line-delimited output. Used by the backup
// manager when polling persistence state.
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
}
