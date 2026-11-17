package catalog

import (
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// SearchName lowercases a name and strips accents and punctuation.
func SearchName(name string) string {
	var b strings.Builder
	pendingSpace := false
	for _, r := range norm.NFD.String(name) {
		switch {
		case unicode.Is(unicode.Mn, r):

		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if pendingSpace && b.Len() > 0 {
				b.WriteByte(' ')
			}
			pendingSpace = false
			b.WriteRune(unicode.ToLower(r))
		default:
			pendingSpace = true
		}
	}
	return b.String()
}

// LocalNumber parses a printed number, false when not numeric.
func LocalNumber(localID string) (int, bool) {
	if localID == "" {
		return 0, false
	}
	for _, r := range localID {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	n, err := strconv.Atoi(localID)
	if err != nil {
		return 0, false
	}
	return n, true
}
