package config

import (
	"fmt"
	"net/url"
)

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field   string
	Message string
}

// Validator provides configuration validation
type Validator struct {
	errors []ValidationError
}

// NewValidator creates a new validator
func NewValidator() *Validator {
	return &Validator{
		errors: make([]ValidationError, 0),
	}
}

// ValidateConfig validates the configuration
func ValidateConfig(cfg *Config) error {
	v := NewValidator()

	if cfg == nil {
		return fmt.Errorf("configuration is nil")
	}

	// Required fields
	if cfg.KongAddr == "" {
		v.addError("kong_addr", "Kong address is required")
	} else {
		// Validate URL format
		if _, err := url.Parse(cfg.KongAddr); err != nil {
			v.addError("kong_addr", fmt.Sprintf("Invalid URL format: %v", err))
		}
	}

	// Authentication method validation
	validAuthMethods := map[string]bool{"COOKIE": true, "RBAC": true, "PASSWORD": true}
	if cfg.AuthMethod != "" && !validAuthMethods[cfg.AuthMethod] {
		v.addError("auth_method", fmt.Sprintf("Invalid auth method: %s (must be COOKIE, RBAC, or PASSWORD)", cfg.AuthMethod))
	}

	// Concurrency validation
	// MaxConcurrentWorkspaces controls how many workspaces are processed in parallel.
	// Limited to 100 as a safety boundary to prevent:
	// - Excessive memory consumption (each goroutine has stack, buffers, etc.)
	// - Too many simultaneous HTTP connections overwhelming the system
	// - Kong Admin API connection exhaustion from too many parallel requests
	// Most deployments benefit from 5-20; use 50-100 only for large clusters with 8+ CPU cores
	if cfg.MaxConcurrentWorkspaces > 0 && cfg.MaxConcurrentWorkspaces < 1 {
		v.addError("max_concurrent_workspaces", "Max concurrent workspaces must be at least 1")
	}
	if cfg.MaxConcurrentWorkspaces > 100 {
		v.addError("max_concurrent_workspaces", "Max concurrent workspaces must not exceed 100 (safety boundary)")
	}

	// Return errors if any
	if len(v.errors) > 0 {
		return v
	}

	return nil
}

// addError adds a validation error
func (v *Validator) addError(field, message string) {
	v.errors = append(v.errors, ValidationError{
		Field:   field,
		Message: message,
	})
}

// Error implements the error interface
func (v *Validator) Error() string {
	if len(v.errors) == 0 {
		return "validation succeeded"
	}

	msg := "Configuration validation failed:\n"
	for _, err := range v.errors {
		msg += fmt.Sprintf("  - %s: %s\n", err.Field, err.Message)
	}
	return msg
}
