package naming_test

import (
	"testing"

	"github.com/mickamy/ormgen/internal/naming"
)

func TestSnakeToCamel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"id", "ID"},
		{"name", "Name"},
		{"created_at", "CreatedAt"},
		{"user_id", "UserID"},
		{"http_server", "HTTPServer"},
		{"oauth_token", "OAuthToken"},
		{"user_oauth_accounts", "UserOAuthAccounts"},
		{"qr_image_id", "QRImageID"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			got := naming.SnakeToCamel(tt.input)
			if got != tt.want {
				t.Errorf("SnakeToCamel(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

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
		{"OAuth", "oauth"},
		{"UserOAuthAccount", "user_oauth_account"},
		{"userProfile", "user_profile"},
		{"S3Object", "s3_object"},
		{"QRImageID", "qr_image_id"},
		{"EC2Instance", "ec2_instance"},
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
