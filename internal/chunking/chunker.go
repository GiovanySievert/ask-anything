package chunking

import "strings"

func Split(text string, size, overlap int) []string {
	if size <= 0 {
		return nil
	}
	if overlap < 0 || overlap >= size {
		overlap = 0
	}

	runes := []rune(strings.TrimSpace(text))
	if len(runes) == 0 {
		return nil
	}
	if len(runes) <= size {
		return []string{string(runes)}
	}

	step := size - overlap
	var chunks []string
	for start := 0; start < len(runes); start += step {
		end := start + size
		if end > len(runes) {
			end = len(runes)
		}
		chunk := strings.TrimSpace(string(runes[start:end]))
		if chunk != "" {
			chunks = append(chunks, chunk)
		}
		if end == len(runes) {
			break
		}
	}
	return chunks
}
