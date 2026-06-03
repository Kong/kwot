package models

// WorkspaceConfig represents the workspace configuration from YAML
type WorkspaceConfig struct {
	Config  WorkspaceConfigDetail `yaml:"config"`
	RBAC    []RoleDetail          `yaml:"rbac"`
	Plugins []Plugin              `yaml:"plugins,omitempty"`
}

// WorkspaceConfigDetail contains workspace settings
type WorkspaceConfigDetail struct {
	Portal bool `yaml:"portal" json:"portal"`
}

// RoleDetail represents a role and its permissions
type RoleDetail struct {
	Role        string      `yaml:"role"`
	Comment     string      `yaml:"comment,omitempty"`
	Permissions interface{} `yaml:"permissions"` // Can be array or file path
}

// Permission represents an RBAC permission
type Permission struct {
	Endpoint string `yaml:"endpoint" json:"endpoint"`
	Actions  string `yaml:"actions" json:"actions"`
	Negative bool   `yaml:"negative" json:"negative"`
}

// Plugin represents a Kong plugin configuration
type Plugin struct {
	Name   string                 `yaml:"name" json:"name"`
	Config map[string]interface{} `yaml:"config" json:"config"`
}

// Workspace represents a Kong workspace
type Workspace struct {
	Name   string                `json:"name"`
	Config WorkspaceConfigDetail `json:"config"`
}

// WorkspaceResponse represents the API response for workspace
type WorkspaceResponse struct {
	ID          string                `json:"id"`
	Name        string                `json:"name"`
	Description string                `json:"description,omitempty"`
	Config      WorkspaceConfigDetail `json:"config"`
	CreatedAt   int64                 `json:"created_at,omitempty"`
}

// WorkspaceMeta represents workspace metadata
type WorkspaceMeta struct {
	Counts map[string]int `json:"counts"`
}
