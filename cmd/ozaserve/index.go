package main

import (
	"net/http"
)

type indexArchive struct {
	Slug        string
	Title       string
	Description string
	Filename    string
	DateVal     string
	EntryCount  int
}

type indexData struct {
	Archives      []indexArchive
	SingleArchive bool
	ShowInfo      bool
	FooterHTML    string
}

// writeIndexPage renders the library index: a sortable table of archives with
// instant AJAX search, a ZIM dropdown, and a random article button.
func (lib *library) writeIndexPage(w http.ResponseWriter, r *http.Request) {
	h := w.Header()
	h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; connect-src 'self'; base-uri 'none'; form-action 'none'")

	archives := make([]indexArchive, 0, len(lib.slugs))
	for _, slug := range lib.slugs {
		e := lib.archives[slug]
		archives = append(archives, indexArchive{
			Slug:        slug,
			Title:       e.title,
			Description: e.description,
			Filename:    e.filename,
			DateVal:     e.date,
			EntryCount:  int(e.archive.EntryCount()),
		})
	}

	renderTemplate(w, "index.html", indexData{
		Archives:      archives,
		SingleArchive: len(lib.slugs) == 1,
		ShowInfo:      !lib.noInfo,
		FooterHTML:    footerBarHTML(!lib.noInfo),
	})
}
