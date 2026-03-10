// Package oza provides a pure Go implementation for reading OZA archives.
//
// OZA (Open Zipped Archive) is a modern replacement for the ZIM file format,
// designed for offline content distribution. It features extensible section
// tables, fixed-size entry records, Zstd compression with dictionary support,
// three-tier SHA-256 integrity, and a built-in trigram search index.
//
// Open an OZA file with [Open] or [OpenWithOptions], then use [Archive] methods
// to look up entries by path, title, or ID. Content is accessed through
// [Entry.ReadContent] which resolves redirects automatically.
//
// Basic usage:
//
//	a, err := oza.Open("archive.oza")
//	if err != nil { log.Fatal(err) }
//	defer a.Close()
//
//	entry, _ := a.EntryByPath("index.html")
//	content, _ := entry.ReadContent()
package oza
