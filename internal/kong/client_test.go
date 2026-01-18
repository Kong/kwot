package kong

import (
	"testing"
)

func TestParseKongError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantMsg    string
	}{
		{
			name:       "409 UNIQUE violation",
			statusCode: 409,
			body:       `{"message": "UNIQUE violation detected", "details": {"constraint": "workspaces_name_key"}}`,
			wantMsg:    "Resource already exists",
		},
		{
			name:       "409 duplicate key",
			statusCode: 409,
			body:       `{"message": "duplicate key value violates unique constraint"}`,
			wantMsg:    "Resource already exists",
		},
		{
			name:       "400 bad request - invalid format",
			statusCode: 400,
			body:       `{"message": "Invalid field format: name must be alphanumeric"}`,
			wantMsg:    "Invalid value",
		},
		{
			name:       "400 primary key violation",
			statusCode: 400,
			body:       `{"message": "primary key violation"}`,
			wantMsg:    "Duplicate resource",
		},
		{
			name:       "404 not found",
			statusCode: 404,
			body:       `{"message": "resource not found"}`,
			wantMsg:    "Resource not found",
		},
		{
			name:       "401 unauthorized",
			statusCode: 401,
			body:       `{"message": "invalid credentials"}`,
			wantMsg:    "Authentication failed",
		},
		{
			name:       "403 forbidden",
			statusCode: 403,
			body:       `{"message": "insufficient permissions"}`,
			wantMsg:    "Permission denied",
		},
		{
			name:       "plain text error",
			statusCode: 409,
			body:       `Workspace already exists`,
			wantMsg:    "Resource already exists",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseKongError(tt.statusCode, tt.body)
			if !contains(got, tt.wantMsg) {
				t.Errorf("parseKongError(%d, %q) = %q, want to contain %q", tt.statusCode, tt.body, got, tt.wantMsg)
			}
		})
	}
}

// helper function
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
