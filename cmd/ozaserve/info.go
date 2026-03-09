package main

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"html"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/stazelabs/oza/oza"
)

// handleInfo serves GET /{archive}/-/info — diagnostic overview for a single archive.
func (lib *library) handleInfo(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("archive")
	ae, ok := lib.archives[slug]
	if !ok {
		http.NotFound(w, r)
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

	fmt.Fprint(w, `<h2>Format</h2><table>`)
	fmt.Fprintf(w, `<tr><th>Filename</th><td>%s</td></tr>`, html.EscapeString(ae.filename))
	fmt.Fprintf(w, `<tr><th>UUID</th><td class="mono">%s</td></tr>`, uuidStr)
	fmt.Fprintf(w, `<tr><th>Version</th><td>%d.%d</td></tr>`, hdr.MajorVersion, hdr.MinorVersion)
	fmt.Fprintf(w, `<tr><th>Entry Count</th><td>%s</td></tr>`, commaInt(int(hdr.EntryCount)))
	fmt.Fprintf(w, `<tr><th>Content Size</th><td>%s</td></tr>`, formatBytes(int64(hdr.ContentSize)))
	if chunkSize, ok := rawMeta["chunk_target_size"]; ok {
		fmt.Fprintf(w, `<tr><th>Chunk Target Size</th><td>%s</td></tr>`, formatBytes(parseMetaInt64(chunkSize)))
	}
	fmt.Fprintf(w, `<tr><th>Section Count</th><td>%d</td></tr>`, hdr.SectionCount)
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

	// Indices section.
	sections := a.Sections()
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
	fmt.Fprint(w, footerBarHTML())
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
