package main

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"

	"github.com/stazelabs/oza/cmd/internal/loadutil"
	"github.com/stazelabs/oza/cmd/internal/mcptools"
	"github.com/stazelabs/oza/oza"
)

func main() {
	var dirs []string
	var recursive bool
	var transport string
	var cacheSize int

	cmd := &cobra.Command{
		Use:   "ozamcp [file.oza ...] [--dir <dir>]",
		Short: "MCP server for OZA archives",
		Long: `王座 ozamcp — MCP server exposing OZA archives as tools and resources for LLMs.

Loads one or more OZA files and serves them via the Model Context Protocol.
Archives may be specified as positional arguments, via --dir, or both.`,
		Args: func(cmd *cobra.Command, args []string) error {
			d, _ := cmd.Flags().GetStringArray("dir")
			if len(args) == 0 && len(d) == 0 {
				return errors.New("at least one OZA file or --dir required")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(args, dirs, recursive, transport, cacheSize)
		},
	}

	cmd.Flags().StringArrayVarP(&dirs, "dir", "d", nil, "directory of OZA files (repeatable)")
	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "scan --dir directories recursively")
	cmd.Flags().StringVarP(&transport, "transport", "t", "stdio", "transport: stdio")
	cmd.Flags().IntVarP(&cacheSize, "cache", "c", 64, "chunk cache size per archive")

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(paths []string, dirs []string, recursive bool, transport string, cacheSize int) error {
	dirPaths := collectOZAPaths(dirs, recursive)
	allPaths := append(paths, dirPaths...)

	archives, err := loadArchives(allPaths, len(paths), cacheSize)
	if err != nil {
		return err
	}
	defer func() {
		for _, ai := range archives {
			ai.Archive.Close()
		}
	}()

	for _, ai := range archives {
		log.Printf("loaded: %s — %s (%d entries)", ai.Slug, ai.Title, ai.Archive.EntryCount())
	}

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "ozamcp",
		Version: "0.1.0",
	}, nil)

	// No URL builders: standalone MCP server has no HTTP base URL.
	mcptools.RegisterTools(server, archives, nil, nil)
	mcptools.RegisterResources(server, archives, nil, nil)

	switch transport {
	case "stdio":
		return server.Run(context.Background(), &mcp.StdioTransport{})
	default:
		return fmt.Errorf("unsupported transport: %s (only stdio supported currently)", transport)
	}
}

// loadArchives opens OZA files and returns an ordered slice of ArchiveInfo.
func loadArchives(paths []string, hardFailCount int, cacheSize int) ([]mcptools.ArchiveInfo, error) {
	slugs := make(map[string]bool)
	var archives []mcptools.ArchiveInfo

	for i, path := range paths {
		a, err := oza.OpenWithOptions(path, oza.WithCacheSize(cacheSize))
		if err != nil {
			if i < hardFailCount {
				return nil, fmt.Errorf("opening %s: %w", path, err)
			}
			log.Printf("warning: skipping %s: %v", path, err)
			continue
		}

		slug := makeSlug(path)
		base := slug
		for j := 2; slugs[slug]; j++ {
			slug = fmt.Sprintf("%s_%d", base, j)
		}
		slugs[slug] = true

		title, _ := a.Metadata("title")
		if title == "" {
			title = slug
		}
		desc, _ := a.Metadata("description")
		uuid := a.UUID()

		var faIDs []uint32
		a.ForEachEntryRecord(func(id uint32, rec oza.EntryRecord) {
			if rec.IsFrontArticle() {
				faIDs = append(faIDs, id)
			}
		})

		archives = append(archives, mcptools.ArchiveInfo{
			Archive:         a,
			Slug:            slug,
			Title:           title,
			Description:     desc,
			UUIDHex:         hex.EncodeToString(uuid[:]),
			FrontArticleIDs: faIDs,
		})
	}

	if len(archives) == 0 {
		return nil, errors.New("no valid OZA files found")
	}
	sort.Slice(archives, func(i, j int) bool {
		return strings.ToLower(archives[i].Title) < strings.ToLower(archives[j].Title)
	})
	return archives, nil
}

// collectOZAPaths delegates to the shared loadutil package.
func collectOZAPaths(dirs []string, recursive bool) []string {
	return loadutil.CollectOZAPaths(dirs, recursive)
}

// makeSlug delegates to the shared loadutil package.
func makeSlug(path string) string {
	return loadutil.MakeSlug(path)
}
