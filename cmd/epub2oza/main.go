package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strings"

	"github.com/spf13/cobra"

	"github.com/stazelabs/oza/ozawrite"
)

func main() {
	root := &cobra.Command{
		Use:   "epub2oza <input.epub|dir> <output.oza>",
		Short: "Convert EPUB books to OZA format",
		Long: `王座 epub2oza — convert EPUB books to OZA format.

Single book mode (default):
  epub2oza book.epub output.oza

Collection mode (--collection):
  epub2oza --collection ./epub-dir/ library.oza

Reads EPUB files (2.0 or 3.x) and writes an OZA archive with Zstd
compression, optional dictionary training, trigram search indices, content
minification, and image transcoding.

In collection mode, all .epub files in the input directory are bundled into
a single searchable archive. Entries are namespaced by book slug, shared
resources (CSS, fonts) are deduplicated via SHA-256, and a collection-level
index page is generated.`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				_ = cmd.Help()
				os.Exit(0)
			}
			return cobra.ExactArgs(2)(cmd, args)
		},
		RunE: run,
	}

	root.Flags().Bool("collection", false, "Collection mode: treat input as a directory of .epub files")
	root.Flags().Bool("recursive", false, "In collection mode, recurse into subdirectories")
	root.Flags().Int("zstd-level", 6, "Zstd compression level (1=fastest, 6=default, 19=best)")
	root.Flags().Int("dict-samples", 2000, "Max samples for dictionary training")
	root.Flags().Int("chunk-size", 4*1024*1024, "Target uncompressed chunk size in bytes")
	root.Flags().Bool("no-search", false, "Disable trigram search indices")
	root.Flags().Float64("search-prune", 0.5, "Prune trigrams appearing in >= this fraction of docs (0 to disable)")
	root.Flags().Bool("no-dict", false, "Disable Zstd dictionary training")
	root.Flags().Bool("minify", false, "Enable content minification (HTML, CSS, JS, SVG)")
	root.Flags().Bool("no-optimize-images", false, "Disable lossless image optimization (JPEG metadata strip)")
	root.Flags().String("transcode", "auto", "Image transcoding: auto (use tools if found), off, require")
	root.Flags().Bool("transcode-lossy-jpeg", false, "Enable lossy JPEG→WebP transcoding (opt-in, ~25-35% savings)")
	root.Flags().Bool("transcode-avif", false, "Prefer AVIF over WebP for PNG/JPEG (requires avifenc; brew install libavif)")
	root.Flags().Int("compress-workers", 0, "Parallel compression workers (0 = min(NumCPU, 4))")
	root.Flags().Bool("verbose", false, "Print detailed progress and statistics")
	root.Flags().String("profile", "", "Write CPU and memory profiles to this directory")
	root.Flags().String("title", "", "Collection title (collection mode only; defaults to directory name)")

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	inputPath := args[0]
	outputPath := args[1]
	verbose, _ := cmd.Flags().GetBool("verbose")
	profileDir, _ := cmd.Flags().GetString("profile")

	// Start CPU profiling if requested.
	if profileDir != "" {
		if err := os.MkdirAll(profileDir, 0o755); err != nil {
			return fmt.Errorf("creating profile dir: %w", err)
		}
		cpuf, err := os.Create(filepath.Join(profileDir, "cpu.prof"))
		if err != nil {
			return fmt.Errorf("creating CPU profile: %w", err)
		}
		if err := pprof.StartCPUProfile(cpuf); err != nil {
			cpuf.Close()
			return fmt.Errorf("starting CPU profile: %w", err)
		}
		defer func() {
			pprof.StopCPUProfile()
			cpuf.Close()
		}()
	}

	opts := parseConvertOptions(cmd)

	collection, _ := cmd.Flags().GetBool("collection")

	var stats *Stats
	var err error
	if collection {
		recursive, _ := cmd.Flags().GetBool("recursive")
		title, _ := cmd.Flags().GetString("title")
		stats, err = runCollection(inputPath, outputPath, opts, recursive, title)
	} else {
		c, cerr := NewConverter(inputPath, outputPath, opts)
		if cerr != nil {
			return cerr
		}
		err = c.Run()
		stats = &c.stats
	}
	if err != nil {
		return err
	}

	// Write memory profile after conversion completes.
	if profileDir != "" {
		memf, err := os.Create(filepath.Join(profileDir, "mem.prof"))
		if err == nil {
			runtime.GC()
			_ = pprof.WriteHeapProfile(memf)
			memf.Close()
		}
	}

	if verbose {
		stats.Print(os.Stderr)
	}

	fmt.Fprintf(os.Stderr, "Converted %s -> %s\n", inputPath, outputPath)
	return nil
}

func parseConvertOptions(cmd *cobra.Command) ConvertOptions {
	zstdLevel, _ := cmd.Flags().GetInt("zstd-level")
	dictSamples, _ := cmd.Flags().GetInt("dict-samples")
	chunkSize, _ := cmd.Flags().GetInt("chunk-size")
	noSearch, _ := cmd.Flags().GetBool("no-search")
	searchPrune, _ := cmd.Flags().GetFloat64("search-prune")
	noDict, _ := cmd.Flags().GetBool("no-dict")
	minify, _ := cmd.Flags().GetBool("minify")
	noOptimizeImages, _ := cmd.Flags().GetBool("no-optimize-images")
	compressWorkers, _ := cmd.Flags().GetInt("compress-workers")
	verbose, _ := cmd.Flags().GetBool("verbose")

	transcode, _ := cmd.Flags().GetString("transcode")
	var transcodeTools *ozawrite.TranscodeTools
	switch transcode {
	case "auto":
		tools := ozawrite.DiscoverTranscodeTools()
		if tools.Available() {
			transcodeTools = tools
			if verbose {
				fmt.Fprintf(os.Stderr, "Transcode tools: %s\n", tools.String())
			}
		} else if verbose {
			fmt.Fprintln(os.Stderr, "Transcode tools: none found (install libwebp: brew install webp)")
		}
	case "require":
		tools := ozawrite.DiscoverTranscodeTools()
		if !tools.Available() {
			fmt.Fprintf(os.Stderr, "error: --transcode=require but no tools found\n")
			os.Exit(1)
		}
		transcodeTools = tools
		if verbose {
			fmt.Fprintf(os.Stderr, "Transcode tools: %s\n", tools.String())
		}
	case "off":
		// nil, no transcoding
	default:
		fmt.Fprintf(os.Stderr, "error: unknown --transcode value %q\n", transcode)
		os.Exit(1)
	}

	lossyJPEG, _ := cmd.Flags().GetBool("transcode-lossy-jpeg")
	if lossyJPEG && transcodeTools != nil {
		transcodeTools.LossyJPEG = true
	}
	useAVIF, _ := cmd.Flags().GetBool("transcode-avif")
	if useAVIF && transcodeTools != nil {
		transcodeTools.UseAVIF = true
	}

	return ConvertOptions{
		ZstdLevel:       zstdLevel,
		DictSamples:     dictSamples,
		ChunkSize:       chunkSize,
		BuildSearch:     !noSearch,
		SearchPruneFreq: searchPrune,
		TrainDict:       !noDict,
		Minify:          minify,
		OptimizeImages:  !noOptimizeImages,
		TranscodeTools:  transcodeTools,
		CompressWorkers: compressWorkers,
		Verbose:         verbose,
	}
}

// makeBookSlug derives a URL-friendly slug from an EPUB filename.
// "Alice's Adventures in Wonderland.epub" -> "alices-adventures-in-wonderland"
func makeBookSlug(epubPath string) string {
	name := filepath.Base(epubPath)
	name = strings.TrimSuffix(name, filepath.Ext(name))
	name = strings.ToLower(name)

	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '_' || r == '.':
			if b.Len() > 0 {
				last := b.String()
				if last[len(last)-1] != '-' {
					b.WriteByte('-')
				}
			}
		}
	}
	return strings.Trim(b.String(), "-")
}
