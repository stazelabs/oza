package main

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/stazelabs/oza/cmd/internal/mcptools"
)

func registerMCPTools(server *mcp.Server, lib *library, baseURL string) {
	mcptools.RegisterTools(server, libToMCPArchives(lib),
		func(slug string) string { return baseURL + "/" + slug + "/" },
		func(slug, path string) string { return entryURL(baseURL, slug, path) },
	)
}

func registerMCPResources(server *mcp.Server, lib *library, baseURL string) {
	mcptools.RegisterResources(server, libToMCPArchives(lib),
		func(slug string) string { return baseURL + "/" + slug + "/" },
		func(slug, path string) string { return entryURL(baseURL, slug, path) },
	)
}

// libToMCPArchives converts a library to an ordered []mcptools.ArchiveInfo.
func libToMCPArchives(lib *library) []mcptools.ArchiveInfo {
	result := make([]mcptools.ArchiveInfo, 0, len(lib.slugs))
	for _, slug := range lib.slugs {
		ae := lib.archives[slug]
		result = append(result, mcptools.ArchiveInfo{
			Archive:         ae.archive,
			Slug:            ae.slug,
			Title:           ae.title,
			Description:     ae.description,
			UUIDHex:         ae.uuidHex,
			FrontArticleIDs: ae.frontArticleIDs,
		})
	}
	return result
}
