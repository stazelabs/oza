# ozaserve

HTTP server for browsing OZA archive content. Serves one or more OZA files with a web interface, full-text search, browsing, diagnostic introspection tools, an MCP server, and a JSON API.

## Usage

```
ozaserve [file.oza ...] [--dir <dir>] [flags]
```

OZA files can be specified as positional arguments, via `--dir`, or both.

### Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--addr` | `-a` | `:8080` | Listen address (`host:port`) |
| `--cache` | `-c` | `64` | Decompressed chunk cache size per archive (see [Chunk Cache](#chunk-cache)) |
| `--dir` | `-d` | | Directory of OZA files to serve (repeatable) |
| `--recursive` | `-r` | `false` | Scan `--dir` directories recursively |
| `--no-info` | | `false` | Disable all `/-/info` pages and hide the info icon in the library index |
| `--mcp` | | `false` | Enable MCP server on stdio (runs HTTP + MCP simultaneously) |

### Examples

```sh
# Serve a single OZA file
ozaserve wikipedia_en.oza

# Serve all OZA files in a directory
ozaserve --dir /data/oza

# Custom port, recursive scan, larger cache
ozaserve -a :9090 -c 128 -r -d /data/oza -d /more/ozas

# Public-facing server: disable diagnostic info pages
ozaserve --no-info --dir /data/oza

# Run HTTP server + MCP server on stdio simultaneously
ozaserve --mcp --dir /data/oza
```

## URL Slugs

Each OZA file is assigned a URL slug derived from its filename:

- The `.oza` extension is stripped
- Trailing underscore-separated segments that start with a digit are removed
- Example: `wikipedia_en_all_2024-01.oza` becomes `wikipedia_en_all`

Duplicate slugs are disambiguated with a numeric suffix (`_2`, `_3`, etc.). Slugs are assigned in sorted path order for deterministic results.

## Routes

### Library Index

```
GET /
```

HTML page listing all loaded OZA archives in a sortable table. Columns: Title, File, Date, Entries. Each row also has:

- **Action links** under the title: Browse, Search, Random
- **Info link** (a ⓘ icon) linking to the archive's diagnostic info page

The page includes:

- **Instant search box** — queries the JSON search API with 200ms debounce, displaying results in a dropdown overlay
- **Archive dropdown** — when multiple archives are loaded, a dropdown scopes search to a specific archive or "All". Hidden when only one archive is loaded.
- **Random button** — navigates to a random front article. Respects the dropdown selection.

The table is sortable by clicking column headers. An arrow indicator shows the current sort column and direction.

### Global Favicon

```
GET /favicon.ico
GET /_favicon.svg
```

Both routes serve the same embedded SVG (王座 kanji). Includes `Cache-Control: public, max-age=31536000, immutable`.

### Documentation

```
GET /_docs
```

Renders `ozaserve.md` (this document) as an HTML page with table support. Content is pre-rendered at startup from the embedded Markdown source.

### Server Info

```
GET /_info
```

Process-wide runtime overview page. Disabled when `--no-info` is set. Contains:

- **Process** — uptime, start time, Go version, goroutine count, heap stats, GC cycles
- **Archives** — sortable table with one row per archive: name, file size, structural RAM, total RAM (est.), entries, redirects, chunks, cache fill and hit rate, load time; totals row at the bottom
- **Build** — main module path, module version (if not a dev build), and build settings (GOOS, GOARCH, VCS info, etc.)
- **Dependencies** — alphabetically sorted table of all compiled-in modules with their versions and any replace directives

### OZA Root / Main Page

```
GET /{slug}/
```

Redirects (302) to the archive's main page entry. Returns 404 if the archive has no main entry.

### Content

```
GET /{slug}/{path...}
```

Serves content from the archive. The `{path}` maps to an entry path inside the archive.

**Behavior:**

- OZA-internal redirects are followed and returned as HTTP 302 redirects
- `Content-Type` is set from the entry's MIME type. For `text/*` types that lack a charset, `; charset=utf-8` is appended automatically
- `Content-Length` is set for all responses

**HTML content injection:** For `text/html` entries, a sticky navigation header bar and fixed footer bar are injected (see [Navigation UI](#navigation-ui)).

**Caching headers** (OZA content is immutable):

- `Cache-Control: public, max-age=31536000, immutable`
- `ETag` derived from `MD5(archive_uuid_hex + entry_path)`
- Supports `If-None-Match` conditional requests (returns 304)

### Title Search (JSON API)

```
GET /{slug}/_search?q={query}
GET /_search?q={query}
```

Returns a JSON array of search results using the archive's trigram title index. The per-slug endpoint searches a single archive; the global `/_search` endpoint searches across all loaded archives (iterating in slug order, stopping once the limit is reached).

**Response format:**

```json
[
  {"path": "/{slug}/Article_Title", "title": "Article Title"},
  ...
]
```

Returns `[]` for empty queries. Limited to 20 results per archive.

### Title Search (HTML Page)

```
GET /{slug}/-/search?q={query}&limit={n}&format={html|json}
```

| Parameter | Default | Description |
|-----------|---------|-------------|
| `q` | | Search query |
| `limit` | `25` | Max results (1–100) |
| `format` | `html` | Response format: `html` or `json` |

**HTML format** (default): Renders a search page with a form and clickable results list. Shows result count, or "No results found" for zero matches.

**JSON format** (`format=json`): Same response format as the `/_search` endpoint.

### Browse by Letter

```
GET /{slug}/-/browse?letter={A-Z|#}&offset={n}&limit={n}
```

| Parameter | Default | Description |
|-----------|---------|-------------|
| `letter` | | Letter to browse: `A`–`Z` or `#` (non-alphabetic) |
| `offset` | `0` | Pagination offset |
| `limit` | `50` | Results per page (1–200) |

HTML page with an A–Z + `#` letter bar. Letters with zero matching entries appear greyed out. The currently selected letter is highlighted.

### Random Article

```
GET /{slug}/-/random
GET /_random
```

Redirects (302) to a random front article. Front articles are identified at startup (O(N) scan) and stored for O(1) random selection at request time.

**Per-slug** (`/{slug}/-/random`): Picks a random front article from the named archive.

**Global** (`/_random`): Picks a random archive weighted by front article count, then picks a random front article from it.

### Archive Info / Diagnostics

```
GET /{slug}/-/info
```

Disabled when `--no-info` is set.

Diagnostic overview page for a single OZA archive:

- **Format** — filename, UUID, version, entry count, content size, chunk target size, redirect count, chunk count, front articles, section count, compression ratio, main entry
- **Runtime** — file size on disk, structural RAM (exact), map overhead (est.), total RAM (est.), chunk cache fill and hit rate, load time
- **Indices** — presence of path index, title index, title search index, body search index; document counts where available
- **Metadata** — all metadata key/value pairs from the archive; binary values shown as `<binary N bytes>`; illustration metadata rendered as inline thumbnail images
- **MIME Types** — the archive's registered MIME type list with index numbers
- **Sections** — table of all sections with type, compressed size, uncompressed size, compression algorithm, and SHA-256 (truncated)

## Navigation UI

ozaserve injects UI elements into served HTML pages to aid navigation.

### Header Bar

A sticky navigation bar is injected after the opening `<body>` tag of every `text/html` content page. It contains:

- Library link (王座 icon) — returns to the library index
- Archive title link — returns to the archive's main page
- Search form — submits to `/{slug}/-/search`
- Random button — links to `/{slug}/-/random`
- A–Z letter bar — links to `/{slug}/-/browse?letter=X`. Letters with no matching entries appear greyed out

The bar uses `position: sticky` with `z-index: 999999` so it stays visible while scrolling.

### Footer Bar

A fixed footer bar is injected before the closing `</body>` tag of every HTML page. It contains:

- A link to the ozaserve GitHub repository with a GitHub icon
- A link to the Apache 2.0 license
- A "Server info" link to `/_info` (hidden when `--no-info` is set)

The bar uses `position: fixed` at the bottom with `z-index: 999998`. A `padding-bottom` rule is added to `body` to prevent content from being obscured.

### Injection Behavior

- Tag matching is case-insensitive (`<body>`, `<BODY>`, `<Body>` all work)
- If no `<body>` tag is found, the header bar is prepended to the content
- If no `</body>` tag is found, the footer bar is appended to the content
- Both bars are fully self-contained (inline CSS, no external dependencies)

## HTTP Details

### Allowed Methods

Only `GET` and `HEAD` are accepted. All other methods return `405 Method Not Allowed` with an `Allow: GET, HEAD` header.

### Security Headers

Applied to every response:

| Header | Value |
|--------|-------|
| `X-Content-Type-Options` | `nosniff` |
| `X-Frame-Options` | `SAMEORIGIN` |
| `Referrer-Policy` | `strict-origin-when-cross-origin` |

The library index page also sets a `Content-Security-Policy` restricting scripts, styles, and connections to same-origin.

### Server Timeouts

| Timeout | Value |
|---------|-------|
| Read | 30s |
| Write | 60s |
| Idle | 120s |

### Graceful Shutdown

On `SIGINT` or `SIGTERM`, the server:

1. Stops accepting new connections
2. Drains in-flight requests (10-second timeout)
3. Closes all OZA archives cleanly
4. Exits

## OZA Loading

- Positional arguments are **hard failures** — if a file can't be opened, ozaserve exits with an error
- Files discovered via `--dir` are **soft failures** — invalid files are logged and skipped
- At least one valid OZA archive must be loaded, otherwise ozaserve exits with an error
- Recursive directory scanning (`-r`) does not follow symlinked directories to avoid cycles
- Paths are deduplicated by absolute path and sorted for deterministic slug assignment
- Library is sorted alphabetically by title (case-insensitive) for display

Archives are opened concurrently at startup. Each archive logs its progress:

```
loading: wikipedia_en_all_2024-01.oza
ready:   wikipedia_en_all_2024-01.oza — 6,458,209 entries (2.34s)
```

At startup, each archive is also scanned (O(N)) to:

- Build an A–Z letter count map for the browse page navigation bar
- Collect front article IDs for O(1) random article selection

## Chunk Cache

OZA content is stored in compressed chunks (Zstd or Zstd+dict). The `--cache` flag (`-c`) sets the size of a per-archive FIFO cache of decompressed chunks. The default is 64.

**Memory cost:** The cache holds decompressed chunk data. Per-chunk sizes vary by archive and chunk target size configuration.

**Sizing guidance:** A typical article page access touches 1 chunk for the HTML body plus several more for CSS, JS, and images. A cache of 32–64 is usually adequate for single-user browsing. For concurrent users or archives with heavy image use, 128–256 may be appropriate.

**Runtime diagnostics:** The `/-/info` page for each archive shows a **Chunk Cache** row and **Cache Hit Rate** row with live stats. The `/_info` server info page shows cache stats for all archives in a summary table.

## MCP Server

When `--mcp` is set, ozaserve runs an MCP (Model Context Protocol) server on stdio alongside the HTTP server. The MCP server exposes archive content as resources and tools for AI assistant integration.

The HTTP server starts on the configured address; the MCP server runs on stdin/stdout. When the MCP client disconnects, the HTTP server is shut down gracefully and the process exits.

## Metadata

ozaserve reads these OZA metadata keys (all optional) for the library index and info pages:

| Key | Used for |
|-----|----------|
| `title` | Display name (falls back to slug if absent) |
| `description` | Subtitle under title in the library index |
| `date` | Date column in the library index |
| `chunk_target_size` | Shown in the archive info Format section |
