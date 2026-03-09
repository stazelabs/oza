package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stazelabs/oza/oza"
)

func registerTools(server *mcp.Server, archives []*archiveInfo) {
	bySlug := make(map[string]*archiveInfo, len(archives))
	for _, ai := range archives {
		bySlug[ai.slug] = ai
	}

	// list_archives: list loaded archives with metadata.
	type listArchivesInput struct{}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_archives",
		Description: "List all loaded OZA archives with metadata (title, description, entry count, capabilities).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input listArchivesInput) (*mcp.CallToolResult, any, error) {
		type archiveDesc struct {
			Slug        string `json:"slug"`
			Title       string `json:"title"`
			Description string `json:"description,omitempty"`
			UUID        string `json:"uuid"`
			EntryCount  uint32 `json:"entry_count"`
			HasSearch   bool   `json:"has_search"`
		}
		var list []archiveDesc
		for _, ai := range archives {
			list = append(list, archiveDesc{
				Slug:        ai.slug,
				Title:       ai.title,
				Description: ai.description,
				UUID:        ai.uuidHex,
				EntryCount:  ai.archive.EntryCount(),
				HasSearch:   ai.archive.HasSearch(),
			})
		}
		data, _ := json.MarshalIndent(list, "", "  ")
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
		}, nil, nil
	})

	// search_text: trigram search across archives.
	type searchTextInput struct {
		Query   string `json:"query" jsonschema:"Search query string (min 3 chars for trigram matching)"`
		Archive string `json:"archive,omitempty" jsonschema:"Archive slug to search (omit to search all)"`
		Limit   int    `json:"limit,omitempty" jsonschema:"Maximum results to return (default 20)"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "search_text",
		Description: "Search OZA archives using trigram text search. Returns matching entries with titles and paths. Use list_archives first to see available archives.",
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

		var targets []*archiveInfo
		if input.Archive != "" {
			ai, ok := bySlug[input.Archive]
			if !ok {
				return &mcp.CallToolResult{
					Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("error: archive %q not found", input.Archive)}},
					IsError: true,
				}, nil, nil
			}
			targets = []*archiveInfo{ai}
		} else {
			targets = archives
		}

		type searchHit struct {
			Archive    string `json:"archive"`
			EntryID    uint32 `json:"entry_id"`
			Path       string `json:"path"`
			Title      string `json:"title"`
			TitleMatch bool   `json:"title_match"`
			BodyMatch  bool   `json:"body_match"`
		}
		var hits []searchHit
		for _, ai := range targets {
			if !ai.archive.HasSearch() {
				continue
			}
			results, err := ai.archive.Search(input.Query, oza.SearchOptions{Limit: limit})
			if err != nil {
				continue
			}
			for _, r := range results {
				hits = append(hits, searchHit{
					Archive:    ai.slug,
					EntryID:    r.Entry.ID(),
					Path:       r.Entry.Path(),
					Title:      r.Entry.Title(),
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

	// read_entry: read content of an entry.
	type readEntryInput struct {
		Archive string `json:"archive" jsonschema:"Archive slug (required). Use list_archives to see available archives."`
		EntryID *int   `json:"entry_id,omitempty" jsonschema:"Entry ID (from search results). Provide either entry_id or path."`
		Path    string `json:"path,omitempty" jsonschema:"Entry path within the archive. Provide either entry_id or path."`
		Format  string `json:"format,omitempty" jsonschema:"Output format: markdown (default) or html"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "read_entry",
		Description: "Read the content of an OZA archive entry by ID or path. Returns clean markdown by default.",
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
			entry, err = ai.archive.EntryByID(uint32(*input.EntryID))
		} else if input.Path != "" {
			entry, err = ai.archive.EntryByPath(input.Path)
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

		header := fmt.Sprintf("# %s\n\n", entry.Title())
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: header + text}},
		}, nil, nil
	})
}
