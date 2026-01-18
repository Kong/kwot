package models

// GroupConfigWrapper supports structured format with anchors:
// role_info: {...anchors...}
// config: [- group_name: ..., ...]
type GroupConfigWrapper struct {
	RoleInfo map[string]interface{} `yaml:"role_info"` // For anchors
	Config   []GroupDetail          `yaml:"config"`    // Groups list
}

// GroupConfig represents the groups configuration from YAML
type GroupConfig struct {
	Config []GroupDetail `yaml:"-"`
}

// GroupDetail represents a group and its roles
type GroupDetail struct {
	GroupName    string      `yaml:"group_name"`
	GroupComment string      `yaml:"group_comment"`
	Roles        []GroupRole `yaml:"roles"`
}

// GroupRole represents a role assignment in a group
type GroupRole struct {
	Workspace string `yaml:"workspace"`
	Role      string `yaml:"role"`
}

// Group represents a Kong group
type Group struct {
	Name    string `json:"name"`
	Comment string `json:"comment,omitempty"`
}

// GroupResponse represents the API response for a group
type GroupResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Comment   string `json:"comment,omitempty"`
	CreatedAt int64  `json:"created_at,omitempty"`
}

// GroupRoleAssignment represents assigning a role to a group
type GroupRoleAssignment struct {
	WorkspaceID string `json:"workspace_id"`
	RBACRoleID  string `json:"rbac_role_id"`
}
