// Package chunker splits text into overlapping word-based chunks.
package chunker

import "strings"

// Chunk splits text into chunks of chunkSize words with overlap words of overlap.
func Chunk(text string, chunkSize, overlap int) []string {
	words := strings.Fields(text)

	if len(words) <= chunkSize {
		return []string{text}
	}

	var chunks []string
	start := 0

	for start < len(words) {
		end := start + chunkSize
		if end > len(words) {
			end = len(words)
		}

		chunk := strings.Join(words[start:end], " ")
		chunks = append(chunks, chunk)

		start = end - overlap

		// If remaining text would be too small, grab it as final chunk.
		if start < len(words) && start+chunkSize-overlap >= len(words) {
			final := strings.Join(words[start:], " ")
			chunks = append(chunks, final)
			break
		}
	}

	return chunks
}
