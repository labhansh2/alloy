package nodes

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

const minSignificantRunes = 50

// significantChange reports whether after meaningfully differs from before.
func significantChange(before, after string) bool {
	before = strings.TrimSpace(before)
	after = strings.TrimSpace(after)
	if before == after {
		return false
	}
	br := utf8.RuneCountInString(before)
	ar := utf8.RuneCountInString(after)
	if absInt(ar-br) >= minSignificantRunes {
		return true
	}
	return wordDiff(before, after) >= 12
}

func containsBangAI(content string) bool {
	return strings.Contains(strings.ToLower(content), "!ai")
}

func wordDiff(a, b string) int {
	wa := wordSet(a)
	wb := wordSet(b)
	diff := 0
	for w := range wa {
		if !wb[w] {
			diff++
		}
	}
	for w := range wb {
		if !wa[w] {
			diff++
		}
	}
	return diff
}

func wordSet(s string) map[string]bool {
	out := make(map[string]bool)
	var b strings.Builder
	flush := func() {
		if b.Len() == 0 {
			return
		}
		out[strings.ToLower(b.String())] = true
		b.Reset()
	}
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			continue
		}
		flush()
	}
	flush()
	return out
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
