package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"

	"github.com/spf13/cobra"

	"github.com/stazelabs/oza/cmd/internal/classify"
	"github.com/stazelabs/oza/ozawrite"
)

func main() {
	root := &cobra.Command{
		Use:   "zim2oza <input.zim> <output.oza>",
		Short: "Convert a ZIM archive to OZA format",
		Args:  cobra.ExactArgs(2),
		RunE:  run,
	}

	root.Flags().Int("zstd-level", 6, "Zstd compression level (1=fastest, 6=default, 19=best)")
	root.Flags().Int("dict-samples", 2000, "Max samples for dictionary training")
	root.Flags().Int("chunk-size", 4*1024*1024, "Target uncompressed chunk size in bytes")
	root.Flags().Bool("no-search", false, "Disable trigram search indices")
	root.Flags().Float64("search-prune", 0.5, "Prune trigrams appearing in >= this fraction of docs (0 to disable)")
	root.Flags().Bool("no-dict", false, "Disable Zstd dictionary training")
	root.Flags().Bool("minify", false, "Enable content minification (HTML, CSS, JS, SVG)")
	root.Flags().Bool("no-optimize-images", false, "Disable lossless image optimization (JPEG metadata strip)")
	root.Flags().String("transcode", "auto", "image transcoding: auto (use tools if found), off, require")
	root.Flags().Int("compress-workers", 0, "Parallel compression workers (0 = min(NumCPU, 4))")
	root.Flags().Bool("verbose", false, "Print detailed progress and statistics")
	root.Flags().Bool("json-stats", false, "Output statistics as JSON (implies --verbose)")
	root.Flags().Bool("dry-run", false, "Scan and report statistics without writing output")
	root.Flags().Bool("auto", false, "Auto-detect content profile and apply recommended conversion parameters")
	root.Flags().String("profile", "", "Write CPU and memory profiles to this directory")

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	inputPath := args[0]
	outputPath := args[1]

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
	jsonStats, _ := cmd.Flags().GetBool("json-stats")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	if jsonStats {
		verbose = true
	}
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
			return fmt.Errorf("--transcode=require but no tools found (install libwebp: brew install webp / apt install webp)")
		}
		transcodeTools = tools
		if verbose {
			fmt.Fprintf(os.Stderr, "Transcode tools: %s\n", tools.String())
		}
	case "off":
		// nil, no transcoding
	default:
		return fmt.Errorf("unknown --transcode value %q (use auto, off, or require)", transcode)
	}

	autoMode, _ := cmd.Flags().GetBool("auto")

	opts := ConvertOptions{
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
		DryRun:          dryRun,
	}

	if autoMode {
		qs, err := prescanZIM(inputPath)
		if err != nil {
			return fmt.Errorf("auto prescan: %w", err)
		}
		features := classify.ExtractFromZIMQuick(qs)
		result := classify.Classify(features)
		applyAutoRecs(&opts, result.Recommendations, cmd.Flags().Changed)
		if verbose {
			printAutoProfile(result.Profile, result.Confidence, result.Recommendations)
		}
	}

	c, err := NewConverter(inputPath, outputPath, opts)
	if err != nil {
		return err
	}
	defer c.Close()

	if err := c.Run(); err != nil {
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

	if verbose || dryRun {
		if jsonStats {
			c.stats.PrintJSON(os.Stdout)
		} else {
			c.stats.Print(os.Stderr)
		}
	}

	if !dryRun {
		fmt.Fprintf(os.Stderr, "Converted %s -> %s\n", inputPath, outputPath)
	}
	return nil
}
