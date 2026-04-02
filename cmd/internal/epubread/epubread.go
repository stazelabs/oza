// Package epubread provides a minimal EPUB 2/3 reader that extracts entries
// with their media types and Dublin Core metadata from an EPUB archive.
//
// It parses only three XML files: META-INF/container.xml (OCF), the OPF
// package document, and optionally the NCX/nav table of contents. All content
// is read from the underlying ZIP; no decompression beyond ZIP's deflate is
// performed.
package epubread

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"strings"
)

// Book represents an opened EPUB archive.
type Book struct {
	meta    Metadata
	entries []Entry
	spine   []string // ordered item IDs from <spine>
	toc     []TOCEntry
}

// Metadata holds Dublin Core metadata extracted from the OPF <metadata> element.
type Metadata struct {
	Title       string
	Creator     string
	Language    string
	Date        string
	Description string
	Publisher   string
	Subject     string
	Rights      string
	Identifier  string
}

// Entry represents a single resource inside the EPUB.
type Entry struct {
	// Path is the archive-relative path (resolved relative to OPF location).
	Path string

	// MediaType is the MIME type declared in the OPF manifest.
	MediaType string

	// IsSpine is true if this entry appears in the OPF <spine> (reading order).
	IsSpine bool

	// SpineIndex is the position in the spine (-1 if not a spine item).
	SpineIndex int

	// Content is the raw bytes of the entry.
	Content []byte
}

// TOCEntry represents a navigation point from the NCX or EPUB3 nav document.
type TOCEntry struct {
	Title    string
	Href     string
	Children []TOCEntry
}

// Open reads an EPUB file and returns a Book with all entries loaded.
func Open(filePath string) (*Book, error) {
	zr, err := zip.OpenReader(filePath)
	if err != nil {
		return nil, fmt.Errorf("epubread: open zip: %w", err)
	}
	defer zr.Close()

	return openFromZip(&zr.Reader)
}

// openFromZip is the internal implementation that works with a *zip.Reader.
func openFromZip(zr *zip.Reader) (*Book, error) {
	// Build a map of ZIP entries for fast lookup.
	zipFiles := make(map[string]*zip.File, len(zr.File))
	for _, f := range zr.File {
		zipFiles[f.Name] = f
	}

	// Step 1: Parse META-INF/container.xml to find the OPF path.
	opfPath, err := parseContainer(zipFiles)
	if err != nil {
		return nil, err
	}

	// Step 2: Parse the OPF package document.
	opfDir := path.Dir(opfPath)
	if opfDir == "." {
		opfDir = ""
	}

	pkg, err := parseOPF(zipFiles, opfPath)
	if err != nil {
		return nil, err
	}

	// Step 3: Build spine set for quick lookup.
	spineSet := make(map[string]int, len(pkg.Spine.ItemRefs))
	for i, ref := range pkg.Spine.ItemRefs {
		spineSet[ref.IDRef] = i
	}

	// Step 4: Build entries from manifest items.
	var entries []Entry
	var spineIDs []string
	for _, ref := range pkg.Spine.ItemRefs {
		spineIDs = append(spineIDs, ref.IDRef)
	}

	for _, item := range pkg.Manifest.Items {
		// Resolve href relative to OPF directory.
		entryPath := item.Href
		if opfDir != "" {
			entryPath = opfDir + "/" + item.Href
		}

		content, err := readZipEntry(zipFiles, entryPath)
		if err != nil {
			// Try URL-decoded path as fallback.
			continue
		}

		spineIdx := -1
		isSpine := false
		if idx, ok := spineSet[item.ID]; ok {
			isSpine = true
			spineIdx = idx
		}

		entries = append(entries, Entry{
			Path:       entryPath,
			MediaType:  item.MediaType,
			IsSpine:    isSpine,
			SpineIndex: spineIdx,
			Content:    content,
		})
	}

	// Step 5: Extract metadata.
	meta := extractMetadata(&pkg.Metadata)

	// Step 6: Parse TOC (NCX for EPUB2, nav for EPUB3).
	// TOC hrefs are relative to the NCX/nav file; resolve them relative to
	// the OPF directory so they match the full entry paths we built above.
	var toc []TOCEntry
	if ncxID := pkg.Spine.TOC; ncxID != "" {
		// EPUB2: NCX file referenced by spine@toc attribute.
		for _, item := range pkg.Manifest.Items {
			if item.ID == ncxID {
				ncxPath := item.Href
				if opfDir != "" {
					ncxPath = opfDir + "/" + item.Href
				}
				toc, _ = parseNCX(zipFiles, ncxPath)
				ncxDir := path.Dir(ncxPath)
				resolveTOCHrefs(toc, ncxDir)
				break
			}
		}
	}
	if len(toc) == 0 {
		// EPUB3: look for nav document (properties="nav").
		for _, item := range pkg.Manifest.Items {
			if strings.Contains(item.Properties, "nav") {
				navPath := item.Href
				if opfDir != "" {
					navPath = opfDir + "/" + item.Href
				}
				toc, _ = parseNav(zipFiles, navPath)
				navDir := path.Dir(navPath)
				resolveTOCHrefs(toc, navDir)
				break
			}
		}
	}

	return &Book{
		meta:    meta,
		entries: entries,
		spine:   spineIDs,
		toc:     toc,
	}, nil
}

// Metadata returns the book's Dublin Core metadata.
func (b *Book) Metadata() Metadata { return b.meta }

// Entries returns all manifest entries with their content.
func (b *Book) Entries() []Entry { return b.entries }

// SpineEntries returns only entries in the reading order (spine).
func (b *Book) SpineEntries() []Entry {
	var result []Entry
	for _, e := range b.entries {
		if e.IsSpine {
			result = append(result, e)
		}
	}
	return result
}

// TOC returns the table of contents.
func (b *Book) TOC() []TOCEntry { return b.toc }

// --- OCF container.xml ---

type ocfContainer struct {
	XMLName  xml.Name      `xml:"container"`
	RootFile []ocfRootFile `xml:"rootfiles>rootfile"`
}

type ocfRootFile struct {
	FullPath  string `xml:"full-path,attr"`
	MediaType string `xml:"media-type,attr"`
}

func parseContainer(files map[string]*zip.File) (string, error) {
	data, err := readZipEntry(files, "META-INF/container.xml")
	if err != nil {
		return "", fmt.Errorf("epubread: missing META-INF/container.xml: %w", err)
	}
	var c ocfContainer
	if err := xml.Unmarshal(data, &c); err != nil {
		return "", fmt.Errorf("epubread: parse container.xml: %w", err)
	}
	for _, rf := range c.RootFile {
		if rf.MediaType == "application/oebps-package+xml" || rf.MediaType == "" {
			return rf.FullPath, nil
		}
	}
	if len(c.RootFile) > 0 {
		return c.RootFile[0].FullPath, nil
	}
	return "", fmt.Errorf("epubread: no rootfile in container.xml")
}

// --- OPF package document ---

type opfPackage struct {
	XMLName  xml.Name    `xml:"package"`
	Metadata opfMetadata `xml:"metadata"`
	Manifest opfManifest `xml:"manifest"`
	Spine    opfSpine    `xml:"spine"`
}

type opfMetadata struct {
	Title       []opfDCElement `xml:"title"`
	Creator     []opfDCElement `xml:"creator"`
	Language    []opfDCElement `xml:"language"`
	Date        []opfDCElement `xml:"date"`
	Description []opfDCElement `xml:"description"`
	Publisher   []opfDCElement `xml:"publisher"`
	Subject     []opfDCElement `xml:"subject"`
	Rights      []opfDCElement `xml:"rights"`
	Identifier  []opfDCElement `xml:"identifier"`
}

type opfDCElement struct {
	Value string `xml:",chardata"`
}

type opfManifest struct {
	Items []opfItem `xml:"item"`
}

type opfItem struct {
	ID         string `xml:"id,attr"`
	Href       string `xml:"href,attr"`
	MediaType  string `xml:"media-type,attr"`
	Properties string `xml:"properties,attr"`
}

type opfSpine struct {
	TOC      string       `xml:"toc,attr"`
	ItemRefs []opfItemRef `xml:"itemref"`
}

type opfItemRef struct {
	IDRef string `xml:"idref,attr"`
}

func parseOPF(files map[string]*zip.File, opfPath string) (*opfPackage, error) {
	data, err := readZipEntry(files, opfPath)
	if err != nil {
		return nil, fmt.Errorf("epubread: read OPF %s: %w", opfPath, err)
	}
	var pkg opfPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return nil, fmt.Errorf("epubread: parse OPF %s: %w", opfPath, err)
	}
	return &pkg, nil
}

func extractMetadata(m *opfMetadata) Metadata {
	return Metadata{
		Title:       firstDC(m.Title),
		Creator:     firstDC(m.Creator),
		Language:    firstDC(m.Language),
		Date:        firstDC(m.Date),
		Description: firstDC(m.Description),
		Publisher:   firstDC(m.Publisher),
		Subject:     firstDC(m.Subject),
		Rights:      firstDC(m.Rights),
		Identifier:  firstDC(m.Identifier),
	}
}

func firstDC(elems []opfDCElement) string {
	if len(elems) > 0 {
		return strings.TrimSpace(elems[0].Value)
	}
	return ""
}

// --- NCX (EPUB2 table of contents) ---

type ncxDoc struct {
	XMLName xml.Name  `xml:"ncx"`
	NavMap  ncxNavMap `xml:"navMap"`
}

type ncxNavMap struct {
	Points []ncxNavPoint `xml:"navPoint"`
}

type ncxNavPoint struct {
	Label   ncxNavLabel   `xml:"navLabel"`
	Content ncxContent    `xml:"content"`
	Points  []ncxNavPoint `xml:"navPoint"`
}

type ncxNavLabel struct {
	Text string `xml:"text"`
}

type ncxContent struct {
	Src string `xml:"src,attr"`
}

func parseNCX(files map[string]*zip.File, ncxPath string) ([]TOCEntry, error) {
	data, err := readZipEntry(files, ncxPath)
	if err != nil {
		return nil, err
	}
	var doc ncxDoc
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("epubread: parse NCX: %w", err)
	}
	return convertNCXPoints(doc.NavMap.Points), nil
}

func convertNCXPoints(points []ncxNavPoint) []TOCEntry {
	var result []TOCEntry
	for _, p := range points {
		entry := TOCEntry{
			Title:    strings.TrimSpace(p.Label.Text),
			Href:     p.Content.Src,
			Children: convertNCXPoints(p.Points),
		}
		result = append(result, entry)
	}
	return result
}

// --- EPUB3 nav document ---

func parseNav(files map[string]*zip.File, navPath string) ([]TOCEntry, error) {
	data, err := readZipEntry(files, navPath)
	if err != nil {
		return nil, err
	}

	// The nav document is XHTML. We parse it with the XML decoder looking for
	// <nav epub:type="toc"> then extract <a> elements from the nested <ol>/<li>.
	// This is a simplified parser that handles the common structure.
	decoder := xml.NewDecoder(strings.NewReader(string(data)))
	decoder.Strict = false
	decoder.AutoClose = xml.HTMLAutoClose
	decoder.Entity = xml.HTMLEntity

	// Find the <nav> element with epub:type="toc".
	var inTocNav bool
	var depth int
	var entries []TOCEntry

	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}

		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "nav" {
				for _, a := range t.Attr {
					if a.Name.Local == "type" && strings.Contains(a.Value, "toc") {
						inTocNav = true
						depth = 0
					}
				}
			}
			if inTocNav && t.Name.Local == "ol" {
				depth++
			}
			if inTocNav && t.Name.Local == "a" && depth == 1 {
				href := ""
				for _, a := range t.Attr {
					if a.Name.Local == "href" {
						href = a.Value
					}
				}
				// Read link text.
				var text strings.Builder
				for {
					inner, err := decoder.Token()
					if err != nil {
						break
					}
					if end, ok := inner.(xml.EndElement); ok && end.Name.Local == "a" {
						break
					}
					if cd, ok := inner.(xml.CharData); ok {
						text.Write(cd)
					}
				}
				entries = append(entries, TOCEntry{
					Title: strings.TrimSpace(text.String()),
					Href:  href,
				})
			}

		case xml.EndElement:
			if inTocNav && t.Name.Local == "ol" {
				depth--
			}
			if inTocNav && t.Name.Local == "nav" {
				inTocNav = false
			}
		}
	}

	return entries, nil
}

// --- helpers ---

// resolveTOCHrefs prefixes TOC entry hrefs with baseDir so they match
// the full archive-relative paths used by Entry.Path. Fragment identifiers
// (e.g. #chapter1) are preserved.
func resolveTOCHrefs(entries []TOCEntry, baseDir string) {
	if baseDir == "." || baseDir == "" {
		return
	}
	for i := range entries {
		href := entries[i].Href
		if href == "" || strings.HasPrefix(href, "/") || strings.HasPrefix(href, "http") {
			continue
		}
		// Split off fragment.
		frag := ""
		if idx := strings.IndexByte(href, '#'); idx >= 0 {
			frag = href[idx:]
			href = href[:idx]
		}
		entries[i].Href = baseDir + "/" + href + frag
		resolveTOCHrefs(entries[i].Children, baseDir)
	}
}

func readZipEntry(files map[string]*zip.File, name string) ([]byte, error) {
	f, ok := files[name]
	if !ok {
		return nil, fmt.Errorf("not found in ZIP: %s", name)
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}
