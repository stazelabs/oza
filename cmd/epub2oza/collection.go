package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/stazelabs/oza/cmd/internal/epubread"
	"github.com/stazelabs/oza/ozawrite"
)

// catalogEntry describes one book in a collection archive. Serialised as JSON
// in the "catalog" metadata key so ozaserve can surface individual books on
// the library page without special-casing any converter.
type catalogEntry struct {
	Slug     string `json:"slug"`
	Title    string `json:"title"`
	Creator  string `json:"creator"`
	Language string `json:"language"`
	Entry    string `json:"entry"`
	Entries  int    `json:"entries"`
}

// bookInfo holds a parsed EPUB and its slug for collection mode.
type bookInfo struct {
	slug string
	path string
	book *epubread.Book
}

// runCollection converts a directory of EPUB files into a single OZA archive.
func runCollection(inputDir, ozaPath string, opts ConvertOptions, recursive bool, title string) (*Stats, error) {
	totalStart := time.Now()

	// Discover .epub files.
	epubPaths, err := collectEPUBPaths(inputDir, recursive)
	if err != nil {
		return nil, err
	}
	if len(epubPaths) == 0 {
		return nil, fmt.Errorf("no .epub files found in %s", inputDir)
	}

	if opts.Verbose {
		fmt.Fprintf(os.Stderr, "Found %d EPUB files\n", len(epubPaths))
	}

	// Open all EPUBs and assign slugs.
	books, err := openBooks(epubPaths, opts.Verbose)
	if err != nil {
		return nil, err
	}

	// Count total entries across all books.
	var totalEntries int
	for _, b := range books {
		totalEntries += len(b.book.Entries())
	}

	if opts.Verbose {
		fmt.Fprintf(os.Stderr, "Collection: %d books, %d total entries\n", len(books), totalEntries)
	}

	// Create output file.
	f, err := os.Create(ozaPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	wopts := ozawrite.WriterOptions{
		ZstdLevel:        opts.ZstdLevel,
		ChunkTargetSize:  opts.ChunkSize,
		TrainDict:        opts.TrainDict,
		DictSamples:      opts.DictSamples,
		BuildSearch:      opts.BuildSearch,
		BuildTitleSearch: opts.BuildSearch,
		BuildBodySearch:  opts.BuildSearch,
		SearchPruneFreq:  opts.SearchPruneFreq,
		MinifyHTML:       opts.Minify,
		MinifyCSS:        opts.Minify,
		MinifyJS:         opts.Minify,
		MinifySVG:        opts.Minify,
		OptimizeImages:   opts.OptimizeImages,
		TranscodeTools:   opts.TranscodeTools,
		CompressWorkers:  opts.CompressWorkers,
	}
	if opts.Verbose {
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

	// Set collection-level metadata.
	if title == "" {
		title = filepath.Base(inputDir)
	}
	w.SetMetadata("title", title)
	w.SetMetadata("language", detectCollectionLanguage(books))
	w.SetMetadata("creator", "epub2oza collection")
	w.SetMetadata("date", time.Now().Format("2006-01-02"))
	w.SetMetadata("source", inputDir)
	w.SetMetadata("converter", "epub2oza")
	w.SetMetadata("converter_version", Version)
	w.SetMetadata("description", fmt.Sprintf("Collection of %d EPUB books", len(books)))

	// Write a structured catalog so ozaserve can surface individual books
	// on the library page without any converter-specific logic.
	catalog := make([]catalogEntry, len(books))
	for i, b := range books {
		meta := b.book.Metadata()
		catalog[i] = catalogEntry{
			Slug:     b.slug,
			Title:    meta.Title,
			Creator:  meta.Creator,
			Language: meta.Language,
			Entry:    b.slug + "/index.html",
			Entries:  len(b.book.Entries()),
		}
	}
	if catalogJSON, err := json.Marshal(catalog); err == nil {
		w.SetMetadata("catalog", string(catalogJSON))
	}

	// Collect all entries from all books, namespaced by slug.
	type namespacedEntry struct {
		ozaPath   string
		title     string
		mediaType string
		content   []byte
		isFront   bool
		bookSlug  string
	}

	var allEntries []namespacedEntry
	for _, b := range books {
		for _, e := range b.book.Entries() {
			// Namespace path: /{slug}/{original-path}
			nsPath := b.slug + "/" + e.Path

			isFront := e.IsSpine && (e.MediaType == "text/html" ||
				e.MediaType == "application/xhtml+xml")

			entryTitle := ""
			if isFront {
				entryTitle = titleForEntry(e)
				if entryTitle != "" {
					// Prefix with book title for disambiguation in search results.
					meta := b.book.Metadata()
					if meta.Title != "" {
						entryTitle = meta.Title + " — " + entryTitle
					}
				}
			}

			allEntries = append(allEntries, namespacedEntry{
				ozaPath:   nsPath,
				title:     entryTitle,
				mediaType: e.MediaType,
				content:   e.Content,
				isFront:   isFront,
				bookSlug:  b.slug,
			})
		}
	}

	// Sort by (chunkKey, path) for MIME locality in compression.
	sort.Slice(allEntries, func(i, j int) bool {
		ki := ozawrite.ChunkKey(allEntries[i].mediaType, len(allEntries[i].content))
		kj := ozawrite.ChunkKey(allEntries[j].mediaType, len(allEntries[j].content))
		if ki != kj {
			return ki < kj
		}
		return allEntries[i].ozaPath < allEntries[j].ozaPath
	})

	// Add all entries.
	entryCount := 0
	for i, e := range allEntries {
		if _, err := w.AddEntry(e.ozaPath, e.title, e.mediaType, e.content, e.isFront); err != nil {
			return nil, fmt.Errorf("adding entry %s: %w", e.ozaPath, err)
		}
		entryCount++

		if opts.Verbose && (i+1)%500 == 0 {
			fmt.Fprintf(os.Stderr, "Adding entries: %d/%d\n", i+1, len(allEntries))
		}
	}
	if opts.Verbose {
		fmt.Fprintf(os.Stderr, "Adding entries: %d/%d done\n", entryCount, len(allEntries))
	}

	// Generate per-book TOC pages: /{slug}/index.html
	for _, b := range books {
		tocHTML := buildBookTOCPage(b)
		if _, err := w.AddEntry(b.slug+"/index.html", b.book.Metadata().Title, "text/html", []byte(tocHTML), true); err != nil {
			return nil, fmt.Errorf("adding book TOC for %s: %w", b.slug, err)
		}
		entryCount++
	}

	// Generate collection-level index page.
	indexHTML := buildCollectionIndex(title, books)
	if _, err := w.AddEntry("index.html", title, "text/html", []byte(indexHTML), true); err != nil {
		return nil, fmt.Errorf("adding collection index: %w", err)
	}
	entryCount++
	w.SetMetadata("main_entry", "index.html")

	// Finalize.
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("finalizing OZA: %w", err)
	}

	stats := &Stats{
		EntryContent: entryCount,
		EntryTotal:   entryCount,
		TimeTotal:    time.Since(totalStart),
	}

	if info, err := os.Stat(ozaPath); err == nil {
		stats.OutputSize = info.Size()
	}

	// Sum input sizes.
	for _, ep := range epubPaths {
		if info, err := os.Stat(ep); err == nil {
			stats.InputSize += info.Size()
		}
	}

	wt := w.Timings()
	stats.TimeTransform = wt.Transform
	stats.TimeDedup = wt.Dedup
	stats.TimeSearchIndex = wt.SearchIndex
	stats.TimeChunkBuild = wt.ChunkBuild
	stats.TimeDictTrain = wt.DictTrain
	stats.TimeCompress = wt.Compress
	stats.TimeAssemble = wt.Assemble

	return stats, nil
}

// collectEPUBPaths scans a directory for .epub files.
func collectEPUBPaths(dir string, recursive bool) ([]string, error) {
	var paths []string
	if recursive {
		err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil // skip unreadable entries
			}
			if !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".epub") {
				paths = append(paths, p)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walking %s: %w", dir, err)
		}
	} else {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", dir, err)
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".epub") {
				paths = append(paths, filepath.Join(dir, e.Name()))
			}
		}
	}
	sort.Strings(paths)
	return paths, nil
}

// openBooks opens all EPUBs and assigns unique slugs.
func openBooks(paths []string, verbose bool) ([]bookInfo, error) {
	slugCounts := make(map[string]int)
	var books []bookInfo

	for _, p := range paths {
		book, err := epubread.Open(p)
		if err != nil {
			if verbose {
				fmt.Fprintf(os.Stderr, "Warning: skipping %s: %v\n", p, err)
			}
			continue
		}

		slug := makeBookSlug(p)
		if slug == "" {
			slug = "book"
		}

		// Deduplicate slugs.
		slugCounts[slug]++
		if slugCounts[slug] > 1 {
			slug = fmt.Sprintf("%s-%d", slug, slugCounts[slug])
		}

		meta := book.Metadata()
		if verbose {
			fmt.Fprintf(os.Stderr, "  [%s] %q by %q (%d entries)\n",
				slug, meta.Title, meta.Creator, len(book.Entries()))
		}

		books = append(books, bookInfo{
			slug: slug,
			path: p,
			book: book,
		})
	}

	// Sort by title for deterministic ordering.
	sort.Slice(books, func(i, j int) bool {
		return books[i].book.Metadata().Title < books[j].book.Metadata().Title
	})

	return books, nil
}

// detectCollectionLanguage returns the most common language across books.
func detectCollectionLanguage(books []bookInfo) string {
	counts := make(map[string]int)
	for _, b := range books {
		lang := b.book.Metadata().Language
		if lang != "" {
			counts[lang]++
		}
	}
	best := "en"
	bestCount := 0
	for lang, count := range counts {
		if count > bestCount {
			best = lang
			bestCount = count
		}
	}
	return best
}

// buildBookTOCPage generates an HTML table of contents for a single book
// within a collection, with paths relative to the book's namespace.
func buildBookTOCPage(b bookInfo) string {
	meta := b.book.Metadata()
	toc := b.book.TOC()

	var s strings.Builder
	s.WriteString("<!DOCTYPE html>\n<html><head><meta charset=\"utf-8\">")
	s.WriteString("<title>")
	s.WriteString(htmlEscape(meta.Title))
	s.WriteString("</title></head><body>\n")
	s.WriteString("<p><a href=\"../index.html\">&larr; Back to library</a></p>\n")
	s.WriteString("<h1>")
	s.WriteString(htmlEscape(meta.Title))
	s.WriteString("</h1>\n")
	if meta.Creator != "" {
		s.WriteString("<p>")
		s.WriteString(htmlEscape(meta.Creator))
		s.WriteString("</p>\n")
	}
	if meta.Description != "" {
		s.WriteString("<p><em>")
		s.WriteString(htmlEscape(meta.Description))
		s.WriteString("</em></p>\n")
	}

	if len(toc) > 0 {
		s.WriteString("<nav><ol>\n")
		for _, entry := range toc {
			writeCollectionTOCEntry(&s, entry)
		}
		s.WriteString("</ol></nav>\n")
	} else {
		s.WriteString("<nav><ol>\n")
		for _, e := range b.book.SpineEntries() {
			s.WriteString("<li><a href=\"")
			s.WriteString(htmlEscape(e.Path))
			s.WriteString("\">")
			title := titleForEntry(e)
			if title == "" {
				title = path.Base(e.Path)
			}
			s.WriteString(htmlEscape(title))
			s.WriteString("</a></li>\n")
		}
		s.WriteString("</ol></nav>\n")
	}

	s.WriteString("</body></html>")
	return s.String()
}

func writeCollectionTOCEntry(b *strings.Builder, entry epubread.TOCEntry) {
	b.WriteString("<li><a href=\"")
	b.WriteString(htmlEscape(entry.Href))
	b.WriteString("\">")
	b.WriteString(htmlEscape(entry.Title))
	b.WriteString("</a>")
	if len(entry.Children) > 0 {
		b.WriteString("\n<ol>\n")
		for _, child := range entry.Children {
			writeCollectionTOCEntry(b, child)
		}
		b.WriteString("</ol>\n")
	}
	b.WriteString("</li>\n")
}

// buildCollectionIndex generates the top-level library index page.
func buildCollectionIndex(title string, books []bookInfo) string {
	var s strings.Builder
	s.WriteString("<!DOCTYPE html>\n<html><head><meta charset=\"utf-8\">")
	s.WriteString("<title>")
	s.WriteString(htmlEscape(title))
	s.WriteString("</title></head><body>\n")
	s.WriteString("<h1>")
	s.WriteString(htmlEscape(title))
	s.WriteString("</h1>\n")
	fmt.Fprintf(&s, "<p>%d books</p>\n", len(books))
	s.WriteString("<table>\n")
	s.WriteString("<tr><th>Title</th><th>Author</th><th>Language</th></tr>\n")

	for _, b := range books {
		meta := b.book.Metadata()
		s.WriteString("<tr><td><a href=\"")
		s.WriteString(htmlEscape(b.slug + "/index.html"))
		s.WriteString("\">")
		titleText := meta.Title
		if titleText == "" {
			titleText = b.slug
		}
		s.WriteString(htmlEscape(titleText))
		s.WriteString("</a></td><td>")
		s.WriteString(htmlEscape(meta.Creator))
		s.WriteString("</td><td>")
		s.WriteString(htmlEscape(meta.Language))
		s.WriteString("</td></tr>\n")
	}

	s.WriteString("</table>\n")
	s.WriteString("</body></html>")
	return s.String()
}
