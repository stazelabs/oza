package main

import (
	"encoding/json"
	"net/http"
)

type indexArchive struct {
	Slug        string
	Title       string
	Creator     string // optional; shown under title if non-empty
	Description string
	Filename    string
	DateVal     string
	EntryCount  int
	EntryPath   string // optional; if set, title links here instead of /{Slug}/

	// Collection grouping: when IsGroupHeader is true this row is a
	// lightweight header for a collection archive. Catalog items follow
	// immediately after with IsGroupItem set.
	IsGroupHeader bool
	IsGroupItem   bool
	GroupSize     int // number of items in this collection (header row only)
}

type indexData struct {
	Archives      []indexArchive
	SingleArchive bool
	ShowInfo      bool
	FooterHTML    string
}

// catalogEntry is the JSON schema stored in the "catalog" metadata key by
// collection converters (e.g. epub2oza --collection). Each entry describes
// one logical item (book, document) within the archive.
type catalogEntry struct {
	Slug     string `json:"slug"`
	Title    string `json:"title"`
	Creator  string `json:"creator"`
	Language string `json:"language"`
	Entry    string `json:"entry"`
	Entries  int    `json:"entries"`
}

// writeIndexPage renders the library index: a sortable table of archives with
// instant AJAX search, a ZIM dropdown, and a random article button.
func (lib *library) writeIndexPage(w http.ResponseWriter, r *http.Request) {
	h := w.Header()
	h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; connect-src 'self'; base-uri 'none'; form-action 'none'")

	archives := make([]indexArchive, 0, len(lib.slugs))
	for _, slug := range lib.slugs {
		e := lib.archives[slug]

		// If the archive has a catalog, render a group header followed by
		// one row per catalog item.
		if items := parseCatalog(e); len(items) > 0 {
			// Group header row — shows collection title, filename, date, total entries, info link.
			archives = append(archives, indexArchive{
				Slug:          slug,
				Title:         e.title,
				Description:   e.description,
				Filename:      e.filename,
				DateVal:       e.date,
				EntryCount:    int(e.archive.EntryCount()),
				IsGroupHeader: true,
				GroupSize:     len(items),
			})
			// One row per catalog item.
			for _, item := range items {
				entryCount := item.Entries
				if entryCount == 0 {
					entryCount = int(e.archive.EntryCount())
				}
				archives = append(archives, indexArchive{
					Slug:        slug,
					Title:       item.Title,
					Creator:     item.Creator,
					EntryCount:  entryCount,
					EntryPath:   "/" + slug + "/" + item.Entry,
					IsGroupItem: true,
				})
			}
			continue
		}

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

// parseCatalog reads the "catalog" metadata key and returns the decoded items,
// or nil if the key is absent or malformed.
func parseCatalog(e *archiveEntry) []catalogEntry {
	raw, err := e.archive.Metadata("catalog")
	if err != nil {
		return nil
	}
	var items []catalogEntry
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil
	}
	return items
}
