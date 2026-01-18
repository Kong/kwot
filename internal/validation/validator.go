package validation

import (
	"fmt"
	"strings"

	"github.com/Kong/kwot/internal/models"
)

// SystemWorkspaceNames are Kong's built-in workspaces that cannot be deleted
var SystemWorkspaceNames = map[string]bool{
	"default": true,
}

// ValidateWorkspaceName validates a workspace name
// Checks: empty (Kong will reject reserved names like "admin", "api")
func ValidateWorkspaceName(name string) error {
	if name == "" {
		return fmt.Errorf("workspace name cannot be empty")
	}

	return nil
}

// ValidateWorkspaceNameForDeletion validates a workspace name for deletion
// Ensures system workspaces (like "default") cannot be deleted
func ValidateWorkspaceNameForDeletion(name string) error {
	if name == "" {
		return fmt.Errorf("workspace name cannot be empty")
	}

	lower := strings.ToLower(name)
	if SystemWorkspaceNames[lower] {
		return fmt.Errorf("cannot delete workspace '%s': it is a system workspace and cannot be removed", name)
	}

	return nil
}

// ValidateRoleName validates a role name (basic check)
func ValidateRoleName(name string) error {
	if name == "" {
		return fmt.Errorf("role name cannot be empty")
	}
	return nil
}

// ValidateGroupName validates a group name (basic check)
func ValidateGroupName(name string) error {
	if name == "" {
		return fmt.Errorf("group name cannot be empty")
	}
	return nil
}

// ValidateUserName validates a user name (basic check)
func ValidateUserName(name string) error {
	if name == "" {
		return fmt.Errorf("user name cannot be empty")
	}
	return nil
}

// ValidateRBACUserConfig validates RBAC user configuration
func ValidateRBACUserConfig(user models.RBACUser) error {
	if err := ValidateUserName(user.Name); err != nil {
		return fmt.Errorf("invalid RBAC user configuration: %w", err)
	}

	if len(user.Roles) == 0 {
		return fmt.Errorf("RBAC user '%s' has no roles assigned", user.Name)
	}

	return nil
}

// ValidateGroupConfig validates a group configuration
func ValidateGroupConfig(group models.GroupDetail) error {
	if err := ValidateGroupName(group.GroupName); err != nil {
		return fmt.Errorf("invalid group configuration: %w", err)
	}

	if len(group.Roles) == 0 {
		return fmt.Errorf("group '%s' has no roles assigned", group.GroupName)
	}

	return nil
}

// ValidateRoleConfig validates a role configuration
func ValidateRoleConfig(role models.RoleDetail) error {
	if err := ValidateRoleName(role.Role); err != nil {
		return fmt.Errorf("invalid role configuration: %w", err)
	}

	// Permissions can be either an array or a file path (string), so we just check that it's not nil
	if role.Permissions == nil {
		return fmt.Errorf("role '%s' has no permissions defined", role.Role)
	}

	return nil
}

// ValidateAllRoles validates all roles in a workspace config
func ValidateAllRoles(roles []models.RoleDetail, workspaceName string) error {
	for _, role := range roles {
		if err := ValidateRoleConfig(role); err != nil {
			return fmt.Errorf("validation error in workspace '%s': %w", workspaceName, err)
		}
	}

	return nil
}
