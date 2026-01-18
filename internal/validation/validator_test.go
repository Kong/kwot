package validation

import (
	"testing"

	"github.com/Kong/kwot/internal/models"
)

// TestValidateWorkspaceName tests workspace name validation
func TestValidateWorkspaceName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid workspace name",
			input:   "demo1",
			wantErr: false,
		},
		{
			name:    "valid with hyphens",
			input:   "my-workspace",
			wantErr: false,
		},
		{
			name:    "valid with special chars (Kong will validate)",
			input:   "my@workspace",
			wantErr: false,
		},
		{
			name:    "empty workspace name",
			input:   "",
			wantErr: true,
		},
		{
			name:    "admin workspace (Kong will reject)",
			input:   "admin",
			wantErr: false,
		},
		{
			name:    "api workspace (Kong will reject)",
			input:   "api",
			wantErr: false,
		},
		{
			name:    "default workspace",
			input:   "default",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateWorkspaceName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateWorkspaceName() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestValidateWorkspaceNameForDeletion tests workspace deletion validation
func TestValidateWorkspaceNameForDeletion(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "can delete custom workspace",
			input:   "demo1",
			wantErr: false,
		},
		{
			name:    "cannot delete default workspace",
			input:   "default",
			wantErr: true,
		},
		{
			name:    "cannot delete default (uppercase)",
			input:   "DEFAULT",
			wantErr: true,
		},
		{
			name:    "empty workspace name",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateWorkspaceNameForDeletion(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateWorkspaceNameForDeletion() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestValidateRoleName tests role name validation
func TestValidateRoleName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid role",
			input:   "admin",
			wantErr: false,
		},
		{
			name:    "empty role",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRoleName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRoleName() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestValidateGroupConfig tests group configuration validation
func TestValidateGroupConfig(t *testing.T) {
	tests := []struct {
		name    string
		input   models.GroupDetail
		wantErr bool
	}{
		{
			name: "valid group config",
			input: models.GroupDetail{
				GroupName: "admins",
				Roles: []models.GroupRole{
					{Workspace: "demo1", Role: "admin"},
				},
			},
			wantErr: false,
		},
		{
			name: "no roles",
			input: models.GroupDetail{
				GroupName: "admins",
				Roles:     []models.GroupRole{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateGroupConfig(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateGroupConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
