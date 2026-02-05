package naming

import (
	"strings"
	"unicode"
)

// CamelToSnake converts a CamelCase string to snake_case.
// Consecutive uppercase letters (acronyms) are kept together:
// "ID" → "id", "UserID" → "user_id", "CreatedAt" → "created_at".
func CamelToSnake(s string) string {
	runes := []rune(s)
	var b strings.Builder
	for i, r := range runes {
		if unicode.IsUpper(r) {
			if i > 0 {
				prev := runes[i-1]
				next := rune(0)
				if i+1 < len(runes) {
					next = runes[i+1]
				}
				if unicode.IsLower(prev) || (unicode.IsUpper(prev) && unicode.IsLower(next)) {
					b.WriteByte('_')
				}
			}
			b.WriteRune(unicode.ToLower(r))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
