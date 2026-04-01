package ozawrite

import (
	"testing"
)

func TestTrigramBuilder(t *testing.T) {
	// Small input: should produce valid output.
	tb := newTrigramBuilder()
	tb.IndexEntry(0, []byte("hello world"))
	tb.IndexEntry(1, []byte("world peace"))
	tb.IndexEntry(2, []byte("hello again"))

	got, err := tb.Build(0)
	if err != nil {
		t.Fatal(err)
	}

	if len(got) < 16 {
		t.Fatalf("output too short: %d bytes", len(got))
	}
}

func TestTrigramBuilderMultipleCallsSameID(t *testing.T) {
	// Simulate the Writer pattern: IndexEntry called twice per entry (title+body).
	tb := newTrigramBuilder()
	tb.IndexEntry(0, []byte("hello world"))
	tb.IndexEntry(0, []byte("world peace")) // same ID, different text
	tb.IndexEntry(1, []byte("hello world"))
	tb.IndexEntry(1, []byte("another text"))

	got, err := tb.Build(0)
	if err != nil {
		t.Fatal(err)
	}

	if len(got) < 16 {
		t.Fatalf("output too short: %d bytes", len(got))
	}
}

func TestTrigramBuilderCJKBigram(t *testing.T) {
	// Index CJK text and verify:
	//  1. flags bit 0 is set in the output header.
	//  2. CJK character unigrams and bigrams are present in the table.
	//  3. Non-CJK text still produces byte trigrams.
	tb := newTrigramBuilder()
	// "日本語" (3 CJK chars) + ASCII suffix so we exercise mixed text.
	tb.IndexEntry(0, []byte("日本語 hello"))
	data, err := tb.Build(0)
	if err != nil {
		t.Fatal(err)
	}

	if len(data) < 16 {
		t.Fatalf("output too short: %d bytes", len(data))
	}

	// Check flags bit 0.
	flags := uint32(data[4]) | uint32(data[5])<<8 | uint32(data[6])<<16 | uint32(data[7])<<24
	if flags&1 == 0 {
		t.Errorf("flags bit 0 not set for CJK content (flags=0x%x)", flags)
	}

	// Collect indexed grams.
	count := uint32(data[8]) | uint32(data[9])<<8 | uint32(data[10])<<16 | uint32(data[11])<<24
	indexed := make(map[[3]byte]bool)
	for i := uint32(0); i < count; i++ {
		off := 16 + i*12
		var g [3]byte
		copy(g[:], data[off:off+3])
		indexed[g] = true
	}

	// 日=E6 97 A5, 本=E6 9C AC, 語=E8 AA 9E
	wantGrams := [][3]byte{
		{0xE6, 0x97, 0xA5}, // unigram 日
		{0xE6, 0x9C, 0xAC}, // unigram 本
		{0xE8, 0xAA, 0x9E}, // unigram 語
		{0xA5, 0xE6, 0x9C}, // bigram 日本
		{0xAC, 0xE8, 0xAA}, // bigram 本語
	}
	for _, g := range wantGrams {
		if !indexed[g] {
			t.Errorf("expected gram %02x %02x %02x not found in index", g[0], g[1], g[2])
		}
	}

	// ASCII "hel" trigram must be present.
	hel := [3]byte{'h', 'e', 'l'}
	if !indexed[hel] {
		t.Error("ASCII trigram 'hel' not found in index")
	}
}

func TestTrigramBuilderCJKNoPureASCII(t *testing.T) {
	// Pure ASCII should not set the CJK flag.
	tb := newTrigramBuilder()
	tb.IndexEntry(0, []byte("hello world"))
	data, err := tb.Build(0)
	if err != nil {
		t.Fatal(err)
	}

	flags := uint32(data[4]) | uint32(data[5])<<8 | uint32(data[6])<<16 | uint32(data[7])<<24
	if flags&1 != 0 {
		t.Errorf("flags bit 0 should not be set for ASCII-only content (flags=0x%x)", flags)
	}
}
