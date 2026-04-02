package main

import (
	"fmt"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/stazelabs/oza/cmd/internal/epubread"
	"github.com/stazelabs/oza/ozawrite"
)

// ConvertOptions controls the conversion behaviour.
type ConvertOptions struct {
	ZstdLevel       int
	DictSamples     int
	ChunkSize       int
	BuildSearch     bool
	SearchPruneFreq float64
	TrainDict       bool
	Minify          bool
	OptimizeImages  bool
	TranscodeTools  *ozawrite.TranscodeTools
	CompressWorkers int
	Verbose         bool
}

// Converter converts an EPUB archive to OZA.
type Converter struct {
	epubPath string
	ozaPath  string
	opts     ConvertOptions
	book     *epubread.Book
	stats    Stats
}

// NewConverter opens the EPUB file and prepares for conversion.
func NewConverter(epubPath, ozaPath string, opts ConvertOptions) (*Converter, error) {
	book, err := epubread.Open(epubPath)
	if err != nil {
		return nil, fmt.Errorf("opening EPUB: %w", err)
	}
	return &Converter{
		epubPath: epubPath,
		ozaPath:  ozaPath,
		opts:     opts,
		book:     book,
	}, nil
}

// Run executes the full conversion pipeline.
func (c *Converter) Run() error {
	totalStart := time.Now()

	entries := c.book.Entries()
	meta := c.book.Metadata()

	if c.opts.Verbose {
		fmt.Fprintf(os.Stderr, "EPUB: %q by %q (%s)\n", meta.Title, meta.Creator, meta.Language)
		fmt.Fprintf(os.Stderr, "Entries: %d total, %d spine items\n", len(entries), len(c.book.SpineEntries()))
	}

	// Create output file.
	f, err := os.Create(c.ozaPath)
	if err != nil {
		return err
	}
	defer f.Close()

	wopts := ozawrite.WriterOptions{
		ZstdLevel:        c.opts.ZstdLevel,
		ChunkTargetSize:  c.opts.ChunkSize,
		TrainDict:        c.opts.TrainDict,
		DictSamples:      c.opts.DictSamples,
		BuildSearch:      c.opts.BuildSearch,
		BuildTitleSearch: c.opts.BuildSearch,
		BuildBodySearch:  c.opts.BuildSearch,
		SearchPruneFreq:  c.opts.SearchPruneFreq,
		MinifyHTML:       c.opts.Minify,
		MinifyCSS:        c.opts.Minify,
		MinifyJS:         c.opts.Minify,
		MinifySVG:        c.opts.Minify,
		OptimizeImages:   c.opts.OptimizeImages,
		TranscodeTools:   c.opts.TranscodeTools,
		CompressWorkers:  c.opts.CompressWorkers,
	}
	if c.opts.Verbose {
		wopts.Progress = func(phase string, n, total int) {
			switch phase {
			case "dict-train":
				if n == 0 {
					fmt.Fprintf(os.Stderr, "Training dictionaries...\n")
				}
			case "compress":
				if total > 0 && n == total {
					fmt.Fprintf(os.Stderr, "Compressed %d chunks\n", total)
				}
			case "index-path":
				if n == 0 {
					fmt.Fprintf(os.Stderr, "Building path/title indexes...\n")
				}
			case "index-search-title":
				if n == 0 {
					fmt.Fprintf(os.Stderr, "Building title search index...\n")
				}
			case "index-search-body":
				if n == 0 {
					fmt.Fprintf(os.Stderr, "Building body search index...\n")
				}
			case "assemble":
				if n == 0 {
					fmt.Fprintf(os.Stderr, "Assembling file...\n")
				}
			}
		}
	}

	w := ozawrite.NewWriter(f, wopts)

	// Set metadata from EPUB Dublin Core.
	c.writeMetadata(w, meta)

	// Sort entries by (chunkKey, path) for MIME locality in compression.
	sorted := make([]epubread.Entry, len(entries))
	copy(sorted, entries)
	sort.Slice(sorted, func(i, j int) bool {
		ki := ozawrite.ChunkKey(sorted[i].MediaType, len(sorted[i].Content))
		kj := ozawrite.ChunkKey(sorted[j].MediaType, len(sorted[j].Content))
		if ki != kj {
			return ki < kj
		}
		return sorted[i].Path < sorted[j].Path
	})

	// Track which OZA path gets which entry ID, for the TOC main_entry.
	pathToID := make(map[string]uint32, len(sorted))

	// Add all entries.
	for i, e := range sorted {
		isFront := e.IsSpine && (e.MediaType == "text/html" ||
			e.MediaType == "application/xhtml+xml")

		id, err := w.AddEntry(e.Path, titleForEntry(e), e.MediaType, e.Content, isFront)
		if err != nil {
			return fmt.Errorf("adding entry %s: %w", e.Path, err)
		}
		pathToID[e.Path] = id

		if c.opts.Verbose && (i+1)%100 == 0 {
			fmt.Fprintf(os.Stderr, "Adding entries: %d/%d\n", i+1, len(sorted))
		}
	}
	if c.opts.Verbose {
		fmt.Fprintf(os.Stderr, "Adding entries: %d/%d done\n", len(sorted), len(sorted))
	}

	// Generate a synthetic table-of-contents page and set it as main_entry.
	tocHTML := c.buildTOCPage(meta)
	if _, err := w.AddEntry("index.html", meta.Title, "text/html", []byte(tocHTML), true); err != nil {
		return fmt.Errorf("adding TOC entry: %w", err)
	}
	w.SetMetadata("main_entry", "index.html")

	// Finalize.
	if err := w.Close(); err != nil {
		return fmt.Errorf("finalizing OZA: %w", err)
	}

	c.stats.TimeTotal = time.Since(totalStart)
	c.stats.EntryContent = len(sorted) + 1 // +1 for TOC
	c.stats.EntryTotal = len(sorted) + 1

	if info, err := os.Stat(c.ozaPath); err == nil {
		c.stats.OutputSize = info.Size()
	}
	if info, err := os.Stat(c.epubPath); err == nil {
		c.stats.InputSize = info.Size()
	}

	// Pull per-phase timings from writer.
	wt := w.Timings()
	c.stats.TimeTransform = wt.Transform
	c.stats.TimeDedup = wt.Dedup
	c.stats.TimeSearchIndex = wt.SearchIndex
	c.stats.TimeChunkBuild = wt.ChunkBuild
	c.stats.TimeDictTrain = wt.DictTrain
	c.stats.TimeCompress = wt.Compress
	c.stats.TimeAssemble = wt.Assemble

	return nil
}

// writeMetadata maps EPUB Dublin Core metadata to OZA metadata keys.
func (c *Converter) writeMetadata(w *ozawrite.Writer, meta epubread.Metadata) {
	set := func(key, val string) {
		if val != "" {
			w.SetMetadata(key, val)
		}
	}

	set("title", meta.Title)
	set("creator", meta.Creator)
	set("language", meta.Language)
	set("date", meta.Date)
	set("description", meta.Description)
	set("publisher", meta.Publisher)

	// Ensure required keys have at least placeholder values.
	if meta.Title == "" {
		w.SetMetadata("title", "Untitled")
	}
	if meta.Language == "" {
		w.SetMetadata("language", "eng")
	}
	if meta.Creator == "" {
		w.SetMetadata("creator", "Unknown")
	}
	if meta.Date == "" {
		w.SetMetadata("date", time.Now().Format("2006-01-02"))
	}
	w.SetMetadata("source", c.epubPath)
	w.SetMetadata("converter", "epub2oza")
	w.SetMetadata("converter_version", Version)
}

// titleForEntry returns a display title for an entry. Spine items use their
// filename without extension; non-spine items have no title.
func titleForEntry(e epubread.Entry) string {
	if !e.IsSpine {
		return ""
	}
	base := path.Base(e.Path)
	ext := path.Ext(base)
	return strings.TrimSuffix(base, ext)
}

// buildTOCPage generates a minimal HTML table of contents from the EPUB's
// navigation structure (NCX or EPUB3 nav).
func (c *Converter) buildTOCPage(meta epubread.Metadata) string {
	toc := c.book.TOC()

	var b strings.Builder
	b.WriteString("<!DOCTYPE html>\n<html><head><meta charset=\"utf-8\">")
	b.WriteString("<title>")
	b.WriteString(htmlEscape(meta.Title))
	b.WriteString("</title></head><body>\n")
	b.WriteString("<h1>")
	b.WriteString(htmlEscape(meta.Title))
	b.WriteString("</h1>\n")
	if meta.Creator != "" {
		b.WriteString("<p>")
		b.WriteString(htmlEscape(meta.Creator))
		b.WriteString("</p>\n")
	}

	if len(toc) > 0 {
		b.WriteString("<nav><ol>\n")
		for _, entry := range toc {
			writeTOCEntry(&b, entry)
		}
		b.WriteString("</ol></nav>\n")
	} else {
		// Fallback: list spine items.
		b.WriteString("<nav><ol>\n")
		for _, e := range c.book.SpineEntries() {
			b.WriteString("<li><a href=\"")
			b.WriteString(htmlEscape(e.Path))
			b.WriteString("\">")
			title := titleForEntry(e)
			if title == "" {
				title = path.Base(e.Path)
			}
			b.WriteString(htmlEscape(title))
			b.WriteString("</a></li>\n")
		}
		b.WriteString("</ol></nav>\n")
	}

	b.WriteString("</body></html>")
	return b.String()
}

func writeTOCEntry(b *strings.Builder, entry epubread.TOCEntry) {
	b.WriteString("<li><a href=\"")
	b.WriteString(htmlEscape(entry.Href))
	b.WriteString("\">")
	b.WriteString(htmlEscape(entry.Title))
	b.WriteString("</a>")
	if len(entry.Children) > 0 {
		b.WriteString("\n<ol>\n")
		for _, child := range entry.Children {
			writeTOCEntry(b, child)
		}
		b.WriteString("</ol>\n")
	}
	b.WriteString("</li>\n")
}

// htmlEscape escapes special HTML characters.
func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}
