package database

// ChunkBy splits items into chunks of at most chunkSize elements.
func ChunkBy[T any](items []T, chunkSize int) (chunks [][]T) {
	if chunkSize <= 0 || len(items) == 0 {
		return nil
	}
	for chunkSize < len(items) {
		items, chunks = items[chunkSize:], append(chunks, items[0:chunkSize:chunkSize])
	}
	return append(chunks, items)
}
