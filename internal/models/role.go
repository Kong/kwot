package models

// Role represents a Kong RBAC role
type Role struct {
	Name    string `json:"name"`
	Comment string `json:"comment,omitempty"`
}

// RoleResponse represents the API response for a role
type RoleResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// PermissionList is a wrapper for loading permissions from file
type PermissionList struct {
	Permissions []Permission `yaml:"permissions"`
}
