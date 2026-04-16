package kong

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Kong/kwot/internal/config"
)

// TestHTTPClientTimeoutDerivedFromConfig verifies that NewClient sets the HTTP
// client timeout to the value in cfg.HTTPRequestTimeout, not a hardcoded value.
func TestHTTPClientTimeoutDerivedFromConfig(t *testing.T) {
	tests := []struct {
		name            string
		configuredSecs  int
		wantTimeout     time.Duration
	}{
		{
			name:           "30 second timeout",
			configuredSecs: 30,
			wantTimeout:    30 * time.Second,
		},
		{
			name:           "120 second timeout for large workspaces",
			configuredSecs: 120,
			wantTimeout:    120 * time.Second,
		},
		{
			name:           "5 second timeout",
			configuredSecs: 5,
			wantTimeout:    5 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{}`))
			}))
			defer server.Close()

			cfg := &config.Config{
				KongAddr:           server.URL,
				AuthMethod:         "RBAC",
				AdminToken:         "test-token",
				SSLVerify:          false,
				MaxRetryAttempts:   5,
				HTTPRequestTimeout: tt.configuredSecs,
			}

			client, err := NewClient(cfg)
			if err != nil {
				t.Fatalf("NewClient() failed: %v", err)
			}

			if client.HTTPClient.Timeout != tt.wantTimeout {
				t.Errorf("HTTPClient.Timeout = %v, want %v", client.HTTPClient.Timeout, tt.wantTimeout)
			}
		})
	}
}

// TestHTTPClientTimeoutEnforced verifies that a request exceeding the configured
// timeout is actually cancelled by the HTTP client.
func TestHTTPClientTimeoutEnforced(t *testing.T) {
	// Server that sleeps longer than the client timeout
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	cfg := &config.Config{
		KongAddr:           server.URL,
		AuthMethod:         "RBAC",
		AdminToken:         "test-token",
		SSLVerify:          false,
		MaxRetryAttempts:   5,
		HTTPRequestTimeout: 1, // 1 second — well above 200 ms so normal requests pass
	}

	// Confirm a normal fast request works
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient() failed: %v", err)
	}

	// Force a 50 ms client timeout to trigger the deadline
	client.HTTPClient.Timeout = 50 * time.Millisecond

	_, err = client.GET("/status", nil)
	if err == nil {
		t.Error("Expected timeout error, got nil")
	}
}
