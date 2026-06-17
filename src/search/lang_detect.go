package search



var languageStopWords = map[string][]string{
	"en": {"the", "be", "to", "of", "and", "a", "in", "that", "have", "with", "this", "for", "on"},
	"es": {"el", "la", "los", "en", "y", "de", "un", "que", "es", "una", "para", "con", "por"},
	"fr": {"le", "la", "les", "en", "et", "de", "un", "que", "est", "une", "dans", "pour", "avec"},
	"de": {"der", "die", "das", "und", "ist", "in", "zu", "den", "von", "mit", "ein", "eine", "auf"},
	"it": {"il", "la", "i", "in", "e", "di", "un", "che", "ed", "una", "per", "con", "da"},
	"pt": {"o", "a", "os", "as", "em", "e", "de", "um", "que", "uma", "para", "com", "por"},
}

// DetectLanguage estimates the language of the text.
func DetectLanguage(text string) string {
	words := tokenize(text)
	if len(words) == 0 {
		return "unknown"
	}

	wordMap := make(map[string]int)
	for _, w := range words {
		wordMap[w]++
	}

	bestLang := "unknown"
	maxMatches := 0

	for lang, stops := range languageStopWords {
		matches := 0
		for _, stop := range stops {
			if count, ok := wordMap[stop]; ok {
				matches += count
			}
		}
		if matches > maxMatches {
			maxMatches = matches
			bestLang = lang
		}
	}

	if maxMatches > 0 && float64(maxMatches)/float64(len(words)) >= 0.02 {
		return bestLang
	}

	return "unknown"
}
