package normalize

import (
	"strings"
	"unicode"
)

// Title normalizes a game title for deduplication across launchers.
// "Hades™" and "Hades" both become "hades".
func Title(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	prevSpace := true
	for _, r := range strings.ToLower(s) {
		switch {
		case r == '™' || r == '®' || r == '©':
			// drop trademark symbols
		case r == ':' || r == '-' || r == '.' || r == '\'' || r == '"':
			// drop punctuation that varies across storefronts
		case unicode.IsSpace(r):
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			prevSpace = false
		}
	}

	return strings.TrimSpace(b.String())
}
