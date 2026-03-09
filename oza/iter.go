package oza

import "iter"

// Entries returns an iterator over all content entries in ID order.
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
// Includes both content and redirect entries.
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
// Includes both content and redirect entries.
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
