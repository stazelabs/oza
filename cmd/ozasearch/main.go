package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/stazelabs/oza/oza"
)

func main() {
	var limit int
	var jsonOut bool
	var titleOnly bool

	root := &cobra.Command{
		Use:   "ozasearch <archive.oza> <query>",
		Short: "Search an OZA archive using the trigram index",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(args[0], args[1], limit, jsonOut, titleOnly)
		},
	}

	root.Flags().IntVarP(&limit, "limit", "l", 20, "Maximum number of results to return")
	root.Flags().BoolVarP(&jsonOut, "json", "j", false, "Output results as JSON")
	root.Flags().BoolVarP(&titleOnly, "title-only", "t", false, "Search only article titles")

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

type result struct {
	ID         uint32 `json:"id"`
	Path       string `json:"path"`
	Title      string `json:"title"`
	MIME       string `json:"mime"`
	TitleMatch bool   `json:"title_match"`
	BodyMatch  bool   `json:"body_match"`
}

func run(archivePath, query string, limit int, jsonOut, titleOnly bool) error {
	a, err := oza.Open(archivePath)
	if err != nil {
		return err
	}
	defer a.Close()

	if !a.HasSearch() {
		return fmt.Errorf("archive has no search index (rebuild with --build-search)")
	}

	results, err := a.Search(query, oza.SearchOptions{Limit: limit, TitleOnly: titleOnly})
	if err != nil {
		return err
	}

	if len(results) == 0 {
		if jsonOut {
			fmt.Println("[]")
		} else {
			fmt.Fprintf(os.Stderr, "no results for %q\n", query)
		}
		return nil
	}

	if jsonOut {
		out := make([]result, 0, len(results))
		for _, r := range results {
			out = append(out, result{
				ID:         r.Entry.ID(),
				Path:       r.Entry.Path(),
				Title:      r.Entry.Title(),
				MIME:       r.Entry.MIMEType(),
				TitleMatch: r.TitleMatch,
				BodyMatch:  r.BodyMatch,
			})
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	fmt.Printf("%-4s  %-6s  %-40s  %s\n", "SRC", "ID", "PATH", "TITLE")
	fmt.Printf("%-4s  %-6s  %-40s  %s\n", "----", "------", "----------------------------------------", "-----")
	for _, r := range results {
		src := "[B]"
		if r.TitleMatch && r.BodyMatch {
			src = "[TB]"
		} else if r.TitleMatch {
			src = "[T]"
		}
		path := r.Entry.Path()
		if len(path) > 40 {
			path = path[:37] + "..."
		}
		fmt.Printf("%-4s  %-6d  %-40s  %s\n", src, r.Entry.ID(), path, r.Entry.Title())
	}
	return nil
}
