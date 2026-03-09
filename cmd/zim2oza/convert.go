package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/stazelabs/gozim/zim"

	"github.com/stazelabs/oza/ozawrite"
)

// ConvertOptions controls the conversion behaviour.
type ConvertOptions struct {
	ZstdLevel       int
	DictSamples     int
	ChunkSize       int
	BuildSearch     bool
	TrainDict       bool
	Minify          bool
	OptimizeImages  bool
	CompressWorkers int
	Verbose         bool
	DryRun          bool
}

// entryCategory classifies a ZIM entry for the OZA output.
type entryCategory int

const (
	categoryContent  entryCategory = iota
	categoryMetadata               // M/ namespace -> OZA metadata section
	categoryChrome                 // placeholder for Phase 8
	categorySkip                   // X/ namespace etc.
)

// Converter converts a ZIM archive to OZA.
type Converter struct {
	zimPath string
	ozaPath string
	opts    ConvertOptions
	za      *zim.Archive
	stats   Stats
}

// NewConverter opens the ZIM file and prepares for conversion.
func NewConverter(zimPath, ozaPath string, opts ConvertOptions) (*Converter, error) {
	// Entries are sorted by (clusterNum, blobNum) for sequential access.
	// A modest cache handles the few out-of-order accesses from mixed MIME
	// types sharing a cluster. With sequential access this achieves 99%+ hits.
	za, err := zim.OpenWithOptions(zimPath, zim.WithCacheSize(8))
	if err != nil {
		return nil, fmt.Errorf("opening ZIM: %w", err)
	}
	return &Converter{
		zimPath: zimPath,
		ozaPath: ozaPath,
		opts:    opts,
		za:      za,
	}, nil
}

// Close releases the ZIM archive.
func (c *Converter) Close() {
	if c.za != nil {
		c.za.Close()
	}
}

// Run executes the full conversion pipeline.
func (c *Converter) Run() error {
	totalStart := time.Now()

	// Phase 1: Scan all ZIM entries, classify them.
	scanStart := time.Now()
	plan, err := c.scan()
	if err != nil {
		return fmt.Errorf("scan: %w", err)
	}
	c.stats.TimeScan = time.Since(scanStart)

	// Sort content entries by (clusterNum, blobNum) for fast sequential reads.
	sortByClusterOrder(plan)

	if c.opts.Verbose {
		fmt.Fprintf(os.Stderr, "Scanned %d entries (%d content, %d redirect, %d metadata, %d skipped)\n",
			plan.totalScanned, len(plan.content), len(plan.redirects), len(plan.metaKeys), plan.skipped)
	}

	if c.opts.DryRun {
		c.stats.EntryTotal = plan.totalScanned
		c.stats.EntryContent = len(plan.content)
		c.stats.EntryRedirect = len(plan.redirects)
		c.stats.TimeTotal = time.Since(totalStart)
		return nil
	}

	// Phase 2: Write the OZA file.
	if err := c.write(plan); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	c.stats.TimeTotal = time.Since(totalStart)

	// Record output size.
	if info, err := os.Stat(c.ozaPath); err == nil {
		c.stats.OutputSize = info.Size()
	}
	if info, err := os.Stat(c.zimPath); err == nil {
		c.stats.InputSize = info.Size()
	}

	return nil
}

// scanPlan holds the classified entries from the ZIM scan.
type scanPlan struct {
	totalScanned int
	skipped      int

	// content entries: mapped (ozaPath, title, mimeType) + ZIM entry for reading later.
	content []contentEntry

	// redirects: mapped path/title + the ZIM index of the redirect target.
	redirects []redirectEntry

	// metadata key -> value, extracted from M/ namespace.
	metaKeys map[string]string

	// zimIndexToOzaID maps ZIM entry index -> OZA entry ID (for redirect resolution).
	zimIndexToOzaID map[uint32]uint32
}

type contentEntry struct {
	ozaPath        string
	title          string
	mimeType       string
	isFrontArticle bool
	zimEntry       zim.Entry
}

type redirectEntry struct {
	ozaPath        string
	title          string
	zimSelfIdx     uint32 // ZIM index of this redirect entry itself
	zimRedirectIdx uint32 // ZIM index of the redirect target
}

// scan iterates all ZIM entries and classifies them.
func (c *Converter) scan() (*scanPlan, error) {
	plan := &scanPlan{
		metaKeys: make(map[string]string),
	}

	// First pass: collect metadata from M/ namespace.
	for entry := range c.za.EntriesByNamespace('M') {
		plan.totalScanned++
		key := entry.Path()
		content, err := entry.ReadContent()
		if err != nil {
			if c.opts.Verbose {
				fmt.Fprintf(os.Stderr, "Warning: could not read metadata M/%s: %v\n", key, err)
			}
			continue
		}
		plan.metaKeys[mapMetadataKey(key)] = string(content)
	}

	// Second pass: collect content and redirect entries. OZA IDs are NOT
	// assigned here — they are assigned after sorting by cluster order.
	var contentEntries []contentEntry
	var redirectEntries []redirectEntry

	for entry := range c.za.Entries() {
		ns := entry.Namespace()

		// Skip metadata entries (already handled above).
		if ns == 'M' {
			continue
		}

		plan.totalScanned++

		ozaPath, cat := mapZIMPath(ns, entry.Path())

		switch cat {
		case categorySkip, categoryChrome:
			plan.skipped++
			continue
		case categoryMetadata:
			plan.skipped++
			continue
		case categoryContent:
			// OK, process below.
		}

		if entry.IsRedirect() {
			target, err := entry.RedirectTarget()
			if err != nil {
				if c.opts.Verbose {
					fmt.Fprintf(os.Stderr, "Warning: skipping redirect %s: %v\n", ozaPath, err)
				}
				plan.skipped++
				continue
			}

			redirectEntries = append(redirectEntries, redirectEntry{
				ozaPath:        ozaPath,
				title:          entry.Title(),
				zimSelfIdx:     entry.Index(),
				zimRedirectIdx: target.Index(),
			})
		} else {
			isFront := ns == 'C' && entry.MIMEType() == "text/html"
			contentEntries = append(contentEntries, contentEntry{
				ozaPath:        ozaPath,
				title:          entry.Title(),
				mimeType:       entry.MIMEType(),
				isFrontArticle: isFront,
				zimEntry:       entry,
			})
		}
	}

	plan.content = contentEntries
	plan.redirects = redirectEntries
	return plan, nil
}

// sortByClusterOrder sorts content entries by ZIM cluster order so that the
// read loop accesses clusters sequentially, maximizing the cluster cache hit
// rate. OZA ID assignment is deferred until after MIME re-sort in write().
func sortByClusterOrder(plan *scanPlan) {
	sort.Slice(plan.content, func(i, j int) bool {
		ci := plan.content[i].zimEntry.ClusterNum()
		cj := plan.content[j].zimEntry.ClusterNum()
		if ci != cj {
			return ci < cj
		}
		return plan.content[i].zimEntry.BlobNum() < plan.content[j].zimEntry.BlobNum()
	})
}

// resolveChain follows a redirect chain through the ZIM index map until a
// content entry is found. Returns the OZA content ID of the final target,
// or ok=false if the chain is broken or loops.
func resolveChain(plan *scanPlan, rmap map[uint32]*redirectEntry, zimIdx uint32) (uint32, bool) {
	visited := map[uint32]bool{}
	cur := zimIdx
	for {
		if visited[cur] {
			return 0, false // loop
		}
		visited[cur] = true
		// If the target is a content entry, we're done.
		if ozaID, ok := plan.zimIndexToOzaID[cur]; ok {
			return ozaID, true
		}
		// If the target is a redirect, follow the chain.
		re, ok := rmap[cur]
		if !ok {
			return 0, false // broken chain
		}
		cur = re.zimRedirectIdx
	}
}

// write creates the OZA file using the scan plan.
func (c *Converter) write(plan *scanPlan) error {
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
		MinifyHTML:       c.opts.Minify,
		MinifyCSS:        c.opts.Minify,
		MinifyJS:         c.opts.Minify,
		MinifySVG:        c.opts.Minify,
		OptimizeImages:   c.opts.OptimizeImages,
		CompressWorkers:  c.opts.CompressWorkers,
	}
	chunks := 0 // updated by progress callback, shown on the "Adding entries" line
	if c.opts.Verbose {
		wopts.Progress = func(phase string, n, total int) {
			switch phase {
			case "dict-train":
				if n == 0 {
					fmt.Fprintf(os.Stderr, "\x1b[2K\rTraining dictionaries...\n")
				}
			case "compress":
				if total > 0 && n == total {
					// Final report from Close() with known total.
					fmt.Fprintf(os.Stderr, "\x1b[2K\rCompressed %d chunks\n", total)
				} else {
					// Track count; displayed on the "Adding entries" line.
					chunks = n
				}
			case "index-path":
				if n == 0 {
					fmt.Fprintf(os.Stderr, "\x1b[2K\rBuilding path/title indexes...\n")
				}
			case "index-search-title":
				if n == 0 {
					fmt.Fprintf(os.Stderr, "\x1b[2K\rBuilding title search index...\n")
				}
			case "index-search-body":
				if n == 0 {
					fmt.Fprintf(os.Stderr, "\x1b[2K\rBuilding body search index...\n")
				}
			case "assemble":
				if n == 0 {
					fmt.Fprintf(os.Stderr, "\x1b[2K\rAssembling file...\n")
				}
			}
		}
	}
	w := ozawrite.NewWriter(f, wopts)

	if c.opts.Verbose {
		fmt.Fprintf(os.Stderr, "Compression workers: %d\n", w.CompressWorkers())
	}

	// Set metadata.
	c.writeMetadata(w, plan)

	// Phase 2a: Read all content in cluster order (fast sequential ZIM access).
	total := len(plan.content)
	type bufferedEntry struct {
		ozaPath        string
		title          string
		mimeType       string
		isFrontArticle bool
		zimIndex       uint32
		content        []byte
	}
	buffered := make([]bufferedEntry, 0, total)

	readStart := time.Now()
	for i, ce := range plan.content {
		t0 := time.Now()
		content, err := ce.zimEntry.ReadContentCopy()
		c.stats.TimeRead += time.Since(t0)

		if err != nil {
			if c.opts.Verbose {
				fmt.Fprintf(os.Stderr, "Warning: skipping %s: %v\n", ce.ozaPath, err)
			}
			continue
		}
		c.stats.BytesRead += int64(len(content))

		buffered = append(buffered, bufferedEntry{
			ozaPath:        ce.ozaPath,
			title:          ce.title,
			mimeType:       ce.mimeType,
			isFrontArticle: ce.isFrontArticle,
			zimIndex:       ce.zimEntry.Index(),
			content:        content,
		})

		if c.opts.Verbose && (i+1)%10000 == 0 {
			elapsed := time.Since(readStart)
			rate := float64(i+1) / elapsed.Seconds()
			cs := c.za.CacheStats()
			cachePct := float64(0)
			if cs.Hits+cs.Misses > 0 {
				cachePct = float64(cs.Hits) / float64(cs.Hits+cs.Misses) * 100
			}
			fmt.Fprintf(os.Stderr, "\x1b[2K\rReading entries: %d/%d (%.0f/sec, cache %.0f%% hit)", i+1, total, rate, cachePct)
		}
	}
	if c.opts.Verbose && total > 0 {
		fmt.Fprintf(os.Stderr, "\x1b[2K\rReading entries: %d/%d done\n", len(buffered), total)
	}

	// Phase 2b: Re-sort by (chunkKey, path) to restore MIME locality for
	// better compression, while keeping the fast cluster-order reads above.
	sort.Slice(buffered, func(i, j int) bool {
		ki := ozawrite.ChunkKey(buffered[i].mimeType, len(buffered[i].content))
		kj := ozawrite.ChunkKey(buffered[j].mimeType, len(buffered[j].content))
		if ki != kj {
			return ki < kj
		}
		return buffered[i].ozaPath < buffered[j].ozaPath
	})

	// Build ZIM index -> OZA ID map based on the final order.
	plan.zimIndexToOzaID = make(map[uint32]uint32, len(buffered))
	for i, be := range buffered {
		plan.zimIndexToOzaID[be.zimIndex] = uint32(i)
	}

	// Phase 2c: Add entries to the writer.
	loopStart := time.Now()
	addTotal := len(buffered)
	var entryCount atomic.Int64

	if c.opts.Verbose && addTotal > 0 {
		done := make(chan struct{})
		defer close(done)
		go func() {
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-done:
					return
				case <-ticker.C:
					n := int(entryCount.Load())
					if n == 0 {
						continue
					}
					elapsed := time.Since(loopStart)
					rate := float64(n) / elapsed.Seconds()
					remaining := time.Duration(float64(addTotal-n) / rate * float64(time.Second))
					pct := float64(n) / float64(addTotal)
					if pct >= 0.05 {
						estChunks := int(float64(chunks) / pct)
						fmt.Fprintf(os.Stderr, "\x1b[2K\rAdding entries: %d/%d (%.0f/sec, chunk %d/~%d, ETA %s)", n, addTotal, rate, chunks, estChunks, remaining.Round(time.Second))
					} else {
						fmt.Fprintf(os.Stderr, "\x1b[2K\rAdding entries: %d/%d (%.0f/sec, chunk %d, ETA %s)", n, addTotal, rate, chunks, remaining.Round(time.Second))
					}
				}
			}
		}()
	}

	for i, be := range buffered {
		if _, err := w.AddEntry(be.ozaPath, be.title, be.mimeType, be.content, be.isFrontArticle); err != nil {
			return fmt.Errorf("adding entry %s: %w", be.ozaPath, err)
		}
		entryCount.Store(int64(i + 1))
	}
	// Release buffered content to free memory before Close().
	buffered = nil

	if c.opts.Verbose && addTotal > 0 {
		fmt.Fprintf(os.Stderr, "\x1b[2K\rAdding entries: %d/%d done\n", addTotal, addTotal)
	}

	// Add redirects. Resolve chains so each redirect points to a content entry.
	zimIdxToRedirect := make(map[uint32]*redirectEntry, len(plan.redirects))
	for i := range plan.redirects {
		zimIdxToRedirect[plan.redirects[i].zimSelfIdx] = &plan.redirects[i]
	}
	for i, re := range plan.redirects {
		targetOzaID, ok := resolveChain(plan, zimIdxToRedirect, re.zimRedirectIdx)
		if !ok {
			if c.opts.Verbose {
				fmt.Fprintf(os.Stderr, "Warning: redirect %s target not found in OZA, skipping\n", re.ozaPath)
			}
			continue
		}
		if _, err := w.AddRedirect(re.ozaPath, re.title, targetOzaID); err != nil {
			return fmt.Errorf("adding redirect %s: %w", re.ozaPath, err)
		}
		if c.opts.Verbose && (i+1)%1000 == 0 {
			fmt.Fprintf(os.Stderr, "\x1b[2K\rAdding redirects: %d/%d", i+1, len(plan.redirects))
		}
	}
	if c.opts.Verbose && len(plan.redirects) > 0 {
		fmt.Fprintf(os.Stderr, "\x1b[2K\rAdding redirects: %d/%d done\n", len(plan.redirects), len(plan.redirects))
	}

	// Finalize.
	closeStart := time.Now()
	if err := w.Close(); err != nil {
		return fmt.Errorf("finalizing OZA: %w", err)
	}
	c.stats.TimeClose = time.Since(closeStart)

	// Pull per-phase timings from writer.
	wt := w.Timings()
	c.stats.TimeTransform = wt.Transform
	c.stats.TimeDedup = wt.Dedup
	c.stats.TimeSearchIndex = wt.SearchIndex
	c.stats.TimeChunkBuild = wt.ChunkBuild
	c.stats.TimeDictTrain = wt.DictTrain
	c.stats.TimeCompress = wt.Compress
	c.stats.TimeAssemble = wt.Assemble

	// Cluster cache stats from gozim.
	cs := c.za.CacheStats()
	c.stats.CacheHits = cs.Hits
	c.stats.CacheMisses = cs.Misses

	c.stats.EntryTotal = len(plan.content) + len(plan.redirects)
	c.stats.EntryContent = len(plan.content)
	c.stats.EntryRedirect = len(plan.redirects)

	return nil
}

// writeMetadata maps ZIM metadata to OZA metadata keys.
func (c *Converter) writeMetadata(w *ozawrite.Writer, plan *scanPlan) {
	// Map known ZIM metadata keys to OZA required keys.
	keyMap := map[string]string{
		"Title":       "title",
		"Language":    "language",
		"Creator":     "creator",
		"Date":        "date",
		"Source":      "source",
		"Description": "description",
		"Publisher":   "publisher",
		"Name":        "name",
	}

	for _, ozaKey := range keyMap {
		if val, ok := plan.metaKeys[ozaKey]; ok {
			w.SetMetadata(ozaKey, val)
		}
	}

	// Ensure required keys have at least placeholder values.
	defaults := map[string]string{
		"title":    "Untitled",
		"language": "eng",
		"creator":  "Unknown",
		"date":     time.Now().Format("2006-01-02"),
		"source":   c.zimPath,
	}
	for key, fallback := range defaults {
		if _, ok := plan.metaKeys[key]; !ok {
			w.SetMetadata(key, fallback)
		}
	}

	// Record provenance: tool, version, and flags that produced this OZA file.
	w.SetMetadata("converter", "zim2oza")
	w.SetMetadata("converter_version", Version)
	w.SetMetadata("converter_flags", c.converterFlags())

	// Map the main page if the ZIM has one. Resolve through any redirect
	// chain so main_entry always points to the final content entry path.
	if c.za.HasMainEntry() {
		main, err := c.za.MainEntry()
		if err == nil {
			resolved, err := main.Resolve()
			if err == nil {
				ozaPath, cat := mapZIMPath(resolved.Namespace(), resolved.Path())
				if cat == categoryContent {
					w.SetMetadata("main_entry", ozaPath)
				}
			}
		}
	}

	// Copy any additional metadata keys not in keyMap.
	for k, v := range plan.metaKeys {
		found := false
		for _, ozaKey := range keyMap {
			if ozaKey == mapMetadataKey(k) {
				found = true
				break
			}
		}
		if !found {
			w.SetMetadata(mapMetadataKey(k), v)
		}
	}
}

// mapZIMPath converts a ZIM namespace + path to an OZA path and category.
func mapZIMPath(namespace byte, path string) (ozaPath string, category entryCategory) {
	switch namespace {
	case 'C':
		if strings.HasPrefix(path, "_mw_/") {
			return "", categoryChrome
		}
		return path, categoryContent
	case 'M':
		return "", categoryMetadata
	case 'X':
		return "", categorySkip
	case 'W':
		return "_well_known/" + path, categoryContent
	case '-':
		// New-format ZIM uses '-' namespace for content.
		return path, categoryContent
	default:
		return "_other/" + string(namespace) + "/" + path, categoryContent
	}
}

// mapMetadataKey normalises a ZIM metadata key. The ZIM convention is Title-Case,
// but OZA uses lowercase.
func mapMetadataKey(zimKey string) string {
	return strings.ToLower(zimKey)
}

// converterFlags returns a compact string summarising the conversion options.
func (c *Converter) converterFlags() string {
	o := c.opts
	var parts []string
	parts = append(parts, fmt.Sprintf("zstd=%d", o.ZstdLevel))
	parts = append(parts, fmt.Sprintf("chunk=%d", o.ChunkSize))
	if o.TrainDict {
		parts = append(parts, fmt.Sprintf("dict=%d", o.DictSamples))
	}
	if o.BuildSearch {
		parts = append(parts, "search=all")
	}
	if o.Minify {
		parts = append(parts, "minify")
	}
	if o.OptimizeImages {
		parts = append(parts, "optimize-images")
	}
	return strings.Join(parts, " ")
}
