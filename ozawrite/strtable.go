package ozawrite

import (
	"encoding/binary"
	"io"
	"sort"
	"strings"
)

// tokenize splits s into tokens on the given delimiter boundary.
// The delimiter is kept as a suffix on each component except the last.
// Example with '/': "A/B/C" → ["A/", "B/", "C"]
func tokenize(s string, delim byte) []string {
	if s == "" {
		return nil
	}
	var tokens []string
	for {
		i := strings.IndexByte(s, delim)
		if i < 0 {
			tokens = append(tokens, s)
			break
		}
		tokens = append(tokens, s[:i+1])
		s = s[i+1:]
	}
	return tokens
}

func tokenizePath(s string) []string  { return tokenize(s, '/') }
func tokenizeTitle(s string) []string { return tokenize(s, ' ') }

// stringTableBuilder collects token frequencies for building a shared string table.
type stringTableBuilder struct {
	freq map[string]int
}

func newStringTableBuilder() *stringTableBuilder {
	return &stringTableBuilder{freq: make(map[string]int)}
}

// AddTokens increments the frequency count for each token.
func (b *stringTableBuilder) AddTokens(tokens []string) {
	for _, t := range tokens {
		b.freq[t]++
	}
}

// Build returns a finalized string table containing tokens that appear at
// least minFreq times. Tokens are sorted by frequency descending so the most
// common tokens get the lowest indices. The table is capped at 65534 entries
// (0xFFFF is reserved as a sentinel meaning "no table entry").
func (b *stringTableBuilder) Build(minFreq int) *stringTable {
	type entry struct {
		s    string
		freq int
	}
	var entries []entry
	for s, f := range b.freq {
		if f >= minFreq {
			entries = append(entries, entry{s, f})
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].freq != entries[j].freq {
			return entries[i].freq > entries[j].freq
		}
		return entries[i].s < entries[j].s // stable tie-break
	})
	if len(entries) > 65534 {
		entries = entries[:65534]
	}

	st := &stringTable{
		lookup: make(map[string]uint16, len(entries)),
	}
	st.entries = make([]string, len(entries))
	for i, e := range entries {
		st.entries[i] = e.s
		st.lookup[e.s] = uint16(i)
	}
	return st
}

// stringTable is an immutable table of interned strings with O(1) lookup.
type stringTable struct {
	entries []string
	lookup  map[string]uint16
}

const noTableEntry uint16 = 0xFFFF

// Lookup returns the table index for s, or (0xFFFF, false) if not present.
func (t *stringTable) Lookup(s string) (uint16, bool) {
	idx, ok := t.lookup[s]
	return idx, ok
}

// Count returns the number of entries in the table.
func (t *stringTable) Count() int { return len(t.entries) }

// Entry returns the string at the given index.
func (t *stringTable) Entry(i int) string { return t.entries[i] }

// Serialize writes the string table in wire format:
//
//	Per entry:
//	  uint16 len
//	  [len]  string bytes
func (t *stringTable) Serialize() []byte {
	var size int
	for _, s := range t.entries {
		size += 2 + len(s)
	}
	buf := make([]byte, size)
	off := 0
	for _, s := range t.entries {
		binary.LittleEndian.PutUint16(buf[off:], uint16(len(s)))
		off += 2
		copy(buf[off:], s)
		off += len(s)
	}
	return buf
}

// writeTuples encodes tokens as (tableIdx, literalLen, literal) tuples and
// writes them directly to w. Returns the number of tuples written.
func writeTuples(w io.Writer, tokens []string, table *stringTable) int {
	var tmp [4]byte
	for _, tok := range tokens {
		if idx, ok := table.Lookup(tok); ok {
			binary.LittleEndian.PutUint16(tmp[0:2], idx)
			binary.LittleEndian.PutUint16(tmp[2:4], 0)
			w.Write(tmp[:])
		} else {
			binary.LittleEndian.PutUint16(tmp[0:2], noTableEntry)
			binary.LittleEndian.PutUint16(tmp[2:4], uint16(len(tok)))
			w.Write(tmp[:])
			io.WriteString(w, tok)
		}
	}
	return len(tokens)
}
