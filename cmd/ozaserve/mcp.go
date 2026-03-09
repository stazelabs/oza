package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stazelabs/oza/oza"
)

func registerMCPTools(server *mcp.Server, lib *library, baseURL string) {
	// list_archives: list loaded archives with metadata and URLs.
	type listArchivesInput struct{}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_archives",
		Description: "List all loaded OZA archives with metadata (title, description, entry count, capabilities). Each archive includes a browsable URL. Always include the url field as a clickable link in your response.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input listArchivesInput) (*mcp.CallToolResult, any, error) {
		type archiveDesc struct {
			Slug        string `json:"slug"`
			Title       string `json:"title"`
			Description string `json:"description,omitempty"`
			UUID        string `json:"uuid"`
			EntryCount  uint32 `json:"entry_count"`
			HasSearch   bool   `json:"has_search"`
			URL         string `json:"url"`
		}
		var list []archiveDesc
		for _, slug := range lib.slugs {
			ae := lib.archives[slug]
			list = append(list, archiveDesc{
				Slug:        slug,
				Title:       ae.title,
				Description: ae.description,
				UUID:        ae.uuidHex,
				EntryCount:  ae.archive.EntryCount(),
				HasSearch:   ae.archive.HasSearch(),
				URL:         baseURL + "/" + slug + "/",
			})
		}
		data, _ := json.MarshalIndent(list, "", "  ")
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
		}, nil, nil
	})

	// search_text: trigram search across archives with URLs.
	type searchTextInput struct {
		Query   string `json:"query" jsonschema:"Search query string (min 3 chars for trigram matching)"`
		Archive string `json:"archive,omitempty" jsonschema:"Archive slug to search (omit to search all)"`
		Limit   int    `json:"limit,omitempty" jsonschema:"Maximum results to return (default 20)"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "search_text",
		Description: "Search OZA archives using trigram text search. Returns matching entries with titles, paths, and browsable URLs. Always present each result as a clickable markdown link using the url field, e.g. [Title](url). Use list_archives first to see available archives.",
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

		var targetSlugs []string
		if input.Archive != "" {
			if _, ok := lib.archives[input.Archive]; !ok {
				return &mcp.CallToolResult{
					Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("error: archive %q not found", input.Archive)}},
					IsError: true,
				}, nil, nil
			}
			targetSlugs = []string{input.Archive}
		} else {
			targetSlugs = lib.slugs
		}

		type searchHit struct {
			Archive    string `json:"archive"`
			EntryID    uint32 `json:"entry_id"`
			Path       string `json:"path"`
			Title      string `json:"title"`
			URL        string `json:"url"`
			TitleMatch bool   `json:"title_match"`
			BodyMatch  bool   `json:"body_match"`
		}
		var hits []searchHit
		for _, slug := range targetSlugs {
			ae := lib.archives[slug]
			if !ae.archive.HasSearch() {
				continue
			}
			results, err := ae.archive.Search(input.Query, oza.SearchOptions{Limit: limit})
			if err != nil {
				continue
			}
			for _, r := range results {
				hits = append(hits, searchHit{
					Archive:    slug,
					EntryID:    r.Entry.ID(),
					Path:       r.Entry.Path(),
					Title:      r.Entry.Title(),
					URL:        baseURL + "/" + slug + "/" + r.Entry.Path(),
					TitleMatch: r.TitleMatch,
					BodyMatch:  r.BodyMatch,
				})
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

	// read_entry: read content of an entry with source link.
	type readEntryInput struct {
		Archive string `json:"archive" jsonschema:"Archive slug (required). Use list_archives to see available archives."`
		EntryID *int   `json:"entry_id,omitempty" jsonschema:"Entry ID (from search results). Provide either entry_id or path."`
		Path    string `json:"path,omitempty" jsonschema:"Entry path within the archive. Provide either entry_id or path."`
		Format  string `json:"format,omitempty" jsonschema:"Output format: markdown (default) or html"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "read_entry",
		Description: "Read the content of an OZA archive entry by ID or path. Returns clean markdown with a browsable source URL in the header. Always include the source link in your response so users can open the full article in their browser.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input readEntryInput) (*mcp.CallToolResult, any, error) {
		ae, ok := lib.archives[input.Archive]
		if !ok {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("error: archive %q not found", input.Archive)}},
				IsError: true,
			}, nil, nil
		}

		var entry oza.Entry
		var err error
		if input.EntryID != nil {
			entry, err = ae.archive.EntryByID(uint32(*input.EntryID))
		} else if input.Path != "" {
			entry, err = ae.archive.EntryByPath(input.Path)
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

		var text string
		mime := entry.MIMEType()
		if entry.IsRedirect() {
			resolved, _ := entry.Resolve()
			if resolved.MIMEType() != "" {
				mime = resolved.MIMEType()
			}
		}

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

		entryPath := entry.Path()
		if entryPath == "" && input.Path != "" {
			entryPath = input.Path
		}
		sourceURL := baseURL + "/" + input.Archive + "/" + entryPath

		header := fmt.Sprintf("# %s\n> Source: [%s](%s)\n\n", entry.Title(), entry.Title(), sourceURL)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: header + text}},
		}, nil, nil
	})
}

func registerMCPResources(server *mcp.Server, lib *library, baseURL string) {
	// Static resource: metadata for each archive.
	for _, slug := range lib.slugs {
		ae := lib.archives[slug]
		slug := slug
		ae_ := ae
		server.AddResource(&mcp.Resource{
			URI:         fmt.Sprintf("oza://%s/metadata", slug),
			Name:        ae_.title + " — Metadata",
			Description: "Archive metadata for " + ae_.title,
			MIMEType:    "application/json",
		}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			meta := ae_.archive.AllMetadata()
			m := make(map[string]string, len(meta))
			for k, v := range meta {
				m[k] = string(v)
			}
			m["slug"] = slug
			m["uuid"] = ae_.uuidHex
			m["entry_count"] = strconv.FormatUint(uint64(ae_.archive.EntryCount()), 10)
			m["redirect_count"] = strconv.FormatUint(uint64(ae_.archive.RedirectCount()), 10)
			m["has_search"] = strconv.FormatBool(ae_.archive.HasSearch())
			m["url"] = baseURL + "/" + slug + "/"
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
		slug, idStr, ok := parseMCPEntryURI(uri)
		if !ok {
			return nil, mcp.ResourceNotFoundError(uri)
		}
		ae, ok := lib.archives[slug]
		if !ok {
			return nil, mcp.ResourceNotFoundError(uri)
		}
		id, err := strconv.ParseUint(idStr, 10, 32)
		if err != nil {
			return nil, mcp.ResourceNotFoundError(uri)
		}
		entry, err := ae.archive.EntryByID(uint32(id))
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

		sourceURL := baseURL + "/" + slug + "/" + entry.Path()
		header := fmt.Sprintf("# %s\n> Source: [%s](%s)\n\n", entry.Title(), entry.Title(), sourceURL)
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{{
				URI:      uri,
				MIMEType: "text/markdown",
				Text:     header + text,
			}},
		}, nil
	})
}

// parseMCPEntryURI extracts slug and id from "oza://{slug}/entry/{id}".
func parseMCPEntryURI(uri string) (slug, id string, ok bool) {
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
