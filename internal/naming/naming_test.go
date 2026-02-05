package naming_test

import (
	"testing"

	"github.com/mickamy/ormgen/internal/naming"
)

func TestCamelToSnake(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"ID", "id"},
		{"Name", "name"},
		{"CreatedAt", "created_at"},
		{"UserID", "user_id"},
		{"HTTPServer", "http_server"},
		{"userProfile", "user_profile"},
		{"A", "a"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			got := naming.CamelToSnake(tt.input)
			if got != tt.want {
				t.Errorf("CamelToSnake(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
