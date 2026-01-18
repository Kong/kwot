package config

import (
	"strings"
	"testing"
)

// TestValidateConfigRequiredFields tests required field validation
func TestValidateConfigRequiredFields(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name:    "Nil config",
			cfg:     nil,
			wantErr: true,
		},
		{
			name:    "Empty Kong address",
			cfg:     &Config{KongAddr: ""},
			wantErr: true,
		},
		{
			name:    "Valid address",
			cfg:     &Config{KongAddr: "http://localhost:8001"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfig(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestValidateAuthMethod tests authentication method validation
func TestValidateAuthMethod(t *testing.T) {
	tests := []struct {
		name       string
		authMethod string
		wantErr    bool
	}{
		{"Cookie auth", "COOKIE", false},
		{"RBAC auth", "RBAC", false},
		{"Password auth", "PASSWORD", false},
		{"Invalid", "invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				KongAddr:   "http://localhost:8001",
				AuthMethod: tt.authMethod,
			}
			err := ValidateConfig(cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestValidationErrorMessage tests error message formatting
func TestValidationErrorMessage(t *testing.T) {
	cfg := &Config{
		KongAddr: "",
	}

	err := ValidateConfig(cfg)
	if err == nil {
		t.Error("Expected validation error")
	}

	errMsg := err.Error()
	if errMsg == "" {
		t.Error("Expected error message")
	}

	// Check that error mentions specific fields
	if !strings.Contains(errMsg, "kong_addr") {
		t.Errorf("Error message should mention invalid fields: %s", errMsg)
	}
}
