package testutil

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stazelabs/oza/ozawrite"
)

// TestArchiveOption configures a test archive built by [BuildTestArchive].
type TestArchiveOption func(*testArchiveConfig)

type testArchiveConfig struct {
	entryCount      int
	mimeTypes       []string
	redirectCount   int
	search          bool
	frontArticlePct float64
	brotli          bool
}

// WithEntryCount adds n extra generic entries beyond the standard fixed set.
// Default: 0 extra entries.
func WithEntryCount(n int) TestArchiveOption {
	return func(c *testArchiveConfig) { c.entryCount = n }
}

// WithMIMETypes sets the MIME types used for extra entries added by [WithEntryCount].
// Types are distributed in round-robin order. Default: ["text/html"].
func WithMIMETypes(types []string) TestArchiveOption {
	return func(c *testArchiveConfig) { c.mimeTypes = types }
}

// WithRedirects sets the total number of redirect entries.
// The first is named "old-page.html"; subsequent ones are "redirect-N.html".
// Default: 1.
func WithRedirects(n int) TestArchiveOption {
	return func(c *testArchiveConfig) { c.redirectCount = n }
}

// WithSearch enables or disables search index building. Default: false.
func WithSearch(enabled bool) TestArchiveOption {
	return func(c *testArchiveConfig) { c.search = enabled }
}

// WithFrontArticlePct sets the fraction (0..1) of extra entries marked as front
// articles. Does not affect the standard fixed entries. Default: 0.5.
func WithFrontArticlePct(pct float64) TestArchiveOption {
	return func(c *testArchiveConfig) { c.frontArticlePct = pct }
}

// WithBrotli signals that the archive should favour conditions where the writer's
// automatic Brotli trial applies (non-dict text chunks). When true, dictionary
// training is disabled so all text chunks are eligible for the Brotli trial.
// Default: false.
func WithBrotli(enabled bool) TestArchiveOption {
	return func(c *testArchiveConfig) { c.brotli = enabled }
}

// extForMIME returns a file extension for the given MIME type.
func extForMIME(mime string) string {
	switch mime {
	case "text/html":
		return "html"
	case "text/css":
		return "css"
	case "text/plain":
		return "txt"
	case "text/markdown":
		return "md"
	case "image/png":
		return "png"
	case "image/jpeg":
		return "jpg"
	case "image/svg+xml":
		return "svg"
	case "application/javascript":
		return "js"
	default:
		return "bin"
	}
}

// contentForMIME returns synthetic content for the given MIME type and index.
func contentForMIME(mime string, i int) []byte {
	switch mime {
	case "text/html":
		return []byte(fmt.Sprintf(
			"<html><body><h1>Extra %d</h1><p>Content %d. Keywords: alpha beta gamma.</p></body></html>", i, i))
	case "text/css":
		return []byte(fmt.Sprintf("/* extra %d */ body { margin: %dpx; }", i, i))
	case "text/plain":
		return []byte(fmt.Sprintf("Extra entry %d.\n", i))
	case "text/markdown":
		return []byte(fmt.Sprintf(
			"# Extra %d\n\nContent %d with **bold** text.\n\n| A | B |\n|---|---|\n| %d | %d |\n", i, i, i, i+1))
	case "image/png":
		return []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	case "image/jpeg":
		return []byte{0xff, 0xd8, 0xff, 0xe0}
	case "image/svg+xml":
		return []byte(fmt.Sprintf("<svg xmlns=\"http://www.w3.org/2000/svg\"><text>extra%d</text></svg>", i))
	case "application/javascript":
		return []byte(fmt.Sprintf("// extra %d\nconsole.log(%d);\n", i, i))
	default:
		return []byte(fmt.Sprintf("extra binary entry %d", i))
	}
}

// BuildTestArchive creates a configurable OZA archive in a temp file, returning
// its path. The file is removed automatically via t.Cleanup (through t.TempDir).
//
// The archive always contains a standard set of entries covering common MIME types
// suitable for handler and CLI smoke tests:
//   - index.html          (text/html, front article)
//   - articles/alpha.html (text/html, front article, "alpha"/"quantum physics" text)
//   - articles/beta.html  (text/html, front article, "beta"/"relativity" text)
//   - style.css           (text/css)
//   - logo.png            (image/png)
//   - guide.md            (text/markdown with heading, bold text, and table)
//   - old-page.html       (redirect → index.html)
//
// Use options to extend the archive with extra entries for scale or MIME-variety
// tests. Extra entries are named "extra-N.<ext>" and appended after the fixed set.
//
// Example:
//
//	path := testutil.BuildTestArchive(t,
//	    testutil.WithEntryCount(50),
//	    testutil.WithMIMETypes([]string{"text/html", "text/css"}),
//	    testutil.WithSearch(true),
//	)
func BuildTestArchive(t *testing.T, opts ...TestArchiveOption) string {
	t.Helper()

	cfg := testArchiveConfig{
		entryCount:      0,
		mimeTypes:       []string{"text/html"},
		redirectCount:   1,
		search:          false,
		frontArticlePct: 0.5,
		brotli:          false,
	}
	for _, o := range opts {
		o(&cfg)
	}
	if len(cfg.mimeTypes) == 0 {
		cfg.mimeTypes = []string{"text/html"}
	}

	path := filepath.Join(t.TempDir(), "test.oza")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	writerOpts := ozawrite.WriterOptions{
		ZstdLevel:   3,
		TrainDict:   false,
		BuildSearch: cfg.search,
	}

	w := ozawrite.NewWriter(f, writerOpts)
	w.SetMetadata("title", "Test Archive")
	w.SetMetadata("language", "en")
	w.SetMetadata("creator", "test")
	w.SetMetadata("date", "2026-01-01")
	w.SetMetadata("source", "https://example.com")
	w.SetMetadata("main_entry", "0")

	type fixedEntry struct {
		path, title, mime string
		content           []byte
		front             bool
	}
	fixed := []fixedEntry{
		{
			"index.html", "Main Page", "text/html",
			[]byte("<html><head><title>Main</title></head><body><h1>Main Page</h1><p>Welcome.</p></body></html>"),
			true,
		},
		{
			"articles/alpha.html", "Alpha Article", "text/html",
			[]byte("<html><body><h1>Alpha</h1><p>Alpha content about quantum physics.</p></body></html>"),
			true,
		},
		{
			"articles/beta.html", "Beta Article", "text/html",
			[]byte("<html><body><h1>Beta</h1><p>Beta content about relativity.</p></body></html>"),
			true,
		},
		{
			"style.css", "Style", "text/css",
			[]byte("body { margin: 0; }"),
			false,
		},
		{
			"logo.png", "Logo", "image/png",
			[]byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a},
			false,
		},
		{
			"guide.md", "Guide", "text/markdown",
			[]byte("# Guide\n\nThis is a **markdown** guide.\n\n## Section\n\nWith a table:\n\n| A | B |\n|---|---|\n| 1 | 2 |\n"),
			true,
		},
	}
	for _, e := range fixed {
		if _, err := w.AddEntry(e.path, e.title, e.mime, e.content, e.front); err != nil {
			t.Fatal(err)
		}
	}

	// Extra generic entries controlled by WithEntryCount / WithMIMETypes.
	frontN := int(float64(cfg.entryCount) * cfg.frontArticlePct)
	for i := range cfg.entryCount {
		mime := cfg.mimeTypes[i%len(cfg.mimeTypes)]
		ep := fmt.Sprintf("extra-%d.%s", i, extForMIME(mime))
		content := contentForMIME(mime, i)
		if _, err := w.AddEntry(ep, fmt.Sprintf("Extra Entry %d", i), mime, content, i < frontN); err != nil {
			t.Fatal(err)
		}
	}

	// Redirects.
	for i := range cfg.redirectCount {
		rPath := fmt.Sprintf("redirect-%d.html", i)
		if i == 0 {
			rPath = "old-page.html"
		}
		if _, err := w.AddRedirect(rPath, fmt.Sprintf("Redirect %d", i), 0); err != nil {
			t.Fatal(err)
		}
	}

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}
