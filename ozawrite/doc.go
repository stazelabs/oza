// Package ozawrite produces OZA archive files.
//
// Create a [Writer] with [NewWriter], call [Writer.SetMetadata] to set required
// metadata keys (title, language, creator, date, source), then call
// [Writer.AddEntry] and [Writer.AddRedirect] to populate the archive. Calling
// [Writer.Close] triggers the full assembly pipeline: metadata validation,
// optional Zstd dictionary training, chunk building, compression, index
// construction, SHA-256 checksum computation at section and file level, and
// final binary layout with a backfilled header and section table.
//
// The underlying [io.WriteSeeker] must also implement [io.Reader]; in practice
// callers pass an *os.File opened with os.Create.
//
// Basic usage:
//
//	f, _ := os.Create("output.oza")
//	w := ozawrite.NewWriter(f, ozawrite.WriterOptions{ZstdLevel: 3})
//	w.SetMetadata("title", "My Archive")
//	w.AddEntry("index.html", "Home", "text/html", content, true)
//	w.Close()
//	f.Close()
package ozawrite
