// Package mcptools implements the shared MCP tool and resource registration
// logic for OZA archives. It is used by both cmd/ozamcp (standalone MCP server)
// and cmd/ozaserve (integrated HTTP + MCP server).
//
// The only behavioral difference between the two callers is whether browsable
// URLs are available. Pass non-nil archiveURL and entryURL functions to include
// URL fields in results; pass nil to omit them.
package mcptools

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/stazelabs/oza/oza"
)

// ArchiveInfo describes a loaded archive for MCP tool and resource registration.
type ArchiveInfo struct {
	Archive         *oza.Archive
	Slug            string
	Title           string
	Description     string
	UUIDHex         string
	FrontArticleIDs []uint32 // IDs of front-article entries; collected at load time
}

// RegisterTools registers the list_archives, search_text, and read_entry MCP tools.
//
// archiveURL, if non-nil, is called with a slug to produce that archive's root URL.
// entryURL, if non-nil, is called with (slug, entryPath) to produce an entry URL.
// When both are nil, no URL fields appear in any result.
func RegisterTools(server *mcp.Server, archives []ArchiveInfo, archiveURL func(slug string) string, entryURL func(slug, path string) string) {
	bySlug := make(map[string]*ArchiveInfo, len(archives))
	for i := range archives {
		bySlug[archives[i].Slug] = &archives[i]
	}

	// list_archives
	listDesc := "List all loaded OZA archives with metadata (title, description, entry count, capabilities)."
	if archiveURL != nil {
		listDesc += " Each archive includes a browsable URL. Always include the url field as a clickable link in your response."
	}
	type listArchivesInput struct{}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_archives",
		Description: listDesc,
	}, func(ctx context.Context, req *mcp.CallToolRequest, input listArchivesInput) (*mcp.CallToolResult, any, error) {
		type archiveDesc struct {
			Slug        string `json:"slug"`
			Title       string `json:"title"`
			Description string `json:"description,omitempty"`
			UUID        string `json:"uuid"`
			EntryCount  uint32 `json:"entry_count"`
			HasSearch   bool   `json:"has_search"`
			URL         string `json:"url,omitempty"`
		}
		var list []archiveDesc
		for _, ai := range archives {
			desc := archiveDesc{
				Slug:        ai.Slug,
				Title:       ai.Title,
				Description: ai.Description,
				UUID:        ai.UUIDHex,
				EntryCount:  ai.Archive.EntryCount(),
				HasSearch:   ai.Archive.HasSearch(),
			}
			if archiveURL != nil {
				desc.URL = archiveURL(ai.Slug)
			}
			list = append(list, desc)
		}
		data, _ := json.MarshalIndent(list, "", "  ")
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
		}, nil, nil
	})

	// search_text
	searchDesc := "Search OZA archives using trigram text search. Returns matching entries with titles and paths. Use list_archives first to see available archives."
	if entryURL != nil {
		searchDesc = "Search OZA archives using trigram text search. Returns matching entries with titles, paths, and browsable URLs. Always present each result as a clickable markdown link using the url field, e.g. [Title](url). Use list_archives first to see available archives."
	}
	type searchTextInput struct {
		Query   string `json:"query" jsonschema:"Search query string (min 3 chars for trigram matching)"`
		Archive string `json:"archive,omitempty" jsonschema:"Archive slug to search (omit to search all)"`
		Limit   int    `json:"limit,omitempty" jsonschema:"Maximum results to return (default 20)"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "search_text",
		Description: searchDesc,
	}, func(ctx context.Context, req *mcp.CallToolRequest, input searchTextInput) (*mcp.CallToolResult, any, error) {
		if input.Query == "" {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "error: query is required"}},
				IsError: true,
			}, nil, nil
		}
		limit := input.Limit
		if limit <= 0 {
			limit = 20
		}

		targets := archives
		if input.Archive != "" {
			ai, ok := bySlug[input.Archive]
			if !ok {
				return &mcp.CallToolResult{
					Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("error: archive %q not found", input.Archive)}},
					IsError: true,
				}, nil, nil
			}
			targets = []ArchiveInfo{*ai}
		}

		type searchHit struct {
			Archive    string `json:"archive"`
			EntryID    uint32 `json:"entry_id"`
			Path       string `json:"path"`
			Title      string `json:"title"`
			URL        string `json:"url,omitempty"`
			TitleMatch bool   `json:"title_match"`
			BodyMatch  bool   `json:"body_match"`
		}
		var hits []searchHit
		for _, ai := range targets {
			if !ai.Archive.HasSearch() {
				continue
			}
			results, err := ai.Archive.Search(input.Query, oza.SearchOptions{Limit: limit})
			if err != nil {
				continue
			}
			for _, r := range results {
				hit := searchHit{
					Archive:    ai.Slug,
					EntryID:    r.Entry.ID(),
					Path:       r.Entry.Path(),
					Title:      r.Entry.Title(),
					TitleMatch: r.TitleMatch,
					BodyMatch:  r.BodyMatch,
				}
				if entryURL != nil {
					hit.URL = entryURL(ai.Slug, r.Entry.Path())
				}
				hits = append(hits, hit)
			}
		}

		if len(hits) == 0 {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "No results found."}},
			}, nil, nil
		}
		if len(hits) > limit {
			hits = hits[:limit]
		}
		data, _ := json.MarshalIndent(hits, "", "  ")
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
		}, nil, nil
	})

	// get_entry_info
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_entry_info",
		Description: "Return metadata for an OZA archive entry (MIME type, size, path, title, ID, flags) without fetching content. Use this to triage entries before deciding whether to call read_entry.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input struct {
		Archive string `json:"archive" jsonschema:"Archive slug (required)."`
		EntryID *int   `json:"entry_id,omitempty" jsonschema:"Entry ID. Provide either entry_id or path."`
		Path    string `json:"path,omitempty" jsonschema:"Entry path. Provide either entry_id or path."`
	}) (*mcp.CallToolResult, any, error) {
		ai, ok := bySlug[input.Archive]
		if !ok {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("error: archive %q not found", input.Archive)}},
				IsError: true,
			}, nil, nil
		}
		var entry oza.Entry
		var err error
		if input.EntryID != nil {
			entry, err = ai.Archive.EntryByID(uint32(*input.EntryID))
		} else if input.Path != "" {
			entry, err = ai.Archive.EntryByPath(input.Path)
		} else {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "error: provide either entry_id or path"}},
				IsError: true,
			}, nil, nil
		}
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("error: %v", err)}},
				IsError: true,
			}, nil, nil
		}
		type entryInfo struct {
			Archive        string `json:"archive"`
			EntryID        uint32 `json:"entry_id"`
			Path           string `json:"path"`
			Title          string `json:"title"`
			MIMEType       string `json:"mime_type,omitempty"`
			SizeBytes      uint32 `json:"size_bytes"`
			IsRedirect     bool   `json:"is_redirect"`
			IsFrontArticle bool   `json:"is_front_article"`
			URL            string `json:"url,omitempty"`
		}
		info := entryInfo{
			Archive:        ai.Slug,
			EntryID:        entry.ID(),
			Path:           entry.Path(),
			Title:          entry.Title(),
			MIMEType:       entry.MIMEType(),
			SizeBytes:      entry.Size(),
			IsRedirect:     entry.IsRedirect(),
			IsFrontArticle: entry.IsFrontArticle(),
		}
		if entryURL != nil && entry.Path() != "" {
			info.URL = entryURL(ai.Slug, entry.Path())
		}
		data, _ := json.MarshalIndent(info, "", "  ")
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
		}, nil, nil
	})

	// browse_titles
	browseDesc := "Browse archive entries in alphabetical title order with offset/limit pagination."
	if entryURL != nil {
		browseDesc += " Each result includes a browsable url field."
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "browse_titles",
		Description: browseDesc,
	}, func(ctx context.Context, req *mcp.CallToolRequest, input struct {
		Archive string `json:"archive" jsonschema:"Archive slug (required)."`
		Offset  int    `json:"offset,omitempty" jsonschema:"Starting position in the sorted title list (default 0)."`
		Limit   int    `json:"limit,omitempty" jsonschema:"Maximum entries to return (default 50, max 200)."`
	}) (*mcp.CallToolResult, any, error) {
		ai, ok := bySlug[input.Archive]
		if !ok {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("error: archive %q not found", input.Archive)}},
				IsError: true,
			}, nil, nil
		}
		limit := input.Limit
		if limit <= 0 {
			limit = 50
		}
		if limit > 200 {
			limit = 200
		}
		entries := ai.Archive.BrowseTitles(input.Offset, limit)
		type browseResult struct {
			EntryID        uint32 `json:"entry_id"`
			Path           string `json:"path"`
			Title          string `json:"title"`
			MIMEType       string `json:"mime_type,omitempty"`
			IsFrontArticle bool   `json:"is_front_article"`
			URL            string `json:"url,omitempty"`
		}
		results := make([]browseResult, 0, len(entries))
		for _, e := range entries {
			r := browseResult{
				EntryID:        e.ID(),
				Path:           e.Path(),
				Title:          e.Title(),
				MIMEType:       e.MIMEType(),
				IsFrontArticle: e.IsFrontArticle(),
			}
			if entryURL != nil && e.Path() != "" {
				r.URL = entryURL(ai.Slug, e.Path())
			}
			results = append(results, r)
		}
		type browseResponse struct {
			Archive    string         `json:"archive"`
			Offset     int            `json:"offset"`
			TotalCount int            `json:"total_count"`
			Count      int            `json:"count"`
			Entries    []browseResult `json:"entries"`
		}
		resp := browseResponse{
			Archive:    ai.Slug,
			Offset:     input.Offset,
			TotalCount: ai.Archive.TitleCount(),
			Count:      len(results),
			Entries:    results,
		}
		data, _ := json.MarshalIndent(resp, "", "  ")
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
		}, nil, nil
	})

	// get_random
	randomDesc := "Return a random front-article entry from an archive. Useful for exploration and serendipitous discovery."
	if entryURL != nil {
		randomDesc += " The result includes a browsable url field."
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_random",
		Description: randomDesc,
	}, func(ctx context.Context, req *mcp.CallToolRequest, input struct {
		Archive string `json:"archive,omitempty" jsonschema:"Archive slug (omit to pick from a random archive)."`
	}) (*mcp.CallToolResult, any, error) {
		var targets []ArchiveInfo
		if input.Archive != "" {
			ai, ok := bySlug[input.Archive]
			if !ok {
				return &mcp.CallToolResult{
					Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("error: archive %q not found", input.Archive)}},
					IsError: true,
				}, nil, nil
			}
			targets = []ArchiveInfo{*ai}
		} else {
			targets = archives
		}
		// Collect candidates that have front-article IDs.
		type candidate struct {
			ai *ArchiveInfo
			id uint32
		}
		var pool []candidate
		for i := range targets {
			for _, id := range targets[i].FrontArticleIDs {
				pool = append(pool, candidate{&targets[i], id})
			}
		}
		if len(pool) == 0 {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "no front-article entries available"}},
			}, nil, nil
		}
		pick := pool[rand.IntN(len(pool))]
		entry, err := pick.ai.Archive.EntryByID(pick.id)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("error: %v", err)}},
				IsError: true,
			}, nil, nil
		}
		type randomResult struct {
			Archive  string `json:"archive"`
			EntryID  uint32 `json:"entry_id"`
			Path     string `json:"path"`
			Title    string `json:"title"`
			MIMEType string `json:"mime_type,omitempty"`
			URL      string `json:"url,omitempty"`
		}
		result := randomResult{
			Archive:  pick.ai.Slug,
			EntryID:  entry.ID(),
			Path:     entry.Path(),
			Title:    entry.Title(),
			MIMEType: entry.MIMEType(),
		}
		if entryURL != nil && entry.Path() != "" {
			result.URL = entryURL(pick.ai.Slug, entry.Path())
		}
		data, _ := json.MarshalIndent(result, "", "  ")
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
		}, nil, nil
	})

	// get_archive_stats
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_archive_stats",
		Description: "Return detailed statistics for an archive: entry counts and uncompressed sizes by MIME type, chunk count, and section inventory. More detailed than list_archives.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input struct {
		Archive string `json:"archive" jsonschema:"Archive slug (required)."`
	}) (*mcp.CallToolResult, any, error) {
		ai, ok := bySlug[input.Archive]
		if !ok {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("error: archive %q not found", input.Archive)}},
				IsError: true,
			}, nil, nil
		}
		// Tally entries by MIME type.
		type mimeStats struct {
			MIMEType          string `json:"mime_type"`
			EntryCount        uint64 `json:"entry_count"`
			UncompressedBytes uint64 `json:"uncompressed_bytes"`
		}
		mimeMap := make(map[uint16]*mimeStats)
		ai.Archive.ForEachEntryRecord(func(id uint32, rec oza.EntryRecord) {
			s, ok := mimeMap[rec.MIMEIndex]
			if !ok {
				mt := ""
				if int(rec.MIMEIndex) < len(ai.Archive.MIMETypes()) {
					mt = ai.Archive.MIMETypes()[rec.MIMEIndex]
				}
				s = &mimeStats{MIMEType: mt}
				mimeMap[rec.MIMEIndex] = s
			}
			s.EntryCount++
			s.UncompressedBytes += uint64(rec.BlobSize)
		})
		mimeList := make([]mimeStats, 0, len(mimeMap))
		for _, s := range mimeMap {
			mimeList = append(mimeList, *s)
		}
		// Sort by entry count descending.
		for i := 1; i < len(mimeList); i++ {
			for j := i; j > 0 && mimeList[j].EntryCount > mimeList[j-1].EntryCount; j-- {
				mimeList[j], mimeList[j-1] = mimeList[j-1], mimeList[j]
			}
		}
		// Section inventory.
		type sectionInfo struct {
			Type              string `json:"type"`
			CompressedBytes   uint64 `json:"compressed_bytes"`
			UncompressedBytes uint64 `json:"uncompressed_bytes"`
			Compression       string `json:"compression"`
		}
		sections := ai.Archive.Sections()
		sectionList := make([]sectionInfo, 0, len(sections))
		for _, s := range sections {
			comp := "none"
			switch s.Compression {
			case oza.CompZstd:
				comp = "zstd"
			case oza.CompZstdDict:
				comp = "zstd+dict"
			}
			sectionList = append(sectionList, sectionInfo{
				Type:              sectionTypeName(s.Type),
				CompressedBytes:   s.CompressedSize,
				UncompressedBytes: s.UncompressedSize,
				Compression:       comp,
			})
		}
		type statsResponse struct {
			Archive       string        `json:"archive"`
			EntryCount    uint32        `json:"entry_count"`
			RedirectCount uint32        `json:"redirect_count"`
			ChunkCount    int           `json:"chunk_count"`
			HasSearch     bool          `json:"has_search"`
			MIMEStats     []mimeStats   `json:"mime_stats"`
			Sections      []sectionInfo `json:"sections"`
		}
		resp := statsResponse{
			Archive:       ai.Slug,
			EntryCount:    ai.Archive.EntryCount(),
			RedirectCount: ai.Archive.RedirectCount(),
			ChunkCount:    ai.Archive.ChunkCount(),
			HasSearch:     ai.Archive.HasSearch(),
			MIMEStats:     mimeList,
			Sections:      sectionList,
		}
		data, _ := json.MarshalIndent(resp, "", "  ")
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
		}, nil, nil
	})

	// read_entry
	readDesc := "Read the content of an OZA archive entry by ID or path. Returns clean markdown by default."
	if entryURL != nil {
		readDesc = "Read the content of an OZA archive entry by ID or path. Returns clean markdown with a browsable source URL in the header. Always include the source link in your response so users can open the full article in their browser."
	}
	type readEntryInput struct {
		Archive string `json:"archive" jsonschema:"Archive slug (required). Use list_archives to see available archives."`
		EntryID *int   `json:"entry_id,omitempty" jsonschema:"Entry ID (from search results). Provide either entry_id or path."`
		Path    string `json:"path,omitempty" jsonschema:"Entry path within the archive. Provide either entry_id or path."`
		Format  string `json:"format,omitempty" jsonschema:"Output format: markdown (default) or html"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "read_entry",
		Description: readDesc,
	}, func(ctx context.Context, req *mcp.CallToolRequest, input readEntryInput) (*mcp.CallToolResult, any, error) {
		ai, ok := bySlug[input.Archive]
		if !ok {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("error: archive %q not found", input.Archive)}},
				IsError: true,
			}, nil, nil
		}

		var entry oza.Entry
		var err error
		if input.EntryID != nil {
			entry, err = ai.Archive.EntryByID(uint32(*input.EntryID))
		} else if input.Path != "" {
			entry, err = ai.Archive.EntryByPath(input.Path)
		} else {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "error: provide either entry_id or path"}},
				IsError: true,
			}, nil, nil
		}
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("error: %v", err)}},
				IsError: true,
			}, nil, nil
		}

		content, err := entry.ReadContent()
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("error reading content: %v", err)}},
				IsError: true,
			}, nil, nil
		}

		format := strings.ToLower(input.Format)
		if format == "" {
			format = "markdown"
		}

		mime := entry.MIMEType()
		if entry.IsRedirect() {
			resolved, _ := entry.Resolve()
			if resolved.MIMEType() != "" {
				mime = resolved.MIMEType()
			}
		}

		var text string
		if format == "markdown" && strings.Contains(mime, "html") {
			md, err := htmltomarkdown.ConvertString(string(content))
			if err != nil {
				text = string(content)
			} else {
				text = md
			}
		} else {
			text = string(content)
		}

		var header string
		if entryURL != nil {
			entryPath := entry.Path()
			if entryPath == "" && input.Path != "" {
				entryPath = input.Path
			}
			url := entryURL(input.Archive, entryPath)
			header = fmt.Sprintf("# %s\n> Source: [%s](%s)\n\n", entry.Title(), entry.Title(), url)
		} else {
			header = fmt.Sprintf("# %s\n\n", entry.Title())
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: header + text}},
		}, nil, nil
	})
}

// RegisterResources registers the oza://slug/metadata static resources and
// the oza://{slug}/entry/{id} resource template.
//
// archiveURL, if non-nil, is called with a slug to include a "url" field in metadata.
// entryURL, if non-nil, is called with (slug, entryPath) to include a source link in
// the entry resource content header.
func RegisterResources(server *mcp.Server, archives []ArchiveInfo, archiveURL func(slug string) string, entryURL func(slug, path string) string) {
	bySlug := make(map[string]*ArchiveInfo, len(archives))
	for i := range archives {
		bySlug[archives[i].Slug] = &archives[i]
	}

	// Static resource: metadata for each archive.
	for i := range archives {
		ai := &archives[i]
		server.AddResource(&mcp.Resource{
			URI:         fmt.Sprintf("oza://%s/metadata", ai.Slug),
			Name:        ai.Title + " — Metadata",
			Description: "Archive metadata for " + ai.Title,
			MIMEType:    "application/json",
		}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			meta := ai.Archive.AllMetadata()
			m := make(map[string]string, len(meta))
			for k, v := range meta {
				m[k] = string(v)
			}
			m["slug"] = ai.Slug
			m["uuid"] = ai.UUIDHex
			m["entry_count"] = strconv.FormatUint(uint64(ai.Archive.EntryCount()), 10)
			m["redirect_count"] = strconv.FormatUint(uint64(ai.Archive.RedirectCount()), 10)
			m["has_search"] = strconv.FormatBool(ai.Archive.HasSearch())
			if archiveURL != nil {
				m["url"] = archiveURL(ai.Slug)
			}
			data, _ := json.MarshalIndent(m, "", "  ")
			return &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{{
					URI:      req.Params.URI,
					MIMEType: "application/json",
					Text:     string(data),
				}},
			}, nil
		})
	}

	// Resource template: entry content by slug and ID.
	server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "oza://{slug}/entry/{id}",
		Name:        "OZA Entry",
		Description: "Read an entry from an OZA archive by slug and entry ID.",
		MIMEType:    "text/markdown",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		uri := req.Params.URI
		slug, idStr, ok := ParseEntryURI(uri)
		if !ok {
			return nil, mcp.ResourceNotFoundError(uri)
		}
		ai, ok := bySlug[slug]
		if !ok {
			return nil, mcp.ResourceNotFoundError(uri)
		}
		id, err := strconv.ParseUint(idStr, 10, 32)
		if err != nil {
			return nil, mcp.ResourceNotFoundError(uri)
		}
		entry, err := ai.Archive.EntryByID(uint32(id))
		if err != nil {
			return nil, mcp.ResourceNotFoundError(uri)
		}
		content, err := entry.ReadContent()
		if err != nil {
			return nil, fmt.Errorf("reading entry content: %w", err)
		}

		mime := entry.MIMEType()
		if entry.IsRedirect() {
			resolved, _ := entry.Resolve()
			if resolved.MIMEType() != "" {
				mime = resolved.MIMEType()
			}
		}

		var text string
		if strings.Contains(mime, "html") {
			md, err := htmltomarkdown.ConvertString(string(content))
			if err != nil {
				text = string(content)
			} else {
				text = md
			}
		} else {
			text = string(content)
		}

		var header string
		if entryURL != nil {
			url := entryURL(slug, entry.Path())
			header = fmt.Sprintf("# %s\n> Source: [%s](%s)\n\n", entry.Title(), entry.Title(), url)
		} else {
			header = fmt.Sprintf("# %s\n\n", entry.Title())
		}

		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{{
				URI:      uri,
				MIMEType: "text/markdown",
				Text:     header + text,
			}},
		}, nil
	})
}

// sectionTypeName returns a human-readable name for an OZA section type.
func sectionTypeName(t oza.SectionType) string {
	switch t {
	case oza.SectionMetadata:
		return "METADATA"
	case oza.SectionMIMETable:
		return "MIME_TABLE"
	case oza.SectionEntryTable:
		return "ENTRY_TABLE"
	case oza.SectionPathIndex:
		return "PATH_INDEX"
	case oza.SectionTitleIndex:
		return "TITLE_INDEX"
	case oza.SectionContent:
		return "CONTENT"
	case oza.SectionRedirectTab:
		return "REDIRECT_TABLE"
	case oza.SectionChrome:
		return "CHROME"
	case oza.SectionSignatures:
		return "SIGNATURES"
	case oza.SectionZstdDict:
		return "ZSTD_DICT"
	case oza.SectionSearchTitle:
		return "SEARCH_TITLE"
	case oza.SectionSearchBody:
		return "SEARCH_BODY"
	default:
		return fmt.Sprintf("UNKNOWN(0x%04x)", uint32(t))
	}
}

// ParseEntryURI extracts slug and id from "oza://{slug}/entry/{id}".
func ParseEntryURI(uri string) (slug, id string, ok bool) {
	const prefix = "oza://"
	if !strings.HasPrefix(uri, prefix) {
		return "", "", false
	}
	rest := uri[len(prefix):]
	parts := strings.SplitN(rest, "/entry/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}
