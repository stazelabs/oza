package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"

	"github.com/stazelabs/oza/cmd/internal/loadutil"
	"github.com/stazelabs/oza/oza"
)

func main() {
	var addr string
	var cacheSize int
	var dirs []string
	var recursive bool
	var noInfo bool
	var mcpEnabled bool

	cmd := &cobra.Command{
		Use:   "ozaserve [file.oza ...] [--dir <dir>]",
		Short: "Serve OZA archives over HTTP",
		Long: `王座 ozaserve — HTTP server for browsing OZA archive content.

Serves one or more OZA files at http://localhost:8080 (by default).
Each archive is accessible under a URL slug derived from its filename.
If only one archive is loaded, the root URL redirects to its main page.

OZA files may be specified as positional arguments, via --dir, or both.`,
		Args: func(cmd *cobra.Command, args []string) error {
			d, _ := cmd.Flags().GetStringArray("dir")
			if len(args) == 0 && len(d) == 0 {
				return errors.New("at least one OZA file or --dir required")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return serve(args, dirs, recursive, addr, cacheSize, noInfo, mcpEnabled)
		},
	}

	cmd.Flags().StringVarP(&addr, "addr", "a", ":8080", "listen address (host:port)")
	cmd.Flags().IntVarP(&cacheSize, "cache", "c", 64, "chunk cache size per archive")
	cmd.Flags().StringArrayVarP(&dirs, "dir", "d", nil, "directory of OZA files to serve (repeatable)")
	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "scan --dir directories recursively")
	cmd.Flags().BoolVar(&noInfo, "no-info", false, "disable info pages")
	cmd.Flags().BoolVar(&mcpEnabled, "mcp", false, "enable MCP server on stdio (runs HTTP + MCP simultaneously)")

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

type library struct {
	archives  map[string]*archiveEntry
	slugs     []string // sorted by title (ascending)
	noInfo    bool
	startTime time.Time
}

type archiveEntry struct {
	archive         *oza.Archive
	slug            string
	filename        string
	title           string
	description     string
	date            string
	uuidHex         string
	letterCounts    map[byte]int // A–Z -> count of entries whose title starts with that letter
	frontArticleIDs []uint32     // IDs of front-article entries, for random navigation
	fileSize        int64
	loadDuration    time.Duration
}

func serve(paths []string, dirs []string, recursive bool, addr string, cacheSize int, noInfo bool, mcpEnabled bool) error {
	initTemplates()
	startTime := time.Now()
	dirPaths := collectOZAPaths(dirs, recursive)
	allPaths := append(paths, dirPaths...)
	lib, err := loadLibrary(allPaths, len(paths), cacheSize)
	if err != nil {
		return err
	}
	lib.noInfo = noInfo
	lib.startTime = startTime
	defer func() {
		for _, e := range lib.archives {
			e.archive.Close()
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/", lib.handleRoot)
	mux.HandleFunc("/favicon.ico", handleFaviconSVG)
	mux.HandleFunc("/_favicon.svg", handleFaviconSVG)
	mux.HandleFunc("/_docs", handleDocs)
	mux.HandleFunc("/_random", lib.handleRandomAll)
	mux.HandleFunc("/_search", lib.handleSearchAll)
	mux.HandleFunc("/{archive}/_search", lib.handleSearchJSON)
	mux.HandleFunc("/{archive}/-/search", lib.handleSearchPage)
	mux.HandleFunc("/{archive}/-/random", lib.handleRandom)
	mux.HandleFunc("/{archive}/-/browse", lib.handleBrowse)
	if !noInfo {
		mux.HandleFunc("/_info", lib.handleGlobalInfo)
		mux.HandleFunc("/{archive}/-/info", lib.handleInfo)
		mux.HandleFunc("/{archive}/-/info.json", lib.handleInfoJSON)
	}
	mux.HandleFunc("/{archive}/{path...}", lib.handleContent)

	srv := &http.Server{
		Addr:              addr,
		Handler:           securityHeaders(methodCheck(mux)),
		ReadTimeout:       30 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	if mcpEnabled {
		return serveMCP(srv, lib, addr)
	}
	return serveHTTP(srv, addr)
}

// serveMCP runs the HTTP server in a background goroutine and the MCP server
// on stdio in the foreground. When the MCP client disconnects, the HTTP server
// is shut down and the process exits.
func serveMCP(srv *http.Server, lib *library, addr string) error {
	// Compute base URL from listen address.
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("parsing addr %q: %w", addr, err)
	}
	if host == "" {
		host = "localhost"
	}
	baseURL := "http://" + net.JoinHostPort(host, port)

	// Start HTTP server in background.
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", addr, err)
	}
	go func() {
		log.Printf("HTTP listening on %s", addr)
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	// Create and run MCP server on stdio.
	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:    "ozaserve",
		Version: "0.1.0",
	}, nil)
	registerMCPTools(mcpServer, lib, baseURL)
	registerMCPResources(mcpServer, lib, baseURL)

	log.Printf("MCP server running on stdio (base URL: %s)", baseURL)
	mcpErr := mcpServer.Run(context.Background(), &mcp.StdioTransport{})

	// MCP client disconnected — shut down HTTP.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("shutdown: %v", err)
	}

	return mcpErr
}

// serveHTTP runs the HTTP server with graceful shutdown on SIGINT/SIGTERM.
func serveHTTP(srv *http.Server, addr string) error {
	done := make(chan error, 1)
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigCh
		log.Printf("received %v, shutting down...", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		done <- srv.Shutdown(ctx)
	}()

	log.Printf("listening on %s", addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return <-done
}

// collectOZAPaths delegates to the shared loadutil package.
func collectOZAPaths(dirs []string, recursive bool) []string {
	return loadutil.CollectOZAPaths(dirs, recursive)
}

// loadLibrary opens archives and populates the library. The first hardFailCount
// paths are positional arguments (hard failures); the rest come from --dir (soft failures).
// Archives are opened concurrently; results are processed in original order so slug
// assignment is deterministic.
func loadLibrary(paths []string, hardFailCount int, cacheSize int) (*library, error) {
	type loadResult struct {
		archive         *oza.Archive
		letterCounts    map[byte]int
		frontArticleIDs []uint32
		fileSize        int64
		loadDuration    time.Duration
		err             error
	}
	results := make([]loadResult, len(paths))

	var wg sync.WaitGroup
	for i, path := range paths {
		wg.Add(1)
		go func(i int, path string) {
			defer wg.Done()
			log.Printf("loading: %s", filepath.Base(path))
			t0 := time.Now()
			a, err := oza.OpenWithOptions(path, oza.WithCacheSize(cacheSize))
			if err != nil {
				results[i] = loadResult{err: err}
				return
			}
			lc := computeLetterCounts(a)
			fa := collectFrontArticleIDs(a)
			dur := time.Since(t0)
			var fsz int64
			if fi, serr := os.Stat(path); serr == nil {
				fsz = fi.Size()
			}
			log.Printf("ready:   %s — %d entries (%.1fs)", filepath.Base(path), a.EntryCount(), dur.Seconds())
			results[i] = loadResult{archive: a, letterCounts: lc, frontArticleIDs: fa, fileSize: fsz, loadDuration: dur}
		}(i, path)
	}
	wg.Wait()

	lib := &library{archives: make(map[string]*archiveEntry)}
	for i, res := range results {
		if res.err != nil {
			if i < hardFailCount {
				// Close any successfully opened archives before returning.
				for _, r := range results {
					if r.archive != nil {
						r.archive.Close()
					}
				}
				return nil, fmt.Errorf("opening %s: %w", paths[i], res.err)
			}
			log.Printf("warning: skipping %s: %v", paths[i], res.err)
			continue
		}

		a := res.archive
		slug := makeSlug(paths[i])
		base := slug
		for j := 2; lib.archives[slug] != nil; j++ {
			slug = fmt.Sprintf("%s_%d", base, j)
		}

		title, _ := a.Metadata("title")
		if title == "" {
			title = slug
		}
		desc, _ := a.Metadata("description")
		date, _ := a.Metadata("date")
		uuid := a.UUID()

		lib.archives[slug] = &archiveEntry{
			archive:         a,
			slug:            slug,
			filename:        filepath.Base(paths[i]),
			title:           title,
			description:     desc,
			date:            date,
			uuidHex:         hex.EncodeToString(uuid[:]),
			letterCounts:    res.letterCounts,
			frontArticleIDs: res.frontArticleIDs,
			fileSize:        res.fileSize,
			loadDuration:    res.loadDuration,
		}
		lib.slugs = append(lib.slugs, slug)
	}

	if len(lib.slugs) == 0 {
		return nil, errors.New("no valid OZA files found")
	}
	sort.Slice(lib.slugs, func(i, j int) bool {
		return strings.ToLower(lib.archives[lib.slugs[i]].title) <
			strings.ToLower(lib.archives[lib.slugs[j]].title)
	})
	return lib, nil
}

// computeLetterCounts scans the title index once at load time to build an A–Z
// count map used by the navigation bar and browse page.
// Uses ForEachTitleKey (O(N)) rather than EntriesByTitle (O(N×restartInterval/2)).
func computeLetterCounts(a *oza.Archive) map[byte]int {
	counts := make(map[byte]int, 26)
	a.ForEachTitleKey(func(t string) {
		if len(t) == 0 {
			return
		}
		c := t[0]
		if c >= 'a' && c <= 'z' {
			c -= 32
		}
		if c >= 'A' && c <= 'Z' {
			counts[c]++
		}
	})
	return counts
}

// collectFrontArticleIDs gathers IDs of all front-article entries at load time
// for O(1) random article selection at request time.
// Uses ForEachEntryRecord to skip the idToPath/idToTitle lookups.
func collectFrontArticleIDs(a *oza.Archive) []uint32 {
	var ids []uint32
	a.ForEachEntryRecord(func(id uint32, rec oza.EntryRecord) {
		if rec.IsFrontArticle() {
			ids = append(ids, id)
		}
	})
	return ids
}

// makeSlug delegates to the shared loadutil package.
func makeSlug(path string) string {
	return loadutil.MakeSlug(path)
}

// makeETag generates an ETag for a content entry from the archive UUID and path.
// Uses SHA-256 truncated to 16 bytes for a compact, non-cryptographic-MD5 identifier.
func makeETag(ae *archiveEntry, entryPath string) string {
	h := sha256.New()
	h.Write([]byte(ae.uuidHex))
	h.Write([]byte(entryPath))
	sum := h.Sum(nil)
	return `"` + hex.EncodeToString(sum[:16]) + `"`
}

// faviconSVG is the 王座 kanji served at /favicon.ico and /_favicon.svg.
const faviconSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><text y=".88em" font-size="55" fill="#C9A84C">&#x738B;&#x5EA7;</text></svg>`

func handleFaviconSVG(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Write([]byte(faviconSVG))
}

func (lib *library) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		write404(w, r)
		return
	}
	lib.writeIndexPage(w, r)
}

// securityHeaders adds OWASP-recommended response headers to every response.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "SAMEORIGIN")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

// methodCheck rejects any method other than GET and HEAD.
func methodCheck(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		next.ServeHTTP(w, r)
	})
}
