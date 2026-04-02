package ozawrite

import (
	"fmt"
	"os"
	"sort"
	"sync"

	"github.com/stazelabs/oza/oza"
)

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

	// Trial compression: discard any dictionary that doesn't produce a
	// net size reduction when its storage cost is included. This avoids
	// bloating small archives with disproportionately large dictionaries.
	//
	// Open design questions (not yet exposed as options):
	//   - Should there be a --force-dict flag to skip this trial?
	//   - Should --dict-trial-chunks N control the sample count (default 8)?
	//   - Should trial results be reported in converter stats?
	w.trialCompressDicts()

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
		if err := w.addToChunk(pe.entry, pe.content); err != nil {
			w.pipelineErr = err
			break
		}
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

// trialCompressDicts compresses a sample of pending chunks both with and
// without each trained dictionary. If a dictionary's storage cost exceeds the
// compression savings it provides, it is discarded. This prevents small
// archives from being bloated by disproportionately large dictionaries.
func (w *Writer) trialCompressDicts() {
	if len(w.dicts) == 0 {
		return
	}

	// Group pending entry content by chunk key.
	groups := make(map[string][][]byte)
	for _, pe := range w.pendingEntries {
		key := ChunkKey(pe.entry.mimeType, len(pe.content))
		groups[key] = append(groups[key], pe.content)
	}

	const maxTrialChunks = 8

	for key, dict := range w.dicts {
		entries := groups[key]
		if len(entries) == 0 {
			continue
		}

		// Build trial chunks by concatenating entries up to ChunkTargetSize.
		var chunks [][]byte
		var current []byte
		for _, content := range entries {
			current = append(current, content...)
			if len(current) >= w.opts.ChunkTargetSize {
				chunks = append(chunks, current)
				current = nil
				if len(chunks) >= maxTrialChunks {
					break
				}
			}
		}
		if len(current) > 0 && len(chunks) < maxTrialChunks {
			chunks = append(chunks, current)
		}

		// Compress each trial chunk both ways.
		var totalWithDict, totalWithoutDict int64
		for _, chunk := range chunks {
			withDict, err := compressZstd(chunk, w.opts.ZstdLevel, dict)
			if err != nil {
				continue
			}
			withoutDict, err := compressZstd(chunk, w.opts.ZstdLevel, nil)
			if err != nil {
				continue
			}
			totalWithDict += int64(len(withDict))
			totalWithoutDict += int64(len(withoutDict))
		}

		// Include dictionary storage cost.
		totalWithDict += int64(len(dict))

		if totalWithDict >= totalWithoutDict {
			delete(w.dicts, key)
			delete(w.dictIDs, key)
		}
	}
}
