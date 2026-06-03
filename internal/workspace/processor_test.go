package workspace

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Kong/kwot/internal/config"
	"github.com/Kong/kwot/internal/kong"
	"github.com/Kong/kwot/internal/models"
)

// newTestProcessor creates a Processor backed by a mock HTTP server for unit tests.
func newTestProcessor(t *testing.T, handler http.HandlerFunc, timeoutSecs int, maxRetry int) (*Processor, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
	cfg := &config.Config{
		KongAddr:           server.URL,
		AuthMethod:         "RBAC",
		AdminToken:         "test-token",
		SSLVerify:          false,
		MaxRetryAttempts:   maxRetry,
		HTTPRequestTimeout: timeoutSecs,
	}
	client, err := kong.NewClient(cfg)
	if err != nil {
		server.Close()
		t.Fatalf("NewClient() failed: %v", err)
	}
	return NewProcessor(client, cfg, false), server
}

func pluginFixture(name string) models.Plugin {
	return models.Plugin{Name: name, Config: map[string]interface{}{}}
}

// ── DeleteWorkspace — timeout vs. non-timeout error messages ─────────────────

// workspaceListHandler returns an http.HandlerFunc that serves getWorkspaceID's
// paginated GET /workspaces call, returning wsID for wsName, then delegates
// DELETE requests to the provided deleteFn.
func workspaceListHandler(wsName, wsID string, deleteFn func(w http.ResponseWriter, r *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/workspaces" {
			// Respond to getWorkspaceID pagination call
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":[{"id":"` + wsID + `","name":"` + wsName + `"}],"offset":""}`))
			return
		}
		if r.Method == http.MethodDelete {
			// Assert the expected path and cascade=true param are present
			expectedPath := "/workspaces/" + wsID
			if r.URL.Path != expectedPath {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"message":"unexpected delete path: ` + r.URL.Path + `"}`))
				return
			}
			if r.URL.Query().Get("cascade") != "true" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"message":"missing cascade=true"}`))
				return
			}
			deleteFn(w, r)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}
}

// TestDeleteWorkspace_TimeoutErrorMessage verifies that when the cascade DELETE
// times out the error tells the operator to raise KONG_REQUEST_TIMEOUT and does
// NOT produce the misleading "check Kong version" hint.
func TestDeleteWorkspace_TimeoutErrorMessage(t *testing.T) {
	handler := workspaceListHandler("test-ws", "abc-123", func(w http.ResponseWriter, r *http.Request) {
		// Simulate Kong still processing — sleep beyond client timeout
		time.Sleep(300 * time.Millisecond)
		w.WriteHeader(http.StatusNoContent)
	})

	proc, server := newTestProcessor(t, handler, 30, 5)
	defer server.Close()

	// Override to a tiny timeout to trigger context.DeadlineExceeded
	proc.client.HTTPClient.Timeout = 50 * time.Millisecond

	err := proc.DeleteWorkspace("test-ws")
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "KONG_REQUEST_TIMEOUT") {
		t.Errorf("timeout error should mention KONG_REQUEST_TIMEOUT, got: %s", errMsg)
	}
	if strings.Contains(errMsg, "cascade=true requires Kong Gateway") {
		t.Errorf("timeout error must not include Kong version hint, got: %s", errMsg)
	}
}

// TestDeleteWorkspace_NonTimeoutErrorMessage verifies that a non-timeout error
// (e.g. 404) still returns the cascade / Kong-version hint.
func TestDeleteWorkspace_NonTimeoutErrorMessage(t *testing.T) {
	handler := workspaceListHandler("test-ws", "abc-123", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"workspace not found"}`))
	})

	proc, server := newTestProcessor(t, handler, 30, 5)
	defer server.Close()

	err := proc.DeleteWorkspace("test-ws")
	if err == nil {
		t.Fatal("expected error for 404 delete, got nil")
	}
	if !strings.Contains(err.Error(), "cascade=true requires Kong Gateway") {
		t.Errorf("non-timeout error should contain cascade hint, got: %s", err.Error())
	}
}

// ── applyPlugin — retry logic ─────────────────────────────────────────────────

// TestApplyPlugin_SuccessOnFirstAttempt verifies the happy path.
func TestApplyPlugin_SuccessOnFirstAttempt(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/test-ws/plugins" {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"plug-1","name":"key-auth"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"unexpected request"}`))
	})

	proc, server := newTestProcessor(t, handler, 30, 5)
	defer server.Close()

	if err := proc.applyPlugin("test-ws", pluginFixture("key-auth")); err != nil {
		t.Errorf("expected success, got: %v", err)
	}
}

// TestApplyPlugin_IdempotentOn409 verifies that a 409 conflict is not an error.
func TestApplyPlugin_IdempotentOn409(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/test-ws/plugins" {
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"message":"UNIQUE violation"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"unexpected request"}`))
	})

	proc, server := newTestProcessor(t, handler, 30, 5)
	defer server.Close()

	if err := proc.applyPlugin("test-ws", pluginFixture("key-auth")); err != nil {
		t.Errorf("409 should be treated as success (idempotent), got: %v", err)
	}
}

// TestApplyPlugin_RetriesOnTransientWorkspaceNotFound verifies that a 404
// "Workspace ... not found" triggers retries and succeeds once the node is ready.
func TestApplyPlugin_RetriesOnTransientWorkspaceNotFound(t *testing.T) {
	var callCount int32

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			n := atomic.AddInt32(&callCount, 1)
			if n < 3 {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"message":"Workspace 'test-ws' not found"}`))
				return
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"plug-1","name":"key-auth"}`))
		}
	})

	proc, server := newTestProcessor(t, handler, 30, 5)
	defer server.Close()

	if err := proc.applyPlugin("test-ws", pluginFixture("key-auth")); err != nil {
		t.Errorf("expected success after retries, got: %v", err)
	}
	if callCount < 3 {
		t.Errorf("expected at least 3 POST attempts, got %d", callCount)
	}
}

// TestApplyPlugin_ExhaustsRetriesOnPersistentWorkspaceNotFound verifies that
// after MaxRetryAttempts the function returns a descriptive error.
func TestApplyPlugin_ExhaustsRetriesOnPersistentWorkspaceNotFound(t *testing.T) {
	var callCount int32

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			atomic.AddInt32(&callCount, 1)
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"Workspace 'test-ws' not found"}`))
		}
	})

	proc, server := newTestProcessor(t, handler, 30, 3)
	defer server.Close()

	err := proc.applyPlugin("test-ws", pluginFixture("key-auth"))
	if err == nil {
		t.Fatal("expected error after exhausting retries, got nil")
	}
	if !strings.Contains(err.Error(), "3 attempts") {
		t.Errorf("error should mention attempt count, got: %v", err)
	}
	if int(callCount) != 3 {
		t.Errorf("expected exactly 3 POST calls (MaxRetryAttempts), got %d", callCount)
	}
}

// TestApplyPlugin_PermanentErrorNotRetried verifies that a non-workspace error
// (e.g. 400 schema violation) fails immediately without any retry.
func TestApplyPlugin_PermanentErrorNotRetried(t *testing.T) {
	var callCount int32

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			atomic.AddInt32(&callCount, 1)
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"message":"schema violation: unknown field"}`))
		}
	})

	proc, server := newTestProcessor(t, handler, 30, 5)
	defer server.Close()

	err := proc.applyPlugin("test-ws", pluginFixture("key-auth"))
	if err == nil {
		t.Error("expected error for permanent 400, got nil")
	}
	if callCount != 1 {
		t.Errorf("permanent error should not retry: expected 1 call, got %d", callCount)
	}
}

// TestApplyPlugin_NonWorkspace404NotRetried verifies that a 404 whose message
// does NOT mention "workspace" is treated as permanent and not retried.
func TestApplyPlugin_NonWorkspace404NotRetried(t *testing.T) {
	var callCount int32

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			atomic.AddInt32(&callCount, 1)
			w.WriteHeader(http.StatusNotFound)
			// "endpoint not found" — no "workspace" keyword
			_, _ = w.Write([]byte(`{"message":"endpoint not found"}`))
		}
	})

	proc, server := newTestProcessor(t, handler, 30, 5)
	defer server.Close()

	err := proc.applyPlugin("test-ws", pluginFixture("key-auth"))
	if err == nil {
		t.Error("expected error for permanent 404, got nil")
	}
	if callCount != 1 {
		t.Errorf("non-workspace 404 must not retry: expected 1 call, got %d", callCount)
	}
}

func TestResolveEnvVar(t *testing.T) {
	t.Setenv("MY_TOKEN", "secret123")

	tests := []struct {
		input    string
		expected string
	}{
		{"${MY_TOKEN}", "secret123"},
		{"$MY_TOKEN", "secret123"},
		{"literal-token", "literal-token"},
		{"", ""},
		{"${UNSET_VAR}", ""},
		{"$UNSET_VAR", ""},
	}

	for _, tc := range tests {
		got := resolveEnvVar("test-user", tc.input)
		if got != tc.expected {
			t.Errorf("resolveEnvVar(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}

	// Ensure a dynamically set env var is resolved (t.Setenv handles cleanup)
	t.Setenv("DYNAMIC_TOKEN", "dynval")
	if got := resolveEnvVar("u", "${DYNAMIC_TOKEN}"); got != "dynval" {
		t.Errorf("expected dynval, got %q", got)
	}
}
