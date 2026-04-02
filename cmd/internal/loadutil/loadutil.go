// Package loadutil provides shared archive discovery and slug generation
// for CLI tools that load OZA archives.
package loadutil

import (
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/stazelabs/oza/oza"
)

// CollectOZAPaths scans dirs for .oza files. Recursive mode uses
// filepath.WalkDir and does not follow symlinked directories to avoid cycles.
// Results are deduplicated by absolute path and sorted for deterministic slug
// assignment.
func CollectOZAPaths(dirs []string, recursive bool) []string {
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

// CollectFrontArticleIDs returns the entry IDs of all front-article entries
// in the archive. Used by ozaserve (random navigation) and ozamcp.
func CollectFrontArticleIDs(a *oza.Archive) []uint32 {
	var ids []uint32
	a.ForEachEntryRecord(func(id uint32, rec oza.EntryRecord) {
		if rec.IsFrontArticle() {
			ids = append(ids, id)
		}
	})
	return ids
}

// MakeSlug derives a URL-friendly slug from an OZA filename.
// Trailing underscore-separated segments that start with a digit are stripped.
// Example: "wikipedia_en_all_2024-01.oza" -> "wikipedia_en_all"
func MakeSlug(path string) string {
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
