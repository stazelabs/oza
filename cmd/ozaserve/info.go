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

	// Collect all metadata keys, sorted.
	rawMeta := a.AllMetadata()
	metaKeys := make([]string, 0, len(rawMeta))
	for k := range rawMeta {
		metaKeys = append(metaKeys, k)
	}
	sort.Strings(metaKeys)

	uuidStr := fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		hdr.UUID[0:4], hdr.UUID[4:6], hdr.UUID[6:8], hdr.UUID[8:10], hdr.UUID[10:16])

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>&#x738B;&#x5EA7; Info &#x2014; %s</title>
`+faviconLink+`
<style>
body { font-family: system-ui, sans-serif; max-width: 1000px; margin: 40px auto; padding: 0 20px; }
h1 { border-bottom: 1px solid #ddd; padding-bottom: 10px; margin-bottom: 4px; }
h1 a { color: inherit; text-decoration: none; }
h2 { font-size: 1.15em; margin-top: 28px; color: #333; }
table { border-collapse: collapse; width: 100%%; margin-bottom: 16px; }
th, td { text-align: left; padding: 6px 10px; border-bottom: 1px solid #eee; }
th { width: 200px; color: #555; font-weight: 600; white-space: nowrap; }
td { word-break: break-all; }
td.num { text-align: right; font-variant-numeric: tabular-nums; }
td.mono { font-family: ui-monospace, monospace; font-size: 0.9em; }
.badge { display: inline-block; padding: 2px 8px; border-radius: 10px; font-size: 0.8em; font-weight: 600; }
.badge-yes { background: #dcffe4; color: #1a7f37; }
.badge-no { background: #ffebe9; color: #cf222e; }
a { color: #0366d6; text-decoration: none; }
a:hover { text-decoration: underline; }
.nav { margin-top: 20px; font-size: 0.9em; }
</style></head><body>
<h1><a href="/"><span style="color:#C9A84C">&#x738B;&#x5EA7;</span> OZA</a></h1><h2>Info &#x2014; <a href="/%s/">%s</a></h2>`,
		html.EscapeString(ae.title),
		html.EscapeString(slug), html.EscapeString(ae.title))

	// Format / header section.
	yesNo := func(b bool) string {
		if b {
			return `<span class="badge badge-yes">Yes</span>`
		}
		return `<span class="badge badge-no">No</span>`
	}

	sections := a.Sections()

	fmt.Fprint(w, `<h2>Format</h2><table>`)
	fmt.Fprintf(w, `<tr><th>Filename</th><td>%s</td></tr>`, html.EscapeString(ae.filename))
	fmt.Fprintf(w, `<tr><th>UUID</th><td class="mono">%s</td></tr>`, uuidStr)
	fmt.Fprintf(w, `<tr><th>Version</th><td>%d.%d</td></tr>`, hdr.MajorVersion, hdr.MinorVersion)
	fmt.Fprintf(w, `<tr><th>Entry Count</th><td>%s</td></tr>`, commaInt(int(hdr.EntryCount)))
	fmt.Fprintf(w, `<tr><th>Content Size</th><td>%s</td></tr>`, formatBytes(int64(hdr.ContentSize)))
	if chunkSize, ok := rawMeta["chunk_target_size"]; ok {
		fmt.Fprintf(w, `<tr><th>Chunk Target Size</th><td>%s</td></tr>`, formatBytes(parseMetaInt64(chunkSize)))
	}
	fmt.Fprintf(w, `<tr><th>Redirect Count</th><td>%s</td></tr>`, commaInt(int(a.RedirectCount())))
	fmt.Fprintf(w, `<tr><th>Chunk Count</th><td>%s</td></tr>`, commaInt(a.ChunkCount()))
	fmt.Fprintf(w, `<tr><th>Front Articles</th><td>%s</td></tr>`, commaInt(len(ae.frontArticleIDs)))
	fmt.Fprintf(w, `<tr><th>Section Count</th><td>%d</td></tr>`, hdr.SectionCount)
	// Compression ratio across all sections.
	var totalComp, totalUncomp int64
	for _, s := range sections {
		totalComp += int64(s.CompressedSize)
		if s.UncompressedSize > 0 {
			totalUncomp += int64(s.UncompressedSize)
		}
	}
	if totalUncomp > 0 {
		ratio := float64(totalComp) / float64(totalUncomp) * 100
		fmt.Fprintf(w, `<tr><th>Compression Ratio</th><td>%.1f%% of uncompressed</td></tr>`, ratio)
	}
	// Main entry.
	main, mainErr := a.MainEntry()
	if mainErr == nil {
		resolved, _ := main.Resolve()
		fmt.Fprintf(w, `<tr><th>Main Entry</th><td><a href="/%s/%s">%s</a></td></tr>`,
			html.EscapeString(slug), html.EscapeString(resolved.Path()),
			html.EscapeString(resolved.Path()))
	} else {
		fmt.Fprintf(w, `<tr><th>Main Entry</th><td>%s</td></tr>`, yesNo(false))
	}
	fmt.Fprint(w, `</table>`)

	// Runtime section.
	var structuralRAM int64
	for _, s := range sections {
		if s.Type != oza.SectionContent {
			structuralRAM += int64(s.UncompressedSize)
		}
	}
	structuralRAM += int64(a.ChunkCount()) * oza.ChunkDescSize // chunk table kept in memory
	mapOverhead := int64(a.EntryCount()+a.RedirectCount()) * 48
	totalRAM := structuralRAM + mapOverhead
	cacheNow, cacheCap, cacheHits, cacheMisses := a.CacheStats()
	var hitRateStr string
	if total := cacheHits + cacheMisses; total > 0 {
		hitRateStr = fmt.Sprintf("%.1f%% (%s hits / %s requests)",
			float64(cacheHits)/float64(total)*100,
			commaInt(int(cacheHits)), commaInt(int(total)))
	} else {
		hitRateStr = "— (no requests yet)"
	}
	fmt.Fprint(w, `<h2>Runtime</h2><table>`)
	if ae.fileSize > 0 {
		fmt.Fprintf(w, `<tr><th>File Size</th><td>%s</td></tr>`, formatBytes(ae.fileSize))
	}
	fmt.Fprintf(w, `<tr><th>Structural RAM</th><td>%s</td></tr>`, formatBytes(structuralRAM))
	fmt.Fprintf(w, `<tr><th>Map Overhead (est.)</th><td>%s</td></tr>`, formatBytes(mapOverhead))
	fmt.Fprintf(w, `<tr><th>Total RAM (est.)</th><td>%s</td></tr>`, formatBytes(totalRAM))
	fmt.Fprintf(w, `<tr><th>Chunk Cache</th><td>%d / %d chunks</td></tr>`, cacheNow, cacheCap)
	fmt.Fprintf(w, `<tr><th>Cache Hit Rate</th><td>%s</td></tr>`, html.EscapeString(hitRateStr))
	if ae.loadDuration > 0 {
		fmt.Fprintf(w, `<tr><th>Load Time</th><td>%.2fs</td></tr>`, ae.loadDuration.Seconds())
	}
	fmt.Fprint(w, `<tr><td colspan="2" style="font-size:0.85em;color:#666">Structural RAM is exact (sum of decompressed sections). Map overhead is ~48 bytes/entry. Chunk content loads on demand and is not counted.</td></tr>`)
	fmt.Fprint(w, `</table>`)

	// Indices section.
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
	fmt.Fprint(w, `<h2>Indices</h2><table>
<tr><th style="width:200px">Index</th><th>Present</th><th>Details</th></tr>`)
	fmt.Fprintf(w, `<tr><td>Path</td><td>%s</td><td></td></tr>`, yesNo(hasPathIdx))
	fmt.Fprintf(w, `<tr><td>Title</td><td>%s</td><td></td></tr>`, yesNo(hasTitleIdx))

	titleSearchDetail := ""
	if hasTitleSearch && titleDocCount > 0 {
		titleSearchDetail = commaInt(int(titleDocCount)) + " docs indexed"
	}
	fmt.Fprintf(w, `<tr><td>Search &#x2014; Title</td><td>%s</td><td>%s</td></tr>`,
		yesNo(hasTitleSearch), html.EscapeString(titleSearchDetail))

	bodySearchDetail := ""
	if hasBodySearch && bodyDocCount > 0 {
		bodySearchDetail = commaInt(int(bodyDocCount)) + " docs indexed"
	}
	fmt.Fprintf(w, `<tr><td>Search &#x2014; Body</td><td>%s</td><td>%s</td></tr>`,
		yesNo(hasBodySearch), html.EscapeString(bodySearchDetail))

	fmt.Fprint(w, `</table>`)

	// Metadata section.
	if len(metaKeys) > 0 {
		fmt.Fprint(w, `<h2>Metadata</h2><table>`)
		for _, k := range metaKeys {
			raw := rawMeta[k]
			var display string
			switch {
			case strings.HasPrefix(k, "illustration_") && isBinaryMeta(raw):
				// Render inline as a thumbnail image.
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
			fmt.Fprintf(w, `<tr><th>%s</th><td>%s</td></tr>`, html.EscapeString(k), display)
		}
		fmt.Fprint(w, `</table>`)
	}

	// MIME types section.
	mimeTypes := a.MIMETypes()
	if len(mimeTypes) > 0 {
		fmt.Fprint(w, `<h2>MIME Types</h2><table><tr><th style="width:60px">Index</th><th>Type</th></tr>`)
		for i, m := range mimeTypes {
			fmt.Fprintf(w, `<tr><td>%d</td><td class="mono">%s</td></tr>`, i, html.EscapeString(m))
		}
		fmt.Fprint(w, `</table>`)
	}

	// Sections table.
	if len(sections) > 0 {
		fmt.Fprint(w, `<h2>Sections</h2><table>
<tr><th style="width:40px">#</th><th>Type</th><th style="text-align:right">Comp. Size</th><th style="text-align:right">Uncomp. Size</th><th>Compression</th><th>SHA-256</th></tr>`)
		for i, s := range sections {
			shaHex := hex.EncodeToString(s.SHA256[:])
			shaShort := shaHex[:16] + "..."
			fmt.Fprintf(w, `<tr><td>%d</td><td class="mono">%s</td><td class="num">%s</td><td class="num">%s</td><td class="mono">%s</td><td class="mono" style="font-size:0.8em">%s</td></tr>`,
				i, html.EscapeString(sectionTypeName(s.Type)),
				formatBytes(int64(s.CompressedSize)),
				formatBytes(int64(s.UncompressedSize)),
				html.EscapeString(compressionName(s.Compression)),
				shaShort)
		}
		fmt.Fprint(w, `</table>`)
	}

	fmt.Fprintf(w, `<div class="nav"><a href="/">Library</a> · <a href="/%s/">Main page</a> · <a href="/%s/-/search">Search</a> · <a href="/%s/-/browse">Browse</a></div>`,
		html.EscapeString(slug), html.EscapeString(slug), html.EscapeString(slug))
	fmt.Fprint(w, footerBarHTML(!lib.noInfo))
	fmt.Fprint(w, `</body></html>`)
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
	case oza.SectionSearchTitle:
		return "SEARCH_TITLE"
	case oza.SectionSearchBody:
		return "SEARCH_BODY"
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

// handleGlobalInfo serves GET /_info — process-wide runtime overview.
func (lib *library) handleGlobalInfo(w http.ResponseWriter, r *http.Request) {
	uptime := time.Since(lib.startTime)

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>&#x738B;&#x5EA7; Server Info</title>
`+faviconLink+`
<style>
body { font-family: system-ui, sans-serif; max-width: 1000px; margin: 40px auto; padding: 0 20px; }
h1 { border-bottom: 1px solid #ddd; padding-bottom: 10px; margin-bottom: 4px; }
h1 a { color: inherit; text-decoration: none; }
h2 { font-size: 1.15em; margin-top: 28px; color: #333; }
table { border-collapse: collapse; width: 100%%; margin-bottom: 16px; }
th, td { text-align: left; padding: 6px 10px; border-bottom: 1px solid #eee; }
th { width: 200px; color: #555; font-weight: 600; white-space: nowrap; }
td { word-break: break-all; }
td.num { text-align: right; font-variant-numeric: tabular-nums; }
td.mono { font-family: ui-monospace, monospace; font-size: 0.9em; }
a { color: #0366d6; text-decoration: none; }
a:hover { text-decoration: underline; }
.nav { margin-top: 20px; font-size: 0.9em; }
.archives-wrap { overflow-x: auto; }
.archives { width: max-content; min-width: 100%%; }
.archives td, .archives th { white-space: nowrap; }
.archives .col-name { white-space: normal; min-width: 180px; word-break: break-word; }
.archives .col-name a.ilink { color: #aaa; font-size: 0.85em; margin-left: 4px; vertical-align: middle; text-decoration: none; }
.archives .col-name a.ilink:hover { color: #0366d6; }
.archives th[data-col] { cursor: pointer; user-select: none; }
.archives th[data-col]:hover { background: #f6f8fa; }
.archives th.sorted { color: #0366d6; }
.arrow { font-size: 0.75em; margin-left: 4px; }
</style></head><body>
<h1><a href="/"><span style="color:#C9A84C">&#x738B;&#x5EA7;</span> OZA</a></h1><h2>Server Info</h2>`)

	// Process section.
	fmt.Fprint(w, `<h2>Process</h2><table>`)
	fmt.Fprintf(w, `<tr><th>Uptime</th><td>%s</td></tr>`, html.EscapeString(formatUptime(uptime)))
	fmt.Fprintf(w, `<tr><th>Started</th><td>%s</td></tr>`, lib.startTime.Format("2006-01-02 15:04:05 MST"))
	fmt.Fprintf(w, `<tr><th>Go Version</th><td class="mono">%s</td></tr>`, html.EscapeString(runtime.Version()))
	fmt.Fprintf(w, `<tr><th>Goroutines</th><td>%d</td></tr>`, runtime.NumGoroutine())
	fmt.Fprintf(w, `<tr><th>Go Heap In Use</th><td>%s</td></tr>`, formatBytes(int64(ms.HeapInuse)))
	fmt.Fprintf(w, `<tr><th>Go Heap Idle</th><td>%s</td></tr>`, formatBytes(int64(ms.HeapIdle)))
	fmt.Fprintf(w, `<tr><th>Go Sys</th><td>%s</td></tr>`, formatBytes(int64(ms.Sys)))
	fmt.Fprintf(w, `<tr><th>GC Cycles</th><td>%d</td></tr>`, ms.NumGC)
	fmt.Fprint(w, `</table>`)

	// Per-archive table.
	fmt.Fprint(w, `<h2>Archives</h2><div class="archives-wrap">
<table class="archives" id="archives-tbl">
<thead><tr>
  <th class="col-name" data-col="0">Archive<span class="arrow"></span></th>
  <th style="text-align:right" data-col="1">File Size<span class="arrow"></span></th>
  <th style="text-align:right" data-col="2">Structural RAM<span class="arrow"></span></th>
  <th style="text-align:right" data-col="3">Total RAM (est.)<span class="arrow"></span></th>
  <th style="text-align:right" data-col="4">Entries<span class="arrow"></span></th>
  <th style="text-align:right" data-col="5">Redirects<span class="arrow"></span></th>
  <th style="text-align:right" data-col="6">Chunks<span class="arrow"></span></th>
  <th style="text-align:right" data-col="7">Cache<span class="arrow"></span></th>
  <th style="text-align:right" data-col="8">Load<span class="arrow"></span></th>
</tr></thead>
<tbody>`)

	var totFileSize, totStructural, totTotal int64
	var totEntries, totRedirects, totChunks int
	var totCacheNow, totCacheCap int

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

		fmt.Fprintf(w, `<tr>
  <td class="col-name" data-val="%s"><a href="/%s/">%s</a><a href="/%s/-/info" class="ilink" title="Archive info">ⓘ</a></td>
  <td class="num" data-val="%d">%s</td>
  <td class="num" data-val="%d">%s</td>
  <td class="num" data-val="%d">%s</td>
  <td class="num" data-val="%d">%s</td>
  <td class="num" data-val="%d">%s</td>
  <td class="num" data-val="%d">%s</td>
  <td class="num mono" data-val="%.4f" style="font-size:0.9em">%s</td>
  <td class="num" data-val="%.4f">%s</td>
</tr>`,
			html.EscapeString(ae.title),
			html.EscapeString(slug), html.EscapeString(ae.title),
			html.EscapeString(slug),
			ae.fileSize, formatBytesShort(ae.fileSize),
			structural, formatBytesShort(structural),
			total, formatBytesShort(total),
			a.EntryCount(), commaInt(int(a.EntryCount())),
			a.RedirectCount(), commaInt(int(a.RedirectCount())),
			a.ChunkCount(), commaInt(a.ChunkCount()),
			cacheVal, html.EscapeString(cacheStr),
			loadSecs, html.EscapeString(loadStr),
		)
	}

	// Totals row.
	fmt.Fprintf(w, `</tbody><tfoot><tr style="font-weight:600;border-top:2px solid #ccc">
  <td>Total (%d archives)</td>
  <td class="num">%s</td>
  <td class="num">%s</td>
  <td class="num">%s</td>
  <td class="num">%s</td>
  <td class="num">%s</td>
  <td class="num">%s</td>
  <td class="num">%d/%d</td>
  <td></td>
</tr></tfoot>`,
		len(lib.slugs),
		formatBytesShort(totFileSize),
		formatBytesShort(totStructural),
		formatBytesShort(totTotal),
		commaInt(totEntries),
		commaInt(totRedirects),
		commaInt(totChunks),
		totCacheNow, totCacheCap,
	)
	fmt.Fprint(w, `</table></div>
<script>
(function(){
  var col = 0, asc = true;
  var tbl = document.getElementById('archives-tbl');
  var ths = tbl.querySelectorAll('thead th[data-col]');
  var tbody = tbl.querySelector('tbody');
  var numCols = {1:1,2:1,3:1,4:1,5:1,6:1,7:1,8:1};
  function sort(c, a) {
    col = c; asc = a;
    ths.forEach(function(th, i) {
      var arrow = th.querySelector('.arrow');
      arrow.textContent = i === c ? (a ? ' \u25b2' : ' \u25bc') : '';
      th.classList.toggle('sorted', i === c);
    });
    var rows = Array.from(tbody.rows);
    rows.sort(function(ra, rb) {
      var av = ra.cells[c].dataset.val;
      var bv = rb.cells[c].dataset.val;
      var cmp = numCols[c] ? +av - +bv : av.toLowerCase().localeCompare(bv.toLowerCase());
      return a ? cmp : -cmp;
    });
    rows.forEach(function(r){ tbody.appendChild(r); });
  }
  ths.forEach(function(th, i){
    th.addEventListener('click', function(){ sort(i, col === i ? !asc : true); });
  });
  sort(0, true);
})();
</script>`)
	fmt.Fprint(w, `<p style="font-size:0.85em;color:#666">Structural RAM: exact (decompressed sections + chunk table). Total RAM: adds ~48 bytes/entry for reverse maps. Chunk content not counted (on-demand).</p>`)

	// Dependencies section.
	if bi, ok := debug.ReadBuildInfo(); ok {
		// Build settings (GOOS, GOARCH, VCS info, etc.)
		var settings []string
		for _, s := range bi.Settings {
			if s.Value != "" {
				settings = append(settings, html.EscapeString(s.Key)+"="+html.EscapeString(s.Value))
			}
		}
		fmt.Fprint(w, `<h2>Build</h2><table>`)
		fmt.Fprintf(w, `<tr><th>Main Module</th><td class="mono">%s</td></tr>`, html.EscapeString(bi.Main.Path))
		if bi.Main.Version != "" && bi.Main.Version != "(devel)" {
			fmt.Fprintf(w, `<tr><th>Module Version</th><td class="mono">%s</td></tr>`, html.EscapeString(bi.Main.Version))
		}
		if len(settings) > 0 {
			fmt.Fprintf(w, `<tr><th>Settings</th><td class="mono" style="font-size:0.85em">%s</td></tr>`, strings.Join(settings, " &nbsp;·&nbsp; "))
		}
		fmt.Fprint(w, `</table>`)

		if len(bi.Deps) > 0 {
			fmt.Fprint(w, `<h2>Dependencies</h2>
<table>
<tr><th style="width:auto">Module</th><th style="width:160px">Version</th><th style="width:auto">Replace</th></tr>`)
			deps := make([]*debug.Module, len(bi.Deps))
			copy(deps, bi.Deps)
			sort.Slice(deps, func(i, j int) bool { return deps[i].Path < deps[j].Path })
			for _, d := range deps {
				replaceCell := ""
				if d.Replace != nil {
					replaceCell = html.EscapeString(d.Replace.Path)
					if d.Replace.Version != "" {
						replaceCell += " " + html.EscapeString(d.Replace.Version)
					}
				}
				fmt.Fprintf(w, `<tr><td class="mono" style="word-break:break-all">%s</td><td class="mono">%s</td><td class="mono" style="font-size:0.85em;color:#666">%s</td></tr>`,
					html.EscapeString(d.Path),
					html.EscapeString(d.Version),
					replaceCell,
				)
			}
			fmt.Fprint(w, `</table>`)
		}
	}

	fmt.Fprint(w, `<div class="nav"><a href="/">Library</a></div>`)
	fmt.Fprint(w, footerBarHTML(!lib.noInfo))
	fmt.Fprint(w, `</body></html>`)
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
