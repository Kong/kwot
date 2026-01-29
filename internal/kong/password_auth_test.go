package kong

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Kong/kwot/internal/config"
)

// TestPasswordAuthRedirectHandling tests that PASSWORD auth properly handles
// Kong's 302 redirect response from the /auth endpoint
func TestPasswordAuthRedirectHandling(t *testing.T) {
	// Create a mock Kong server that simulates PASSWORD auth flow
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify correct headers are sent
		if r.URL.Path == "/auth" {
			// Kong's /auth endpoint returns 302 with session cookie
			w.Header().Set("Location", "http://localhost:8002")
			http.SetCookie(w, &http.Cookie{
				Name:     "session",
				Value:    "test-session-cookie-value",
				Path:     "/",
				HttpOnly: true,
			})
			w.WriteHeader(http.StatusFound)
			return
		}

		// For all other requests, check if session cookie is present
		_, err := r.Cookie("session")
		if err == http.ErrNoCookie {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"message": "Unauthorized"}`))
			return
		}

		// If we have the cookie, respond with success
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"authenticated": true}`))
	}))
	defer server.Close()

	// Create config with PASSWORD auth
	cfg := &config.Config{
		KongAddr:         server.URL,
		AuthMethod:       "PASSWORD",
		AdminUser:        "test_admin",
		Base64UIDPwd:     "dGVzdDp0ZXN0", // base64(test:test)
		SSLVerify:        false,
		MaxRetryAttempts: 5,
	}

	// Create client - this will call setupAuth() which makes the /auth call
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient() failed: %v", err)
	}

	// Verify cookies were captured from 302 response
	if len(client.Cookies) == 0 {
		t.Error("Expected cookies to be captured from /auth response, got none")
	}

	// Verify the session cookie was captured
	sessionCookieFound := false
	for _, cookie := range client.Cookies {
		if cookie.Name == "session" && cookie.Value == "test-session-cookie-value" {
			sessionCookieFound = true
			break
		}
	}
	if !sessionCookieFound {
		t.Error("Expected session cookie to be captured from /auth response")
	}
}

// TestRBACAuthTokenHeader tests that RBAC auth uses Kong-Admin-Token header
func TestRBACAuthTokenHeader(t *testing.T) {
	// Create a mock Kong server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for Kong-Admin-Token header
		token := r.Header.Get("Kong-Admin-Token")
		if token == "" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"message": "Missing Kong-Admin-Token header"}`))
			return
		}

		if token != "test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"message": "Invalid token"}`))
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"authenticated": true}`))
	}))
	defer server.Close()

	// Create config with RBAC auth
	cfg := &config.Config{
		KongAddr:         server.URL,
		AuthMethod:       "RBAC",
		AdminToken:       "test-token",
		SSLVerify:        false,
		MaxRetryAttempts: 5,
	}

	// Create client
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient() failed: %v", err)
	}

	// Verify the token is in the auth header
	if client.AuthHeader["Kong-Admin-Token"] != "test-token" {
		t.Errorf("Expected Kong-Admin-Token header to be set, got %v", client.AuthHeader)
	}
}

// TestPasswordAuthHeaders tests that PASSWORD auth sets correct headers
func TestPasswordAuthHeaders(t *testing.T) {
	// Create a mock Kong server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth" {
			// Verify correct headers
			if r.Header.Get("Kong-Admin-User") == "" {
				t.Error("Expected Kong-Admin-User header to be set")
			}
			if r.Header.Get("Authorization") == "" {
				t.Error("Expected Authorization header (Basic auth) to be set")
			}

			http.SetCookie(w, &http.Cookie{
				Name:  "session",
				Value: "test-session",
				Path:  "/",
			})
			w.WriteHeader(http.StatusFound)
			w.Header().Set("Location", "http://localhost:8002")
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create config
	cfg := &config.Config{
		KongAddr:         server.URL,
		AuthMethod:       "PASSWORD",
		AdminUser:        "kong_admin",
		Base64UIDPwd:     "dGVzdDp0ZXN0",
		SSLVerify:        false,
		MaxRetryAttempts: 5,
	}

	// Create client
	_, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient() failed: %v", err)
	}

	// If we got here without error, auth was successful
	// The test server will have verified the headers were sent
}

// TestInvalidAuthMethod tests that invalid auth methods are rejected
func TestInvalidAuthMethod(t *testing.T) {
	cfg := &config.Config{
		KongAddr:   "http://localhost:8001",
		AuthMethod: "INVALID",
	}

	_, err := NewClient(cfg)
	if err == nil {
		t.Error("Expected error for invalid AUTH_METHOD, got none")
	}
}
