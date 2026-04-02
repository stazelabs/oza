package epubread

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// buildTestEPUB writes a minimal valid EPUB to a temp file and returns its path.
func buildTestEPUB(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	epubPath := filepath.Join(dir, "test.epub")

	f, err := os.Create(epubPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	w := zip.NewWriter(f)

	// mimetype must be first and stored (not compressed).
	mh := &zip.FileHeader{Name: "mimetype", Method: zip.Store}
	mw, _ := w.CreateHeader(mh)
	mw.Write([]byte("application/epub+zip"))

	writeZip := func(name, content string) {
		fw, _ := w.Create(name)
		fw.Write([]byte(content))
	}

	// META-INF/container.xml
	writeZip("META-INF/container.xml", `<?xml version="1.0"?>
<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0">
  <rootfiles>
    <rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>`)

	// OPF package document
	writeZip("OEBPS/content.opf", `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Test Book</dc:title>
    <dc:creator>Test Author</dc:creator>
    <dc:language>en</dc:language>
    <dc:date>2025-01-01</dc:date>
    <dc:description>A test EPUB for unit testing.</dc:description>
    <dc:identifier>urn:uuid:12345678-1234-1234-1234-123456789abc</dc:identifier>
  </metadata>
  <manifest>
    <item id="ch1" href="chapter1.xhtml" media-type="application/xhtml+xml"/>
    <item id="ch2" href="chapter2.xhtml" media-type="application/xhtml+xml"/>
    <item id="style" href="style.css" media-type="text/css"/>
    <item id="cover" href="images/cover.png" media-type="image/png"/>
    <item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/>
  </manifest>
  <spine toc="ncx">
    <itemref idref="ch1"/>
    <itemref idref="ch2"/>
  </spine>
</package>`)

	// Chapter 1
	writeZip("OEBPS/chapter1.xhtml", `<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml">
<head><title>Chapter 1</title></head>
<body><h1>Chapter 1</h1><p>It was a dark and stormy night.</p></body>
</html>`)

	// Chapter 2
	writeZip("OEBPS/chapter2.xhtml", `<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml">
<head><title>Chapter 2</title></head>
<body><h1>Chapter 2</h1><p>The sun rose bright and early.</p></body>
</html>`)

	// CSS
	writeZip("OEBPS/style.css", `body { font-family: serif; margin: 1em; }`)

	// Cover (tiny 1x1 PNG)
	writeZip("OEBPS/images/cover.png", "\x89PNG\r\n\x1a\n") // truncated, but fine for testing

	// NCX (EPUB2 TOC)
	writeZip("OEBPS/toc.ncx", `<?xml version="1.0" encoding="UTF-8"?>
<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1">
  <navMap>
    <navPoint id="np1">
      <navLabel><text>Chapter 1</text></navLabel>
      <content src="chapter1.xhtml"/>
    </navPoint>
    <navPoint id="np2">
      <navLabel><text>Chapter 2</text></navLabel>
      <content src="chapter2.xhtml"/>
    </navPoint>
  </navMap>
</ncx>`)

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	return epubPath
}

func TestOpen(t *testing.T) {
	epubPath := buildTestEPUB(t)

	book, err := Open(epubPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	// Check metadata.
	meta := book.Metadata()
	if meta.Title != "Test Book" {
		t.Errorf("Title = %q, want %q", meta.Title, "Test Book")
	}
	if meta.Creator != "Test Author" {
		t.Errorf("Creator = %q, want %q", meta.Creator, "Test Author")
	}
	if meta.Language != "en" {
		t.Errorf("Language = %q, want %q", meta.Language, "en")
	}
	if meta.Date != "2025-01-01" {
		t.Errorf("Date = %q, want %q", meta.Date, "2025-01-01")
	}
	if meta.Identifier != "urn:uuid:12345678-1234-1234-1234-123456789abc" {
		t.Errorf("Identifier = %q", meta.Identifier)
	}
}

func TestEntries(t *testing.T) {
	epubPath := buildTestEPUB(t)

	book, err := Open(epubPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	entries := book.Entries()
	// Manifest has 5 items: ch1, ch2, style, cover, ncx.
	if len(entries) != 5 {
		t.Fatalf("Entries count = %d, want 5", len(entries))
	}

	// Build a map for easier checking.
	byPath := make(map[string]Entry, len(entries))
	for _, e := range entries {
		byPath[e.Path] = e
	}

	// Chapter 1 should be a spine item.
	ch1, ok := byPath["OEBPS/chapter1.xhtml"]
	if !ok {
		t.Fatal("chapter1.xhtml not found in entries")
	}
	if !ch1.IsSpine {
		t.Error("chapter1.xhtml: IsSpine = false, want true")
	}
	if ch1.SpineIndex != 0 {
		t.Errorf("chapter1.xhtml: SpineIndex = %d, want 0", ch1.SpineIndex)
	}
	if ch1.MediaType != "application/xhtml+xml" {
		t.Errorf("chapter1.xhtml: MediaType = %q", ch1.MediaType)
	}
	if !bytes.Contains(ch1.Content, []byte("dark and stormy night")) {
		t.Error("chapter1.xhtml: content missing expected text")
	}

	// CSS should NOT be a spine item.
	css, ok := byPath["OEBPS/style.css"]
	if !ok {
		t.Fatal("style.css not found in entries")
	}
	if css.IsSpine {
		t.Error("style.css: IsSpine = true, want false")
	}
	if css.MediaType != "text/css" {
		t.Errorf("style.css: MediaType = %q", css.MediaType)
	}
}

func TestSpineEntries(t *testing.T) {
	epubPath := buildTestEPUB(t)

	book, err := Open(epubPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	spine := book.SpineEntries()
	if len(spine) != 2 {
		t.Fatalf("SpineEntries count = %d, want 2", len(spine))
	}
}

func TestTOC(t *testing.T) {
	epubPath := buildTestEPUB(t)

	book, err := Open(epubPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	toc := book.TOC()
	if len(toc) != 2 {
		t.Fatalf("TOC count = %d, want 2", len(toc))
	}
	if toc[0].Title != "Chapter 1" {
		t.Errorf("TOC[0].Title = %q, want %q", toc[0].Title, "Chapter 1")
	}
	if toc[0].Href != "OEBPS/chapter1.xhtml" {
		t.Errorf("TOC[0].Href = %q, want %q", toc[0].Href, "OEBPS/chapter1.xhtml")
	}
	if toc[1].Title != "Chapter 2" {
		t.Errorf("TOC[1].Title = %q, want %q", toc[1].Title, "Chapter 2")
	}
}

func TestOpenInvalidPath(t *testing.T) {
	_, err := Open("/nonexistent/path.epub")
	if err == nil {
		t.Fatal("Open: expected error for nonexistent file")
	}
}

func TestOpenMissingContainer(t *testing.T) {
	dir := t.TempDir()
	epubPath := filepath.Join(dir, "bad.epub")

	f, _ := os.Create(epubPath)
	w := zip.NewWriter(f)
	fw, _ := w.Create("dummy.txt")
	fw.Write([]byte("not an epub"))
	w.Close()
	f.Close()

	_, err := Open(epubPath)
	if err == nil {
		t.Fatal("Open: expected error for missing container.xml")
	}
}
