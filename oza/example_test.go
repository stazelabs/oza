package oza_test

import (
	"fmt"
	"log"
	"os"

	"github.com/stazelabs/oza/oza"
	"github.com/stazelabs/oza/ozawrite"
)

// createExampleArchive writes a small OZA archive to a temp file and returns
// the path. The caller must remove the file when done.
func createExampleArchive(search bool) string {
	f, err := os.CreateTemp("", "example*.oza")
	if err != nil {
		log.Fatal(err)
	}
	w := ozawrite.NewWriter(f, ozawrite.WriterOptions{
		ZstdLevel:   1,
		BuildSearch: search,
	})
	w.SetMetadata("title", "Example Archive")
	w.SetMetadata("language", "en")
	w.SetMetadata("creator", "test")
	w.SetMetadata("date", "2026-01-01")
	w.SetMetadata("source", "https://example.com")

	w.AddEntry("index.html", "Home", "text/html",
		[]byte("<html><body><h1>Welcome</h1><p>Hello, world!</p></body></html>"), true)
	w.AddEntry("about.html", "About", "text/html",
		[]byte("<html><body><h1>About</h1><p>This is the about page.</p></body></html>"), true)
	w.AddEntry("style.css", "Style", "text/css",
		[]byte("body { color: #333; }"), false)

	if err := w.Close(); err != nil {
		f.Close()
		log.Fatal(err)
	}
	f.Close()
	return f.Name()
}

func ExampleOpen() {
	path := createExampleArchive(false)
	defer os.Remove(path)

	a, err := oza.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer a.Close()

	title, _ := a.Metadata("title")
	fmt.Println(title)
	fmt.Println(a.EntryCount())
	// Output:
	// Example Archive
	// 3
}

func ExampleArchive_EntryByPath() {
	path := createExampleArchive(false)
	defer os.Remove(path)

	a, err := oza.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer a.Close()

	entry, err := a.EntryByPath("about.html")
	if err != nil {
		log.Fatal(err)
	}

	content, err := entry.ReadContent()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(entry.Title())
	fmt.Println(entry.MIMEType())
	fmt.Println(len(content) > 0)
	// Output:
	// About
	// text/html
	// true
}

func ExampleArchive_Entries() {
	path := createExampleArchive(false)
	defer os.Remove(path)

	a, err := oza.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer a.Close()

	for e := range a.Entries() {
		fmt.Printf("%s (%s)\n", e.Path(), e.MIMEType())
	}
	// Output:
	// index.html (text/html)
	// about.html (text/html)
	// style.css (text/css)
}

func ExampleArchive_Search() {
	path := createExampleArchive(true)
	defer os.Remove(path)

	a, err := oza.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer a.Close()

	results, err := a.Search("about", oza.SearchOptions{Limit: 10})
	if err != nil {
		log.Fatal(err)
	}

	for _, r := range results {
		fmt.Printf("%s (title_match=%v)\n", r.Entry.Title(), r.TitleMatch)
	}
	// Output:
	// About (title_match=true)
}

func ExampleOpenWithOptions() {
	path := createExampleArchive(false)
	defer os.Remove(path)

	a, err := oza.OpenWithOptions(path,
		oza.WithCacheSize(32),
		oza.WithMaxDecompressedSize(64<<20),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer a.Close()

	fmt.Println(a.EntryCount())
	// Output:
	// 3
}
