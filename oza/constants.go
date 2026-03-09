package oza

const (
	Magic                = 0x01415A4F // "OZA\x01" on disk (little-endian uint32)
	HeaderSize           = 64
	SectionSize          = 80
	EntryTableHeaderSize = 8 // uint32 entry_count + uint32 record_data_offset
	ChunkDescSize        = 28
	RedirectRecordSize   = 5 // 1-byte flags + 4-byte target_id

	MajorVersion = 1
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

// Header flags
const (
	FlagHasSearch     = 1 << 0
	FlagHasChrome     = 1 << 1
	FlagHasSignatures = 1 << 2
)

// SectionType enum
type SectionType uint32

const (
	SectionMetadata    SectionType = 0x0001
	SectionMIMETable   SectionType = 0x0002
	SectionEntryTable  SectionType = 0x0003
	SectionPathIndex   SectionType = 0x0004
	SectionTitleIndex  SectionType = 0x0005
	SectionContent     SectionType = 0x0006
	SectionRedirectTab SectionType = 0x0007
	SectionChrome      SectionType = 0x0009
	SectionSignatures  SectionType = 0x000A
	SectionZstdDict    SectionType = 0x000B
	SectionSearchTitle SectionType = 0x000C // Trigram index of front-article titles only.
	SectionSearchBody  SectionType = 0x000D // Trigram index of front-article body content.
)

// EntryType enum
type EntryType uint8

const (
	EntryContent     EntryType = 0
	EntryRedirect    EntryType = 1
	EntryMetadataRef EntryType = 2
)

// IndexV1Magic is the magic number for the IDX1 index format.
const IndexV1Magic uint32 = 0x49445831 // "IDX1"

// Compression values
const (
	CompNone     = 0
	CompZstd     = 1
	CompZstdDict = 2
)
