package oza

import "iter"

// Entries returns an iterator over all content entries in ID order.
// Iteration stops silently on error. Use [Archive.EntriesErr] if error
// propagation is needed.
func (a *Archive) Entries() iter.Seq[Entry] {
	return func(yield func(Entry) bool) {
		for i := uint32(0); i < a.EntryCount(); i++ {
			e, err := a.EntryByID(i)
			if err != nil {
				return
			}
			if !yield(e) {
				return
			}
		}
	}
}

// RedirectEntries returns an iterator over all redirect entries in index order.
// Iteration stops silently on error. Use [Archive.RedirectEntriesErr] if error
// propagation is needed.
func (a *Archive) RedirectEntries() iter.Seq[Entry] {
	return func(yield func(Entry) bool) {
		for i := uint32(0); i < a.redirectCount; i++ {
			e, err := a.EntryByID(MakeRedirectID(i))
			if err != nil {
				return
			}
			if !yield(e) {
				return
			}
		}
	}
}

// entryFromIndex builds an Entry from an index record (path or title index).
// Handles both content IDs and tagged redirect IDs.
func (a *Archive) entryFromIndex(id uint32, knownKey string, isPath bool) (Entry, bool) {
	if IsRedirectID(id) {
		e, err := a.redirectEntryByIndex(id)
		if err != nil {
			return Entry{}, false
		}
		if isPath {
			e.path = knownKey
		} else {
			e.title = knownKey
		}
		return e, true
	}
	rec, err := a.contentEntryRecord(id)
	if err != nil {
		return Entry{}, false
	}
	e := Entry{archive: a, record: rec}
	if isPath {
		e.path = knownKey
		e.title = a.idToTitle[id]
	} else {
		e.path = a.idToPath[id]
		e.title = knownKey
	}
	return e, true
}

// EntriesByPath returns an iterator over all entries in path-sorted order.
// Includes both content and redirect entries. Iteration stops silently on
// error. Use [Archive.EntriesByPathErr] if error propagation is needed.
func (a *Archive) EntriesByPath() iter.Seq[Entry] {
	return func(yield func(Entry) bool) {
		if a.pathIdx == nil {
			return
		}
		for i := 0; i < a.pathIdx.Count(); i++ {
			id, path, err := a.pathIdx.Record(i)
			if err != nil {
				return
			}
			e, ok := a.entryFromIndex(id, path, true)
			if !ok {
				return
			}
			if !yield(e) {
				return
			}
		}
	}
}

// EntriesByTitle returns an iterator over all entries in title-sorted order.
// Includes both content and redirect entries. Iteration stops silently on
// error. Use [Archive.EntriesByTitleErr] if error propagation is needed.
func (a *Archive) EntriesByTitle() iter.Seq[Entry] {
	return func(yield func(Entry) bool) {
		if a.titleIdx == nil {
			return
		}
		for i := 0; i < a.titleIdx.Count(); i++ {
			id, title, err := a.titleIdx.Record(i)
			if err != nil {
				return
			}
			e, ok := a.entryFromIndex(id, title, false)
			if !ok {
				return
			}
			if !yield(e) {
				return
			}
		}
	}
}

// FrontArticles returns an iterator over content entries that have the front-article flag set.
// Iteration stops silently on error. Use [Archive.FrontArticlesErr] if error
// propagation is needed.
func (a *Archive) FrontArticles() iter.Seq[Entry] {
	return func(yield func(Entry) bool) {
		for i := uint32(0); i < a.EntryCount(); i++ {
			e, err := a.EntryByID(i)
			if err != nil {
				return
			}
			if !e.IsFrontArticle() {
				continue
			}
			if !yield(e) {
				return
			}
		}
	}
}

// EntriesErr returns an iterator over all content entries in ID order.
// Unlike [Archive.Entries], errors are propagated to the caller. When a non-nil
// error is yielded the Entry is zero-valued and iteration should stop.
func (a *Archive) EntriesErr() iter.Seq2[Entry, error] {
	return func(yield func(Entry, error) bool) {
		for i := uint32(0); i < a.EntryCount(); i++ {
			e, err := a.EntryByID(i)
			if err != nil {
				yield(Entry{}, err)
				return
			}
			if !yield(e, nil) {
				return
			}
		}
	}
}

// RedirectEntriesErr returns an iterator over all redirect entries in index order.
// Unlike [Archive.RedirectEntries], errors are propagated to the caller.
func (a *Archive) RedirectEntriesErr() iter.Seq2[Entry, error] {
	return func(yield func(Entry, error) bool) {
		for i := uint32(0); i < a.redirectCount; i++ {
			e, err := a.EntryByID(MakeRedirectID(i))
			if err != nil {
				yield(Entry{}, err)
				return
			}
			if !yield(e, nil) {
				return
			}
		}
	}
}

// entryFromIndexErr is like entryFromIndex but returns an error instead of a bool.
func (a *Archive) entryFromIndexErr(id uint32, knownKey string, isPath bool) (Entry, error) {
	if IsRedirectID(id) {
		e, err := a.redirectEntryByIndex(id)
		if err != nil {
			return Entry{}, err
		}
		if isPath {
			e.path = knownKey
		} else {
			e.title = knownKey
		}
		return e, nil
	}
	rec, err := a.contentEntryRecord(id)
	if err != nil {
		return Entry{}, err
	}
	e := Entry{archive: a, record: rec}
	if isPath {
		e.path = knownKey
		e.title = a.idToTitle[id]
	} else {
		e.path = a.idToPath[id]
		e.title = knownKey
	}
	return e, nil
}

// EntriesByPathErr returns an iterator over all entries in path-sorted order.
// Unlike [Archive.EntriesByPath], errors are propagated to the caller.
func (a *Archive) EntriesByPathErr() iter.Seq2[Entry, error] {
	return func(yield func(Entry, error) bool) {
		if a.pathIdx == nil {
			return
		}
		for i := 0; i < a.pathIdx.Count(); i++ {
			id, path, err := a.pathIdx.Record(i)
			if err != nil {
				yield(Entry{}, err)
				return
			}
			e, err := a.entryFromIndexErr(id, path, true)
			if err != nil {
				yield(Entry{}, err)
				return
			}
			if !yield(e, nil) {
				return
			}
		}
	}
}

// EntriesByTitleErr returns an iterator over all entries in title-sorted order.
// Unlike [Archive.EntriesByTitle], errors are propagated to the caller.
func (a *Archive) EntriesByTitleErr() iter.Seq2[Entry, error] {
	return func(yield func(Entry, error) bool) {
		if a.titleIdx == nil {
			return
		}
		for i := 0; i < a.titleIdx.Count(); i++ {
			id, title, err := a.titleIdx.Record(i)
			if err != nil {
				yield(Entry{}, err)
				return
			}
			e, err := a.entryFromIndexErr(id, title, false)
			if err != nil {
				yield(Entry{}, err)
				return
			}
			if !yield(e, nil) {
				return
			}
		}
	}
}

// FrontArticlesErr returns an iterator over content entries with the front-article flag.
// Unlike [Archive.FrontArticles], errors are propagated to the caller.
func (a *Archive) FrontArticlesErr() iter.Seq2[Entry, error] {
	return func(yield func(Entry, error) bool) {
		for i := uint32(0); i < a.EntryCount(); i++ {
			e, err := a.EntryByID(i)
			if err != nil {
				yield(Entry{}, err)
				return
			}
			if !e.IsFrontArticle() {
				continue
			}
			if !yield(e, nil) {
				return
			}
		}
	}
}
