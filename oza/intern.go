package oza

import "strings"

// stringInterner deduplicates identical strings so that multiple map entries
// holding the same path or title share a single backing allocation.
type stringInterner struct {
	m map[string]string
}

func newStringInterner() *stringInterner {
	return &stringInterner{m: make(map[string]string)}
}

// Intern returns a canonical copy of s. If s has been seen before, the
// previously stored string is returned (same pointer, no new allocation).
// Otherwise a detached copy is made (via strings.Clone) to avoid pinning
// larger backing arrays such as mmap'd section data.
func (si *stringInterner) Intern(s string) string {
	if existing, ok := si.m[s]; ok {
		return existing
	}
	owned := strings.Clone(s)
	si.m[owned] = owned
	return owned
}
