package naming

import (
	"strings"
	"unicode"
)

// commonAcronyms are words that should be fully uppercased in CamelCase.
var commonAcronyms = map[string]bool{
	"id": true, "url": true, "api": true, "http": true,
	"json": true, "xml": true, "sql": true, "html": true,
	"ip": true, "tcp": true, "udp": true, "uuid": true,
}

// SnakeToCamel converts a snake_case string to CamelCase.
// Common acronyms are fully uppercased:
// "user_id" → "UserID", "created_at" → "CreatedAt", "id" → "ID".
func SnakeToCamel(s string) string {
	parts := strings.Split(s, "_")
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		if commonAcronyms[p] {
			b.WriteString(strings.ToUpper(p))
		} else {
			b.WriteString(strings.ToUpper(p[:1]) + p[1:])
		}
	}
	return b.String()
}

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
