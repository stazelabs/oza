package ozawrite_test

import (
	"fmt"
	"log"
	"os"

	"github.com/stazelabs/oza/oza"
	"github.com/stazelabs/oza/ozawrite"
)

func ExampleNewWriter() {
	f, err := os.CreateTemp("", "example*.oza")
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(f.Name())
	defer f.Close()

	w := ozawrite.NewWriter(f, ozawrite.WriterOptions{ZstdLevel: 3})

	// Required metadata.
	w.SetMetadata("title", "My Archive")
	w.SetMetadata("language", "en")
	w.SetMetadata("creator", "example")
	w.SetMetadata("date", "2026-01-01")
	w.SetMetadata("source", "https://example.com")

	// Add content entries.
	id, err := w.AddEntry("index.html", "Home", "text/html",
		[]byte("<html><body>Hello</body></html>"), true)
	if err != nil {
		log.Fatal(err)
	}

	// Add a redirect.
	_, err = w.AddRedirect("home", "Home Redirect", id)
	if err != nil {
		log.Fatal(err)
	}

	// Close triggers the full assembly pipeline.
	if err := w.Close(); err != nil {
		log.Fatal(err)
	}

	// Verify the archive can be opened.
	f.Close()
	a, err := oza.Open(f.Name())
	if err != nil {
		log.Fatal(err)
	}
	defer a.Close()

	title, _ := a.Metadata("title")
	fmt.Println(title)
	fmt.Println(a.EntryCount())
	// Output:
	// My Archive
	// 1
}

func ExampleWriter_AddEntry() {
	f, err := os.CreateTemp("", "example*.oza")
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(f.Name())
	defer f.Close()

	w := ozawrite.NewWriter(f, ozawrite.WriterOptions{ZstdLevel: 1})
	w.SetMetadata("title", "Multi-Type Archive")
	w.SetMetadata("language", "en")
	w.SetMetadata("creator", "example")
	w.SetMetadata("date", "2026-01-01")
	w.SetMetadata("source", "https://example.com")

	entries := []struct {
		path, title, mime string
		content           []byte
		front             bool
	}{
		{"index.html", "Home", "text/html", []byte("<html><body>Home</body></html>"), true},
		{"data.json", "Data", "application/json", []byte(`{"key":"value"}`), false},
		{"style.css", "Style", "text/css", []byte("body{margin:0}"), false},
	}

	for _, e := range entries {
		if _, err := w.AddEntry(e.path, e.title, e.mime, e.content, e.front); err != nil {
			log.Fatal(err)
		}
	}

	if err := w.Close(); err != nil {
		log.Fatal(err)
	}

	f.Close()
	a, err := oza.Open(f.Name())
	if err != nil {
		log.Fatal(err)
	}
	defer a.Close()

	fmt.Println(a.EntryCount())
	for e := range a.Entries() {
		fmt.Printf("%s: %s\n", e.Path(), e.MIMEType())
	}
	// Output:
	// 3
	// index.html: text/html
	// data.json: application/json
	// style.css: text/css
}
