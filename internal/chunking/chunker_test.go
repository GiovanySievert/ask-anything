package chunking

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSplit_EmptyText(t *testing.T) {
	require.Nil(t, Split("", 10, 2))
	require.Nil(t, Split("   ", 10, 2))
}

func TestSplit_ShorterThanSize(t *testing.T) {
	chunks := Split("hello", 10, 2)
	require.Equal(t, []string{"hello"}, chunks)
}

func TestSplit_ExactMultiple(t *testing.T) {
	chunks := Split("abcdef", 3, 0)
	require.Equal(t, []string{"abc", "def"}, chunks)
}

func TestSplit_WithOverlap(t *testing.T) {
	chunks := Split("abcdefg", 4, 2)
	require.Equal(t, []string{"abcd", "cdef", "efg"}, chunks)
}

func TestSplit_InvalidSize(t *testing.T) {
	require.Nil(t, Split("abc", 0, 0))
	require.Nil(t, Split("abc", -1, 0))
}

func TestSplit_OverlapClampedWhenTooLarge(t *testing.T) {
	chunks := Split("abcdef", 3, 3)
	require.Equal(t, []string{"abc", "def"}, chunks)
}

func TestSplit_DoesNotBreakMultibyteRunes(t *testing.T) {
	chunks := Split("áéíóú", 2, 0)
	require.Equal(t, []string{"áé", "íó", "ú"}, chunks)
	for _, c := range chunks {
		require.True(t, len([]rune(c)) <= 2)
	}
}
