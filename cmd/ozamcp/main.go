package main

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
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
	cmd.Flags().StringVarP(&transport, "transport", "t", "stdio", "transport: stdio or sse")
	cmd.Flags().IntVarP(&cacheSize, "cache", "c", 64, "chunk cache size per archive")

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// archiveInfo holds a loaded archive and its derived metadata.
type archiveInfo struct {
	archive     *oza.Archive
	slug        string
	title       string
	description string
	uuidHex     string
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
			ai.archive.Close()
		}
	}()

	for _, ai := range archives {
		log.Printf("loaded: %s — %s (%d entries)", ai.slug, ai.title, ai.archive.EntryCount())
	}

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "ozamcp",
		Version: "0.1.0",
	}, nil)

	registerTools(server, archives)
	registerResources(server, archives)

	switch transport {
	case "stdio":
		return server.Run(context.Background(), &mcp.StdioTransport{})
	default:
		return fmt.Errorf("unsupported transport: %s (only stdio supported currently)", transport)
	}
}

// loadArchives opens OZA files and returns archiveInfo slices keyed by slug.
func loadArchives(paths []string, hardFailCount int, cacheSize int) ([]*archiveInfo, error) {
	slugs := make(map[string]bool)
	var archives []*archiveInfo

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

		archives = append(archives, &archiveInfo{
			archive:     a,
			slug:        slug,
			title:       title,
			description: desc,
			uuidHex:     hex.EncodeToString(uuid[:]),
		})
	}

	if len(archives) == 0 {
		return nil, errors.New("no valid OZA files found")
	}
	sort.Slice(archives, func(i, j int) bool {
		return strings.ToLower(archives[i].title) < strings.ToLower(archives[j].title)
	})
	return archives, nil
}

// collectOZAPaths scans directories for .oza files.
func collectOZAPaths(dirs []string, recursive bool) []string {
	seen := make(map[string]bool)
	var paths []string
	for _, dir := range dirs {
		if recursive {
			filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error { //nolint:errcheck
				if err != nil {
					log.Printf("warning: skipping %s: %v", path, err)
					return nil
				}
				if d.Type()&fs.ModeSymlink != 0 {
					info, err := os.Stat(path)
					if err == nil && info.IsDir() {
						return filepath.SkipDir
					}
				}
				if !d.IsDir() && strings.HasSuffix(path, ".oza") {
					if abs, err := filepath.Abs(path); err == nil && !seen[abs] {
						seen[abs] = true
						paths = append(paths, abs)
					}
				}
				return nil
			})
		} else {
			entries, err := os.ReadDir(dir)
			if err != nil {
				log.Printf("warning: cannot read directory %s: %v", dir, err)
				continue
			}
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".oza") {
					if abs, err := filepath.Abs(filepath.Join(dir, e.Name())); err == nil && !seen[abs] {
						seen[abs] = true
						paths = append(paths, abs)
					}
				}
			}
		}
	}
	sort.Strings(paths)
	return paths
}

// makeSlug derives a URL-friendly slug from an OZA filename.
func makeSlug(path string) string {
	name := filepath.Base(path)
	name = strings.TrimSuffix(name, ".oza")
	parts := strings.Split(name, "_")
	for len(parts) > 1 {
		last := parts[len(parts)-1]
		if len(last) >= 4 && last[0] >= '0' && last[0] <= '9' {
			parts = parts[:len(parts)-1]
		} else {
			break
		}
	}
	return strings.Join(parts, "_")
}
