package ozawrite

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/stazelabs/oza/oza"
	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/css"
	"github.com/tdewolff/minify/v2/html"
	"github.com/tdewolff/minify/v2/js"
	"github.com/tdewolff/minify/v2/svg"
)

// Timings holds per-phase durations accumulated during AddEntry and Close.
type Timings struct {
	// Accumulated during AddEntry:
	Transform   time.Duration // minify + image optimize
	Dedup       time.Duration // SHA-256 + dedup lookup
	SearchIndex time.Duration // extractVisibleText + trigram indexing
	ChunkBuild  time.Duration // addToChunk + flushChunk (includes compression backpressure)

	// Set during Close:
	DictTrain time.Duration // Zstd dictionary training
	Compress  time.Duration // chunk compression
	Assemble  time.Duration // serialization, SHA-256, and disk write
}

// ProgressFunc is called during Close to report progress.
// phase is a short identifier: "dict-train", "compress", "index-build", "assemble".
// For "compress": n is the 1-based chunk index, total is the chunk count.
// For all other phases: n=0 signals start, n=1 signals completion.
type ProgressFunc func(phase string, n, total int)

// WriterOptions controls the behaviour of Close.
type WriterOptions struct {
	ZstdLevel        int          // compression level (1-22); default 6
	ChunkTargetSize  int          // uncompressed bytes per chunk; default 4 MB
	TrainDict        bool         // train per-MIME Zstd dictionaries; default true
	DictSamples      int          // max samples for dictionary training; default 2000
	BuildSearch      bool         // convenience: sets both BuildTitleSearch and BuildBodySearch
	BuildTitleSearch bool         // build title trigram search index
	BuildBodySearch  bool         // build body trigram search index
	MinifyHTML       bool         // minify text/html content; default true
	MinifyCSS        bool         // minify text/css content; default true
	MinifyJS         bool         // minify application/javascript content; default true
	MinifySVG        bool         // minify image/svg+xml content; default true
	OptimizeImages   bool         // lossless image optimization: JPEG metadata strip; default true
	CompressWorkers  int          // parallel compression workers; 0 = min(NumCPU, 4)
	Progress         ProgressFunc // optional; called during Close to report progress
	SigningKeys      []SigningKey  // optional; if set, a SIGNATURES trailer is appended after the file checksum
}

func defaultOptions() WriterOptions {
	return WriterOptions{
		ZstdLevel:        6,
		ChunkTargetSize:  4 * 1024 * 1024,
		TrainDict:        true,
		DictSamples:      2000,
		BuildSearch:      true,
		BuildTitleSearch: true,
		BuildBodySearch:  true,
		MinifyHTML:       true,
		MinifyCSS:        true,
		MinifyJS:         true,
		MinifySVG:        true,
		OptimizeImages:   true,
	}
}

// entryBuilder holds everything known about one entry before Close.
// Content is NOT retained after the entry is chunked — only metadata survives.
type entryBuilder struct {
	id             uint32
	path           string
	title          string
	mimeType       string
	contentHash    [32]byte // SHA-256 of (transformed) content
	isFrontArticle bool
	isRedirect     bool
	redirectIndex  uint32 // index into redirectEntries (only valid for redirects)
	targetID       uint32 // only valid for redirects

	// filled during chunk assignment:
	chunkID    uint32
	blobOffset uint32
	blobSize   uint32
	deduped    bool // true if blob was shared with an earlier entry
}

// rawSection is a section's type tag paired with its serialized bytes.
type rawSection struct {
	typ              oza.SectionType
	data             []byte // on-disk bytes (compressed if applicable)
	uncompressedSize uint64 // original size before compression
	compression      uint8  // oza.CompNone, CompZstd, etc.
}

// Writer assembles an OZA archive.
//
// Content blobs are transformed, compressed into chunks, and flushed to a
// temporary file during AddEntry — only entry metadata is held in memory.
// Close assembles the final archive from the temp file and metadata.
type Writer struct {
	w               io.ReadWriteSeeker
	opts            WriterOptions
	meta            map[string][]byte
	entries         []*entryBuilder   // content entries only
	redirectEntries []*entryBuilder   // redirect entries (separate compact section)
	closed          bool
	timings         Timings

	// Streaming chunk state — content is flushed to chunkTmp during AddEntry.
	minifier   *minify.M        // lazily initialised
	dedup      *dedupMap         // content hash deduplication
	openChunks map[string]*chunkBuilder // mimeGroup -> current open chunk
	chunkDescs []chunkDesc       // descriptors of flushed chunks
	chunkTmp   *os.File          // temp file holding compressed chunk data
	chunkOff   uint64            // current write offset in chunkTmp
	nextChunk  uint32            // next chunk ID to assign
	cache      encoderCache      // reused zstd encoders (serial path only)

	// Parallel compression pipeline (nil when CompressWorkers == 1).
	compressIn  chan compressJob
	compressOut chan compressResult
	writerDone  chan error
	pipelineErr error // first error from pipeline, checked lazily

	// Dictionary training: buffer first DictSamples entries per group,
	// then train and switch to streaming mode.
	dictSamples    map[string][][]byte  // mimeGroup -> sample blobs
	dictTrained    bool                 // true once dicts have been trained
	dicts          map[string][]byte    // mimeGroup -> trained dict bytes
	dictIDs        map[string]uint32    // mimeGroup -> dict ID
	pendingEntries []*pendingEntry      // entries buffered during training phase

	// Search index builders — fed incrementally during AddEntry.
	titleTB *TrigramBuilder
	bodyTB  *TrigramBuilder

	// Progress tracking for AddEntry.
	contentCount int // number of content entries added so far
}

// pendingEntry holds an entry's content while we buffer for dictionary training.
type pendingEntry struct {
	entry   *entryBuilder
	content []byte
}

// Timings returns per-phase timing data captured during Close.
// Only valid after Close has been called.
func (w *Writer) Timings() Timings { return w.timings }

// CompressWorkers returns the resolved number of parallel compression workers.
func (w *Writer) CompressWorkers() int { return w.opts.CompressWorkers }

// NewWriter creates a Writer that will write the archive to wr.
// If opts is the zero value, sensible defaults are applied.
func NewWriter(wr io.ReadWriteSeeker, opts WriterOptions) *Writer {
	d := defaultOptions()
	if opts.ZstdLevel != 0 {
		d.ZstdLevel = opts.ZstdLevel
	}
	if opts.ChunkTargetSize != 0 {
		d.ChunkTargetSize = opts.ChunkTargetSize
	}
	if opts.DictSamples != 0 {
		d.DictSamples = opts.DictSamples
	}
	// Boolean fields: use caller value (false = off).
	d.TrainDict = opts.TrainDict
	d.BuildSearch = opts.BuildSearch
	d.BuildTitleSearch = opts.BuildTitleSearch
	d.BuildBodySearch = opts.BuildBodySearch
	// BuildSearch is a convenience that sets both.
	if opts.BuildSearch {
		d.BuildTitleSearch = true
		d.BuildBodySearch = true
	}
	d.MinifyHTML = opts.MinifyHTML
	d.MinifyCSS = opts.MinifyCSS
	d.MinifyJS = opts.MinifyJS
	d.MinifySVG = opts.MinifySVG
	d.OptimizeImages = opts.OptimizeImages
	d.Progress = opts.Progress
	d.SigningKeys = opts.SigningKeys
	if opts.CompressWorkers != 0 {
		d.CompressWorkers = opts.CompressWorkers
	}
	if d.CompressWorkers <= 0 {
		// Default to min(NumCPU, 4). Each zstd encoder with 8 MB window
		// uses significant internal state; more than 4 workers adds
		// memory pressure with diminishing throughput.
		n := runtime.NumCPU()
		if n > 4 {
			n = 4
		}
		d.CompressWorkers = n
	}
	if d.CompressWorkers < 1 {
		d.CompressWorkers = 1
	}

	w := &Writer{
		w:          wr,
		opts:       d,
		meta:       make(map[string][]byte),
		dedup:      newDedupMap(),
		openChunks: make(map[string]*chunkBuilder),
		cache:      newEncoderCache(),
	}

	// Initialise minifier if any transform is enabled.
	if d.MinifyHTML || d.MinifyCSS || d.MinifyJS || d.MinifySVG || d.OptimizeImages {
		m := minify.New()
		if d.MinifyHTML {
			m.AddFunc("text/html", html.Minify)
		}
		if d.MinifyCSS {
			m.AddFunc("text/css", css.Minify)
		}
		if d.MinifyJS {
			m.AddFunc("application/javascript", js.Minify)
			m.AddFunc("text/javascript", js.Minify)
		}
		if d.MinifySVG {
			m.AddFunc("image/svg+xml", svg.Minify)
		}
		w.minifier = m
	}

	// Initialise dictionary training buffers.
	if d.TrainDict {
		w.dictSamples = make(map[string][][]byte)
		w.dicts = make(map[string][]byte)
		w.dictIDs = make(map[string]uint32)
	} else {
		w.dictTrained = true // no training needed
		w.dicts = make(map[string][]byte)
		w.dictIDs = make(map[string]uint32)
	}

	// Initialise search index builders.
	if d.BuildTitleSearch {
		w.titleTB = newTrigramBuilder()
	}
	if d.BuildBodySearch {
		w.bodyTB = newTrigramBuilder()
	}

	return w
}

// SetMetadata stores a metadata key-value pair. Required keys: title, language,
// creator, date, source.
func (w *Writer) SetMetadata(key, value string) {
	w.meta[key] = []byte(value)
}

// AddEntry registers a content entry and returns its assigned ID.
// The content is transformed, hashed, search-indexed, and added to the current
// chunk. Once a chunk fills up it is compressed and flushed to a temp file.
// After this call returns, content is no longer held in memory.
func (w *Writer) AddEntry(path, title, mimeType string, content []byte, isFrontArticle bool) (uint32, error) {
	if w.closed {
		return 0, fmt.Errorf("ozawrite: writer is closed")
	}

	id := uint32(len(w.entries))

	// 1. Transform content in-place (minify, image optimise).
	tTransform := time.Now()
	content = w.transformContent(mimeType, content)
	w.timings.Transform += time.Since(tTransform)

	// 2. Compute content hash + dedup check.
	tDedup := time.Now()
	hash := sha256.Sum256(content)

	// 3. Create entry (without content).
	e := &entryBuilder{
		id:             id,
		path:           path,
		title:          title,
		mimeType:       mimeType,
		contentHash:    hash,
		isFrontArticle: isFrontArticle,
	}
	w.entries = append(w.entries, e)

	// 4. Feed search index before content is released.
	w.timings.Dedup += time.Since(tDedup)
	tSearch := time.Now()
	if isFrontArticle {
		if w.titleTB != nil {
			w.titleTB.IndexEntry(id, []byte(title))
		}
		if w.bodyTB != nil {
			w.bodyTB.IndexEntry(id, []byte(title+" "+path))
			if len(content) > 0 {
				searchContent := content
				if mimeType == "text/html" {
					searchContent = extractVisibleText(content)
				}
				if len(searchContent) > 0 {
					w.bodyTB.IndexEntry(id, searchContent)
				}
			}
		}
	}
	w.timings.SearchIndex += time.Since(tSearch)

	// 5. Deduplication check.
	tDedup2 := time.Now()
	if ref, ok := w.dedup.CheckHash(hash); ok {
		e.chunkID = ref.chunkID
		e.blobOffset = ref.blobOffset
		e.blobSize = ref.blobSize
		e.deduped = true
		w.timings.Dedup += time.Since(tDedup2)
		w.contentCount++
		return id, nil
	}
	w.timings.Dedup += time.Since(tDedup2)

	// 6. If still in dictionary training phase, buffer the entry.
	if !w.dictTrained {
		tChunk := time.Now()
		w.bufferForTraining(e, content)
		w.timings.ChunkBuild += time.Since(tChunk)
		w.contentCount++
		return id, nil
	}

	// 7. Add to chunk and flush if full.
	tChunk := time.Now()
	if err := w.addToChunk(e, content); err != nil {
		return 0, err
	}
	w.timings.ChunkBuild += time.Since(tChunk)

	w.contentCount++
	return id, nil
}

// AddRedirect registers a redirect entry and returns its tagged redirect ID.
// The returned ID has bit 31 set; bits 0-30 are the redirect record index.
// targetID must be a content entry ID (bit 31 clear).
func (w *Writer) AddRedirect(path, title string, targetID uint32) (uint32, error) {
	if w.closed {
		return 0, fmt.Errorf("ozawrite: writer is closed")
	}
	ridx := uint32(len(w.redirectEntries))
	w.redirectEntries = append(w.redirectEntries, &entryBuilder{
		id:            oza.MakeRedirectID(ridx),
		redirectIndex: ridx,
		path:          path,
		title:         title,
		isRedirect:    true,
		targetID:      targetID,
	})
	return oza.MakeRedirectID(ridx), nil
}

// transformContent applies minification and image optimization to content.
func (w *Writer) transformContent(mimeType string, content []byte) []byte {
	if len(content) == 0 {
		return content
	}
	if w.minifier != nil {
		content = minifyContent(w.minifier, mimeType, content)
	}
	if w.opts.OptimizeImages && isImageMIME(mimeType) {
		content = optimizeImage(mimeType, content)
	}
	return content
}

// bufferForTraining stores an entry for later processing once dictionaries are trained.
func (w *Writer) bufferForTraining(e *entryBuilder, content []byte) {
	// Collect samples for dictionary training (skip images).
	key := ChunkKey(e.mimeType, len(content))
	if key != "image" {
		if len(w.dictSamples[key]) < w.opts.DictSamples {
			// Make a copy for the sample since we hold it.
			sample := make([]byte, len(content))
			copy(sample, content)
			w.dictSamples[key] = append(w.dictSamples[key], sample)
		}
	}

	// Buffer the full entry+content for flushing after training.
	contentCopy := make([]byte, len(content))
	copy(contentCopy, content)
	w.pendingEntries = append(w.pendingEntries, &pendingEntry{
		entry:   e,
		content: contentCopy,
	})

	// Check if we have enough samples across all groups.
	if w.haveSufficientSamples() {
		w.trainAndFlushPending()
	}
}

// haveSufficientSamples returns true when we've collected enough dictionary
// training samples to proceed (or when we've buffered enough entries that
// it's time to train regardless).
func (w *Writer) haveSufficientSamples() bool {
	// Train after DictSamples entries across all html buckets,
	// or after 2*DictSamples total entries (whichever comes first).
	htmlTotal := len(w.dictSamples["html"]) + len(w.dictSamples["html-small"])
	if htmlTotal >= w.opts.DictSamples {
		return true
	}
	return len(w.pendingEntries) >= 2*w.opts.DictSamples
}

// trainAndFlushPending trains dictionaries from collected samples, then flushes
// all buffered entries through the normal chunk pipeline.
func (w *Writer) trainAndFlushPending() {
	if w.opts.Progress != nil {
		w.opts.Progress("dict-train", 0, 1)
	}

	nextID := uint32(1)
	for key, samps := range w.dictSamples {
		if len(samps) < 10 {
			continue
		}
		id := nextID
		nextID++
		d, err := trainDictionary(id, samps, 1024*1024)
		if err != nil || len(d) == 0 {
			continue
		}
		w.dicts[key] = d
		w.dictIDs[key] = id
	}

	if w.opts.Progress != nil {
		w.opts.Progress("dict-train", 1, 1)
	}

	// Free samples.
	w.dictSamples = nil
	w.dictTrained = true

	// Start parallel compression pipeline now that dictionaries are ready.
	w.startPipeline()

	// Sort pending entries by chunk key (MIME group + size bucket) then path
	// so that entries in the same bucket are chunked together.
	sort.Slice(w.pendingEntries, func(i, j int) bool {
		ki := ChunkKey(w.pendingEntries[i].entry.mimeType, len(w.pendingEntries[i].content))
		kj := ChunkKey(w.pendingEntries[j].entry.mimeType, len(w.pendingEntries[j].content))
		if ki != kj {
			return ki < kj
		}
		return w.pendingEntries[i].entry.path < w.pendingEntries[j].entry.path
	})

	// Flush all pending entries.
	for _, pe := range w.pendingEntries {
		// Already dedup-checked during AddEntry; add directly to chunk.
		_ = w.addToChunk(pe.entry, pe.content)
	}
	w.pendingEntries = nil
}

// addToChunk assigns an entry to the current open chunk for its MIME group.
// If the chunk reaches the target size, it is compressed and flushed to disk.
func (w *Writer) addToChunk(e *entryBuilder, content []byte) error {
	key := ChunkKey(e.mimeType, len(content))
	cb, ok := w.openChunks[key]
	if !ok || cb.uncompSize >= w.opts.ChunkTargetSize {
		// Flush the old chunk if it exists and is full.
		if ok && cb.uncompSize >= w.opts.ChunkTargetSize {
			if err := w.flushChunk(cb); err != nil {
				return err
			}
		}
		cb = &chunkBuilder{id: w.nextChunk, mimeGroup: key}
		w.nextChunk++
		w.openChunks[key] = cb
	}

	offset := cb.addBlob(content)
	e.chunkID = cb.id
	e.blobOffset = offset
	e.blobSize = uint32(len(content))

	w.dedup.Register(e.contentHash, dedupRef{
		chunkID:    cb.id,
		blobOffset: offset,
		blobSize:   e.blobSize,
	})

	// Check if this chunk is now full.
	if cb.uncompSize >= w.opts.ChunkTargetSize {
		if err := w.flushChunk(cb); err != nil {
			return err
		}
		delete(w.openChunks, key)
	}

	return nil
}

// startPipeline launches the parallel compression workers and writer goroutine.
// It is a no-op if CompressWorkers == 1 or the pipeline is already running.
func (w *Writer) startPipeline() {
	if w.opts.CompressWorkers <= 1 || w.compressIn != nil {
		return
	}
	workers := w.opts.CompressWorkers
	w.compressIn = make(chan compressJob, 2*workers)
	w.compressOut = make(chan compressResult, 2*workers)
	w.writerDone = make(chan error, 1)

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w.compressionWorker()
		}()
	}
	go func() {
		wg.Wait()
		close(w.compressOut)
	}()
	go w.chunkWriterLoop()
}

// flushChunk compresses a chunk and writes it to the temp file.
// In parallel mode, it dispatches to the worker pool instead.
func (w *Writer) flushChunk(cb *chunkBuilder) error {
	raw := cb.uncompressedBytes()
	cb.blobs = nil // free memory immediately

	if w.compressIn != nil {
		// Parallel path: check for pipeline errors, then dispatch.
		if w.pipelineErr != nil {
			return w.pipelineErr
		}
		select {
		case err := <-w.writerDone:
			w.pipelineErr = err
			return err
		default:
		}

		w.compressIn <- compressJob{
			chunkID:   cb.id,
			mimeGroup: cb.mimeGroup,
			raw:       raw,
			dict:      w.dicts[cb.mimeGroup],
			dictID:    w.dictIDs[cb.mimeGroup],
			level:     w.opts.ZstdLevel,
		}
		return nil
	}

	// Serial path.
	return w.flushChunkSync(cb.id, cb.mimeGroup, raw)
}

// flushChunkSync compresses a chunk synchronously and writes it to the temp file.
func (w *Writer) flushChunkSync(id uint32, mimeGroup string, raw []byte) error {
	// Ensure temp file is open.
	if w.chunkTmp == nil {
		f, err := os.CreateTemp("", "ozawrite-chunks-*")
		if err != nil {
			return fmt.Errorf("ozawrite: creating chunk temp file: %w", err)
		}
		w.chunkTmp = f
	}

	var compData []byte
	var compression uint8
	var dictID uint32

	if mimeGroup != "image" {
		dict := w.dicts[mimeGroup]
		dictID = w.dictIDs[mimeGroup]
		cd, err := w.cache.compress(raw, w.opts.ZstdLevel, dict, dictID)
		if err != nil {
			return fmt.Errorf("ozawrite: compressing chunk %d: %w", id, err)
		}
		compData = cd
		if len(dict) > 0 {
			compression = oza.CompZstdDict
		} else {
			compression = oza.CompZstd
		}
	} else {
		compData = raw
		compression = oza.CompNone
	}

	// Write to temp file.
	if _, err := w.chunkTmp.Write(compData); err != nil {
		return fmt.Errorf("ozawrite: writing chunk %d to temp file: %w", id, err)
	}

	w.chunkDescs = append(w.chunkDescs, chunkDesc{
		ID:             id,
		CompressedOff:  w.chunkOff,
		CompressedSize: uint64(len(compData)),
		DictID:         dictID,
		Compression:    compression,
	})
	w.chunkOff += uint64(len(compData))

	if w.opts.Progress != nil {
		w.opts.Progress("compress", len(w.chunkDescs), 0) // total unknown during streaming
	}

	return nil
}

// Close finalises the archive and writes it to the underlying writer.
func (w *Writer) Close() error {
	if w.closed {
		return fmt.Errorf("ozawrite: already closed")
	}
	w.closed = true

	// 1. Record writer parameters as metadata.
	if _, ok := w.meta["chunk_target_size"]; !ok {
		w.meta["chunk_target_size"] = []byte(strconv.Itoa(w.opts.ChunkTargetSize))
	}

	// 2. If dictionary training never triggered (small archive), train now and flush.
	if !w.dictTrained {
		w.trainAndFlushPending()
	}

	// 3. Flush any remaining open chunks.
	for group, cb := range w.openChunks {
		if cb.uncompSize > 0 {
			if err := w.flushChunk(cb); err != nil {
				return err
			}
		}
		delete(w.openChunks, group)
	}

	// 3b. Drain parallel compression pipeline.
	if w.compressIn != nil {
		close(w.compressIn)
		if err := <-w.writerDone; err != nil {
			return fmt.Errorf("ozawrite: compression pipeline: %w", err)
		}
		w.compressIn = nil
	}

	// Sort chunk descriptors by ID so that chunkDescs[i].ID == i.
	// Chunks may have been flushed out of ID order when different MIME groups
	// filled at different rates (e.g. CSS chunk flushed before HTML chunk).
	sort.Slice(w.chunkDescs, func(i, j int) bool {
		return w.chunkDescs[i].ID < w.chunkDescs[j].ID
	})

	// Report final chunk count.
	if w.opts.Progress != nil && len(w.chunkDescs) > 0 {
		w.opts.Progress("compress", len(w.chunkDescs), len(w.chunkDescs))
	}

	// 4. Validate metadata.
	if err := oza.ValidateMetadata(w.meta); err != nil {
		return fmt.Errorf("ozawrite: %w", err)
	}

	// 5. Build MIME table.
	t3 := time.Now()
	mimeTypes, mimeMap := w.buildMIMETable()

	// 6. Build entry table.
	entryTableBytes := w.buildEntryTable(mimeMap)

	// 7. Build path and title index sections.
	if w.opts.Progress != nil {
		w.opts.Progress("index-path", 0, 1)
	}
	pathIdxBytes, titleIdxBytes := w.buildIndexSections()
	if w.opts.Progress != nil {
		w.opts.Progress("index-path", 1, 1)
	}

	// 8. Serialize metadata section.
	metaBytes, err := oza.MarshalMetadata(w.meta)
	if err != nil {
		return fmt.Errorf("ozawrite: marshaling metadata: %w", err)
	}

	// 9. Serialize MIME table section.
	mimeBytes, err := oza.MarshalMIMETable(mimeTypes)
	if err != nil {
		return fmt.Errorf("ozawrite: marshaling MIME table: %w", err)
	}

	// 10. Build redirect section.
	redirectBytes := w.buildRedirectSection()

	// 10b. Optionally serialize ZstdDict sections.
	dictSections := buildDictSections(w.dicts, w.dictIDs)

	// 11. Build trigram search indices from the incrementally-built builders.
	var titleSearchBytes, bodySearchBytes []byte
	tSearch := time.Now()

	if w.titleTB != nil {
		if w.opts.Progress != nil {
			w.opts.Progress("index-search-title", 0, 1)
		}
		sb, err := w.titleTB.Build()
		if err != nil {
			return fmt.Errorf("ozawrite: building title search index: %w", err)
		}
		titleSearchBytes = sb
		w.titleTB = nil // free
		if w.opts.Progress != nil {
			w.opts.Progress("index-search-title", 1, 1)
		}
	}

	if w.bodyTB != nil {
		if w.opts.Progress != nil {
			w.opts.Progress("index-search-body", 0, 1)
		}
		sb, err := w.bodyTB.Build()
		if err != nil {
			return fmt.Errorf("ozawrite: building body search index: %w", err)
		}
		bodySearchBytes = sb
		w.bodyTB = nil // free
		if w.opts.Progress != nil {
			w.opts.Progress("index-search-body", 1, 1)
		}
	}
	w.timings.SearchIndex = time.Since(tSearch)

	// 12. Build in-memory sections (everything except CONTENT, which is streamed).
	//     The CONTENT section is written directly from the temp file to avoid
	//     materializing all compressed chunks in RAM.
	sections := []rawSection{
		newRawSection(oza.SectionMetadata, metaBytes),
		newRawSection(oza.SectionMIMETable, mimeBytes),
		compressRawSection(oza.SectionEntryTable, entryTableBytes),
		compressRawSection(oza.SectionPathIndex, pathIdxBytes),
		compressRawSection(oza.SectionTitleIndex, titleIdxBytes),
	}
	// The CONTENT section slot — we know its size but don't materialise it.
	contentSectionIdx := len(sections)
	contentSize := w.contentSectionSize()
	sections = append(sections, rawSection{
		typ:              oza.SectionContent,
		uncompressedSize: contentSize, // content section is not further compressed
		compression:      oza.CompNone,
	})
	if redirectBytes != nil {
		sections = append(sections, compressRawSection(oza.SectionRedirectTab, redirectBytes))
	}
	sections = append(sections, dictSections...)
	if titleSearchBytes != nil {
		sections = append(sections, compressRawSection(oza.SectionSearchTitle, titleSearchBytes))
	}
	if bodySearchBytes != nil {
		sections = append(sections, compressRawSection(oza.SectionSearchBody, bodySearchBytes))
	}

	// 13. Compute section offsets. Content section SHA is deferred (computed during streaming).
	numSections := uint32(len(sections))
	sectionTableOff := uint64(oza.HeaderSize)
	firstDataOff := sectionTableOff + uint64(numSections)*oza.SectionSize

	descs := make([]oza.SectionDesc, numSections)
	off := firstDataOff
	for i, s := range sections {
		var size uint64
		if i == contentSectionIdx {
			size = contentSize // known from chunk descriptors
		} else {
			size = uint64(len(s.data))
		}
		descs[i] = oza.SectionDesc{
			Type:             s.typ,
			Offset:           off,
			CompressedSize:   size,
			UncompressedSize: s.uncompressedSize,
			Compression:      s.compression,
		}
		// SHA-256 filled in below (content section is deferred).
		if i != contentSectionIdx {
			descs[i].SHA256 = computeSectionSHA256(s.data)
		}
		off += size
	}

	// 14. Compute file checksum position.
	checksumOff := off

	// 15. Build header.
	var uuid [16]byte
	if _, err := rand.Read(uuid[:]); err != nil {
		return fmt.Errorf("ozawrite: generating UUID: %w", err)
	}
	var totalContent uint64
	for _, e := range w.entries {
		totalContent += uint64(e.blobSize)
	}
	var flags uint32
	if titleSearchBytes != nil || bodySearchBytes != nil {
		flags |= oza.FlagHasSearch
	}
	if len(w.opts.SigningKeys) > 0 {
		flags |= oza.FlagHasSignatures
	}
	hdr := oza.Header{
		Magic:           oza.Magic,
		MajorVersion:    oza.MajorVersion,
		MinorVersion:    oza.MinorVersion,
		UUID:            uuid,
		SectionCount:    numSections,
		EntryCount:      uint32(len(w.entries)), // content entries only; redirects are in SectionRedirectTab
		ContentSize:     totalContent,
		SectionTableOff: sectionTableOff,
		ChecksumOff:     checksumOff,
		Flags:           flags,
	}

	// 16. Write placeholder header + section table.
	if w.opts.Progress != nil {
		w.opts.Progress("assemble", 0, 1)
	}
	if _, err := w.w.Write(make([]byte, firstDataOff)); err != nil {
		return fmt.Errorf("ozawrite: writing placeholder header: %w", err)
	}

	// 17. Stream each section to disk. Content section is streamed from temp file.
	for i := range sections {
		if i == contentSectionIdx {
			// Stream content section directly from temp file.
			sha, err := w.writeContentSection(w.w)
			if err != nil {
				return fmt.Errorf("ozawrite: writing content section: %w", err)
			}
			descs[i].SHA256 = sha
		} else {
			if _, err := w.w.Write(sections[i].data); err != nil {
				return fmt.Errorf("ozawrite: writing section: %w", err)
			}
			sections[i].data = nil
		}
	}

	// 18. Seek back and write the real header + section table.
	if _, err := w.w.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("ozawrite: seeking to header: %w", err)
	}
	hdrBytes := hdr.Marshal()
	if _, err := w.w.Write(hdrBytes[:]); err != nil {
		return fmt.Errorf("ozawrite: writing header: %w", err)
	}
	for _, d := range descs {
		db := d.Marshal()
		if _, err := w.w.Write(db[:]); err != nil {
			return fmt.Errorf("ozawrite: writing section table: %w", err)
		}
	}

	// 19. Stream-read to compute file-level SHA-256.
	if _, err := w.w.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("ozawrite: seeking for checksum: %w", err)
	}
	h := sha256.New()
	if _, err := io.CopyN(h, w.w, int64(checksumOff)); err != nil {
		return fmt.Errorf("ozawrite: computing file hash: %w", err)
	}
	var fileHash [32]byte
	copy(fileHash[:], h.Sum(nil))

	// 20. Write the 32-byte file hash.
	if _, err := w.w.Seek(int64(checksumOff), io.SeekStart); err != nil {
		return fmt.Errorf("ozawrite: seeking to checksum offset: %w", err)
	}
	if _, err := w.w.Write(fileHash[:]); err != nil {
		return fmt.Errorf("ozawrite: writing file hash: %w", err)
	}

	// 21. Append SIGNATURES trailer if signing keys are configured.
	if len(w.opts.SigningKeys) > 0 {
		trailer, err := buildSignatureTrailer(fileHash, w.opts.SigningKeys)
		if err != nil {
			return fmt.Errorf("ozawrite: building signature trailer: %w", err)
		}
		if _, err := w.w.Write(trailer); err != nil {
			return fmt.Errorf("ozawrite: writing signature trailer: %w", err)
		}
	}

	w.timings.Assemble = time.Since(t3)
	if w.opts.Progress != nil {
		w.opts.Progress("assemble", 1, 1)
	}

	// 21. Cleanup temp file.
	w.cleanupTemp()

	return nil
}

// cleanupTemp removes the chunk temp file and trigram temp files.
func (w *Writer) cleanupTemp() {
	if w.chunkTmp != nil {
		name := w.chunkTmp.Name()
		w.chunkTmp.Close()
		os.Remove(name)
		w.chunkTmp = nil
	}
	w.titleTB = nil
	w.bodyTB = nil
}

// contentSectionSize returns the total byte size of the CONTENT section.
func (w *Writer) contentSectionSize() uint64 {
	n := len(w.chunkDescs)
	tableBytes := uint64(4 + n*oza.ChunkDescSize)
	return tableBytes + w.chunkOff
}

// writeContentSection streams the CONTENT section directly to dst, computing
// its SHA-256 as it goes. This avoids materializing the entire section in memory.
func (w *Writer) writeContentSection(dst io.Writer) ([32]byte, error) {
	h := sha256.New()
	mw := io.MultiWriter(dst, h)

	n := len(w.chunkDescs)

	// Write chunk table header.
	var countBuf [4]byte
	binary.LittleEndian.PutUint32(countBuf[:], uint32(n))
	if _, err := mw.Write(countBuf[:]); err != nil {
		return [32]byte{}, err
	}
	for _, cd := range w.chunkDescs {
		b := marshalChunkDesc(cd)
		if _, err := mw.Write(b[:]); err != nil {
			return [32]byte{}, err
		}
	}

	// Stream compressed chunk data from temp file.
	if w.chunkTmp != nil && w.chunkOff > 0 {
		if _, err := w.chunkTmp.Seek(0, io.SeekStart); err != nil {
			return [32]byte{}, fmt.Errorf("seeking chunk temp file: %w", err)
		}
		if _, err := io.Copy(mw, w.chunkTmp); err != nil {
			return [32]byte{}, fmt.Errorf("streaming chunk data: %w", err)
		}
	}

	var sha [32]byte
	copy(sha[:], h.Sum(nil))
	return sha, nil
}

// buildMIMETable constructs the MIME type list (enforcing index 0/1/2) and a
// lookup map from type string to index.
func (w *Writer) buildMIMETable() ([]string, map[string]uint16) {
	// Start with the three mandatory types.
	types := []string{"text/html", "text/css", "application/javascript"}
	m := map[string]uint16{
		"text/html":              0,
		"text/css":               1,
		"application/javascript": 2,
	}
	for _, e := range w.entries {
		mt := e.mimeType
		if _, ok := m[mt]; !ok {
			m[mt] = uint16(len(types))
			types = append(types, mt)
		}
	}
	return types, m
}

// buildEntryTable serialises content entries as variable-length records with an
// offset table for O(1) random access.
//
// Layout: uint32 entry_count, uint32 record_data_offset,
//
//	uint32[N] offsets, variable-length records.
func (w *Writer) buildEntryTable(mimeMap map[string]uint16) []byte {
	count := len(w.entries)
	records := make([]byte, 0, count*16) // estimated avg ~15 bytes/record
	offsets := make([]uint32, count)

	for i, e := range w.entries {
		offsets[i] = uint32(len(records))
		var rec oza.EntryRecord
		rec.Type = oza.EntryContent
		rec.MIMEIndex = mimeMap[e.mimeType]
		rec.ChunkID = e.chunkID
		rec.BlobOffset = e.blobOffset
		rec.BlobSize = e.blobSize
		rec.ContentHash = truncateHash(e.contentHash)
		if e.isFrontArticle {
			rec.Flags |= oza.EntryFlagFrontArticle
		}
		records = oza.AppendVarEntryRecord(records, rec)
	}

	headerSize := oza.EntryTableHeaderSize + count*4
	buf := make([]byte, headerSize+len(records))
	binary.LittleEndian.PutUint32(buf[0:4], uint32(count))
	binary.LittleEndian.PutUint32(buf[4:8], uint32(headerSize))
	for i, off := range offsets {
		binary.LittleEndian.PutUint32(buf[oza.EntryTableHeaderSize+i*4:], off)
	}
	copy(buf[headerSize:], records)
	return buf
}

// buildRedirectSection serialises redirect entries into the compact format.
// Wire format: uint32 count + 5-byte records (flags, target_id).
func (w *Writer) buildRedirectSection() []byte {
	if len(w.redirectEntries) == 0 {
		return nil
	}
	data := make([]byte, 4+len(w.redirectEntries)*oza.RedirectRecordSize)
	binary.LittleEndian.PutUint32(data[0:4], uint32(len(w.redirectEntries)))
	for i, e := range w.redirectEntries {
		var rr oza.RedirectRecord
		rr.TargetID = e.targetID
		if e.isFrontArticle {
			rr.Flags |= oza.EntryFlagFrontArticle
		}
		b := rr.Marshal()
		copy(data[4+i*oza.RedirectRecordSize:], b[:])
	}
	return data
}

// buildIndexSections builds the raw path and title index section bytes.
// Both content and redirect entries are included; redirect entries use
// tagged IDs (bit 31 set).
func (w *Writer) buildIndexSections() (pathIdx []byte, titleIdx []byte) {
	total := len(w.entries) + len(w.redirectEntries)
	paths := make([]pathRecord, 0, total)
	titles := make([]titleRecord, 0, total)
	for _, e := range w.entries {
		paths = append(paths, pathRecord{entryID: e.id, path: e.path})
		titles = append(titles, titleRecord{entryID: e.id, title: e.title})
	}
	for _, e := range w.redirectEntries {
		taggedID := oza.MakeRedirectID(e.redirectIndex)
		paths = append(paths, pathRecord{entryID: taggedID, path: e.path})
		titles = append(titles, titleRecord{entryID: taggedID, title: e.title})
	}
	return buildPathIndex(paths), buildTitleIndex(titles)
}

// buildDictSections creates one rawSection per trained dictionary.
// Dict section data: [0:4] dictID uint32, [4:] raw dict bytes.
func buildDictSections(dicts map[string][]byte, dictIDs map[string]uint32) []rawSection {
	var out []rawSection
	for group, d := range dicts {
		id := dictIDs[group]
		data := make([]byte, 4+len(d))
		binary.LittleEndian.PutUint32(data[0:4], id)
		copy(data[4:], d)
		out = append(out, newRawSection(oza.SectionZstdDict, data))
	}
	return out
}

// newRawSection creates an uncompressed rawSection.
func newRawSection(typ oza.SectionType, data []byte) rawSection {
	return rawSection{
		typ:              typ,
		data:             data,
		uncompressedSize: uint64(len(data)),
		compression:      oza.CompNone,
	}
}

// compressRawSection compresses data with Zstd. If compression doesn't reduce
// size (or the section is tiny), it falls back to uncompressed.
func compressRawSection(typ oza.SectionType, data []byte) rawSection {
	const minCompressSize = 256 // don't bother compressing tiny sections
	if len(data) < minCompressSize {
		return newRawSection(typ, data)
	}
	compressed, err := compressZstd(data, 19, nil)
	if err != nil || len(compressed) >= len(data) {
		return newRawSection(typ, data)
	}
	return rawSection{
		typ:              typ,
		data:             compressed,
		uncompressedSize: uint64(len(data)),
		compression:      oza.CompZstd,
	}
}
