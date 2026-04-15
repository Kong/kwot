package config

import (
	"os"
	"testing"
)

// TestHTTPRequestTimeoutParsing verifies that KONG_REQUEST_TIMEOUT is read from the
// environment and that the default/fallback behaviour is correct.
func TestHTTPRequestTimeoutParsing(t *testing.T) {
	tests := []struct {
		name     string
		envValue string // empty means unset
		want     int
	}{
		{
			name:     "explicit valid value",
			envValue: "60",
			want:     60,
		},
		{
			name:     "unset uses default 30",
			envValue: "",
			want:     30,
		},
		{
			name:     "zero falls back to default 30",
			envValue: "0",
			want:     30,
		},
		{
			name:     "negative falls back to default 30",
			envValue: "-5",
			want:     30,
		},
		{
			name:     "non-numeric falls back to default 30",
			envValue: "notanumber",
			want:     30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				t.Setenv("KONG_REQUEST_TIMEOUT", tt.envValue)
			} else {
				if err := os.Unsetenv("KONG_REQUEST_TIMEOUT"); err != nil {
					t.Fatalf("os.Unsetenv failed: %v", err)
				}
			}
			// Provide the minimum required env vars so LoadConfig doesn't error
			t.Setenv("AUTH_METHOD", "RBAC")
			t.Setenv("ADMIN_TOKEN", "test-token")

			if err := LoadConfig(""); err != nil {
				t.Fatalf("LoadConfig() unexpected error: %v", err)
			}
			cfg := GetConfig()
			if cfg.HTTPRequestTimeout != tt.want {
				t.Errorf("HTTPRequestTimeout = %d, want %d (env=%q)", cfg.HTTPRequestTimeout, tt.want, tt.envValue)
			}
		})
	}
}

// TestMaxRetryAttemptsDefaultFallback confirms the existing guard for MaxRetryAttempts
// still works alongside the new HTTPRequestTimeout guard.
func TestMaxRetryAttemptsDefaultFallback(t *testing.T) {
	t.Setenv("MAX_RETRY_ATTEMPTS", "0")
	t.Setenv("KONG_REQUEST_TIMEOUT", "0")
	t.Setenv("AUTH_METHOD", "RBAC")
	t.Setenv("ADMIN_TOKEN", "test-token")

	if err := LoadConfig(""); err != nil {
		t.Fatalf("LoadConfig() unexpected error: %v", err)
	}
	cfg := GetConfig()
	if cfg.MaxRetryAttempts != 5 {
		t.Errorf("MaxRetryAttempts = %d, want 5 (fallback)", cfg.MaxRetryAttempts)
	}
	if cfg.HTTPRequestTimeout != 30 {
		t.Errorf("HTTPRequestTimeout = %d, want 30 (fallback)", cfg.HTTPRequestTimeout)
	}
}
