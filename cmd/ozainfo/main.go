package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/stazelabs/oza/oza"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "Usage: ozainfo <archive.oza>\n")
		os.Exit(1)
	}
	if err := run(os.Args[1]); err != nil {
		fmt.Fprintf(os.Stderr, "ozainfo: %v\n", err)
		os.Exit(1)
	}
}

func run(path string) error {
	a, err := oza.Open(path)
	if err != nil {
		return err
	}
	defer a.Close()

	hdr := a.FileHeader()

	fmt.Printf("File:          %s\n", path)
	fmt.Printf("Magic:         0x%08X\n", hdr.Magic)
	fmt.Printf("Version:       %d.%d\n", hdr.MajorVersion, hdr.MinorVersion)
	fmt.Printf("UUID:          %s\n", formatUUID(hdr.UUID))
	fmt.Printf("Sections:      %d\n", hdr.SectionCount)
	fmt.Printf("Entries:       %d\n", hdr.EntryCount)
	fmt.Printf("Redirects:     %d\n", a.RedirectCount())
	fmt.Printf("Content size:  %d bytes\n", hdr.ContentSize)
	fmt.Printf("Flags:         0x%08X%s\n", hdr.Flags, formatFlags(hdr))
	fmt.Println()

	// Section table.
	sections := a.Sections()
	fmt.Printf("Section Table (%d sections):\n", len(sections))
	fmt.Printf("  %-3s  %-15s  %-12s  %-12s  %-12s  %-10s  %s\n",
		"#", "Type", "Offset", "Comp.Size", "Uncomp.Size", "Compr.", "SHA-256")
	for i, s := range sections {
		sha := fmt.Sprintf("%x", s.SHA256)
		if len(sha) > 16 {
			sha = sha[:16] + "..."
		}
		fmt.Printf("  %-3d  %-15s  0x%010x  %-12d  %-12d  %-10s  %s\n",
			i, sectionTypeName(s.Type), s.Offset,
			s.CompressedSize, s.UncompressedSize,
			compressionName(s.Compression), sha)
	}
	fmt.Println()

	// MIME types.
	mimeTypes := a.MIMETypes()
	fmt.Printf("MIME Types (%d):\n", len(mimeTypes))
	for i, mt := range mimeTypes {
		fmt.Printf("  %3d  %s\n", i, mt)
	}
	fmt.Println()

	// Metadata.
	meta := a.AllMetadata()
	keys := make([]string, 0, len(meta))
	for k := range meta {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	fmt.Printf("Metadata (%d keys):\n", len(keys))
	for _, k := range keys {
		raw := meta[k]
		var v string
		if isBinary(raw) {
			v = fmt.Sprintf("<binary %d bytes>", len(raw))
		} else {
			v = string(raw)
			if len(v) > 80 {
				v = v[:80] + "..."
			}
		}
		fmt.Printf("  %-20s = %s\n", k, v)
	}

	return nil
}

// isBinary returns true if b contains bytes outside printable ASCII + common whitespace.
func isBinary(b []byte) bool {
	for _, c := range b {
		if (c < 0x20 && c != '\t' && c != '\n' && c != '\r') || c > 0x7e {
			return true
		}
	}
	return false
}

func formatUUID(uuid [16]byte) string {
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16])
}

func formatFlags(hdr oza.Header) string {
	var parts []string
	if hdr.HasSearch() {
		parts = append(parts, "has-search")
	}
	if hdr.HasChrome() {
		parts = append(parts, "has-chrome")
	}
	if hdr.HasSignatures() {
		parts = append(parts, "has-signatures")
	}
	if len(parts) == 0 {
		return ""
	}
	return " [" + strings.Join(parts, ", ") + "]"
}

func sectionTypeName(t oza.SectionType) string {
	switch t {
	case oza.SectionMetadata:
		return "METADATA"
	case oza.SectionMIMETable:
		return "MIME_TABLE"
	case oza.SectionEntryTable:
		return "ENTRY_TABLE"
	case oza.SectionPathIndex:
		return "PATH_INDEX"
	case oza.SectionTitleIndex:
		return "TITLE_INDEX"
	case oza.SectionContent:
		return "CONTENT"
	case oza.SectionRedirectTab:
		return "REDIRECT_TAB"
	case oza.SectionChrome:
		return "CHROME"
	case oza.SectionSignatures:
		return "SIGNATURES"
	case oza.SectionZstdDict:
		return "ZSTD_DICT"
	default:
		return fmt.Sprintf("0x%04x", uint32(t))
	}
}

func compressionName(c uint8) string {
	switch c {
	case oza.CompNone:
		return "none"
	case oza.CompZstd:
		return "zstd"
	case oza.CompZstdDict:
		return "zstd+dict"
	default:
		return fmt.Sprintf("0x%02x", c)
	}
}
