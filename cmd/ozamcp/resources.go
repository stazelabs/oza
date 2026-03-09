package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerResources(server *mcp.Server, archives []*archiveInfo) {
	bySlug := make(map[string]*archiveInfo, len(archives))
	for _, ai := range archives {
		bySlug[ai.slug] = ai
	}

	// Static resource: metadata for each archive.
	for _, ai := range archives {
		ai := ai
		server.AddResource(&mcp.Resource{
			URI:         fmt.Sprintf("oza://%s/metadata", ai.slug),
			Name:        ai.title + " — Metadata",
			Description: "Archive metadata for " + ai.title,
			MIMEType:    "application/json",
		}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			meta := ai.archive.AllMetadata()
			m := make(map[string]string, len(meta))
			for k, v := range meta {
				m[k] = string(v)
			}
			m["slug"] = ai.slug
			m["uuid"] = ai.uuidHex
			m["entry_count"] = strconv.FormatUint(uint64(ai.archive.EntryCount()), 10)
			m["redirect_count"] = strconv.FormatUint(uint64(ai.archive.RedirectCount()), 10)
			m["has_search"] = strconv.FormatBool(ai.archive.HasSearch())
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
		// Parse: oza://{slug}/entry/{id}
		slug, idStr, ok := parseEntryURI(uri)
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
		entry, err := ai.archive.EntryByID(uint32(id))
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

		header := fmt.Sprintf("# %s\n\n", entry.Title())
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{{
				URI:      uri,
				MIMEType: "text/markdown",
				Text:     header + text,
			}},
		}, nil
	})
}

// parseEntryURI extracts slug and id from "oza://{slug}/entry/{id}".
func parseEntryURI(uri string) (slug, id string, ok bool) {
	const prefix = "oza://"
	if !strings.HasPrefix(uri, prefix) {
		return "", "", false
	}
	rest := uri[len(prefix):]
	// rest = "{slug}/entry/{id}"
	parts := strings.SplitN(rest, "/entry/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}
