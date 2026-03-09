package oza

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sort"
	"unicode/utf8"

	"github.com/RoaringBitmap/roaring/v2"
)

// flagCJKBigram is the flags bit that indicates the index uses character-aligned
// grams for CJK text (see TrigramBuilder in the ozawrite package).
const flagCJKBigram uint32 = 1

// TrigramIndex is a parsed trigram search section (SEARCH_TITLE or SEARCH_BODY).
//
// Wire format (v2):
//
//	Header (16 bytes):
//	  [0:4]   version    uint32 = 2
//	  [4:8]   flags      uint32
//	  [8:12]  count      uint32 (number of distinct trigrams)
//	  [12:16] doc_count  uint32 (number of distinct entry IDs indexed)
//
//	Trigram table (count * 12 bytes, sorted lexicographically):
//	  per entry:
//	    [3]byte  trigram
//	    1 byte   reserved (0)
//	    uint32   posting_offset  (byte offset from start of section)
//	    uint32   posting_len     (byte count of posting list)
//
// Posting lists:
//
//	per trigram: serialized roaring bitmap (portable format)
type TrigramIndex struct {
	data     []byte
	flags    uint32
	count    uint32
	docCount uint32
}

// ParseTrigramIndex parses a SEARCH_TITLE or SEARCH_BODY section (wire format v2).
func ParseTrigramIndex(data []byte) (*TrigramIndex, error) {
	if len(data) < 16 {
		return nil, fmt.Errorf("oza: trigram index section too short")
	}
	version := binary.LittleEndian.Uint32(data[0:4])
	if version != 3 {
		return nil, fmt.Errorf("oza: unsupported trigram index version %d", version)
	}

	flags := binary.LittleEndian.Uint32(data[4:8])
	count := binary.LittleEndian.Uint32(data[8:12])
	docCount := binary.LittleEndian.Uint32(data[12:16])

	need := 16 + int(count)*12
	if len(data) < need {
		return nil, fmt.Errorf("oza: trigram index truncated: need %d bytes for %d trigrams, got %d", need, count, len(data))
	}
	return &TrigramIndex{
		data:     data,
		flags:    flags,
		count:    count,
		docCount: docCount,
	}, nil
}

// DocCount returns the number of distinct entry IDs indexed (v2 only; 0 for v1).
func (idx *TrigramIndex) DocCount() uint32 { return idx.docCount }

// isCJKRune reports whether r is in a CJK Unicode block.
func isCJKRune(r rune) bool {
	return (r >= 0x3000 && r <= 0x9FFF) ||
		(r >= 0xAC00 && r <= 0xD7AF) ||
		(r >= 0xF900 && r <= 0xFAFF)
}

// cjkQueryGrams extracts search grams from a lowercased query when CJK bigram
// mode is active. CJK character runs yield:
//   - a unigram key (the character's 3 UTF-8 bytes) for each character, and
//   - a bigram key [last_of_c1, first_of_c2, second_of_c2] for each adjacent pair.
//
// Non-CJK runs are indexed with the standard 3-byte sliding window.
func cjkQueryGrams(text []byte) [][3]byte {
	seen := make(map[[3]byte]bool)
	var grams [][3]byte
	addGram := func(g [3]byte) {
		if !seen[g] {
			seen[g] = true
			grams = append(grams, g)
		}
	}

	var prevCJK bool
	var prevLastByte byte
	nonCJKStart := 0
	i := 0

	for i < len(text) {
		r, size := utf8.DecodeRune(text[i:])

		if r != utf8.RuneError && isCJKRune(r) {
			// Flush preceding non-CJK run as byte trigrams.
			if nonCJKStart < i {
				run := text[nonCJKStart:i]
				for j := 0; j <= len(run)-3; j++ {
					var g [3]byte
					copy(g[:], run[j:j+3])
					addGram(g)
				}
			}

			// Unigram: the character's own UTF-8 bytes (3-byte key only).
			if size == 3 {
				var g [3]byte
				copy(g[:], text[i:i+3])
				addGram(g)
			}

			// Bigram with previous CJK character.
			if prevCJK && size >= 2 {
				addGram([3]byte{prevLastByte, text[i], text[i+1]})
			}

			prevCJK = true
			prevLastByte = text[i+size-1]
			nonCJKStart = i + size
		} else {
			prevCJK = false
		}
		i += size
	}

	// Flush trailing non-CJK run.
	if nonCJKStart < len(text) {
		run := text[nonCJKStart:]
		for j := 0; j <= len(run)-3; j++ {
			var g [3]byte
			copy(g[:], run[j:j+3])
			addGram(g)
		}
	}

	return grams
}

// Search returns up to limit entry IDs whose indexed text contains all
// trigrams present in query. Results are sorted by entry ID.
//
// Returns nil if no results are found or the query is shorter than 3 bytes.
// In CJK bigram mode (flags bit 0) the query is decomposed into
// character-aligned grams rather than a raw byte sliding window.
func (idx *TrigramIndex) Search(query string, limit int) []uint32 {
	lower := bytes.ToLower([]byte(query))
	if len(lower) < 3 {
		return nil
	}

	// Collect unique grams from the query.
	var trigrams [][3]byte
	if idx.flags&flagCJKBigram != 0 {
		trigrams = cjkQueryGrams(lower)
	} else {
		seen := make(map[[3]byte]bool)
		for i := 0; i <= len(lower)-3; i++ {
			var tri [3]byte
			copy(tri[:], lower[i:i+3])
			if !seen[tri] {
				seen[tri] = true
				trigrams = append(trigrams, tri)
			}
		}
	}
	if len(trigrams) == 0 {
		return nil
	}

	// Decode posting bitmap for each trigram.
	bitmaps := make([]*roaring.Bitmap, 0, len(trigrams))
	for _, tri := range trigrams {
		bm := idx.lookup(tri)
		if bm == nil {
			return nil // missing trigram means no results
		}
		bitmaps = append(bitmaps, bm)
	}

	// Sort by cardinality; intersect smallest first.
	sort.Slice(bitmaps, func(i, j int) bool { return bitmaps[i].GetCardinality() < bitmaps[j].GetCardinality() })

	result := bitmaps[0]
	for _, bm := range bitmaps[1:] {
		result.And(bm)
		if result.IsEmpty() {
			return nil
		}
	}

	if limit > 0 && int(result.GetCardinality()) > limit {
		ids := make([]uint32, 0, limit)
		it := result.Iterator()
		for it.HasNext() && len(ids) < limit {
			ids = append(ids, it.Next())
		}
		return ids
	}
	return result.ToArray()
}

// lookup binary-searches for tri in the trigram table and decodes its posting
// bitmap. Returns nil if the trigram is not present.
func (idx *TrigramIndex) lookup(tri [3]byte) *roaring.Bitmap {
	n := int(idx.count)
	const tableOff = 16  // header size
	const entrySize = 12 // 3 trigram + 1 reserved + 4 offset + 4 length

	pos := sort.Search(n, func(i int) bool {
		off := tableOff + i*entrySize
		return bytes.Compare(idx.data[off:off+3], tri[:]) >= 0
	})
	if pos >= n {
		return nil
	}
	off := tableOff + pos*entrySize
	if !bytes.Equal(idx.data[off:off+3], tri[:]) {
		return nil
	}

	postingOff := binary.LittleEndian.Uint32(idx.data[off+4 : off+8])
	postingLen := binary.LittleEndian.Uint32(idx.data[off+8 : off+12])
	end := int(postingOff) + int(postingLen)
	if end > len(idx.data) {
		return nil
	}
	bm := roaring.New()
	if _, err := bm.ReadFrom(bytes.NewReader(idx.data[postingOff:end])); err != nil {
		return nil
	}
	return bm
}

