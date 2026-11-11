package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChunkBy(t *testing.T) {
	t.Run("empty slice", func(t *testing.T) {
		var items []int
		got := ChunkBy(items, 2)
		assert.Nil(t, got)
	})

	t.Run("zero or negative chunk size", func(t *testing.T) {
		items := []int{1, 2, 3}
		assert.Nil(t, ChunkBy(items, 0))
		assert.Nil(t, ChunkBy(items, -1))
	})

	t.Run("chunk size larger than items", func(t *testing.T) {
		items := []int{1, 2, 3}
		got := ChunkBy(items, 5)
		assert.Equal(t, [][]int{{1, 2, 3}}, got)
	})

	t.Run("exact division", func(t *testing.T) {
		items := []int{1, 2, 3, 4}
		got := ChunkBy(items, 2)
		assert.Equal(t, [][]int{{1, 2}, {3, 4}}, got)
	})

	t.Run("remainder chunk", func(t *testing.T) {
		items := []int{1, 2, 3, 4, 5}
		got := ChunkBy(items, 2)
		assert.Equal(t, [][]int{{1, 2}, {3, 4}, {5}}, got)
	})
}
