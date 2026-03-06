// Package lazyloader provides chunk-based lazy loading with thread-safe caching.
package lazyloader

import (
	"strings"
	"sync"
)

// LazyLoader loads content in chunks on demand.
type LazyLoader struct {
	mu          sync.Mutex
	chunks      map[int]string
	totalChunks int
	chunkSize   int
	loadFn      func(index int) (string, error)
}

// New creates a LazyLoader.
func New(totalSize, chunkSize int, loadFn func(index int) (string, error)) *LazyLoader {
	return &LazyLoader{
		chunks:      make(map[int]string),
		totalChunks: (totalSize + chunkSize - 1) / chunkSize,
		chunkSize:   chunkSize,
		loadFn:      loadFn,
	}
}

// GetChunk returns the chunk at the given index, loading it if needed.
func (l *LazyLoader) GetChunk(index int) (string, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if index < 0 || index >= l.totalChunks {
		return "", false
	}

	if chunk, ok := l.chunks[index]; ok {
		return chunk, true
	}

	chunk, err := l.loadFn(index)
	if err != nil {
		return "", false
	}

	l.chunks[index] = chunk
	return chunk, true
}

// Clear removes all cached chunks.
func (l *LazyLoader) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.chunks = make(map[int]string)
}

// CachedCount returns the number of cached chunks.
func (l *LazyLoader) CachedCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.chunks)
}

// StringLoader provides lazy loading for string content split into lines.
type StringLoader struct {
	loader *LazyLoader
	lines  []string
}

// NewStringLoader creates a StringLoader from content.
func NewStringLoader(content string, chunkSize int) *StringLoader {
	lines := strings.Split(content, "\n")
	sl := &StringLoader{lines: lines}

	sl.loader = New(len(lines), chunkSize, func(index int) (string, error) {
		start := index * chunkSize
		end := start + chunkSize
		if end > len(sl.lines) {
			end = len(sl.lines)
		}
		if start >= len(sl.lines) {
			return "", nil
		}
		return strings.Join(sl.lines[start:end], "\n"), nil
	})

	return sl
}

// GetChunk returns the chunk at the given index.
func (sl *StringLoader) GetChunk(index int) (string, bool) {
	return sl.loader.GetChunk(index)
}

// GetLines returns lines in the given range.
func (sl *StringLoader) GetLines(startLine, endLine int) []string {
	if startLine < 0 || endLine <= startLine || startLine >= len(sl.lines) {
		return nil
	}
	if endLine > len(sl.lines) {
		endLine = len(sl.lines)
	}
	return sl.lines[startLine:endLine]
}

// Clear removes all cached chunks.
func (sl *StringLoader) Clear() {
	sl.loader.Clear()
}

// CachedCount returns cached chunk count.
func (sl *StringLoader) CachedCount() int {
	return sl.loader.CachedCount()
}
