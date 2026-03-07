package lazyloader

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestGetChunk_LoadFnError tests GetChunk when the load function returns
// an error, covering the error branch in GetChunk.
func TestGetChunk_LoadFnError(t *testing.T) {
	ll := New(100, 10, func(index int) (string, error) {
		return "", errors.New("load error")
	})
	chunk, ok := ll.GetChunk(0)
	assert.False(t, ok)
	assert.Equal(t, "", chunk)
}

// TestGetChunk_NegativeIndex tests GetChunk with a negative index.
func TestGetChunk_NegativeIndex(t *testing.T) {
	ll := New(100, 10, func(index int) (string, error) {
		return "data", nil
	})
	chunk, ok := ll.GetChunk(-1)
	assert.False(t, ok)
	assert.Equal(t, "", chunk)
}

// TestNewStringLoader_EmptyContent tests NewStringLoader with empty content.
func TestNewStringLoader_EmptyContent(t *testing.T) {
	sl := NewStringLoader("", 5)
	assert.NotNil(t, sl)

	// Empty content splits into one empty string
	chunk, ok := sl.GetChunk(0)
	assert.True(t, ok)
	assert.Equal(t, "", chunk)
}

// TestNewStringLoader_ChunkBeyondLines tests NewStringLoader when the
// start index is beyond the available lines, covering the
// "start >= len(sl.lines)" branch in the load function.
func TestNewStringLoader_ChunkBeyondLines(t *testing.T) {
	// Create content with exactly 3 lines, chunkSize=2.
	// This creates 2 chunks: chunk 0 (lines 0-1), chunk 1 (line 2).
	sl := NewStringLoader("a\nb\nc", 2)

	// Chunk 0 should work
	chunk, ok := sl.GetChunk(0)
	assert.True(t, ok)
	assert.Equal(t, "a\nb", chunk)

	// Chunk 1 should work (last line)
	chunk, ok = sl.GetChunk(1)
	assert.True(t, ok)
	assert.Equal(t, "c", chunk)
}

// TestNewStringLoader_ExactChunkBoundary tests NewStringLoader when the
// content lines divide evenly by chunkSize.
func TestNewStringLoader_ExactChunkBoundary(t *testing.T) {
	lines := make([]string, 10)
	for i := range lines {
		lines[i] = fmt.Sprintf("line-%d", i)
	}
	content := strings.Join(lines, "\n")
	sl := NewStringLoader(content, 5)

	chunk, ok := sl.GetChunk(0)
	assert.True(t, ok)
	assert.Equal(t, "line-0\nline-1\nline-2\nline-3\nline-4", chunk)

	chunk, ok = sl.GetChunk(1)
	assert.True(t, ok)
	assert.Equal(t, "line-5\nline-6\nline-7\nline-8\nline-9", chunk)
}

// TestGetLines_NegativeStartLine tests GetLines with a negative startLine.
func TestGetLines_NegativeStartLine(t *testing.T) {
	sl := NewStringLoader("a\nb\nc", 2)
	result := sl.GetLines(-1, 2)
	assert.Nil(t, result)
}

// TestGetLines_EndLessOrEqualStart tests GetLines when endLine <= startLine.
func TestGetLines_EndLessOrEqualStart(t *testing.T) {
	sl := NewStringLoader("a\nb\nc", 2)
	result := sl.GetLines(2, 2)
	assert.Nil(t, result)

	result = sl.GetLines(3, 1)
	assert.Nil(t, result)
}

// TestGetLines_StartBeyondContent tests GetLines when startLine >= len(lines).
func TestGetLines_StartBeyondContent(t *testing.T) {
	sl := NewStringLoader("a\nb\nc", 2)
	result := sl.GetLines(100, 200)
	assert.Nil(t, result)
}

// TestGetLines_EndBeyondContent tests GetLines when endLine > len(lines),
// covering the endLine clamping branch.
func TestGetLines_EndBeyondContent(t *testing.T) {
	sl := NewStringLoader("a\nb\nc", 2)
	result := sl.GetLines(0, 100)
	assert.Equal(t, 3, len(result))
	assert.Equal(t, "a", result[0])
	assert.Equal(t, "b", result[1])
	assert.Equal(t, "c", result[2])
}

// TestGetLines_PartialRange tests GetLines with a range within bounds.
func TestGetLines_PartialRange(t *testing.T) {
	lines := make([]string, 20)
	for i := range lines {
		lines[i] = fmt.Sprintf("line-%d", i)
	}
	content := strings.Join(lines, "\n")
	sl := NewStringLoader(content, 5)

	result := sl.GetLines(5, 10)
	assert.Equal(t, 5, len(result))
	assert.Equal(t, "line-5", result[0])
	assert.Equal(t, "line-9", result[4])
}

// TestStringLoader_CachedCount_Initial tests CachedCount before loading.
func TestStringLoader_CachedCount_Initial(t *testing.T) {
	sl := NewStringLoader("a\nb\nc", 2)
	assert.Equal(t, 0, sl.CachedCount())
}

// TestNewStringLoader_SingleLineContent tests NewStringLoader with
// single-line content.
func TestNewStringLoader_SingleLineContent(t *testing.T) {
	sl := NewStringLoader("single line", 1)
	chunk, ok := sl.GetChunk(0)
	assert.True(t, ok)
	assert.Equal(t, "single line", chunk)
}

// TestNewStringLoader_ChunkSizeLargerThanContent tests when chunkSize
// exceeds the line count, so all lines are in chunk 0.
func TestNewStringLoader_ChunkSizeLargerThanContent(t *testing.T) {
	sl := NewStringLoader("a\nb\nc", 100)
	chunk, ok := sl.GetChunk(0)
	assert.True(t, ok)
	assert.Equal(t, "a\nb\nc", chunk)

	// Chunk 1 should not exist
	_, ok = sl.GetChunk(1)
	assert.False(t, ok)
}
