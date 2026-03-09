package main

import (
	"bytes"
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

type docsData struct {
	RenderedDocs string
	FooterHTML   string
}

func handleDocs(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "docs.html", docsData{
		RenderedDocs: renderedDocs,
		FooterHTML:   footerBarHTML(false),
	})
}
