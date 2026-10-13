package criteriainput

const featureHashStart = uint64(14695981039346656037)

func featureHashWord(hash, word uint64) uint64 {
	for range 8 {
		hash = (hash ^ (word & 255)) * 1099511628211
		word >>= 8
	}
	return hash
}

func featureHashText(hash uint64, text string) uint64 {
	hash = featureHashWord(hash, uint64(len(text)))
	for i := range len(text) {
		hash = (hash ^ uint64(text[i])) * 1099511628211
	}
	return hash
}

// Stream Unicode code points through ordered 1/2/3-grams. Source case, spacing
// and punctuation are retained. No tokenizer, text generation or labels are read.
func intentFeatures(text string, output []float32) {
	var window [3]rune
	count := 0
	for _, symbol := range text {
		window[0], window[1], window[2] = window[1], window[2], symbol
		if count < len(window) {
			count++
		}
		for width := 1; width <= count; width++ {
			hash := featureHashWord(featureHashStart, uint64(width))
			for _, value := range window[len(window)-width:] {
				hash = featureHashWord(hash, uint64(value))
			}
			output[hash%uint64(len(output))]++
		}
	}
}
