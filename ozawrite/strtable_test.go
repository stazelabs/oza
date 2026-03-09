package ozawrite

import (
	"testing"
)

func TestTokenizePath(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"", nil},
		{"index.html", []string{"index.html"}},
		{"A/Einstein", []string{"A/", "Einstein"}},
		{"A/B/C", []string{"A/", "B/", "C"}},
		{"A/", []string{"A/", ""}},
	}
	for _, tt := range tests {
		got := tokenizePath(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("tokenizePath(%q) = %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("tokenizePath(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestTokenizeTitle(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"", nil},
		{"Einstein", []string{"Einstein"}},
		{"Albert Einstein", []string{"Albert ", "Einstein"}},
		{"The Theory of Relativity", []string{"The ", "Theory ", "of ", "Relativity"}},
	}
	for _, tt := range tests {
		got := tokenizeTitle(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("tokenizeTitle(%q) = %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("tokenizeTitle(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestStringTableBuilder(t *testing.T) {
	stb := newStringTableBuilder()
	// Add "A/" 5 times, "B/" 2 times, "Einstein" 4 times.
	for i := 0; i < 5; i++ {
		stb.AddTokens([]string{"A/"})
	}
	for i := 0; i < 2; i++ {
		stb.AddTokens([]string{"B/"})
	}
	for i := 0; i < 4; i++ {
		stb.AddTokens([]string{"Einstein"})
	}

	// With minFreq=3, only "A/" (5) and "Einstein" (4) should be in the table.
	table := stb.Build(3)
	if table.Count() != 2 {
		t.Fatalf("table.Count() = %d, want 2", table.Count())
	}

	// "A/" should be first (highest frequency).
	if table.Entry(0) != "A/" {
		t.Errorf("table.Entry(0) = %q, want %q", table.Entry(0), "A/")
	}
	if table.Entry(1) != "Einstein" {
		t.Errorf("table.Entry(1) = %q, want %q", table.Entry(1), "Einstein")
	}

	// Lookup should work.
	if idx, ok := table.Lookup("A/"); !ok || idx != 0 {
		t.Errorf("Lookup(A/) = %d, %v; want 0, true", idx, ok)
	}
	if _, ok := table.Lookup("B/"); ok {
		t.Error("Lookup(B/) should be false (below minFreq)")
	}
}

func TestStringTableSerializeRoundTrip(t *testing.T) {
	stb := newStringTableBuilder()
	tokens := []string{"hello", "world", "foo"}
	for i := 0; i < 5; i++ {
		stb.AddTokens(tokens)
	}
	table := stb.Build(1)

	data := table.Serialize()
	if len(data) == 0 {
		t.Fatal("Serialize returned empty data")
	}

	// Verify we can read it back (using the reader's parseStringTable via manual decode).
	off := 0
	for i := 0; i < table.Count(); i++ {
		if off+2 > len(data) {
			t.Fatalf("entry %d: data truncated", i)
		}
		slen := int(data[off]) | int(data[off+1])<<8
		off += 2
		if off+slen > len(data) {
			t.Fatalf("entry %d: string truncated", i)
		}
		got := string(data[off : off+slen])
		if got != table.Entry(i) {
			t.Errorf("entry %d: got %q, want %q", i, got, table.Entry(i))
		}
		off += slen
	}
}
