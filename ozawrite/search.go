package ozawrite

import (
	"bytes"
	"encoding/binary"
	"sort"
	"unicode/utf8"

	"github.com/RoaringBitmap/roaring/v2"
)

// TrigramBuilder accumulates trigrams from content entries and serializes them
// into a trigram search section (SEARCH_TITLE or SEARCH_BODY).
//
// Wire format v2:
//
//	Header (16 bytes):
//	  [0:4]   version    uint32 = 2
//	  [4:8]   flags      uint32 (bit 0: CJK bigram mode)
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
//	Posting lists (immediately after trigram table):
//	  per trigram: serialized roaring bitmap (portable format)
//
// When CJK characters are detected in indexed text (flags bit 0 set), CJK runs
// use character-aligned grams instead of the raw byte sliding window:
//   - Each 3-byte CJK character emits its UTF-8 bytes as a unigram key.
//   - Each adjacent pair of CJK characters emits a bigram key:
//     [last_byte_of_c1, first_byte_of_c2, second_byte_of_c2].
//
// This avoids cross-byte-boundary trigrams that split multi-byte CJK characters
// and improves search precision for CJK queries.
type TrigramBuilder struct {
	trigrams      map[[3]byte][]uint32 // trigram -> posting list (entry IDs, insertion order)
	seen          map[[3]byte]int      // epoch of last time this trigram was added for the current entry
	docs          map[uint32]bool      // distinct entry IDs indexed
	epoch         int                  // incremented per IndexEntry call to avoid clearing seen
	lower         []byte               // reused lowercase buffer
	hasCJK        bool                 // true if any CJK characters have been indexed
	totalPostings int                  // count of uint32 entries across all posting lists
}

// isCJKRune reports whether r is in a CJK Unicode block used for Chinese,
// Japanese, or Korean text, including CJK symbols, unified ideographs, Hangul
// syllables, and CJK compatibility ideographs.
func isCJKRune(r rune) bool {
	return (r >= 0x3000 && r <= 0x9FFF) || // CJK Symbols … CJK Unified Ideographs
		(r >= 0xAC00 && r <= 0xD7AF) || // Hangul Syllables
		(r >= 0xF900 && r <= 0xFAFF) // CJK Compatibility Ideographs
}

func newTrigramBuilder() *TrigramBuilder {
	return &TrigramBuilder{
		trigrams: make(map[[3]byte][]uint32),
		seen:     make(map[[3]byte]int),
		docs:     make(map[uint32]bool),
	}
}

// IndexEntry adds all trigrams from text to the index for entryID.
// ASCII bytes are lowercased before extraction; non-ASCII bytes pass through
// unchanged (valid for UTF-8 encoded HTML where the vast majority of tokens
// are ASCII). Each (trigram, entryID) pair is recorded at most once even if
// IndexEntry is called multiple times for the same entryID.
//
// For CJK text the indexer uses character-aligned grams (see TrigramBuilder
// comment) instead of a raw byte sliding window.
func (b *TrigramBuilder) IndexEntry(entryID uint32, text []byte) {
	// Track distinct entry IDs for doc_count.
	b.docs[entryID] = true

	// Advance epoch so seen entries from previous calls are implicitly invalidated.
	b.epoch++

	// Lowercase into reusable buffer (ASCII fast path; avoids bytes.ToLower allocation).
	if cap(b.lower) < len(text) {
		b.lower = make([]byte, len(text))
	}
	lower := b.lower[:len(text)]
	for i, c := range text {
		if c >= 'A' && c <= 'Z' {
			lower[i] = c + ('a' - 'A')
		} else {
			lower[i] = c
		}
	}

	b.emitGrams(entryID, lower)
}

// emitGrams adds grams from text to the index for entryID.
// Non-CJK runs use a 3-byte sliding window. CJK runs use character-aligned
// unigrams and bigrams (see TrigramBuilder comment).
func (b *TrigramBuilder) emitGrams(entryID uint32, text []byte) {
	addGram := func(tri [3]byte) {
		if b.seen[tri] == b.epoch {
			return
		}
		b.seen[tri] = b.epoch
		b.trigrams[tri] = append(b.trigrams[tri], entryID)
		b.totalPostings++
	}

	var prevCJK bool
	var prevLastByte byte // last UTF-8 byte of the previous CJK character
	nonCJKStart := 0
	i := 0

	for i < len(text) {
		r, size := utf8.DecodeRune(text[i:])

		if r != utf8.RuneError && isCJKRune(r) {
			// Flush any preceding non-CJK run as byte trigrams.
			if nonCJKStart < i {
				run := text[nonCJKStart:i]
				for j := 0; j <= len(run)-3; j++ {
					var g [3]byte
					copy(g[:], run[j:j+3])
					addGram(g)
				}
			}

			// Emit unigram: the character's own UTF-8 bytes (3-byte key).
			// 4-byte characters (CJK Extension B+) don't fit; skip their unigram.
			if size == 3 {
				var g [3]byte
				copy(g[:], text[i:i+3])
				addGram(g)
			}

			// Emit bigram with the previous CJK character:
			// [last_byte_of_prev, first_byte_of_curr, second_byte_of_curr].
			if prevCJK && size >= 2 {
				addGram([3]byte{prevLastByte, text[i], text[i+1]})
			}

			b.hasCJK = true
			prevCJK = true
			prevLastByte = text[i+size-1]
			nonCJKStart = i + size
		} else {
			prevCJK = false
		}
		i += size
	}

	// Flush any trailing non-CJK run.
	if nonCJKStart < len(text) {
		run := text[nonCJKStart:]
		for j := 0; j <= len(run)-3; j++ {
			var g [3]byte
			copy(g[:], run[j:j+3])
			addGram(g)
		}
	}
}

// triEntry holds a sorted posting list for one trigram.
type triEntry struct {
	tri [3]byte
	ids []uint32
}

// Build serializes the trigram index into the v2 wire format.
func (b *TrigramBuilder) Build() ([]byte, error) {
	entries := make([]triEntry, 0, len(b.trigrams))
	for tri, ids := range b.trigrams {
		// Sort and deduplicate: the same entryID can appear multiple times if
		// IndexEntry was called for both title and content of the same entry.
		idsCopy := make([]uint32, len(ids))
		copy(idsCopy, ids)
		sort.Slice(idsCopy, func(i, j int) bool { return idsCopy[i] < idsCopy[j] })
		deduped := idsCopy[:1]
		for _, id := range idsCopy[1:] {
			if id != deduped[len(deduped)-1] {
				deduped = append(deduped, id)
			}
		}
		entries = append(entries, triEntry{tri: tri, ids: deduped})
	}
	return b.serializeEntries(entries)
}

// serializeEntries encodes a sorted slice of triEntries into the v2 wire format.
func (b *TrigramBuilder) serializeEntries(entries []triEntry) ([]byte, error) {
	sort.Slice(entries, func(i, j int) bool {
		return bytes.Compare(entries[i].tri[:], entries[j].tri[:]) < 0
	})

	count := uint32(len(entries))
	const headerSize = 16
	const entrySize = 12
	tableSize := int(count) * entrySize

	// Encode posting lists as roaring bitmaps to compute offsets.
	var postingBuf bytes.Buffer
	postingBase := uint32(headerSize + tableSize)
	offsets := make([]uint32, count)
	lengths := make([]uint32, count)

	for i, e := range entries {
		offsets[i] = postingBase + uint32(postingBuf.Len())
		start := postingBuf.Len()
		bm := roaring.New()
		bm.AddMany(e.ids)
		bm.RunOptimize()
		if _, err := bm.WriteTo(&postingBuf); err != nil {
			return nil, err
		}
		lengths[i] = uint32(postingBuf.Len() - start)
	}

	totalSize := headerSize + tableSize + postingBuf.Len()
	buf := make([]byte, totalSize)

	flags := uint32(0)
	if b.hasCJK {
		flags = 1 // bit 0: CJK bigram mode
	}
	binary.LittleEndian.PutUint32(buf[0:4], 3)     // version
	binary.LittleEndian.PutUint32(buf[4:8], flags) // flags
	binary.LittleEndian.PutUint32(buf[8:12], count)                // trigram_count
	binary.LittleEndian.PutUint32(buf[12:16], uint32(len(b.docs))) // doc_count

	for i, e := range entries {
		off := headerSize + i*entrySize
		copy(buf[off:off+3], e.tri[:])
		buf[off+3] = 0 // reserved
		binary.LittleEndian.PutUint32(buf[off+4:off+8], offsets[i])
		binary.LittleEndian.PutUint32(buf[off+8:off+12], lengths[i])
	}

	copy(buf[headerSize+tableSize:], postingBuf.Bytes())
	return buf, nil
}
