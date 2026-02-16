package naming

import (
	"strings"
	"unicode"
)

// commonInitialisms maps lowercase words to their Go-idiomatic CamelCase form.
var commonInitialisms = map[string]string{
	"id": "ID", "url": "URL", "api": "API", "http": "HTTP",
	"json": "JSON", "xml": "XML", "sql": "SQL", "html": "HTML",
	"ip": "IP", "tcp": "TCP", "udp": "UDP", "uuid": "UUID",
	"oauth": "OAuth",
}

// SnakeToCamel converts a snake_case string to CamelCase.
// Common initialisms use their Go-idiomatic form:
// "user_id" → "UserID", "oauth_token" → "OAuthToken".
func SnakeToCamel(s string) string {
	parts := strings.Split(s, "_")
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		if rep, ok := commonInitialisms[p]; ok {
			b.WriteString(rep)
		} else {
			b.WriteString(strings.ToUpper(p[:1]) + p[1:])
		}
	}
	return b.String()
}

// CamelToSnake converts a CamelCase string to snake_case.
// Consecutive uppercase letters (acronyms) are kept together:
// "ID" → "id", "UserID" → "user_id", "OAuth" → "oauth".
func CamelToSnake(s string) string {
	// Normalize mixed-case initialisms (e.g. "OAuth" → "Oauth")
	// so the algorithm treats them as single words.
	for _, rep := range commonInitialisms {
		if rep != strings.ToUpper(rep) {
			s = strings.ReplaceAll(s, rep, strings.ToUpper(rep[:1])+strings.ToLower(rep[1:]))
		}
	}

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
				if unicode.IsLower(prev) || unicode.IsDigit(prev) || (unicode.IsUpper(prev) && unicode.IsLower(next)) {
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
