package oza

import "fmt"

const (
	// Magic is the OZA file magic number: "OZA\x01" stored as a little-endian uint32.
	Magic = 0x01415A4F
	// HeaderSize is the fixed size of the OZA file header in bytes.
	HeaderSize = 128
	// SectionSize is the fixed size of each section descriptor in the section table.
	SectionSize = 80
	// EntryTableHeaderSize is the header of the entry table section (count + record_data_offset).
	EntryTableHeaderSize = 8
	// ChunkDescSize is the fixed size of each chunk descriptor in the content section.
	ChunkDescSize = 28
	// RedirectRecordSize is the fixed size of each redirect record (flags + target_id).
	RedirectRecordSize = 5

	// MajorVersion is the current OZA format major version.
	MajorVersion = 1
	// MinorVersion is the current OZA format minor version.
	MinorVersion = 0
)

// RedirectIDBit is set in entry IDs that refer to the redirect table
// rather than the entry table. Bits 0-30 are the redirect record index.
//
// This is a permanent v1 format constraint: the high bit of every uint32 ID
// is reserved as a type tag. Consequently:
//   - Content entries are limited to 2,147,483,647 (MaxContentEntries).
//   - Redirect entries are limited to 2,147,483,647 (MaxRedirectEntries).
//
// Both limits exceed any foreseeable archive size (Wikipedia ~6 M articles as
// of 2026), but implementations must enforce them and reject archives that
// would exceed them.
const RedirectIDBit uint32 = 1 << 31

// MaxContentEntries is the maximum number of content entries an OZA archive
// may contain. Entry IDs are uint32 with bit 31 reserved as a redirect tag.
const MaxContentEntries = int(RedirectIDBit) - 1 // 2,147,483,647

// MaxRedirectEntries is the maximum number of redirect entries an OZA archive
// may contain. Redirect IDs use bits 0-30 as the record index.
const MaxRedirectEntries = int(RedirectIDBit) - 1 // 2,147,483,647

// IsRedirectID reports whether id refers to a redirect record.
func IsRedirectID(id uint32) bool { return id&RedirectIDBit != 0 }

// RedirectIndex extracts the redirect record index from a tagged redirect ID.
func RedirectIndex(id uint32) uint32 { return id &^ RedirectIDBit }

// MakeRedirectID constructs a tagged redirect ID from a redirect record index.
func MakeRedirectID(index uint32) uint32 { return index | RedirectIDBit }

// Header flag bits stored in the flags field of the file header.
const (
	// FlagHasSearch indicates the archive contains a trigram search section.
	FlagHasSearch = 1 << 0
	// FlagHasChrome indicates the archive contains a chrome/UI assets section.
	FlagHasChrome = 1 << 1
	// FlagHasSignatures indicates the archive contains Ed25519 signatures.
	FlagHasSignatures = 1 << 2
)

// SectionType identifies a section in the section table.
type SectionType uint32

const (
	// SectionMetadata is the structured key-value metadata section.
	SectionMetadata SectionType = 0x0001
	// SectionMIMETable is the deduplicated MIME type string table.
	SectionMIMETable SectionType = 0x0002
	// SectionEntryTable is the variable-length content entry record section.
	SectionEntryTable SectionType = 0x0003
	// SectionPathIndex is the front-coded path lookup index (IDX1 format).
	SectionPathIndex SectionType = 0x0004
	// SectionTitleIndex is the front-coded title lookup index (IDX1 format).
	SectionTitleIndex SectionType = 0x0005
	// SectionContent is the compressed content chunks section.
	SectionContent SectionType = 0x0006
	// SectionRedirectTab is the compact redirect record table.
	SectionRedirectTab SectionType = 0x0007
	// 0x0008 is reserved and intentionally unused.
	// SectionChrome is the optional UI/navigation assets section.
	SectionChrome SectionType = 0x0009
	// SectionSignatures is the optional Ed25519 signature section.
	SectionSignatures SectionType = 0x000A
	// SectionZstdDict is a shared Zstd compression dictionary section.
	SectionZstdDict SectionType = 0x000B
	// SectionSearchTitle is the trigram index of front-article titles.
	SectionSearchTitle SectionType = 0x000C
	// SectionSearchBody is the trigram index of front-article body content.
	SectionSearchBody SectionType = 0x000D
)

// EntryType identifies the kind of entry in an entry record.
type EntryType uint8

const (
	// EntryContent is a regular content entry with blob data.
	EntryContent EntryType = 0
	// EntryRedirect is a redirect entry pointing to another entry.
	EntryRedirect EntryType = 1
	// EntryMetadataRef is a metadata reference entry.
	EntryMetadataRef EntryType = 2
)

// IndexV1Magic is the magic number for the IDX1 index format.
const IndexV1Magic uint32 = 0x49445831 // "IDX1"

// Compression type values stored in section descriptors and chunk descriptors.
const (
	// CompNone indicates no compression (stored raw).
	CompNone = 0
	// CompZstd indicates standard Zstd compression.
	CompZstd = 1
	// CompZstdDict indicates Zstd compression with a shared dictionary.
	CompZstdDict = 2
	// CompBrotli indicates Brotli compression.
	CompBrotli = 3
)

// String returns a human-readable name for a SectionType.
func (t SectionType) String() string {
	switch t {
	case SectionMetadata:
		return "METADATA"
	case SectionMIMETable:
		return "MIME_TABLE"
	case SectionEntryTable:
		return "ENTRY_TABLE"
	case SectionPathIndex:
		return "PATH_INDEX"
	case SectionTitleIndex:
		return "TITLE_INDEX"
	case SectionContent:
		return "CONTENT"
	case SectionRedirectTab:
		return "REDIRECT_TABLE"
	case SectionChrome:
		return "CHROME"
	case SectionSignatures:
		return "SIGNATURES"
	case SectionZstdDict:
		return "ZSTD_DICT"
	case SectionSearchTitle:
		return "SEARCH_TITLE"
	case SectionSearchBody:
		return "SEARCH_BODY"
	default:
		return fmt.Sprintf("0x%04x", uint32(t))
	}
}

// CompressionName returns a human-readable name for a compression type value.
func CompressionName(c uint8) string {
	switch c {
	case CompNone:
		return "none"
	case CompZstd:
		return "zstd"
	case CompZstdDict:
		return "zstd+dict"
	case CompBrotli:
		return "brotli"
	default:
		return fmt.Sprintf("0x%02x", c)
	}
}
