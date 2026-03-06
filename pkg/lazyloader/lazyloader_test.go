package lazyloader

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	ll := New(100, 10, func(index int) (string, error) {
		return fmt.Sprintf("chunk-%d", index), nil
	})
	assert.NotNil(t, ll)
}

func TestGetChunk_ReturnsContent(t *testing.T) {
	ll := New(100, 10, func(index int) (string, error) {
		return fmt.Sprintf("chunk-%d", index), nil
	})
	chunk, ok := ll.GetChunk(0)
	assert.True(t, ok)
	assert.Equal(t, "chunk-0", chunk)
}

func TestGetChunk_OutOfRange(t *testing.T) {
	ll := New(10, 10, func(index int) (string, error) {
		return "", nil
	})
	_, ok := ll.GetChunk(99)
	assert.False(t, ok)
}

func TestGetChunk_Caches(t *testing.T) {
	callCount := 0
	ll := New(100, 10, func(index int) (string, error) {
		callCount++
		return "data", nil
	})
	ll.GetChunk(0)
	ll.GetChunk(0)
	assert.Equal(t, 1, callCount)
}

func TestClear(t *testing.T) {
	ll := New(100, 10, func(index int) (string, error) {
		return "data", nil
	})
	ll.GetChunk(0)
	assert.Equal(t, 1, ll.CachedCount())
	ll.Clear()
	assert.Equal(t, 0, ll.CachedCount())
}

func TestStringLoader_GetChunk(t *testing.T) {
	lines := make([]string, 100)
	for i := range lines {
		lines[i] = fmt.Sprintf("Line %d", i+1)
	}
	content := strings.Join(lines, "\n")
	sl := NewStringLoader(content, 10)

	chunk, ok := sl.GetChunk(0)
	assert.True(t, ok)
	assert.True(t, strings.HasPrefix(chunk, "Line 1"))
}

func TestStringLoader_GetLines(t *testing.T) {
	lines := make([]string, 50)
	for i := range lines {
		lines[i] = fmt.Sprintf("Line %d", i+1)
	}
	content := strings.Join(lines, "\n")
	sl := NewStringLoader(content, 10)

	result := sl.GetLines(0, 5)
	assert.Equal(t, 5, len(result))
	assert.Equal(t, "Line 1", result[0])
	assert.Equal(t, "Line 5", result[4])
}

func TestStringLoader_Clear(t *testing.T) {
	sl := NewStringLoader("a\nb\nc", 2)
	sl.GetChunk(0)
	assert.Equal(t, 1, sl.CachedCount())
	sl.Clear()
	assert.Equal(t, 0, sl.CachedCount())
}
