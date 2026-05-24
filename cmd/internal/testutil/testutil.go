// Package testutil provides shared test helpers for CLI tool smoke tests.
package testutil

import "testing"

// BuildTestOZA creates a small OZA archive in a temp file and returns its path.
//
// Deprecated: use [BuildTestArchive] instead.
func BuildTestOZA(t *testing.T, search bool) string {
	t.Helper()
	return BuildTestArchive(t, WithSearch(search))
}
