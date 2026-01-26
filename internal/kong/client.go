package kong

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/Kong/kwot/internal/config"
	"github.com/Kong/kwot/internal/logger"
)

// Client represents a Kong Admin API client
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	AuthHeader map[string]string
	Cookies    []*http.Cookie
	cfg        *config.Config
}

// NewClient creates a new Kong API client
func NewClient(cfg *config.Config) (*Client, error) {
	// Create HTTP client with optimized transport
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: !cfg.SSLVerify,
		},
		MaxIdleConns:        100, // Increase from default 100
		MaxIdleConnsPerHost: 10,  // Connections per host
		MaxConnsPerHost:     10,  // Max concurrent connections per host
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  false,
		DisableKeepAlives:   false,
	}

	// Load CA certificate if provided
	if cfg.CAPath != "" {
		caCert, err := os.ReadFile(cfg.CAPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate: %w", err)
		}
		caCertPool := x509.NewCertPool()
		if ok := caCertPool.AppendCertsFromPEM(caCert); !ok {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}
		transport.TLSClientConfig.RootCAs = caCertPool
	}

	// Set proxy if configured
	if cfg.Proxy != "" {
		proxyURL, err := url.Parse(cfg.Proxy)
		if err != nil {
			return nil, fmt.Errorf("failed to parse proxy URL: %w", err)
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}

	client := &Client{
		BaseURL:    cfg.KongAddr,
		HTTPClient: httpClient,
		AuthHeader: make(map[string]string),
		cfg:        cfg,
	}

	// Setup authentication
	if err := client.setupAuth(); err != nil {
		return nil, err
	}

	return client, nil
}

// setupAuth configures authentication based on the auth method
func (c *Client) setupAuth() error {
	switch c.cfg.AuthMethod {
	case "RBAC":
		logger.Info("RBAC method of authentication selected")
		c.AuthHeader["Kong-Admin-Token"] = c.cfg.AdminToken

	case "PASSWORD":
		logger.Info("PASSWORD method of authentication selected")
		c.AuthHeader["Kong-Admin-User"] = c.cfg.AdminUser
		c.AuthHeader["Authorization"] = "Basic " + c.cfg.Base64UIDPwd

		// Get auth cookie
		logger.Info("Calling auth endpoint...")
		resp, err := c.doRequest("GET", "/auth", nil, nil)
		if err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}
		defer func() { _ = resp.Body.Close() }()

		// Store cookies
		c.Cookies = resp.Cookies()

		// Remove Authorization header, we'll use cookies now
		delete(c.AuthHeader, "Authorization")

	default:
		return fmt.Errorf("invalid AUTH_METHOD: %s (must be RBAC or PASSWORD)", c.cfg.AuthMethod)
	}

	return nil
}

// Ping tests connectivity to the Kong Admin API
func (c *Client) Ping() error {
	resp, err := c.doRequest("GET", "/status", nil, nil)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	var status map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return fmt.Errorf("failed to decode status response: %w", err)
	}

	// Log full status only in debug mode, as compact JSON
	statusJSON, _ := json.Marshal(status)
	logger.Debugf("Kong Status: %s", string(statusJSON))
	return nil
}

// parseKongError translates Kong API error messages to user-friendly messages
func parseKongError(statusCode int, responseBody string) string {
	var errorMsg string

	// Try to parse Kong's error response
	var kongError map[string]interface{}
	if err := json.Unmarshal([]byte(responseBody), &kongError); err == nil {
		if message, ok := kongError["message"].(string); ok {
			errorMsg = message
		}
		if details, ok := kongError["details"].(map[string]interface{}); ok {
			for key, value := range details {
				errorMsg += fmt.Sprintf("; %s: %v", key, value)
			}
		}
	} else {
		errorMsg = responseBody
	}

	// Provide context for common errors
	switch statusCode {
	case 409:
		// Conflict - resource already exists
		if strings.Contains(errorMsg, "UNIQUE") || strings.Contains(errorMsg, "duplicate") || strings.Contains(errorMsg, "already exists") {
			return fmt.Sprintf("Resource already exists: %s", errorMsg)
		}
		return fmt.Sprintf("Conflict: %s", errorMsg)

	case 400:
		// Bad Request - invalid input
		if strings.Contains(errorMsg, "primary key") || strings.Contains(errorMsg, "duplicate") {
			return fmt.Sprintf("Duplicate resource: %s", errorMsg)
		}
		if strings.Contains(errorMsg, "invalid") || strings.Contains(errorMsg, "Invalid") {
			return fmt.Sprintf("Invalid value: %s", errorMsg)
		}
		return fmt.Sprintf("Bad request: %s", errorMsg)

	case 404:
		return fmt.Sprintf("Resource not found: %s", errorMsg)

	case 401:
		return fmt.Sprintf("Authentication failed: %s", errorMsg)

	case 403:
		return fmt.Sprintf("Permission denied: %s", errorMsg)

	default:
		return fmt.Sprintf("HTTP %d error: %s", statusCode, errorMsg)
	}
}

// doRequest performs an HTTP request with authentication
func (c *Client) doRequest(method, path string, body interface{}, queryParams url.Values) (*http.Response, error) {
	// Build URL
	reqURL := c.BaseURL + path
	if queryParams != nil {
		reqURL += "?" + queryParams.Encode()
	}

	// Prepare request body
	var bodyReader io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewBuffer(jsonData)
	}

	// Create request
	req, err := http.NewRequest(method, reqURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	// Only set Content-Type if there's a body
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range c.AuthHeader {
		req.Header.Set(key, value)
	}

	// Set cookies
	for _, cookie := range c.Cookies {
		req.AddCookie(cookie)
	}

	// Execute request
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	// Check for errors
	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		errorMessage := parseKongError(resp.StatusCode, string(bodyBytes))
		return nil, fmt.Errorf("%s", errorMessage)
	}

	return resp, nil
}

// GET performs a GET request
func (c *Client) GET(path string, queryParams url.Values) (*http.Response, error) {
	return c.doRequest("GET", path, nil, queryParams)
}

// POST performs a POST request
func (c *Client) POST(path string, body interface{}) (*http.Response, error) {
	return c.doRequest("POST", path, body, nil)
}

// PATCH performs a PATCH request
func (c *Client) PATCH(path string, body interface{}) (*http.Response, error) {
	return c.doRequest("PATCH", path, body, nil)
}

// DELETE performs a DELETE request
func (c *Client) DELETE(path string) (*http.Response, error) {
	return c.doRequest("DELETE", path, nil, nil)
}

// DELETEWithParams performs a DELETE request with query parameters
func (c *Client) DELETEWithParams(path string, queryParams url.Values) (*http.Response, error) {
	return c.doRequest("DELETE", path, nil, queryParams)
}

// GetJSON performs a GET request and decodes the JSON response
func (c *Client) GetJSON(path string, queryParams url.Values, result interface{}) error {
	resp, err := c.GET(path, queryParams)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	return json.NewDecoder(resp.Body).Decode(result)
}

// PostJSON performs a POST request and decodes the JSON response
func (c *Client) PostJSON(path string, body interface{}, result interface{}) error {
	resp, err := c.POST(path, body)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if result != nil {
		return json.NewDecoder(resp.Body).Decode(result)
	}
	return nil
}

// PatchJSON performs a PATCH request and decodes the JSON response
func (c *Client) PatchJSON(path string, body interface{}, result interface{}) error {
	resp, err := c.PATCH(path, body)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if result != nil {
		return json.NewDecoder(resp.Body).Decode(result)
	}
	return nil
}
