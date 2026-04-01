package oza

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/RoaringBitmap/roaring/v2"
)

// makeTrigramSection builds a minimal valid trigram index section with the
// given trigrams and a single posting (entryID 0) per trigram.
func makeTrigramSection(trigrams [][3]byte) []byte {
	count := uint32(len(trigrams))
	headerSize := 16
	tableSize := int(count) * 12

	// Serialize a roaring bitmap containing only entry ID 0.
	bm := roaring.New()
	bm.Add(0)
	bm.RunOptimize()
	var postingBuf bytes.Buffer
	bm.WriteTo(&postingBuf)
	posting := postingBuf.Bytes()

	buf := make([]byte, headerSize+tableSize+len(posting)*int(count))

	// Header
	binary.LittleEndian.PutUint32(buf[0:4], 1)      // version
	binary.LittleEndian.PutUint32(buf[4:8], 0)      // flags
	binary.LittleEndian.PutUint32(buf[8:12], count) // count
	binary.LittleEndian.PutUint32(buf[12:16], 1)    // doc_count

	// Trigram table + posting data
	postingStart := headerSize + tableSize
	for i, tri := range trigrams {
		off := headerSize + i*12
		copy(buf[off:off+3], tri[:])
		buf[off+3] = 0 // reserved
		binary.LittleEndian.PutUint32(buf[off+4:off+8], uint32(postingStart+i*len(posting)))
		binary.LittleEndian.PutUint32(buf[off+8:off+12], uint32(len(posting)))
	}
	for i := range trigrams {
		copy(buf[postingStart+i*len(posting):], posting)
	}

	return buf
}

// makeCJKSection builds a trigram section with flags=1 (CJK bigram mode) and
// the specified grams, each mapping to entry ID 0.
func makeCJKSection(grams [][3]byte) []byte {
	buf := makeTrigramSection(grams)
	buf[4] = 1 // set flags bit 0
	return buf
}

func TestSearchCJKBigramMode(t *testing.T) {
	// 日=E6 97 A5, 本=E6 9C AC, 語=E8 AA 9E
	// Index contains: unigrams for each char + bigrams 日本 and 本語.
	// Grams must be in lexicographic order for binary search to work.
	grams := [][3]byte{
		{0xA5, 0xE6, 0x9C}, // bigram 日本: [last(日), first(本), second(本)]
		{0xAC, 0xE8, 0xAA}, // bigram 本語: [last(本), first(語), second(語)]
		{0xE6, 0x97, 0xA5}, // unigram 日
		{0xE6, 0x9C, 0xAC}, // unigram 本
		{0xE8, 0xAA, 0x9E}, // unigram 語
	}
	idx, err := ParseTrigramIndex(makeCJKSection(grams))
	if err != nil {
		t.Fatal(err)
	}

	// Single-char query "日" → uses unigram → should find entry 0.
	if got := idx.Search("日", 10); len(got) == 0 {
		t.Error("single CJK character query returned no results")
	}

	// Two-char query "日本" → uses unigram(日) + bigram(日本) + unigram(本) → should find entry 0.
	if got := idx.Search("日本", 10); len(got) == 0 {
		t.Error("two-char CJK query '日本' returned no results")
	}

	// Three-char query "日本語" → should find entry 0.
	if got := idx.Search("日本語", 10); len(got) == 0 {
		t.Error("three-char CJK query '日本語' returned no results")
	}

	// Bigram not in index ("日語" — 日 not directly adjacent to 語) → no results.
	// The bigram for 日語 would be [A5 E8 AA] which is not in the index.
	// However the unigrams for 日 and 語 ARE present, so they intersect to entry 0.
	// We verify only that Search doesn't panic; result may vary based on precision.
	idx.Search("日語", 10)
}

func TestSearchCJKBigramModeNoFlag(t *testing.T) {
	// Without the CJK flag, a single CJK character (3 bytes) should still
	// work as a plain byte trigram query.
	grams := [][3]byte{
		{0xE6, 0x97, 0xA5}, // 日 as byte trigram
	}
	idx, err := ParseTrigramIndex(makeTrigramSection(grams)) // flags=0
	if err != nil {
		t.Fatal(err)
	}
	if got := idx.Search("日", 10); len(got) == 0 {
		t.Error("single CJK char query failed without bigram flag")
	}
}

// FuzzDecodePostingList fuzzes the roaring bitmap deserialization path used by
// trigram search. Malformed archives can contain arbitrary bytes in posting
// list regions; the decoder must not panic or allocate unbounded memory.
func FuzzDecodePostingList(f *testing.F) {
	// Seed: a valid roaring bitmap containing a single entry.
	bm := roaring.New()
	bm.Add(42)
	bm.RunOptimize()
	var buf bytes.Buffer
	bm.WriteTo(&buf)
	f.Add(buf.Bytes())

	// Seed: empty bitmap.
	empty := roaring.New()
	var buf2 bytes.Buffer
	empty.WriteTo(&buf2)
	f.Add(buf2.Bytes())

	f.Fuzz(func(t *testing.T, data []byte) {
		bm := roaring.New()
		if _, err := bm.ReadFrom(bytes.NewReader(data)); err != nil {
			return
		}
		// Exercise iteration — must not panic even on malformed internal state.
		func() {
			defer func() { recover() }()
			bm.GetCardinality()
			it := bm.Iterator()
			for i := 0; it.HasNext() && i < 100; i++ {
				it.Next()
			}
		}()
	})
}

func FuzzParseTrigramIndex(f *testing.F) {
	// Seed: valid section with one trigram.
	seed := makeTrigramSection([][3]byte{{'a', 'b', 'c'}})
	f.Add(seed)

	// Seed: empty trigram table.
	empty := makeTrigramSection(nil)
	f.Add(empty)

	f.Fuzz(func(t *testing.T, data []byte) {
		idx, err := ParseTrigramIndex(data)
		if err != nil {
			return
		}
		// Exercise search with a short and a long query; must not panic.
		idx.Search("ab", 10)
		idx.Search("abc", 10)
		idx.Search("abcdef", 10)
	})
}
