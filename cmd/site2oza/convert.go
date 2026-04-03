package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io/fs"
	"mime"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/net/html"

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
	Exclude         []string

	// Metadata overrides (empty = auto-detect).
	Title    string
	Language string
	Creator  string
	Date     string
	Source   string
}

// siteEntry represents a single file to include in the archive.
type siteEntry struct {
	RelPath  string // forward-slashed path relative to input dir
	AbsPath  string // absolute filesystem path
	MIMEType string
	Title    string
	IsFront  bool
	Content  []byte
}

// Converter converts a static site directory to OZA.
type Converter struct {
	inputDir string
	ozaPath  string
	opts     ConvertOptions
	stats    Stats
}

// NewConverter validates the input directory and prepares for conversion.
func NewConverter(inputDir, ozaPath string, opts ConvertOptions) (*Converter, error) {
	info, err := os.Stat(inputDir)
	if err != nil {
		return nil, fmt.Errorf("opening site directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("opening site directory: %s is not a directory", inputDir)
	}
	return &Converter{
		inputDir: inputDir,
		ozaPath:  ozaPath,
		opts:     opts,
	}, nil
}

// Run executes the full conversion pipeline.
func (c *Converter) Run() error {
	totalStart := time.Now()

	// Phase 1: Walk and collect file entries.
	walkStart := time.Now()
	entries, err := c.walkDir()
	if err != nil {
		return err
	}
	c.stats.TimeWalk = time.Since(walkStart)
	c.stats.InputFiles = len(entries)

	if c.opts.Verbose {
		fmt.Fprintf(os.Stderr, "Scanned %d files\n", len(entries))
	}

	// Phase 2: Read file contents and extract titles.
	if err := c.readContents(entries); err != nil {
		return err
	}

	for _, e := range entries {
		c.stats.InputSize += int64(len(e.Content))
	}

	// Phase 3: Sort by ChunkKey for compression locality.
	sort.Slice(entries, func(i, j int) bool {
		ki := ozawrite.ChunkKey(entries[i].MIMEType, len(entries[i].Content))
		kj := ozawrite.ChunkKey(entries[j].MIMEType, len(entries[j].Content))
		if ki != kj {
			return ki < kj
		}
		return entries[i].RelPath < entries[j].RelPath
	})

	// Phase 4: Create writer and add entries.
	f, err := os.Create(c.ozaPath)
	if err != nil {
		return fmt.Errorf("creating output: %w", err)
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

	// Generate a TOC index page if none exists or the existing one is a stub.
	entries = c.ensureIndexPage(entries)

	c.writeMetadata(w, entries)

	for i, e := range entries {
		if _, err := w.AddEntry(e.RelPath, e.Title, e.MIMEType, e.Content, e.IsFront); err != nil {
			return fmt.Errorf("adding entry %s: %w", e.RelPath, err)
		}
		if c.opts.Verbose && (i+1)%500 == 0 {
			fmt.Fprintf(os.Stderr, "Adding entries: %d/%d\n", i+1, len(entries))
		}
	}
	if c.opts.Verbose {
		fmt.Fprintf(os.Stderr, "Adding entries: %d/%d done\n", len(entries), len(entries))
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("finalizing OZA: %w", err)
	}

	// Collect stats.
	c.stats.TimeTotal = time.Since(totalStart)
	c.stats.EntryTotal = len(entries)
	if info, err := os.Stat(c.ozaPath); err == nil {
		c.stats.OutputSize = info.Size()
	}

	wt := w.Timings()
	c.stats.TimeTransform = wt.Transform
	c.stats.TimeDedup = wt.Dedup
	c.stats.TimeSearchIndex = wt.SearchIndex
	c.stats.TimeChunkBuild = wt.ChunkBuild
	c.stats.TimeDictTrain = wt.DictTrain
	c.stats.TimeCompress = wt.Compress
	c.stats.TimeAssemble = wt.Assemble
	populateTranscodeStats(&c.stats, c.opts.TranscodeTools)

	return nil
}

// walkDir walks the input directory and collects file entries.
func (c *Converter) walkDir() ([]siteEntry, error) {
	absInput, err := filepath.Abs(c.inputDir)
	if err != nil {
		return nil, fmt.Errorf("resolving input path: %w", err)
	}
	absOutput, _ := filepath.Abs(c.ozaPath)

	var entries []siteEntry
	err = filepath.WalkDir(absInput, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if c.opts.Verbose {
				fmt.Fprintf(os.Stderr, "warning: cannot access %s: %v\n", p, err)
			}
			return nil
		}

		name := d.Name()

		// Skip hidden files and directories.
		if strings.HasPrefix(name, ".") && name != "." {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip well-known non-content directories.
		if d.IsDir() {
			switch name {
			case "node_modules", "__pycache__":
				return filepath.SkipDir
			}
			return nil
		}

		// Skip symlinks.
		if d.Type()&fs.ModeSymlink != 0 {
			if c.opts.Verbose {
				fmt.Fprintf(os.Stderr, "warning: skipping symlink %s\n", p)
			}
			return nil
		}

		// Skip output file if inside input dir.
		if p == absOutput {
			return nil
		}

		// Apply user exclude patterns.
		rel, err := filepath.Rel(absInput, p)
		if err != nil {
			return nil
		}
		relSlash := filepath.ToSlash(rel)
		for _, pattern := range c.opts.Exclude {
			if matched, _ := path.Match(pattern, relSlash); matched {
				return nil
			}
			// Also match against just the filename.
			if matched, _ := path.Match(pattern, name); matched {
				return nil
			}
		}

		mimeType := detectMIME(relSlash)
		isFront := isFrontArticleMIME(mimeType)

		entries = append(entries, siteEntry{
			RelPath:  relSlash,
			AbsPath:  p,
			MIMEType: mimeType,
			IsFront:  isFront,
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking directory: %w", err)
	}
	return entries, nil
}

// readContents reads file contents for all entries and extracts titles.
func (c *Converter) readContents(entries []siteEntry) error {
	for i := range entries {
		e := &entries[i]
		if e.Content != nil {
			continue
		}

		data, err := os.ReadFile(e.AbsPath)
		if err != nil {
			if c.opts.Verbose {
				fmt.Fprintf(os.Stderr, "warning: cannot read %s: %v\n", e.AbsPath, err)
			}
			continue
		}
		e.Content = data

		// Extract title based on content type.
		switch {
		case strings.HasPrefix(e.MIMEType, "text/html"), strings.HasPrefix(e.MIMEType, "application/xhtml+xml"):
			if t := extractHTMLTitle(data); t != "" {
				e.Title = t
			} else {
				e.Title = filenameTitle(e.RelPath)
			}
		case strings.HasPrefix(e.MIMEType, "text/markdown"):
			if t := extractMDTitle(data); t != "" {
				e.Title = t
			} else {
				e.Title = filenameTitle(e.RelPath)
			}
		}
	}
	return nil
}

// writeMetadata sets OZA metadata from options or auto-detected values.
func (c *Converter) writeMetadata(w *ozawrite.Writer, entries []siteEntry) {
	title := c.opts.Title
	if title == "" {
		title = filepath.Base(c.inputDir)
	}
	w.SetMetadata("title", title)

	lang := c.opts.Language
	if lang == "" {
		lang = "en"
	}
	w.SetMetadata("language", lang)

	creator := c.opts.Creator
	if creator == "" {
		creator = "Unknown"
	}
	w.SetMetadata("creator", creator)

	date := c.opts.Date
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	w.SetMetadata("date", date)

	source := c.opts.Source
	if source == "" {
		abs, err := filepath.Abs(c.inputDir)
		if err == nil {
			source = abs
		} else {
			source = c.inputDir
		}
	}
	w.SetMetadata("source", source)

	w.SetMetadata("converter", "site2oza")
	w.SetMetadata("converter_version", Version)

	// Detect main entry.
	if main := detectMainEntry(entries); main != "" {
		w.SetMetadata("main_entry", main)
	}
}

// ensureIndexPage checks whether the archive has a meaningful index page.
// If not, it generates a TOC page linking to all front articles and either
// replaces the stub or adds a new index.html entry.
func (c *Converter) ensureIndexPage(entries []siteEntry) []siteEntry {
	title := c.opts.Title
	if title == "" {
		title = filepath.Base(c.inputDir)
	}

	// Find existing index page (HTML or MD).
	indexIdx := -1
	for i, e := range entries {
		if e.RelPath == "index.html" || e.RelPath == "index.md" {
			indexIdx = i
			break
		}
	}

	// Check if existing index has links (HTML) or markdown links.
	if indexIdx >= 0 {
		content := string(entries[indexIdx].Content)
		hasHTMLLinks := strings.Contains(content, "<a ") || strings.Contains(content, "<a\t")
		hasMDLinks := strings.Contains(content, "](")
		if hasHTMLLinks || hasMDLinks {
			return entries // existing index has links — leave it alone
		}
		if c.opts.Verbose {
			fmt.Fprintf(os.Stderr, "Index page has no links, generating TOC\n")
		}
	}

	// Collect front articles for the TOC, grouped by top-level directory.
	groups := map[string][]tocItem{}
	var groupOrder []string
	groupSeen := map[string]bool{}

	for _, e := range entries {
		if !e.IsFront {
			continue
		}
		if e.RelPath == "index.html" || e.RelPath == "index.md" {
			continue
		}
		if e.Title == "" {
			continue
		}
		// Group by top-level directory only.
		dir := ""
		if idx := strings.IndexByte(e.RelPath, '/'); idx >= 0 {
			dir = e.RelPath[:idx]
		}
		if !groupSeen[dir] {
			groupSeen[dir] = true
			groupOrder = append(groupOrder, dir)
		}
		groups[dir] = append(groups[dir], tocItem{Path: e.RelPath, Title: e.Title})
	}

	// Sort entries within each group alphabetically by title.
	for _, g := range groupOrder {
		items := groups[g]
		sort.Slice(items, func(i, j int) bool {
			return items[i].Title < items[j].Title
		})
		groups[g] = items
	}

	tocHTML := buildTOCPage(title, groupOrder, groups)
	tocEntry := siteEntry{
		RelPath:  "index.html",
		MIMEType: "text/html",
		Title:    title,
		IsFront:  true,
		Content:  []byte(tocHTML),
	}

	if indexIdx >= 0 {
		entries[indexIdx] = tocEntry
	} else {
		entries = append(entries, tocEntry)
	}
	return entries
}

// tocItem represents a single entry in the generated table of contents.
type tocItem struct {
	Path  string
	Title string
}

// buildTOCPage generates an HTML table of contents grouped by directory.
func buildTOCPage(title string, groupOrder []string, groups map[string][]tocItem) string {
	var b strings.Builder
	b.WriteString("<!DOCTYPE html>\n<html><head><meta charset=\"utf-8\"><title>")
	b.WriteString(htmlEscape(title))
	b.WriteString("</title><style>body{padding:.75em 1em 1em}</style></head><body>\n<h1>")
	b.WriteString(htmlEscape(title))
	b.WriteString("</h1>\n")

	for _, dir := range groupOrder {
		items := groups[dir]
		if dir != "" {
			heading := strings.ReplaceAll(dir, "/", " / ")
			parts := strings.Split(heading, " / ")
			for i, p := range parts {
				if len(p) > 0 {
					parts[i] = strings.ToUpper(p[:1]) + p[1:]
				}
			}
			heading = strings.Join(parts, " / ")
			b.WriteString("<h2>")
			b.WriteString(htmlEscape(heading))
			b.WriteString("</h2>\n")
		}
		b.WriteString("<ul>\n")
		for _, item := range items {
			b.WriteString("<li><a href=\"")
			b.WriteString(htmlEscape(item.Path))
			b.WriteString("\">")
			b.WriteString(htmlEscape(item.Title))
			b.WriteString("</a></li>\n")
		}
		b.WriteString("</ul>\n")
	}

	b.WriteString("</body></html>")
	return b.String()
}

// detectMainEntry returns the path of the best candidate for main_entry.
func detectMainEntry(entries []siteEntry) string {
	candidates := []string{"index.html", "index.htm", "index.md", "README.md", "readme.md", "README.html", "readme.html"}
	entryPaths := make(map[string]bool, len(entries))
	for _, e := range entries {
		entryPaths[e.RelPath] = true
	}

	for _, c := range candidates {
		if entryPaths[c] {
			return c
		}
	}

	// Fallback: first root-level HTML or MD file alphabetically.
	var rootContent []string
	for _, e := range entries {
		if strings.Contains(e.RelPath, "/") {
			continue
		}
		if strings.HasSuffix(e.RelPath, ".html") || strings.HasSuffix(e.RelPath, ".md") {
			rootContent = append(rootContent, e.RelPath)
		}
	}
	sort.Strings(rootContent)
	if len(rootContent) > 0 {
		return rootContent[0]
	}
	return ""
}

// MIME detection

var mimeOverrides = map[string]string{
	".md":          "text/markdown; charset=utf-8",
	".markdown":    "text/markdown; charset=utf-8",
	".mdown":       "text/markdown; charset=utf-8",
	".woff":        "font/woff",
	".woff2":       "font/woff2",
	".ico":         "image/x-icon",
	".webp":        "image/webp",
	".avif":        "image/avif",
	".json":        "application/json",
	".xml":         "application/xml",
	".txt":         "text/plain; charset=utf-8",
	".yaml":        "application/yaml",
	".yml":         "application/yaml",
	".toml":        "application/toml",
	".webmanifest": "application/manifest+json",
}

func detectMIME(relPath string) string {
	ext := strings.ToLower(filepath.Ext(relPath))
	if m, ok := mimeOverrides[ext]; ok {
		return m
	}
	if m := mime.TypeByExtension(ext); m != "" {
		return m
	}
	return "application/octet-stream"
}

// Title extraction

func extractHTMLTitle(content []byte) string {
	tok := html.NewTokenizer(bytes.NewReader(content))
	for {
		tt := tok.Next()
		switch tt {
		case html.ErrorToken:
			return ""
		case html.StartTagToken:
			tn, _ := tok.TagName()
			tag := string(tn)
			if tag == "title" {
				if tok.Next() == html.TextToken {
					return strings.TrimSpace(tok.Token().Data)
				}
				return ""
			}
			if tag == "h1" {
				if tok.Next() == html.TextToken {
					return strings.TrimSpace(tok.Token().Data)
				}
				return ""
			}
		}
	}
}

func extractMDTitle(content []byte) string {
	scanner := bufio.NewScanner(bytes.NewReader(content))
	inFrontmatter := false

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// YAML frontmatter parsing.
		if trimmed == "---" {
			if !inFrontmatter {
				inFrontmatter = true
				continue
			}
			inFrontmatter = false
			continue
		}
		if inFrontmatter {
			if strings.HasPrefix(trimmed, "title:") {
				val := strings.TrimSpace(strings.TrimPrefix(trimmed, "title:"))
				val = strings.Trim(val, "\"'")
				if val != "" {
					return val
				}
			}
			continue
		}

		// ATX heading.
		if strings.HasPrefix(trimmed, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
		}
	}
	return ""
}

func filenameTitle(relPath string) string {
	base := path.Base(relPath)
	ext := path.Ext(base)
	return strings.TrimSuffix(base, ext)
}

// Helpers

func isFrontArticleMIME(mimeType string) bool {
	return strings.HasPrefix(mimeType, "text/html") ||
		strings.HasPrefix(mimeType, "application/xhtml+xml") ||
		strings.HasPrefix(mimeType, "text/markdown")
}

func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}
