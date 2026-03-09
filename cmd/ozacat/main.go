package main

import (
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"
	"github.com/stazelabs/oza/oza"
)

func main() {
	root := &cobra.Command{
		Use:   "ozacat <archive.oza> [path]",
		Short: "Extract or list entries from an OZA archive",
		Args:  cobra.RangeArgs(1, 2),
		RunE:  run,
	}

	root.Flags().BoolP("list", "l", false, "List all entries (path, type, MIME, size)")
	root.Flags().BoolP("meta", "m", false, "Show all metadata key-value pairs")
	root.Flags().BoolP("info", "t", false, "Show entry info without extracting content")

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	archivePath := args[0]

	listMode, _ := cmd.Flags().GetBool("list")
	metaMode, _ := cmd.Flags().GetBool("meta")
	infoMode, _ := cmd.Flags().GetBool("info")

	a, err := oza.Open(archivePath)
	if err != nil {
		return err
	}
	defer a.Close()

	switch {
	case listMode:
		return runList(a)
	case metaMode:
		return runMeta(a)
	case infoMode:
		if len(args) < 2 {
			return fmt.Errorf("-t requires an entry path argument")
		}
		return runInfo(a, args[1])
	default:
		if len(args) < 2 {
			return fmt.Errorf("requires an entry path argument (or use -l, -m, -t)")
		}
		return runCat(a, args[1])
	}
}

func runCat(a *oza.Archive, path string) error {
	e, err := a.EntryByPath(path)
	if err != nil {
		return fmt.Errorf("entry %q: %w", path, err)
	}
	data, err := e.ReadContent()
	if err != nil {
		return fmt.Errorf("reading %q: %w", path, err)
	}
	_, err = os.Stdout.Write(data)
	return err
}

func runList(a *oza.Archive) error {
	fmt.Printf("%-10s  %-30s  %10s  %s\n", "TYPE", "MIME", "SIZE", "PATH")
	fmt.Printf("%-10s  %-30s  %10s  %s\n",
		"----------", "------------------------------", "----------", "----")
	for e := range a.EntriesByPath() {
		entryType := "content"
		mime := e.MIMEType()
		size := fmt.Sprintf("%d", e.Size())
		if e.IsRedirect() {
			entryType = "redirect"
			mime = "-"
			size = "-"
		}
		fmt.Printf("%-10s  %-30s  %10s  %s\n", entryType, mime, size, e.Path())
	}
	return nil
}

func runMeta(a *oza.Archive) error {
	meta := a.AllMetadata()
	keys := make([]string, 0, len(meta))
	for k := range meta {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("%-20s = %s\n", k, string(meta[k]))
	}
	return nil
}

func runInfo(a *oza.Archive, path string) error {
	e, err := a.EntryByPath(path)
	if err != nil {
		return fmt.Errorf("entry %q: %w", path, err)
	}
	entryType := "content"
	if e.IsRedirect() {
		entryType = "redirect"
	}
	fmt.Printf("Path:          %s\n", e.Path())
	fmt.Printf("Title:         %s\n", e.Title())
	fmt.Printf("Type:          %s\n", entryType)
	fmt.Printf("MIME:          %s\n", e.MIMEType())
	fmt.Printf("Front article: %v\n", e.IsFrontArticle())
	fmt.Printf("Size:          %d bytes\n", e.Size())
	return nil
}
