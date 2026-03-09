package main

import (
	"bytes"
	"fmt"
	"html"
	"net/http"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"

	"github.com/stazelabs/oza/cmd/ozaserve/docs"
)

// renderedDocs is the pre-rendered HTML body of ozaserve.md, computed at init time.
var renderedDocs string

func init() {
	md := goldmark.New(goldmark.WithExtensions(extension.Table))
	var buf bytes.Buffer
	if err := md.Convert([]byte(docs.OzaserveMD), &buf); err != nil {
		renderedDocs = "<pre>" + html.EscapeString(docs.OzaserveMD) + "</pre>"
		return
	}
	renderedDocs = buf.String()
}

func handleDocs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, `<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>&#x738B;&#x5EA7; Documentation &#x2014; ozaserve</title>
`+faviconLink+`
<style>
body { font-family: system-ui, sans-serif; max-width: 900px; margin: 40px auto; padding: 0 20px; }
h1 { border-bottom: 1px solid #ddd; padding-bottom: 10px; margin-bottom: 4px; }
h1 a { color: inherit; text-decoration: none; }
h2 { margin-top: 28px; border-bottom: 1px solid #eee; padding-bottom: 6px; }
h3 { margin-top: 20px; }
a { color: #0366d6; text-decoration: none; }
a:hover { text-decoration: underline; }
code { font-family: ui-monospace, monospace; font-size: 0.9em; background: #f6f8fa; padding: 2px 6px; border-radius: 3px; }
pre { background: #f6f8fa; padding: 12px 16px; border-radius: 6px; overflow-x: auto; }
pre code { background: none; padding: 0; }
table { border-collapse: collapse; width: 100%; margin-bottom: 16px; }
th, td { text-align: left; padding: 6px 10px; border: 1px solid #ddd; }
th { background: #f6f8fa; font-weight: 600; }
blockquote { border-left: 4px solid #ddd; margin: 16px 0; padding: 0 16px; color: #555; }
hr { border: none; border-top: 1px solid #ddd; margin: 24px 0; }
.nav { margin-top: 20px; font-size: 0.9em; }
</style></head><body>
<h1><a href="/"><span style="color:#C9A84C">&#x738B;&#x5EA7;</span> OZA</a></h1>
<div class="nav" style="margin-bottom:16px"><a href="/">&#x2190; Library</a></div>
`)
	fmt.Fprint(w, renderedDocs)
	fmt.Fprint(w, `<div class="nav"><a href="/">&#x2190; Library</a></div>`)
	fmt.Fprint(w, footerBarHTML(false))
	fmt.Fprint(w, `</body></html>`)
}
