package main

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"html"
	"net/http"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/stazelabs/oza/oza"
)

// --- Shared row types for info templates ---

type infoRow struct {
	Label string
	Value string
	Class string
	Style string
}

type indexInfo struct {
	Name   string
	Badge  string
	Detail string
}

type metaRow struct {
	Key   string
	Value string // rendered as safeHTML in template
}

type mimeTypeRow struct {
	Index int
	Type  string
}

type sectionRow struct {
	Index       int
	Type        string
	CompSize    string
	UncompSize  string
	Compression string
	SHA256Short string
}

// --- Per-archive info page ---

type infoPageData struct {
	Slug        string
	Title       string
	FormatRows  []infoRow
	RuntimeRows []infoRow
	Indices     []indexInfo
	MetaRows    []metaRow
	MIMETypes   []mimeTypeRow
	Sections    []sectionRow
	FooterHTML  string
}

func yesNoBadge(b bool) string {
	if b {
		return `<span class="badge badge-yes">Yes</span>`
	}
	return `<span class="badge badge-no">No</span>`
}

// handleInfo serves GET /{archive}/-/info — diagnostic overview for a single archive.
func (lib *library) handleInfo(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("archive")
	ae, ok := lib.archives[slug]
	if !ok {
		write404(w, r)
		return
	}

	a := ae.archive
	hdr := a.FileHeader()

	uuidStr := fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		hdr.UUID[0:4], hdr.UUID[4:6], hdr.UUID[6:8], hdr.UUID[8:10], hdr.UUID[10:16])

	sections := a.Sections()
	rawMeta := a.AllMetadata()

	// Format rows.
	formatRows := []infoRow{
		{Label: "Filename", Value: ae.filename},
		{Label: "UUID", Value: uuidStr, Class: "mono"},
		{Label: "Version", Value: fmt.Sprintf("%d.%d", hdr.MajorVersion, hdr.MinorVersion)},
		{Label: "Entry Count", Value: commaInt(int(hdr.EntryCount))},
		{Label: "Content Size", Value: formatBytes(int64(hdr.ContentSize))},
	}
	if chunkSize, ok := rawMeta["chunk_target_size"]; ok {
		formatRows = append(formatRows, infoRow{Label: "Chunk Target Size", Value: formatBytes(parseMetaInt64(chunkSize))})
	}
	formatRows = append(formatRows,
		infoRow{Label: "Redirect Count", Value: commaInt(int(a.RedirectCount()))},
		infoRow{Label: "Chunk Count", Value: commaInt(a.ChunkCount())},
		infoRow{Label: "Front Articles", Value: commaInt(len(ae.frontArticleIDs))},
		infoRow{Label: "Section Count", Value: strconv.Itoa(int(hdr.SectionCount))},
	)
	var totalComp, totalUncomp int64
	for _, s := range sections {
		totalComp += int64(s.CompressedSize)
		if s.UncompressedSize > 0 {
			totalUncomp += int64(s.UncompressedSize)
		}
	}
	if totalUncomp > 0 {
		ratio := float64(totalComp) / float64(totalUncomp) * 100
		formatRows = append(formatRows, infoRow{Label: "Compression Ratio", Value: fmt.Sprintf("%.1f%% of uncompressed", ratio)})
	}
	main, mainErr := a.MainEntry()
	if mainErr == nil {
		resolved, _ := main.Resolve()
		formatRows = append(formatRows, infoRow{
			Label: "Main Entry",
			Value: fmt.Sprintf(`<a href="/%s/%s">%s</a>`,
				html.EscapeString(slug), html.EscapeString(resolved.Path()),
				html.EscapeString(resolved.Path())),
		})
	} else {
		formatRows = append(formatRows, infoRow{Label: "Main Entry", Value: yesNoBadge(false)})
	}

	// Runtime rows.
	var structuralRAM int64
	for _, s := range sections {
		if s.Type != oza.SectionContent {
			structuralRAM += int64(s.UncompressedSize)
		}
	}
	structuralRAM += int64(a.ChunkCount()) * oza.ChunkDescSize
	mapOverhead := int64(a.EntryCount()+a.RedirectCount()) * 48
	totalRAM := structuralRAM + mapOverhead
	cacheNow, cacheCap, cacheHits, cacheMisses := a.CacheStats()
	var hitRateStr string
	if total := cacheHits + cacheMisses; total > 0 {
		hitRateStr = fmt.Sprintf("%.1f%% (%s hits / %s requests)",
			float64(cacheHits)/float64(total)*100,
			commaInt(int(cacheHits)), commaInt(int(total)))
	} else {
		hitRateStr = "\u2014 (no requests yet)"
	}
	var runtimeRows []infoRow
	if ae.fileSize > 0 {
		runtimeRows = append(runtimeRows, infoRow{Label: "File Size", Value: formatBytes(ae.fileSize)})
	}
	runtimeRows = append(runtimeRows,
		infoRow{Label: "Structural RAM", Value: formatBytes(structuralRAM)},
		infoRow{Label: "Map Overhead (est.)", Value: formatBytes(mapOverhead)},
		infoRow{Label: "Total RAM (est.)", Value: formatBytes(totalRAM)},
		infoRow{Label: "Chunk Cache", Value: fmt.Sprintf("%d / %d chunks", cacheNow, cacheCap)},
		infoRow{Label: "Cache Hit Rate", Value: hitRateStr},
	)
	if ae.loadDuration > 0 {
		runtimeRows = append(runtimeRows, infoRow{Label: "Load Time", Value: fmt.Sprintf("%.2fs", ae.loadDuration.Seconds())})
	}

	// Indices.
	var hasPathIdx, hasTitleIdx bool
	for _, s := range sections {
		switch s.Type {
		case oza.SectionPathIndex:
			hasPathIdx = true
		case oza.SectionTitleIndex:
			hasTitleIdx = true
		}
	}
	titleDocCount, hasTitleSearch := a.TitleSearchDocCount()
	bodyDocCount, hasBodySearch := a.BodySearchDocCount()

	indices := []indexInfo{
		{Name: "Path", Badge: yesNoBadge(hasPathIdx)},
		{Name: "Title", Badge: yesNoBadge(hasTitleIdx)},
	}
	titleDetail := ""
	if hasTitleSearch && titleDocCount > 0 {
		titleDetail = commaInt(int(titleDocCount)) + " docs indexed"
	}
	indices = append(indices, indexInfo{Name: "Search \u2014 Title", Badge: yesNoBadge(hasTitleSearch), Detail: titleDetail})
	bodyDetail := ""
	if hasBodySearch && bodyDocCount > 0 {
		bodyDetail = commaInt(int(bodyDocCount)) + " docs indexed"
	}
	indices = append(indices, indexInfo{Name: "Search \u2014 Body", Badge: yesNoBadge(hasBodySearch), Detail: bodyDetail})

	// Metadata.
	metaKeys := make([]string, 0, len(rawMeta))
	for k := range rawMeta {
		metaKeys = append(metaKeys, k)
	}
	sort.Strings(metaKeys)
	var metaRows []metaRow
	for _, k := range metaKeys {
		raw := rawMeta[k]
		var display string
		switch {
		case strings.HasPrefix(k, "illustration_") && isBinaryMeta(raw):
			b64 := base64.StdEncoding.EncodeToString(raw)
			display = fmt.Sprintf(`<img src="data:image/png;base64,%s" style="max-width:96px;max-height:96px;image-rendering:pixelated">`, b64)
		case isBinaryMeta(raw):
			display = fmt.Sprintf(`<code>&lt;binary %d bytes&gt;</code>`, len(raw))
		default:
			display = html.EscapeString(string(raw))
			if len(raw) > 200 {
				display = `<div style="max-height:100px;overflow-y:auto">` + display + `</div>`
			}
		}
		metaRows = append(metaRows, metaRow{Key: k, Value: display})
	}

	// MIME types.
	mimeTypes := a.MIMETypes()
	var mimeRows []mimeTypeRow
	for i, m := range mimeTypes {
		mimeRows = append(mimeRows, mimeTypeRow{Index: i, Type: m})
	}

	// Sections.
	var sectionRows []sectionRow
	for i, s := range sections {
		shaHex := hex.EncodeToString(s.SHA256[:])
		sectionRows = append(sectionRows, sectionRow{
			Index:       i,
			Type:        sectionTypeName(s.Type),
			CompSize:    formatBytes(int64(s.CompressedSize)),
			UncompSize:  formatBytes(int64(s.UncompressedSize)),
			Compression: compressionName(s.Compression),
			SHA256Short: shaHex[:16] + "...",
		})
	}

	renderTemplate(w, "info.html", infoPageData{
		Slug:        slug,
		Title:       ae.title,
		FormatRows:  formatRows,
		RuntimeRows: runtimeRows,
		Indices:     indices,
		MetaRows:    metaRows,
		MIMETypes:   mimeRows,
		Sections:    sectionRows,
		FooterHTML:  footerBarHTML(!lib.noInfo),
	})
}

// isBinaryMeta reports whether b contains bytes outside printable ASCII + common whitespace.
func isBinaryMeta(b []byte) bool {
	for _, c := range b {
		if (c < 0x20 && c != '\t' && c != '\n' && c != '\r') || c > 0x7e {
			return true
		}
	}
	return false
}

func sectionTypeName(t oza.SectionType) string { return t.String() }
func compressionName(c uint8) string            { return oza.CompressionName(c) }

// parseMetaInt64 parses a metadata value as an int64, returning 0 on failure.
func parseMetaInt64(b []byte) int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
	return n
}

func formatBytes(b int64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1f GiB (%d bytes)", float64(b)/float64(1<<30), b)
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MiB (%d bytes)", float64(b)/float64(1<<20), b)
	case b >= 1<<10:
		return fmt.Sprintf("%.1f KiB (%d bytes)", float64(b)/float64(1<<10), b)
	default:
		return fmt.Sprintf("%d bytes", b)
	}
}

// formatUptime formats a duration as "Xd Yh Zm Ws", omitting leading zero units.
func formatUptime(d time.Duration) string {
	d = d.Truncate(time.Second)
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	secs := int(d.Seconds()) % 60
	switch {
	case days > 0:
		return fmt.Sprintf("%dd %dh %dm %ds", days, hours, mins, secs)
	case hours > 0:
		return fmt.Sprintf("%dh %dm %ds", hours, mins, secs)
	case mins > 0:
		return fmt.Sprintf("%dm %ds", mins, secs)
	default:
		return fmt.Sprintf("%ds", secs)
	}
}

// --- Global info page ---

type globalInfoArchiveRow struct {
	Slug          string
	Title         string
	FileSizeRaw   int64
	FileSize      string
	StructuralRaw int64
	Structural    string
	TotalRaw      int64
	Total         string
	EntryCount    int
	RedirectCount int
	ChunkCount    int
	CacheVal      float64
	CacheStr      string
	LoadSecs      float64
	LoadStr       string
}

type globalInfoDep struct {
	Path    string
	Version string
	Replace string
}

type globalInfoData struct {
	ProcessRows   []infoRow
	ArchiveRows   []globalInfoArchiveRow
	ArchiveCount  int
	TotFileSize   string
	TotStructural string
	TotTotal      string
	TotEntries    int
	TotRedirects  int
	TotChunks     int
	TotCacheNow   int
	TotCacheCap   int
	BuildInfo     bool
	BuildRows     []infoRow
	Deps          []globalInfoDep
	FooterHTML    string
}

// handleGlobalInfo serves GET /_info — process-wide runtime overview.
func (lib *library) handleGlobalInfo(w http.ResponseWriter, r *http.Request) {
	uptime := time.Since(lib.startTime)

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	processRows := []infoRow{
		{Label: "Uptime", Value: formatUptime(uptime)},
		{Label: "Started", Value: lib.startTime.Format("2006-01-02 15:04:05 MST")},
		{Label: "Go Version", Value: runtime.Version(), Class: "mono"},
		{Label: "Goroutines", Value: strconv.Itoa(runtime.NumGoroutine())},
		{Label: "Go Heap In Use", Value: formatBytes(int64(ms.HeapInuse))},
		{Label: "Go Heap Idle", Value: formatBytes(int64(ms.HeapIdle))},
		{Label: "Go Sys", Value: formatBytes(int64(ms.Sys))},
		{Label: "GC Cycles", Value: strconv.FormatUint(uint64(ms.NumGC), 10)},
	}

	var totFileSize, totStructural, totTotal int64
	var totEntries, totRedirects, totChunks int
	var totCacheNow, totCacheCap int
	var archiveRows []globalInfoArchiveRow

	for _, slug := range lib.slugs {
		ae := lib.archives[slug]
		a := ae.archive

		var structural int64
		for _, s := range a.Sections() {
			if s.Type != oza.SectionContent {
				structural += int64(s.UncompressedSize)
			}
		}
		structural += int64(a.ChunkCount()) * oza.ChunkDescSize
		mapOvhd := int64(a.EntryCount()+a.RedirectCount()) * 48
		total := structural + mapOvhd

		cacheNow, cacheCap, cacheHits, cacheMisses := a.CacheStats()
		var cacheStr string
		var cacheVal float64
		if allReqs := cacheHits + cacheMisses; allReqs > 0 {
			cacheVal = float64(cacheHits) / float64(allReqs) * 100
			cacheStr = fmt.Sprintf("%d/%d (%.0f%%)", cacheNow, cacheCap, cacheVal)
		} else {
			cacheStr = fmt.Sprintf("%d/%d", cacheNow, cacheCap)
		}

		totFileSize += ae.fileSize
		totStructural += structural
		totTotal += total
		totEntries += int(a.EntryCount())
		totRedirects += int(a.RedirectCount())
		totChunks += a.ChunkCount()
		totCacheNow += cacheNow
		totCacheCap += cacheCap

		loadStr := ""
		loadSecs := ae.loadDuration.Seconds()
		if ae.loadDuration > 0 {
			loadStr = fmt.Sprintf("%.1fs", loadSecs)
		}

		archiveRows = append(archiveRows, globalInfoArchiveRow{
			Slug:          slug,
			Title:         ae.title,
			FileSizeRaw:   ae.fileSize,
			FileSize:      formatBytesShort(ae.fileSize),
			StructuralRaw: structural,
			Structural:    formatBytesShort(structural),
			TotalRaw:      total,
			Total:         formatBytesShort(total),
			EntryCount:    int(a.EntryCount()),
			RedirectCount: int(a.RedirectCount()),
			ChunkCount:    a.ChunkCount(),
			CacheVal:      cacheVal,
			CacheStr:      cacheStr,
			LoadSecs:      loadSecs,
			LoadStr:       loadStr,
		})
	}

	data := globalInfoData{
		ProcessRows:   processRows,
		ArchiveRows:   archiveRows,
		ArchiveCount:  len(lib.slugs),
		TotFileSize:   formatBytesShort(totFileSize),
		TotStructural: formatBytesShort(totStructural),
		TotTotal:      formatBytesShort(totTotal),
		TotEntries:    totEntries,
		TotRedirects:  totRedirects,
		TotChunks:     totChunks,
		TotCacheNow:   totCacheNow,
		TotCacheCap:   totCacheCap,
		FooterHTML:    footerBarHTML(!lib.noInfo),
	}

	// Build info + dependencies.
	if bi, ok := debug.ReadBuildInfo(); ok {
		data.BuildInfo = true
		data.BuildRows = append(data.BuildRows, infoRow{Label: "Main Module", Value: bi.Main.Path})
		if bi.Main.Version != "" && bi.Main.Version != "(devel)" {
			data.BuildRows = append(data.BuildRows, infoRow{Label: "Module Version", Value: bi.Main.Version})
		}
		var settings []string
		for _, s := range bi.Settings {
			if s.Value != "" {
				settings = append(settings, html.EscapeString(s.Key)+"="+html.EscapeString(s.Value))
			}
		}
		if len(settings) > 0 {
			data.BuildRows = append(data.BuildRows, infoRow{
				Label: "Settings",
				Value: strings.Join(settings, " &nbsp;·&nbsp; "),
				Style: "font-size:0.85em",
			})
		}

		if len(bi.Deps) > 0 {
			deps := make([]*debug.Module, len(bi.Deps))
			copy(deps, bi.Deps)
			sort.Slice(deps, func(i, j int) bool { return deps[i].Path < deps[j].Path })
			for _, d := range deps {
				replaceStr := ""
				if d.Replace != nil {
					replaceStr = d.Replace.Path
					if d.Replace.Version != "" {
						replaceStr += " " + d.Replace.Version
					}
				}
				data.Deps = append(data.Deps, globalInfoDep{
					Path:    d.Path,
					Version: d.Version,
					Replace: replaceStr,
				})
			}
		}
	}

	renderTemplate(w, "info_global.html", data)
}

// formatBytesShort formats b as a short human-readable string without the raw byte count.
func formatBytesShort(b int64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1f GiB", float64(b)/float64(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MiB", float64(b)/float64(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.1f KiB", float64(b)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
