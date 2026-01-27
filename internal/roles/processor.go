package roles

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Kong/kwot/internal/config"
	"github.com/Kong/kwot/internal/kong"
	"github.com/Kong/kwot/internal/logger"
	"github.com/Kong/kwot/internal/models"
	"github.com/Kong/kwot/internal/validation"
	"gopkg.in/yaml.v3"
)

// Processor handles role operations
type Processor struct {
	client *kong.Client
	cfg    *config.Config
	dryRun bool
}

// NewProcessor creates a new role processor
func NewProcessor(client *kong.Client, cfg *config.Config, dryRun bool) *Processor {
	return &Processor{
		client: client,
		cfg:    cfg,
		dryRun: dryRun,
	}
}

// ProcessRoles processes role configurations for workspaces
func (p *Processor) ProcessRoles(selectedWorkspace, specificRole string) error {
	dirs, err := p.GetWorkspaceDirs()
	if err != nil {
		return fmt.Errorf("failed to get workspace directories: %w", err)
	}

	// Filter directories based on selection
	var targetDirs []string
	for _, dir := range dirs {
		if selectedWorkspace != "all" && selectedWorkspace != dir {
			logger.Warnf("Skipping workspace %s", dir)
			continue
		}
		targetDirs = append(targetDirs, dir)
	}

	// Process workspaces in parallel with concurrency limit
	actualConcurrency := p.cfg.MaxConcurrentWorkspaces
	if len(targetDirs) < actualConcurrency {
		actualConcurrency = len(targetDirs)
	}
	logger.Infof("Processing %d workspaces with %d concurrent workers", len(targetDirs), actualConcurrency)

	semaphore := make(chan struct{}, actualConcurrency)
	var wg sync.WaitGroup
	var errs []error
	var mu sync.Mutex

	for _, dir := range targetDirs {
		wg.Add(1)
		go func(workspaceName string) {
			defer wg.Done()

			// Acquire semaphore slot
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Load workspace config to get RBAC roles
			wsConfig, err := p.loadWorkspaceConfig(workspaceName)
			if err != nil {
				logger.Errorf("Failed to load workspace config for %s: %v", workspaceName, err)
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
				return
			}

			if err := p.ApplyRoles(workspaceName, wsConfig.RBAC, false, specificRole); err != nil {
				logger.Errorf("Failed to apply roles for workspace %s: %v", workspaceName, err)
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		}(dir)
	}

	wg.Wait()

	// Return first error if any occurred
	if len(errs) > 0 {
		return errs[0]
	}

	return nil
}

// ApplyRoles applies RBAC roles and permissions to a workspace
func (p *Processor) ApplyRoles(workspaceName string, rbac []models.RoleDetail, isNew bool, specificRole string) error {
	logger.Infof("Applying roles now for workspace %s", workspaceName)

	// Check if workspace exists (prerequisite for role creation)
	// Skip this check in dry-run mode since workspaces may not be created in Kong yet
	if !isNew && !p.dryRun {
		workspaceExists, err := p.workspaceExists(workspaceName)
		if err != nil {
			return fmt.Errorf("failed to check workspace existence: %w", err)
		}

		if !workspaceExists {
			return fmt.Errorf(
				"cannot apply roles: workspace '%s' not found"+
					"\n  Action required: Create workspace '%s' first"+
					"\n  Configuration location: config/%s/workspace.yaml",
				workspaceName,
				workspaceName,
				workspaceName,
			)
		}
	}

	// Validate roles before applying
	if err := validation.ValidateAllRoles(rbac, workspaceName); err != nil {
		return fmt.Errorf("role configuration validation failed: %w", err)
	}

	// Delete existing roles if configured and workspace is not new
	// Skip deletion in dry-run mode for new workspaces
	if !isNew && p.cfg.DeleteExistingRoles && !p.dryRun {
		if workspaceName == "default" {
			logger.Error("Kong strongly recommends not to delete existing roles in 'default' using this tool")
			return fmt.Errorf("cannot delete roles in default workspace")
		}

		if specificRole != "" {
			logger.Warnf("Deleting only the role '%s' from workspace '%s'", specificRole, workspaceName)
			if err := p.deleteRole(workspaceName, specificRole); err != nil {
				logger.Errorf("Failed to delete role '%s': %v", specificRole, err)
			} else {
				logger.Infof("Deleted role '%s' successfully", specificRole)
			}
		} else {
			logger.Warnf("Deleting existing roles from workspace '%s'", workspaceName)
			logger.Infof("To prevent automatic role deletion, set FEATURE_DELETE_EXISTING_ROLES=false in your .env file")

			if err := p.deleteAllRoles(workspaceName); err != nil {
				logger.Errorf("Failed to delete roles: %v", err)
			}
		}
	}

	// Create roles and permissions
	for _, roleDetail := range rbac {
		// If specific role is requested, skip others
		if specificRole != "" && roleDetail.Role != specificRole {
			continue
		}

		if err := p.createRole(workspaceName, roleDetail); err != nil {
			logger.Errorf("Failed to create role %s: %v", roleDetail.Role, err)
			continue
		}

		// Load permissions
		configDir := filepath.Join(p.cfg.ConfigDir, workspaceName)
		permissions, err := p.loadPermissions(roleDetail, configDir)
		if err != nil {
			logger.Errorf("Failed to load permissions for role %s: %v", roleDetail.Role, err)
			continue
		}

		// Apply permissions in parallel (up to 5 concurrent per role)
		if len(permissions) > 0 {
			maxConcurrentPerms := 5
			if len(permissions) < maxConcurrentPerms {
				maxConcurrentPerms = len(permissions)
			}
			permSemaphore := make(chan struct{}, maxConcurrentPerms)
			var permWg sync.WaitGroup

			for _, permission := range permissions {
				permWg.Add(1)
				go func(perm models.Permission) {
					defer permWg.Done()

					// Acquire semaphore slot
					permSemaphore <- struct{}{}
					defer func() { <-permSemaphore }()

					if err := p.addPermission(workspaceName, roleDetail.Role, perm); err != nil {
						logger.Errorf("Failed to add permission to role %s: %v", roleDetail.Role, err)
					} else if !p.dryRun {
						logger.Debugf("Permission {\"endpoint\":\"%s\",\"negative\":%v,\"actions\":\"%s\"} added for role %s in workspace %s",
							perm.Endpoint, perm.Negative, perm.Actions, roleDetail.Role, workspaceName)
					}
				}(permission)
			}

			permWg.Wait()
		}
	}

	// Log summary
	var msg string
	if specificRole != "" {
		msg = fmt.Sprintf("Role '%s' and its permissions successfully applied for workspace %s", specificRole, workspaceName)
	} else {
		msg = fmt.Sprintf("All roles and permissions successfully applied for workspace %s", workspaceName)
	}

	if p.dryRun {
		logger.Infof("[DRY-RUN] %s", msg)
	} else {
		logger.Infof("%s", msg)
	}

	return nil
}

// loadPermissions loads permissions from role detail (can be inline array or file path)
// If configDir is provided, relative file paths are resolved against it
func (p *Processor) loadPermissions(roleDetail models.RoleDetail, configDir string) ([]models.Permission, error) {
	switch v := roleDetail.Permissions.(type) {
	case []interface{}:
		// Permissions are embedded as array
		var permissions []models.Permission
		data, err := yaml.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal permissions: %w", err)
		}
		if err := yaml.Unmarshal(data, &permissions); err != nil {
			return nil, fmt.Errorf("failed to unmarshal permissions: %w", err)
		}
		return permissions, nil

	case string:
		// Permissions are in external file
		permFilePath := v

		// Check if path starts with './' or '../' BEFORE cleaning (filepath.Clean removes these)
		isRelativeToRoot := strings.HasPrefix(permFilePath, "./") || strings.HasPrefix(permFilePath, "../")

		permFilePath = filepath.Clean(permFilePath)

		// If path is relative and doesn't start with './' or '../', resolve it relative to configDir
		// Paths starting with './' or '../' are relative to current working directory (project root)
		if !filepath.IsAbs(permFilePath) && !isRelativeToRoot {
			permFilePath = filepath.Join(configDir, permFilePath)
		}

		logger.Debugf("Loading permission file for role %s: %s", roleDetail.Role, permFilePath)

		data, err := os.ReadFile(permFilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read permissions file: %w", err)
		}

		var permList models.PermissionList
		if err := yaml.Unmarshal(data, &permList); err != nil {
			return nil, fmt.Errorf("failed to parse permissions YAML: %w", err)
		}

		return permList.Permissions, nil

	default:
		return nil, fmt.Errorf("invalid permissions type: expected array or string")
	}
}

// createRole creates a role in a workspace
func (p *Processor) createRole(workspaceName string, roleDetail models.RoleDetail) error {
	if p.dryRun {
		logger.Infof("[DRY-RUN] Would create role '%s' in workspace '%s'", roleDetail.Role, workspaceName)
		return nil
	}

	path := fmt.Sprintf("/%s/rbac/roles", workspaceName)

	role := models.Role{
		Name: roleDetail.Role,
	}

	var result models.RoleResponse
	if err := p.client.PostJSON(path, role, &result); err != nil {
		// Check if it's a conflict (role already exists)
		errMsg := err.Error()
		if strings.Contains(errMsg, "already exists") || strings.Contains(errMsg, "Duplicate") || strings.Contains(errMsg, "409") {
			logger.Warnf("Role %s already exists in workspace %s", roleDetail.Role, workspaceName)
			return nil
		}
		return fmt.Errorf("failed to create role '%s' in workspace '%s': %w", roleDetail.Role, workspaceName, err)
	}

	logger.Infof("Role %s created in workspace %s", roleDetail.Role, workspaceName)

	// Verify role is available before applying permissions (with configurable retries)
	maxAttempts := getMaxRetryAttemptsFromEnv()
	if err := p.waitForRoleAvailable(workspaceName, roleDetail.Role, maxAttempts); err != nil {
		return err
	}

	return nil
}

// waitForRoleAvailable waits for a role to become available with configurable retry attempts
func (p *Processor) waitForRoleAvailable(workspaceName, roleName string, maxAttempts int) error {
	path := fmt.Sprintf("/%s/rbac/roles/%s", workspaceName, roleName)

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		var result map[string]interface{}
		if err := p.client.GetJSON(path, nil, &result); err == nil {
			logger.Debugf("Role %s is available (attempt %d/%d)", roleName, attempt, maxAttempts)
			return nil // Role is available
		}

		if attempt < maxAttempts {
			// Exponential backoff: 50ms, 100ms, 150ms, 200ms, 250ms
			backoff := time.Duration(attempt*50) * time.Millisecond
			time.Sleep(backoff)
		}
	}

	return fmt.Errorf("role %s was created but is not available after %d attempts", roleName, maxAttempts)
}

// getMaxRetryAttemptsFromEnv retrieves the maximum retry attempts from environment variable or returns default (5)
func getMaxRetryAttemptsFromEnv() int {
	maxAttempts := 5
	if envAttempts := os.Getenv("MAX_RETRY_ATTEMPTS"); envAttempts != "" {
		if attempts, err := strconv.Atoi(envAttempts); err == nil && attempts > 0 {
			maxAttempts = attempts
		}
	}
	return maxAttempts
}

// addPermission adds a permission to a role
func (p *Processor) addPermission(workspaceName, roleName string, permission models.Permission) error {
	if p.dryRun {
		logger.Infof("[DRY-RUN] Would add permission {\"endpoint\":\"%s\",\"negative\":%v,\"actions\":\"%s\"} to role '%s'",
			permission.Endpoint, permission.Negative, permission.Actions, roleName)
		return nil
	}

	path := fmt.Sprintf("/%s/rbac/roles/%s/endpoints", workspaceName, roleName)

	if err := p.client.PostJSON(path, permission, nil); err != nil {
		errMsg := err.Error()
		// Check if it's a conflict (permission already exists)
		if strings.Contains(errMsg, "primary key") || strings.Contains(errMsg, "Duplicate") || strings.Contains(errMsg, "400") {
			logger.Warnf("Permission %v already exists for role %s", permission, roleName)
			return nil
		}
		return fmt.Errorf("failed to add permission %v to role '%s' in workspace '%s': %w", permission, roleName, workspaceName, err)
	}

	return nil
}

// deleteAllRoles deletes all roles from a workspace with pagination support
func (p *Processor) deleteAllRoles(workspaceName string) error {
	path := fmt.Sprintf("/%s/rbac/roles", workspaceName)
	pageSize := 1000
	offset := ""

	for {
		params := url.Values{}
		params.Add("size", strconv.Itoa(pageSize))
		if offset != "" {
			params.Add("offset", offset)
		}

		var response struct {
			Data   []models.RoleResponse `json:"data"`
			Offset string                `json:"offset"`
		}

		if err := p.client.GetJSON(path, params, &response); err != nil {
			return fmt.Errorf("failed to fetch roles: %w", err)
		}

		for _, role := range response.Data {
			if err := p.deleteRole(workspaceName, role.Name); err != nil {
				logger.Errorf("Failed to delete role %s: %v", role.Name, err)
			} else {
				logger.Infof("Deleted role '%s' successfully", role.Name)
			}
		}

		// Check if there are more pages
		if response.Offset == "" || len(response.Data) < pageSize {
			break
		}

		offset = response.Offset
	}

	return nil
}

// deleteRole deletes a specific role from a workspace
func (p *Processor) deleteRole(workspaceName, roleName string) error {
	if p.dryRun {
		logger.Infof("[DRY-RUN] Would delete role '%s' from workspace '%s'", roleName, workspaceName)
		return nil
	}

	path := fmt.Sprintf("/%s/rbac/roles/%s", workspaceName, roleName)
	resp, err := p.client.DELETE(path)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	return nil
}

// LoadWorkspaceConfig loads workspace configuration from YAML
func (p *Processor) LoadWorkspaceConfig(workspaceName string) (*models.WorkspaceConfig, error) {
	return p.loadWorkspaceConfig(workspaceName)
}

// loadWorkspaceConfig loads workspace configuration from YAML
func (p *Processor) loadWorkspaceConfig(workspaceName string) (*models.WorkspaceConfig, error) {
	configPath := filepath.Join(p.cfg.ConfigDir, workspaceName, "workspace.yaml")

	// Check if workspace-specific config exists, otherwise use root config
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		configPath = filepath.Join(p.cfg.ConfigDir, "root-workspace.yaml")
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var wsConfig models.WorkspaceConfig
	if err := yaml.Unmarshal(data, &wsConfig); err != nil {
		return nil, fmt.Errorf("failed to parse config YAML: %w", err)
	}

	return &wsConfig, nil
}

// GetWorkspaceDirs returns a list of workspace directories
func (p *Processor) GetWorkspaceDirs() ([]string, error) {
	entries, err := os.ReadDir(p.cfg.ConfigDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read config directory: %w", err)
	}

	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry.Name())
		}
	}

	return dirs, nil
}

// DeleteRole is a public method to delete a specific role from a workspace
func (p *Processor) DeleteRole(workspaceName, roleName string) error {
	// deleteRole handles dryRun and all error cases
	return p.deleteRole(workspaceName, roleName)
}

// GetAllRolesForWorkspace retrieves all roles in a workspace with pagination
func (p *Processor) GetAllRolesForWorkspace(workspaceName string) ([]models.RoleResponse, error) {
	path := fmt.Sprintf("/%s/rbac/roles", workspaceName)
	pageSize := 1000
	offset := ""
	var allRoles []models.RoleResponse

	for {
		params := url.Values{}
		params.Add("size", strconv.Itoa(pageSize))
		if offset != "" {
			params.Add("offset", offset)
		}

		var response struct {
			Data   []models.RoleResponse `json:"data"`
			Offset string                `json:"offset"`
		}

		if err := p.client.GetJSON(path, params, &response); err != nil {
			return nil, err
		}

		allRoles = append(allRoles, response.Data...)

		// Check if there are more pages
		if response.Offset == "" || len(response.Data) < pageSize {
			break
		}

		offset = response.Offset
	}

	return allRoles, nil
}

// workspaceExists checks if a workspace exists in Kong with pagination support
func (p *Processor) workspaceExists(workspaceName string) (bool, error) {
	pageSize := 1000
	offset := ""

	for {
		params := url.Values{}
		params.Add("size", strconv.Itoa(pageSize))
		if offset != "" {
			params.Add("offset", offset)
		}

		var response struct {
			Data   []map[string]interface{} `json:"data"`
			Offset string                   `json:"offset"`
		}
		if err := p.client.GetJSON("/workspaces", params, &response); err != nil {
			return false, err
		}

		// Check if workspace is in the list
		for _, ws := range response.Data {
			if name, ok := ws["name"].(string); ok && name == workspaceName {
				return true, nil
			}
		}

		// Check if there are more pages
		if response.Offset == "" || len(response.Data) < pageSize {
			break
		}

		offset = response.Offset
	}

	return false, nil
}
