package groups

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/Kong/kwot/internal/config"
	"github.com/Kong/kwot/internal/kong"
	"github.com/Kong/kwot/internal/logger"
	"github.com/Kong/kwot/internal/models"
	"github.com/Kong/kwot/internal/validation"
	"gopkg.in/yaml.v3"
)

const groupConfigName = "groups-and-roles.yaml"

// Processor handles group operations
type Processor struct {
	client *kong.Client
	cfg    *config.Config
	dryRun bool
}

// NewProcessor creates a new group processor
func NewProcessor(client *kong.Client, cfg *config.Config, dryRun bool) *Processor {
	return &Processor{
		client: client,
		cfg:    cfg,
		dryRun: dryRun,
	}
}

// ProcessGroups processes group configurations
func (p *Processor) ProcessGroups(selectedWorkspace string) error {
	if p.dryRun {
		logger.Info("[DRY-RUN] Groups processing: no changes will be made")
	} else {
		logger.Info("Adding groups. Make sure workspaces and roles are already created.")
	}

	// Get all existing groups
	allGroups, err := p.GetAllGroups()
	if err != nil {
		return fmt.Errorf("failed to get existing groups: %w", err)
	}

	logger.Debugf("Number of existing groups = %d", len(allGroups))

	// Get all existing workspaces
	allWorkspaces, err := p.GetAllWorkspaces()
	if err != nil {
		return fmt.Errorf("failed to get existing workspaces: %w", err)
	}

	logger.Debugf("Number of existing workspaces = %d", len(allWorkspaces))

	// Load group configuration
	groupConfig, err := p.loadGroupConfig(selectedWorkspace)
	if err != nil {
		return fmt.Errorf("failed to load group config: %w", err)
	}

	// Validate group configuration (Kong API will reject duplicate groups)
	for _, group := range groupConfig {
		if err := validation.ValidateGroupConfig(group); err != nil {
			return fmt.Errorf("group configuration validation failed: %w", err)
		}
	}

	// Filter groups by workspace if specified
	var filteredGroups []models.GroupDetail
	if selectedWorkspace != "all" {
		logger.Infof("Will only create groups and apply roles for workspace %s", selectedWorkspace)

		for _, group := range groupConfig {
			for _, role := range group.Roles {
				if role.Workspace == selectedWorkspace {
					filteredGroups = append(filteredGroups, group)
					break
				}
			}
		}

		if len(filteredGroups) == 0 {
			logger.Debugf("%d groups found that match %s", 0, selectedWorkspace)
			return nil
		}

		logger.Debugf("%d groups found that match %s", len(filteredGroups), selectedWorkspace)
	} else {
		filteredGroups = groupConfig
	}

	// Cache all roles for all workspaces to avoid repeated API calls (parallelized)
	workspaceRoles := make(map[string][]models.RoleResponse)
	rolesChan := make(chan struct {
		workspace string
		roles     []models.RoleResponse
		err       error
	}, len(allWorkspaces))

	// Use a semaphore to limit concurrent role fetches
	maxWorkers := 5
	semaphore := make(chan struct{}, maxWorkers)

	for _, ws := range allWorkspaces {
		semaphore <- struct{}{} // Acquire token
		go func(wsName string) {
			defer func() { <-semaphore }() // Release token
			roles, err := p.GetAllRolesForWorkspace(wsName)
			rolesChan <- struct {
				workspace string
				roles     []models.RoleResponse
				err       error
			}{wsName, roles, err}
		}(ws.Name)
	}

	// Collect results from all goroutines
	for i := 0; i < len(allWorkspaces); i++ {
		result := <-rolesChan
		if result.err != nil {
			logger.Warnf("Failed to fetch roles for workspace %s: %v", result.workspace, result.err)
			// Continue anyway - we might get the roles later when needed
		} else {
			workspaceRoles[result.workspace] = result.roles
		}
	}

	// Process each group
	for _, groupInfo := range filteredGroups {
		groupID, err := p.createOrGetGroup(groupInfo, allGroups)
		if err != nil {
			logger.Errorf("Failed to create group %s: %v", groupInfo.GroupName, err)
			return err
		}

		// Skip role assignment in dry-run mode (group doesn't actually exist)
		if p.dryRun {
			for _, role := range groupInfo.Roles {
				if selectedWorkspace != "all" && role.Workspace != selectedWorkspace {
					continue
				}
				logger.Infof("[DRY-RUN] Would assign role '%s' in workspace '%s' to group '%s'", role.Role, role.Workspace, groupInfo.GroupName)
			}
			continue
		}

		// Assign roles to group in parallel
		rolesToAssign := make([]models.GroupRole, 0)
		for _, role := range groupInfo.Roles {
			if selectedWorkspace != "all" && role.Workspace != selectedWorkspace {
				continue
			}
			rolesToAssign = append(rolesToAssign, role)
		}

		if len(rolesToAssign) > 0 {
			roleSemaphore := make(chan struct{}, 5) // Allow 5 concurrent role assignments per group
			var roleWg sync.WaitGroup

			for _, role := range rolesToAssign {
				roleWg.Add(1)
				go func(r models.GroupRole) {
					defer roleWg.Done()

					// Acquire semaphore slot
					roleSemaphore <- struct{}{}
					defer func() { <-roleSemaphore }()

					created, err := p.assignRoleToGroupWithCache(groupID, r, allWorkspaces, workspaceRoles)
					if err != nil {
						logger.Errorf("❌ Failed to assign role '%s' from workspace '%s' to group '%s':\n   %v",
							r.Role, r.Workspace, groupInfo.GroupName, err)
					} else if created {
						logger.Infof("Role created in group %s mapping workspace %s and role %s",
							groupInfo.GroupName, r.Workspace, r.Role)
					}
				}(role)
			}

			roleWg.Wait()
		}
	}

	return nil
}

// loadGroupConfig loads group configuration from YAML.
// Lookup precedence (workspace-local file takes priority over global):
//   - For a specific workspace: <CONFIG_DIR>/<workspace>/groups-and-roles.yaml, then <CONFIG_DIR>/groups-and-roles.yaml
//   - For "all": each workspace dir is checked for a local file; workspaces without one fall back to the global file
//
// Supports two YAML formats:
// 1. Direct array: [- group_name: ..., - group_name: ...]
// 2. Structured: role_info: {...}, config: [- group_name: ..., ...]
func (p *Processor) loadGroupConfig(selectedWorkspace string) ([]models.GroupDetail, error) {
	if selectedWorkspace != "all" {
		return p.loadGroupConfigForWorkspace(selectedWorkspace)
	}

	// "all" mode: scan workspace subdirs, aggregate groups from local files;
	// workspaces without a local file fall back to the global file.
	entries, err := os.ReadDir(p.cfg.ConfigDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read config directory: %w", err)
	}

	// Load global file once (may not exist)
	globalGroups, _ := p.parseGroupConfigFile(filepath.Join(p.cfg.ConfigDir, groupConfigName))

	var allGroups []models.GroupDetail
	workspacesWithLocalFile := make(map[string]bool)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		wsName := entry.Name()
		localPath := filepath.Join(p.cfg.ConfigDir, wsName, groupConfigName)
		if _, statErr := os.Stat(localPath); statErr == nil {
			groups, err := p.parseGroupConfigFile(localPath)
			if err != nil {
				return nil, fmt.Errorf("failed to load groups for workspace %s: %w", wsName, err)
			}
			allGroups = append(allGroups, groups...)
			workspacesWithLocalFile[wsName] = true
		}
	}

	// Include global entries for workspaces that don't have a local file
	for _, g := range globalGroups {
		hasLocalFile := false
		for _, role := range g.Roles {
			if workspacesWithLocalFile[role.Workspace] {
				hasLocalFile = true
				break
			}
		}
		if !hasLocalFile {
			allGroups = append(allGroups, g)
		}
	}

	if len(allGroups) == 0 {
		return nil, fmt.Errorf("no %s found in workspace directories or config root", groupConfigName)
	}

	return allGroups, nil
}

// loadGroupConfigForWorkspace loads group config for a single workspace,
// preferring the workspace-local file over the global file.
func (p *Processor) loadGroupConfigForWorkspace(workspace string) ([]models.GroupDetail, error) {
	localPath := filepath.Join(p.cfg.ConfigDir, workspace, groupConfigName)
	if _, err := os.Stat(localPath); err == nil {
		logger.Debugf("Using workspace-local %s for workspace %s", groupConfigName, workspace)
		return p.parseGroupConfigFile(localPath)
	}

	globalPath := filepath.Join(p.cfg.ConfigDir, groupConfigName)
	if _, err := os.Stat(globalPath); err == nil {
		logger.Debugf("Using global %s for workspace %s", groupConfigName, workspace)
		return p.parseGroupConfigFile(globalPath)
	}

	return nil, fmt.Errorf("no %s found in %s/ or config root", groupConfigName, workspace)
}

// parseGroupConfigFile reads and parses a groups-and-roles YAML file.
// Supports two formats:
// 1. Direct array: [- group_name: ..., - group_name: ...]
// 2. Structured: role_info: {...}, config: [- group_name: ..., ...]
func (p *Processor) parseGroupConfigFile(path string) ([]models.GroupDetail, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read group config file: %w", err)
	}

	// Try format 2 first (structured with role_info and config)
	var wrapper models.GroupConfigWrapper
	if err := yaml.Unmarshal(data, &wrapper); err == nil && len(wrapper.Config) > 0 {
		return wrapper.Config, nil
	}

	// Fall back to format 1 (direct array)
	var groups []models.GroupDetail
	if err := yaml.Unmarshal(data, &groups); err != nil {
		return nil, fmt.Errorf("failed to parse group config YAML: %w", err)
	}

	return groups, nil
}

// getAllGroups retrieves all existing groups
// GetAllGroups retrieves all existing groups with proper pagination
func (p *Processor) GetAllGroups() ([]models.GroupResponse, error) {
	var allGroups []models.GroupResponse
	pageSize := 1000 // Kong default page size
	offset := ""

	for {
		params := url.Values{}
		params.Add("size", strconv.Itoa(pageSize))
		if offset != "" {
			params.Add("offset", offset)
		}

		var response struct {
			Data   []models.GroupResponse `json:"data"`
			Offset string                 `json:"offset"`
		}

		if err := p.client.GetJSON("/groups", params, &response); err != nil {
			return nil, err
		}

		allGroups = append(allGroups, response.Data...)

		// Check if there are more pages
		if response.Offset == "" || len(response.Data) < pageSize {
			break
		}

		offset = response.Offset
	}

	return allGroups, nil
}

// GetAllWorkspaces retrieves all existing workspaces with proper pagination
func (p *Processor) GetAllWorkspaces() ([]models.WorkspaceResponse, error) {
	var allWorkspaces []models.WorkspaceResponse
	pageSize := 1000 // Kong default page size
	offset := ""

	for {
		params := url.Values{}
		params.Add("size", strconv.Itoa(pageSize))
		if offset != "" {
			params.Add("offset", offset)
		}

		var response struct {
			Data   []models.WorkspaceResponse `json:"data"`
			Offset string                     `json:"offset"`
		}

		if err := p.client.GetJSON("/workspaces", params, &response); err != nil {
			return nil, err
		}

		allWorkspaces = append(allWorkspaces, response.Data...)

		// Check if there are more pages
		if response.Offset == "" || len(response.Data) < pageSize {
			break
		}

		offset = response.Offset
	}

	return allWorkspaces, nil
}

// createOrGetGroup creates a group if it doesn't exist and returns its ID
func (p *Processor) createOrGetGroup(groupInfo models.GroupDetail, existingGroups []models.GroupResponse) (string, error) {
	logger.Debugf("Group Name to search: %s", groupInfo.GroupName)

	// Group already exists - find and return its ID
	for _, group := range existingGroups {
		if group.Name == groupInfo.GroupName {
			logger.Infof("Group %s already exists", groupInfo.GroupName)
			return group.ID, nil
		}
	}

	// Create new group
	if p.dryRun {
		logger.Infof("[DRY-RUN] Processing group: %s", groupInfo.GroupName)
	} else {
		logger.Infof("Creating group: %s", groupInfo.GroupName)
	}

	group := models.Group{
		Name:    groupInfo.GroupName,
		Comment: groupInfo.GroupComment,
	}

	if p.dryRun {
		logger.Infof("[DRY-RUN] Would create group: '%s'", groupInfo.GroupName)
		return "", nil
	}

	var result models.GroupResponse
	if err := p.client.PostJSON("/groups", group, &result); err != nil {
		// Check for conflict (group already exists)
		errMsg := err.Error()
		if strings.Contains(errMsg, "already exists") || strings.Contains(errMsg, "Duplicate") || strings.Contains(errMsg, "409") {
			logger.Warnf("Group %s already exists", groupInfo.GroupName)
			// Look up the ID from cached list instead of making another API call
			for _, cachedGroup := range existingGroups {
				if cachedGroup.Name == groupInfo.GroupName {
					return cachedGroup.ID, nil
				}
			}
			// Fallback: fetch from API if not in cache (shouldn't happen in normal flow)
			path := fmt.Sprintf("/groups/%s", groupInfo.GroupName)
			var groupResp models.GroupResponse
			if err := p.client.GetJSON(path, nil, &groupResp); err != nil {
				return "", fmt.Errorf("failed to get group ID for existing group '%s': %w", groupInfo.GroupName, err)
			}
			return groupResp.ID, nil
		}
		logger.Errorf("Group %s may already exist. Possible duplication in group config", groupInfo.GroupName)
		return "", fmt.Errorf("failed to create group '%s': %w", groupInfo.GroupName, err)
	}

	logger.Infof("Group %s created", groupInfo.GroupName)
	return result.ID, nil
}

// getGroupID retrieves the ID of a group by name
func (p *Processor) getGroupID(groupName string) (string, error) {
	path := fmt.Sprintf("/groups/%s", groupName)

	var group models.GroupResponse
	if err := p.client.GetJSON(path, nil, &group); err != nil {
		return "", fmt.Errorf("failed to get group '%s': %w", groupName, err)
	}

	return group.ID, nil
}

// getRoleID retrieves the ID of a role in a workspace with pagination support
func (p *Processor) getRoleID(workspaceName, roleName string) (string, error) {
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
			return "", err
		}

		for _, role := range response.Data {
			if role.Name == roleName {
				return role.ID, nil
			}
		}

		// Check if there are more pages
		if response.Offset == "" || len(response.Data) < pageSize {
			break
		}

		offset = response.Offset
	}

	return "", fmt.Errorf("role %s not found in workspace %s", roleName, workspaceName)
}

// LoadGroupConfig loads group configuration from file
func (p *Processor) LoadGroupConfig(selectedWorkspace string) ([]models.GroupDetail, error) {
	return p.loadGroupConfig(selectedWorkspace)
}
func (p *Processor) GetAllRolesForWorkspace(workspaceName string) ([]models.RoleResponse, error) {
	var allRoles []models.RoleResponse
	pageSize := 1000 // Kong default page size
	offset := ""

	for {
		path := fmt.Sprintf("/%s/rbac/roles", workspaceName)
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

// assignRoleToGroupWithCache assigns a role to a group using cached role data
// This avoids repeated API calls to fetch roles
func (p *Processor) assignRoleToGroupWithCache(groupID string, role models.GroupRole, allWorkspaces []models.WorkspaceResponse, workspaceRoles map[string][]models.RoleResponse) (bool, error) {
	// Get workspace ID
	var workspaceID string
	for _, ws := range allWorkspaces {
		if ws.Name == role.Workspace {
			workspaceID = ws.ID
			break
		}
	}
	if workspaceID == "" {
		return false, fmt.Errorf(
			"workspace '%s' not found"+
				"\n  Action required: Create workspace '%s' first"+
				"\n  Configuration location: config/%s/workspace.yaml",
			role.Workspace,
			role.Workspace,
			role.Workspace,
		)
	}

	// Get role ID from cached roles
	var roleID string
	if cachedRoles, exists := workspaceRoles[role.Workspace]; exists {
		for _, r := range cachedRoles {
			if r.Name == role.Role {
				roleID = r.ID
				break
			}
		}
	}

	// If role not found in cache, fall back to fetching from API
	if roleID == "" {
		var err error
		roleID, err = p.getRoleID(role.Workspace, role.Role)
		if err != nil {
			return false, fmt.Errorf("failed to get role ID for '%s' in workspace '%s': %w", role.Role, role.Workspace, err)
		}
	}

	// Assign role to group
	path := fmt.Sprintf("/groups/%s/roles", groupID)

	roleAssignment := models.GroupRoleAssignment{
		WorkspaceID: workspaceID,
		RBACRoleID:  roleID,
	}

	if err := p.client.PostJSON(path, roleAssignment, nil); err != nil {
		// Group-roles post throws 400 instead of 409 when record exists
		errMsg := err.Error()
		if strings.Contains(errMsg, "primary key") || strings.Contains(errMsg, "Duplicate") || strings.Contains(errMsg, "400") {
			logger.Warnf("Role %s is already assigned to group in workspace %s", role.Role, role.Workspace)
			return false, nil
		}
		return false, fmt.Errorf("failed to assign role '%s' to group in workspace '%s': %w", role.Role, role.Workspace, err)
	}

	return true, nil
}

// DeleteGroup deletes a group from Kong
func (p *Processor) DeleteGroup(groupName string) error {
	if p.dryRun {
		logger.Infof("[DRY-RUN] Would delete group '%s'", groupName)
		return nil
	}

	// Get the group ID
	groupID, err := p.getGroupID(groupName)
	if err != nil {
		return fmt.Errorf("failed to get group ID for '%s': %w", groupName, err)
	}

	// Delete the group
	path := fmt.Sprintf("/groups/%s", groupID)
	resp, err := p.client.DELETE(path)
	if err != nil {
		return fmt.Errorf("failed to delete group: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	logger.Infof("Group '%s' (ID: %s) deleted successfully", groupName, groupID)
	return nil
}

// DeleteAllGroups deletes all groups from Kong
func (p *Processor) DeleteAllGroups() error {
	if p.dryRun {
		logger.Infof("[DRY-RUN] Would delete all groups")
		return nil
	}

	// Get all groups
	allGroups, err := p.GetAllGroups()
	if err != nil {
		return fmt.Errorf("failed to get list of groups: %w", err)
	}

	if len(allGroups) == 0 {
		logger.Infof("No groups found to delete")
		return nil
	}

	var deleteErrors []error
	for _, group := range allGroups {
		path := fmt.Sprintf("/groups/%s", group.ID)
		resp, err := p.client.DELETE(path)
		if err != nil {
			logger.Warnf("Failed to delete group '%s': %v", group.Name, err)
			deleteErrors = append(deleteErrors, fmt.Errorf("group %s: %w", group.Name, err))
			continue
		}
		_ = resp.Body.Close()
		logger.Infof("✓ Group '%s' (ID: %s) deleted", group.Name, group.ID)
	}

	if len(deleteErrors) > 0 {
		return fmt.Errorf("failed to delete %d group(s)", len(deleteErrors))
	}

	logger.Infof("✓ All %d group(s) deleted successfully", len(allGroups))
	return nil
}
