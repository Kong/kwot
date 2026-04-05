package workspace

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
	"github.com/Kong/kwot/internal/roles"
	"github.com/Kong/kwot/internal/validation"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

const (
	workspaceConfigName          = "workspace.yaml"
	workspaceRBACUsersConfigName = "workspace-rbac-user.yaml"
	rootWorkspaceConfig          = "root-workspace.yaml"
)

// Processor handles workspace operations
type Processor struct {
	client        *kong.Client
	cfg           *config.Config
	roleProcessor *roles.Processor
	dryRun        bool
}

// NewProcessor creates a new workspace processor
func NewProcessor(client *kong.Client, cfg *config.Config, dryRun bool) *Processor {
	return &Processor{
		client:        client,
		cfg:           cfg,
		roleProcessor: roles.NewProcessor(client, cfg, dryRun),
		dryRun:        dryRun,
	}
}

// ProcessWorkspaces processes workspace configurations
func (p *Processor) ProcessWorkspaces(selectedWorkspace string) error {
	dirs, err := p.GetWorkspaceDirs()
	if err != nil {
		return fmt.Errorf("failed to get workspace directories: %w", err)
	}

	// Validate that selected workspace exists in configuration
	if selectedWorkspace != "all" {
		found := false
		for _, dir := range dirs {
			if selectedWorkspace == dir {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("workspace '%s' not found in configuration", selectedWorkspace)
		}
	}

	// Filter workspaces based on selection
	var workspacesToProcess []string
	for _, dir := range dirs {
		if selectedWorkspace != "all" && selectedWorkspace != dir {
			logger.Debugf("Skipping workspace %s", dir)
			continue
		}
		workspacesToProcess = append(workspacesToProcess, dir)
	}

	if len(workspacesToProcess) == 0 {
		return nil
	}

	// Calculate actual concurrency (use minimum of configured and actual workspaces)
	actualConcurrency := p.cfg.MaxConcurrentWorkspaces
	if len(workspacesToProcess) < actualConcurrency {
		actualConcurrency = len(workspacesToProcess)
	}

	logger.Infof("Processing %d workspaces with %d concurrent workers", len(workspacesToProcess), actualConcurrency)

	// Process workspaces in parallel with concurrency limit
	semaphore := make(chan struct{}, actualConcurrency)
	var wg sync.WaitGroup
	errors := make(chan error, len(workspacesToProcess))

	for _, workspace := range workspacesToProcess {
		wg.Add(1)
		go func(ws string) {
			defer wg.Done()

			// Acquire semaphore slot
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			if err := p.processWorkspace(ws); err != nil {
				logger.Errorf("Failed to process workspace %s: %v", ws, err)
				errors <- fmt.Errorf("workspace %s: %w", ws, err)
			} else {
				errors <- nil
			}
		}(workspace)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errors)

	// Collect errors
	var errorList []error
	for err := range errors {
		if err != nil {
			errorList = append(errorList, err)
		}
	}

	// Log summary
	if len(errorList) > 0 {
		logger.Warnf("Completed workspace processing with %d error(s)", len(errorList))
		for _, err := range errorList {
			logger.Errorf("%v", err)
		}
	}

	return nil
}

// ApplyRBACUsersForWorkspaces applies RBAC users for specified workspaces
// This is typically called in Step 2 (after roles) to ensure RBAC users are created for both new and existing workspaces
func (p *Processor) ApplyRBACUsersForWorkspaces(selectedWorkspace string) error {
	// Skip RBAC user processing in dry-run mode since workspaces may not exist yet
	if p.dryRun {
		logger.Infof("[DRY-RUN] RBAC users would be created (skipping actual creation in dry-run mode)")
		return nil
	}

	if !p.cfg.CreateRBACUsers {
		logger.Debugf("FEATURE_CREATE_RBAC_USERS is disabled, skipping RBAC user creation")
		return nil
	}

	var workspacesToProcess []string
	var err error

	// "all" means process all workspaces, empty string also means all workspaces
	if selectedWorkspace != "" && selectedWorkspace != "all" {
		workspacesToProcess = []string{selectedWorkspace}
	} else {
		workspacesToProcess, err = p.GetWorkspaceDirs()
		if err != nil {
			return fmt.Errorf("failed to get workspace directories: %w", err)
		}
	}

	// Calculate actual concurrency
	actualConcurrency := p.cfg.MaxConcurrentWorkspaces
	if len(workspacesToProcess) < actualConcurrency {
		actualConcurrency = len(workspacesToProcess)
	}

	// Process RBAC users in parallel
	semaphore := make(chan struct{}, actualConcurrency)
	var wg sync.WaitGroup
	errors := make(chan error, len(workspacesToProcess))

	for _, workspaceName := range workspacesToProcess {
		wg.Add(1)
		go func(ws string) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			if err := p.applyWorkspaceRBACUsers(ws); err != nil {
				logger.Errorf("Failed to apply RBAC users for workspace '%s': %v", ws, err)
				errors <- fmt.Errorf("workspace %s: %w", ws, err)
			}
		}(workspaceName)
	}

	wg.Wait()
	close(errors)

	var errorList []error
	for err := range errors {
		if err != nil {
			errorList = append(errorList, err)
		}
	}

	if len(errorList) > 0 {
		logger.Warnf("Completed RBAC user application with %d error(s)", len(errorList))
		return fmt.Errorf("failed to apply RBAC users for %d workspace(s); first error: %w", len(errorList), errorList[0])
	}

	return nil
}

// GetKongWorkspaces retrieves all workspaces from Kong with proper pagination
func (p *Processor) GetKongWorkspaces() ([]string, error) {
	var names []string
	pageSize := 1000
	offset := ""

	for {
		params := url.Values{}
		params.Add("size", strconv.Itoa(pageSize))
		if offset != "" {
			params.Add("offset", offset)
		}

		var result struct {
			Data []struct {
				Name string `json:"name"`
			} `json:"data"`
			Offset string `json:"offset"`
		}

		if err := p.client.GetJSON("/workspaces", params, &result); err != nil {
			return nil, fmt.Errorf("failed to get workspaces from Kong: %w", err)
		}

		for _, ws := range result.Data {
			names = append(names, ws.Name)
		}

		// Check if there are more pages
		if result.Offset == "" || len(result.Data) < pageSize {
			break
		}

		offset = result.Offset
	}

	return names, nil
}

// processWorkspace processes a single workspace
func (p *Processor) processWorkspace(workspaceName string) error {
	logger.Infof("Processing workspace: %s", workspaceName)

	// Validate workspace name first
	if err := validation.ValidateWorkspaceName(workspaceName); err != nil {
		return fmt.Errorf("invalid workspace name: %w", err)
	}

	// Load workspace configuration
	wsConfig, err := p.loadWorkspaceConfig(workspaceName)
	if err != nil {
		return fmt.Errorf("failed to load workspace config: %w", err)
	}

	// Validate workspace configuration before applying
	if err := p.validateWorkspaceConfig(workspaceName, wsConfig); err != nil {
		return fmt.Errorf("workspace configuration validation failed: %w", err)
	}

	// Create or reapply workspace - check exists and create atomically in one call
	// This avoids race conditions when multiple workers check/create simultaneously
	alreadyExisted, err := p.createWorkspaceWithExistenceCheck(workspaceName, wsConfig)
	if err != nil {
		return err
	}

	if alreadyExisted {
		// Workspace already existed in Kong - skip creation, plugins will be reapplied
		if p.dryRun {
			logger.Infof("[DRY-RUN] Workspace %s already exists (skipping creation)", workspaceName)
		} else {
			logger.Infof("Workspace %s already exists (skipping creation)", workspaceName)
		}
	} else {
		// Workspace was just created
		if p.dryRun {
			logger.Infof("[DRY-RUN] Workspace %s would be created", workspaceName)
		} else {
			logger.Infof("Workspace %s created", workspaceName)
		}
	}

	// Apply plugins declaratively for all workspaces (create new, delete removed ones)
	if err := p.applyPluginsDeclarative(workspaceName, wsConfig.Plugins); err != nil {
		logger.Errorf("Failed to apply plugins declaratively to workspace %s: %v", workspaceName, err)
		return fmt.Errorf("failed to apply plugins declaratively: %w", err)
	}

	// Note: RBAC users will be applied in Step 2 (after roles are created) via ApplyRBACUsersForWorkspaces
	// This ensures users can be assigned to roles that already exist

	return nil
}

// loadWorkspaceConfig loads workspace configuration from YAML
func (p *Processor) loadWorkspaceConfig(workspaceName string) (*models.WorkspaceConfig, error) {
	configPath := filepath.Join(p.cfg.ConfigDir, workspaceName, workspaceConfigName)

	// Check if workspace-specific config exists, otherwise use root config
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		configPath = filepath.Join(p.cfg.ConfigDir, rootWorkspaceConfig)
		logger.Infof("Using root workspace config for %s: %s", workspaceName, configPath)
	} else {
		logger.Infof("Using workspace-specific config for %s: %s", workspaceName, configPath)
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

// createWorkspaceWithExistenceCheck creates a new workspace and returns whether it already existed
// Returns (alreadyExisted bool, error)
func (p *Processor) createWorkspaceWithExistenceCheck(name string, wsConfig *models.WorkspaceConfig) (bool, error) {
	workspace := models.Workspace{
		Name:   name,
		Config: wsConfig.Config,
	}

	if p.dryRun {
		logger.Infof("[DRY-RUN] Would create workspace %s", name)
		return false, nil
	}

	var result models.WorkspaceResponse
	if err := p.client.PostJSON("/workspaces", workspace, &result); err != nil {
		// If workspace already exists (409 UNIQUE violation or UNIQUE constraint error), return true to indicate it existed
		// Don't log here - the caller will handle the message
		if strings.Contains(err.Error(), "409") || strings.Contains(err.Error(), "UNIQUE") {
			return true, nil
		}
		return false, fmt.Errorf("failed to create workspace: %w", err)
	}

	// Workspace was successfully created
	// Verify workspace is available before proceeding (with configurable retries)
	maxAttempts := p.cfg.MaxRetryAttempts
	if err := p.waitForWorkspaceAvailable(name, maxAttempts); err != nil {
		return false, err
	}

	return false, nil
}

// waitForWorkspaceAvailable waits for a workspace to become available with configurable retry attempts
func (p *Processor) waitForWorkspaceAvailable(name string, maxAttempts int) error {
	path := fmt.Sprintf("/workspaces/%s", name)

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		var result map[string]interface{}
		if err := p.client.GetJSON(path, nil, &result); err == nil {
			logger.Debugf("Workspace %s is available (attempt %d/%d)", name, attempt, maxAttempts)
			return nil // Workspace is available
		} else {
			lastErr = err
		}

		if attempt < maxAttempts {
			// Linear backoff: 50ms, 100ms, 150ms, 200ms, 250ms
			backoff := time.Duration(attempt*50) * time.Millisecond
			time.Sleep(backoff)
		}
	}

	return fmt.Errorf("workspace %s was created but is not available after %d attempts: %w", name, maxAttempts, lastErr)
}

// waitForRBACUserAvailable waits for an RBAC user to become available with configurable retry attempts
func (p *Processor) waitForRBACUserAvailable(workspaceName, userName string, maxAttempts int) error {
	path := fmt.Sprintf("/%s/rbac/users/%s", workspaceName, userName)

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		var result map[string]interface{}
		if err := p.client.GetJSON(path, nil, &result); err == nil {
			logger.Debugf("RBAC user %s is available (attempt %d/%d)", userName, attempt, maxAttempts)
			return nil // RBAC user is available
		} else {
			lastErr = err
		}

		if attempt < maxAttempts {
			// Linear backoff: 50ms, 100ms, 150ms, 200ms, 250ms
			backoff := time.Duration(attempt*50) * time.Millisecond
			time.Sleep(backoff)
		}
	}

	return fmt.Errorf("RBAC user %s was created but is not available after %d attempts: %w", userName, maxAttempts, lastErr)
}

// applyWorkspaceRBACUsers applies workspace-scoped RBAC users
func (p *Processor) applyWorkspaceRBACUsers(workspaceName string) error {
	configPath := filepath.Join(p.cfg.ConfigDir, workspaceName, workspaceRBACUsersConfigName)

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		logger.Warnf("No RBAC users config found for workspace %s", workspaceName)
		return nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read RBAC users config: %w", err)
	}

	var users []models.RBACUser
	if err := yaml.Unmarshal(data, &users); err != nil {
		return fmt.Errorf("failed to parse RBAC users YAML: %w", err)
	}

	// Validate RBAC users before processing
	if err := p.validateWorkspaceRBACUsers(users, workspaceName); err != nil {
		return fmt.Errorf("RBAC users validation failed: %w", err)
	}

	return p.processRBACUsers(workspaceName, users)
}

// processRBACUsers processes RBAC users for a workspace
func (p *Processor) processRBACUsers(workspaceName string, users []models.RBACUser) error {
	// Process each user from config
	for _, user := range users {
		if err := p.createOrUpdateRBACUser(workspaceName, user); err != nil {
			logger.Errorf("Failed to process RBAC user '%s': %v", user.Name, err)
			continue
		}

		// Assign roles
		for _, role := range user.Roles {
			if err := p.assignRoleToRBACUser(workspaceName, user.Name, role); err != nil {
				logger.Errorf("Failed to assign role '%s' to user '%s': %v", role, user.Name, err)
			}
		}
	}

	// Delete users not in config if feature is enabled
	// Only fetch list if we need to delete
	if p.cfg.DeleteExistingRBACUsers {
		if workspaceName == "default" {
			logger.Error("Deleting RBAC users in 'default' workspace is strongly discouraged")
			return nil
		}

		// Build set of configured users
		configUserNames := make(map[string]bool)
		for _, user := range users {
			configUserNames[user.Name] = true
		}

		// Get current users only if we're deleting
		currentUsers, err := p.getCurrentRBACUsers(workspaceName)
		if err != nil {
			logger.Errorf("Failed to fetch existing users in workspace '%s': %v", workspaceName, err)
			return nil // Don't fail, just skip deletion
		}

		// Delete users not in config
		for _, userName := range currentUsers {
			if !configUserNames[userName] {
				if err := p.deleteRBACUser(workspaceName, userName); err != nil {
					logger.Errorf("Failed to delete RBAC user '%s': %v", userName, err)
				} else {
					logger.Warnf("RBAC user '%s' deleted from workspace '%s' as it is not in the configuration", userName, workspaceName)
				}
			}
		}
	}

	return nil
}

// getCurrentRBACUsers gets current RBAC users in a workspace with pagination
func (p *Processor) getCurrentRBACUsers(workspaceName string) ([]string, error) {
	path := fmt.Sprintf("/%s/rbac/users", workspaceName)
	pageSize := 1000
	offset := ""
	var userNames []string

	for {
		params := url.Values{}
		params.Add("size", strconv.Itoa(pageSize))
		if offset != "" {
			params.Add("offset", offset)
		}

		var response struct {
			Data   []models.RBACUserResponse `json:"data"`
			Offset string                    `json:"offset"`
		}

		if err := p.client.GetJSON(path, params, &response); err != nil {
			return nil, err
		}

		for _, user := range response.Data {
			userNames = append(userNames, user.Name)
		}

		// Check if there are more pages
		if response.Offset == "" || len(response.Data) < pageSize {
			break
		}

		offset = response.Offset
	}

	return userNames, nil
}

// createOrUpdateRBACUser creates or updates an RBAC user
// Matches Node.js approach: just POST and handle 409 conflict gracefully.
// NOTE: user_token is only applied on initial creation. If the user already
// exists (409), the configured token is NOT reconciled — Kong does not expose
// a way to retrieve the existing token for comparison, so we leave it unchanged.
func (p *Processor) createOrUpdateRBACUser(workspaceName string, user models.RBACUser) error {
	// Use user-specified token if provided, otherwise generate a random UUID
	userToken := user.UserToken
	if userToken == "" {
		userToken = uuid.New().String()
	}

	userData := map[string]string{
		"name":       user.Name,
		"user_token": userToken,
	}

	createPath := fmt.Sprintf("/%s/rbac/users", workspaceName)
	if err := p.client.PostJSON(createPath, userData, nil); err != nil {
		// If user already exists (409 conflict, UNIQUE violation, already exists, Duplicate resource), that's fine - just log and continue
		errStr := err.Error()
		if strings.Contains(errStr, "409") || strings.Contains(errStr, "conflict") ||
			strings.Contains(errStr, "UNIQUE violation") || strings.Contains(errStr, "already exists") ||
			strings.Contains(errStr, "Duplicate resource") {
			if user.UserToken != "" {
				logger.Warnf("RBAC user '%s' already exists in workspace '%s' — configured user_token was not applied", user.Name, workspaceName)
			} else {
				logger.Warnf("RBAC user '%s' already exists in workspace '%s'", user.Name, workspaceName)
			}
			return nil
		}
		return fmt.Errorf("failed to create RBAC user: %w", err)
	}

	if user.UserToken != "" {
		logger.Debugf("RBAC user '%s' created in workspace '%s' with configured user_token", user.Name, workspaceName)
	}
	logger.Infof("RBAC user '%s' created in workspace '%s'", user.Name, workspaceName)

	// Verify RBAC user is available before assigning roles (with configurable retries)
	maxAttempts := p.cfg.MaxRetryAttempts
	if err := p.waitForRBACUserAvailable(workspaceName, user.Name, maxAttempts); err != nil {
		return err
	}

	return nil
}

// assignRoleToRBACUser assigns a role to an RBAC user
func (p *Processor) assignRoleToRBACUser(workspaceName, userName, roleName string) error {
	path := fmt.Sprintf("/%s/rbac/users/%s/roles", workspaceName, userName)

	roleData := models.RoleAssignment{
		Roles: roleName,
	}

	if err := p.client.PostJSON(path, roleData, nil); err != nil {
		// If role is already assigned (409 conflict, UNIQUE violation, already exists, primary key violation), that's fine - just log and continue
		errStr := err.Error()
		if strings.Contains(errStr, "409") || strings.Contains(errStr, "conflict") ||
			strings.Contains(errStr, "UNIQUE violation") || strings.Contains(errStr, "already exists") ||
			strings.Contains(errStr, "Duplicate resource") || strings.Contains(errStr, "primary key violation") {
			logger.Warnf("Role '%s' already assigned to user '%s' in workspace '%s'", roleName, userName, workspaceName)
			return nil
		}
		return fmt.Errorf("failed to assign role: %w", err)
	}

	logger.Infof("Role '%s' assigned to user '%s' in workspace '%s'", roleName, userName, workspaceName)
	return nil
}

// deleteRBACUser deletes an RBAC user
func (p *Processor) deleteRBACUser(workspaceName, userName string) error {
	path := fmt.Sprintf("/%s/rbac/users/%s", workspaceName, userName)

	resp, err := p.client.DELETE(path)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	return nil
}

// DeleteWorkspace deletes a workspace from Kong
func (p *Processor) DeleteWorkspace(workspaceName string) error {
	if workspaceName == "" || workspaceName == "default" {
		logger.Warnf("Cannot delete workspace '%s' - invalid or default workspace", workspaceName)
		return nil
	}

	logger.Infof("Deleting workspace: %s", workspaceName)

	// Get workspace ID first
	workspaceID, err := p.getWorkspaceID(workspaceName)
	if err != nil {
		return fmt.Errorf("failed to get workspace ID for %s: %w", workspaceName, err)
	}

	if p.dryRun {
		logger.Warnf("DRY-RUN: Would delete workspace %s (ID: %s) and all child resources", workspaceName, workspaceID)
		logger.Warnf("DRY-RUN: Would also remove group-role assignments for workspace %s; groups that become empty would be deleted", workspaceName)
		return nil
	}

	// Delete the workspace using cascade=true (Kong Gateway 3.4.0+).
	// This removes the workspace and all its child resources — services, routes, plugins,
	// RBAC roles/users, Dev Portal files, etc. — in a single atomic API call.
	// Cascade delete is performed first so that group-role mappings are only removed
	// after the workspace is confirmed deleted; this prevents partial state where the
	// workspace still exists but its group mappings have already been stripped.
	workspacePath := fmt.Sprintf("/workspaces/%s", workspaceID)
	cascadeParams := url.Values{}
	cascadeParams.Set("cascade", "true")

	resp, err := p.client.DELETEWithParams(workspacePath, cascadeParams)
	if err != nil {
		return fmt.Errorf(
			"failed to delete workspace %s (path: %s): %w -- "+
				"cascade=true requires Kong Gateway 3.4.0+; verify your Kong version and that the workspace exists",
			workspaceName, workspacePath, err,
		)
	}
	defer func() { _ = resp.Body.Close() }()

	// Remove group-role assignments only after the workspace is successfully deleted.
	// Groups are global in Kong — a group can hold roles across many workspaces.
	// We remove only the role mappings that belonged to this workspace.
	// If a group ends up with no remaining role assignments it is deleted automatically.
	// Groups that still have roles in other workspaces are left untouched.
	logger.Infof("Removing group-role assignments for deleted workspace %s", workspaceName)
	if err := p.removeGroupRoleAssignmentsForWorkspace(workspaceName, workspaceID); err != nil {
		logger.Errorf("Failed to remove group-role assignments: %v", err)
		// Workspace is already deleted — log and continue rather than returning an error.
	}

	logger.Infof("Successfully deleted workspace: %s", workspaceName)
	return nil
}

// removeGroupRoleAssignmentsForWorkspace removes all group-role assignments that reference the given workspace
// The actual API response structure is:
// GET /groups/{group_id}/roles returns:
//
//	{
//	  "data": [
//	    {
//	      "workspace": { "id": "workspace-id" },
//	      "rbac_role": { "name": "role-name", "id": "role-id" },
//	      "group": { "id": "group-id", ... }
//	    }
//	  ]
//	}
//
// To delete a role: DELETE /groups/{group_id}/roles?workspace_id=xxx&rbac_role_id=yyy
func (p *Processor) removeGroupRoleAssignmentsForWorkspace(workspaceName string, workspaceID string) error {
	// Get all groups with pagination support
	var allGroups []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	pageSize := 100
	offset := ""

	for {
		var result struct {
			Data []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"data"`
			Offset string `json:"offset"`
		}

		params := url.Values{}
		params.Add("size", strconv.Itoa(pageSize))
		if offset != "" {
			params.Add("offset", offset)
		}

		if err := p.client.GetJSON("/groups", params, &result); err != nil {
			logger.Warnf("Failed to get groups: %v", err)
			return nil // Don't fail if we can't get groups
		}

		if len(result.Data) == 0 {
			break
		}

		allGroups = append(allGroups, result.Data...)

		if result.Offset == "" || len(result.Data) < pageSize {
			break
		}

		offset = result.Offset
	}

	if len(allGroups) == 0 {
		logger.Debugf("No groups found, nothing to clean up")
		return nil
	}

	logger.Infof("Checking %d groups for role assignments in workspace %s", len(allGroups), workspaceName)

	var removedCount int

	// Check each group for role assignments that reference this workspace
	for _, group := range allGroups {
		rolesPath := fmt.Sprintf("/groups/%s/roles", group.ID)
		var rolesResult struct {
			Data []map[string]interface{} `json:"data"`
		}

		if err := p.client.GetJSON(rolesPath, nil, &rolesResult); err != nil {
			logger.Debugf("Failed to get roles for group %s: %v", group.Name, err)
			continue
		}

		if len(rolesResult.Data) == 0 {
			continue
		}

		totalRoles := len(rolesResult.Data)
		var rolesRemovedFromGroup int

		// Delete any role assignments that reference this workspace
		for _, roleAssignment := range rolesResult.Data {
			ws, ok := roleAssignment["workspace"].(map[string]interface{})
			if !ok {
				continue
			}
			wsID, ok := ws["id"].(string)
			if !ok || wsID != workspaceID {
				continue
			}

			rbacRole, ok := roleAssignment["rbac_role"].(map[string]interface{})
			if !ok {
				continue
			}
			roleID, ok := rbacRole["id"].(string)
			if !ok {
				continue
			}

			params := url.Values{}
			params.Add("workspace_id", workspaceID)
			params.Add("rbac_role_id", roleID)

			if _, err := p.client.DELETEWithParams(rolesPath, params); err != nil {
				logger.Errorf("Failed to delete role assignment from group %s: %v", group.Name, err)
				continue
			}
			logger.Infof("Removed role assignment from group '%s' (workspace: %s)", group.Name, workspaceName)
			removedCount++
			rolesRemovedFromGroup++
		}

		if rolesRemovedFromGroup == 0 {
			continue
		}

		remainingRoles := totalRoles - rolesRemovedFromGroup
		if remainingRoles == 0 {
			// All role assignments belonged to this workspace — group is now empty, delete it
			logger.Infof("Group '%s' has no remaining role assignments after workspace %s removal — deleting group", group.Name, workspaceName)
			deletePath := fmt.Sprintf("/groups/%s", group.ID)
			if _, err := p.client.DELETE(deletePath); err != nil {
				logger.Errorf("Failed to delete empty group '%s': %v", group.Name, err)
			} else {
				logger.Infof("Deleted empty group '%s'", group.Name)
			}
		} else {
			// Group still has role mappings in other workspaces — leave it
			logger.Infof("Group '%s' retains %d role assignment(s) in other workspace(s) — group preserved", group.Name, remainingRoles)
		}
	}

	if removedCount == 0 {
		logger.Debugf("No group-role assignments found for workspace %s", workspaceName)
	} else {
		logger.Infof("Removed %d group-role assignment(s) for workspace %s", removedCount, workspaceName)
	}

	return nil
}

// getWorkspaceID retrieves the ID of a workspace by name
func (p *Processor) getWorkspaceID(workspaceName string) (string, error) {
	pageSize := 1000
	offset := ""

	for {
		params := url.Values{}
		params.Add("size", strconv.Itoa(pageSize))
		if offset != "" {
			params.Add("offset", offset)
		}

		var result struct {
			Data []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"data"`
			Offset string `json:"offset"`
		}

		if err := p.client.GetJSON("/workspaces", params, &result); err != nil {
			return "", fmt.Errorf("failed to get workspaces: %w", err)
		}

		for _, ws := range result.Data {
			if ws.Name == workspaceName {
				return ws.ID, nil
			}
		}

		if result.Offset == "" || len(result.Data) < pageSize {
			break
		}
		offset = result.Offset
	}

	return "", fmt.Errorf("workspace %s not found", workspaceName)
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

// validateWorkspaceConfig validates a workspace configuration before applying
func (p *Processor) validateWorkspaceConfig(workspaceName string, wsConfig *models.WorkspaceConfig) error {
	// Validate RBAC roles if present
	if len(wsConfig.RBAC) > 0 {
		if err := validation.ValidateAllRoles(wsConfig.RBAC, workspaceName); err != nil {
			return fmt.Errorf("invalid RBAC roles in workspace %s: %w", workspaceName, err)
		}
	}

	return nil
}

// validateWorkspaceRBACUsers validates RBAC users configuration
func (p *Processor) validateWorkspaceRBACUsers(users []models.RBACUser, workspaceName string) error {
	// Validate each user
	for _, user := range users {
		if err := validation.ValidateRBACUserConfig(user); err != nil {
			return fmt.Errorf("invalid RBAC user in workspace %s: %w", workspaceName, err)
		}
	}

	return nil
}

// applyPluginsDeclarative manages plugins declaratively
// Creates desired plugins and deletes plugins not in the desired state
func (p *Processor) applyPluginsDeclarative(workspaceName string, desiredPlugins []models.Plugin) error {
	// Fetch existing plugins in the workspace
	existingPlugins, err := p.getWorkspacePlugins(workspaceName)
	if err != nil {
		// In dry-run mode with new workspaces, this error is expected (workspace doesn't exist in Kong yet)
		// Only log warning if not in dry-run mode
		if !p.dryRun {
			logger.Warnf("Failed to fetch existing plugins for workspace %s: %v. Proceeding with creation only.", workspaceName, err)
		} else {
			logger.Debugf("Failed to fetch existing plugins for workspace %s (expected in dry-run with new workspaces): %v", workspaceName, err)
		}
		existingPlugins = []map[string]interface{}{}
	}

	// Build map of desired plugin names
	desiredMap := make(map[string]models.Plugin)
	for _, plg := range desiredPlugins {
		desiredMap[plg.Name] = plg
	}

	// Delete plugins that are not in desired state
	for _, existingPlg := range existingPlugins {
		pluginName, ok := existingPlg["name"].(string)
		if !ok {
			continue
		}

		if _, exists := desiredMap[pluginName]; !exists {
			// Plugin is not in desired state, delete it
			if err := p.deletePlugin(workspaceName, pluginName); err != nil {
				logger.Warnf("Failed to delete plugin %s from workspace %s: %v", pluginName, workspaceName, err)
			}
		}
	}

	// Apply desired plugins
	if len(desiredPlugins) > 0 {
		if err := p.applyPlugins(workspaceName, desiredPlugins); err != nil {
			return err
		}
	}

	return nil
}

// getWorkspacePlugins fetches all plugins in a workspace with proper pagination
func (p *Processor) getWorkspacePlugins(workspaceName string) ([]map[string]interface{}, error) {
	path := fmt.Sprintf("/%s/plugins", workspaceName)
	pageSize := 1000
	offset := ""
	plugins := []map[string]interface{}{}

	for {
		params := url.Values{}
		params.Add("size", strconv.Itoa(pageSize))
		if offset != "" {
			params.Add("offset", offset)
		}

		var result struct {
			Data   []map[string]interface{} `json:"data"`
			Offset string                   `json:"offset"`
		}

		if err := p.client.GetJSON(path, params, &result); err != nil {
			return nil, fmt.Errorf("failed to fetch plugins: %w", err)
		}

		plugins = append(plugins, result.Data...)

		// Check if there are more pages
		if result.Offset == "" || len(result.Data) < pageSize {
			break
		}

		offset = result.Offset
	}

	return plugins, nil
}

// deletePlugin deletes a plugin from a workspace
func (p *Processor) deletePlugin(workspaceName, pluginName string) error {
	if p.dryRun {
		logger.Infof("[DRY-RUN] Would delete plugin %s from workspace %s", pluginName, workspaceName)
		return nil
	}

	// First, get the plugin ID
	path := fmt.Sprintf("/%s/plugins", workspaceName)
	var result map[string]interface{}

	if err := p.client.GetJSON(path, nil, &result); err != nil {
		return fmt.Errorf("failed to fetch plugins: %w", err)
	}

	// Find the plugin ID by name
	var pluginID string
	if data, ok := result["data"].([]interface{}); ok {
		for _, item := range data {
			if pluginMap, ok := item.(map[string]interface{}); ok {
				if name, ok := pluginMap["name"].(string); ok && name == pluginName {
					if id, ok := pluginMap["id"].(string); ok {
						pluginID = id
						break
					}
				}
			}
		}
	}

	if pluginID == "" {
		logger.Warnf("Could not find ID for plugin %s in workspace %s", pluginName, workspaceName)
		return nil
	}

	// Delete the plugin using its ID
	deletePath := fmt.Sprintf("/%s/plugins/%s", workspaceName, pluginID)
	if _, err := p.client.DELETE(deletePath); err != nil {
		return fmt.Errorf("failed to delete plugin %s: %w", pluginName, err)
	}

	logger.Infof("Plugin %s deleted from workspace %s", pluginName, workspaceName)
	return nil
}

// applyPlugins applies workspace plugins
func (p *Processor) applyPlugins(workspaceName string, plugins []models.Plugin) error {
	if len(plugins) == 0 {
		logger.Infof("No plugins to apply for workspace %s", workspaceName)
		return nil
	}

	logger.Infof("Applying %d plugins to workspace %s", len(plugins), workspaceName)

	// Process plugins with limited concurrency
	actualConcurrency := 5 // Fixed concurrency for plugin creation
	if len(plugins) < actualConcurrency {
		actualConcurrency = len(plugins)
	}

	semaphore := make(chan struct{}, actualConcurrency)
	var wg sync.WaitGroup
	errors := make(chan error, len(plugins))

	for _, plugin := range plugins {
		wg.Add(1)
		go func(plg models.Plugin) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			if err := p.applyPlugin(workspaceName, plg); err != nil {
				errors <- fmt.Errorf("plugin %s: %w", plg.Name, err)
			} else {
				errors <- nil
			}
		}(plugin)
	}

	wg.Wait()
	close(errors)

	var errorList []error
	for err := range errors {
		if err != nil {
			errorList = append(errorList, err)
		}
	}

	if len(errorList) > 0 {
		return fmt.Errorf("failed to apply %d plugin(s)", len(errorList))
	}

	return nil
}

// applyPlugin applies a single plugin to a workspace
func (p *Processor) applyPlugin(workspaceName string, plugin models.Plugin) error {
	if p.dryRun {
		logger.Infof("[DRY-RUN] Would create plugin %s in workspace %s", plugin.Name, workspaceName)
		return nil
	}

	logger.Infof("Attempting to create plugin %s in workspace %s", plugin.Name, workspaceName)

	// Create plugin in the workspace
	payload := map[string]interface{}{
		"name":   plugin.Name,
		"config": plugin.Config,
	}

	path := fmt.Sprintf("/%s/plugins", workspaceName)
	logger.Debugf("Creating plugin at path: %s with payload: %+v", path, payload)
	var result map[string]interface{}
	if err := p.client.PostJSON(path, payload, &result); err != nil {
		// If plugin already exists (409 UNIQUE violation), just log it as info and continue
		errStr := err.Error()
		if strings.Contains(errStr, "409") || strings.Contains(errStr, "UNIQUE violation") || strings.Contains(errStr, "already exists") {
			logger.Infof("Plugin %s already exists in workspace %s", plugin.Name, workspaceName)
			return nil
		}
		logger.Errorf("Failed to create plugin %s in workspace %s: %v", plugin.Name, workspaceName, err)
		return fmt.Errorf("failed to create plugin %s: %w", plugin.Name, err)
	}

	logger.Infof("Plugin %s created in workspace %s", plugin.Name, workspaceName)
	return nil
}
