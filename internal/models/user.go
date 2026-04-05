package models

// RBACUser represents a workspace-scoped RBAC user
type RBACUser struct {
	Name      string   `yaml:"name" json:"name"`
	UserToken string   `yaml:"user_token,omitempty" json:"user_token,omitempty"`
	Roles     []string `yaml:"roles" json:"roles"`
}

// RBACUserResponse represents the API response for RBAC users
// NOTE: We intentionally don't unmarshal UserToken from responses to avoid logging secrets
type RBACUserResponse struct {
	Name string `json:"name"`
	ID   string `json:"id"`
	// UserToken is deliberately omitted to prevent accidental logging of secrets
}

// RoleAssignment represents assigning a role to a user
type RoleAssignment struct {
	Roles string `json:"roles"`
}
