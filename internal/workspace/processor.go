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
	maxAttempts := getMaxRetryAttempts()
	if err := p.waitForWorkspaceAvailable(name, maxAttempts); err != nil {
		return false, err
	}

	return false, nil
}

// waitForWorkspaceAvailable waits for a workspace to become available with configurable retry attempts
func (p *Processor) waitForWorkspaceAvailable(name string, maxAttempts int) error {
	path := fmt.Sprintf("/workspaces/%s", name)

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		var result map[string]interface{}
		if err := p.client.GetJSON(path, nil, &result); err == nil {
			logger.Debugf("Workspace %s is available (attempt %d/%d)", name, attempt, maxAttempts)
			return nil // Workspace is available
		}

		if attempt < maxAttempts {
			// Exponential backoff: 50ms, 100ms, 150ms, 200ms, 250ms
			backoff := time.Duration(attempt*50) * time.Millisecond
			time.Sleep(backoff)
		}
	}

	return fmt.Errorf("workspace %s was created but is not available after %d attempts", name, maxAttempts)
}

// getMaxRetryAttempts retrieves the maximum retry attempts from environment variable or returns default (5)
func getMaxRetryAttempts() int {
	maxAttempts := 5
	if envAttempts := os.Getenv("MAX_RETRY_ATTEMPTS"); envAttempts != "" {
		if attempts, err := strconv.Atoi(envAttempts); err == nil && attempts > 0 {
			maxAttempts = attempts
		}
	}
	return maxAttempts
}

// waitForRBACUserAvailable waits for an RBAC user to become available with configurable retry attempts
func (p *Processor) waitForRBACUserAvailable(workspaceName, userName string, maxAttempts int) error {
	path := fmt.Sprintf("/%s/rbac/users/%s", workspaceName, userName)

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		var result map[string]interface{}
		if err := p.client.GetJSON(path, nil, &result); err == nil {
			logger.Debugf("RBAC user %s is available (attempt %d/%d)", userName, attempt, maxAttempts)
			return nil // RBAC user is available
		}

		if attempt < maxAttempts {
			// Exponential backoff: 50ms, 100ms, 150ms, 200ms, 250ms
			backoff := time.Duration(attempt*50) * time.Millisecond
			time.Sleep(backoff)
		}
	}

	return fmt.Errorf("RBAC user %s was created but is not available after %d attempts", userName, maxAttempts)
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
// Matches Node.js approach: just POST and handle 409 conflict gracefully
func (p *Processor) createOrUpdateRBACUser(workspaceName string, user models.RBACUser) error {
	// Create user with UUID token
	// Token is randomly generated (UUID) and NOT stored in config or logs
	userToken := uuid.New().String()

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
			logger.Warnf("RBAC user '%s' already exists in workspace '%s'", user.Name, workspaceName)
			return nil
		}
		return fmt.Errorf("failed to create RBAC user: %w", err)
	}

	logger.Infof("RBAC user '%s' created in workspace '%s'", user.Name, workspaceName)

	// Verify RBAC user is available before assigning roles (with configurable retries)
	maxAttempts := getMaxRetryAttempts()
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
		return nil
	}

	// Step 0: Remove group-role assignments that reference this workspace
	logger.Infof("Step 0: Removing group-role assignments for workspace %s", workspaceName)
	if err := p.removeGroupRoleAssignmentsForWorkspace(workspaceName, workspaceID); err != nil {
		logger.Errorf("Failed to remove group-role assignments: %v", err)
		// Don't fail the deletion, just log and continue
	}

	// Delete all child resources in the correct order based on dependencies

	// 1. Delete all ACLs
	logger.Infof("Step 1: Deleting ACLs from workspace %s", workspaceName)
	if err := p.deleteAllWorkspaceACLs(workspaceName); err != nil {
		logger.Errorf("Failed to delete ACLs in workspace %s: %v", workspaceName, err)
	}

	// 2. Delete all credential types (basic-auth, hmac-auth, key-auth, jwt, oauth2, etc.)
	logger.Infof("Step 2: Deleting basic-auth credentials from workspace %s", workspaceName)
	if err := p.deleteAllWorkspaceCredentials(workspaceName, "basic-auths"); err != nil {
		logger.Errorf("Failed to delete basic-auth credentials in workspace %s: %v", workspaceName, err)
	}

	logger.Infof("Step 2b: Deleting hmac-auth credentials from workspace %s", workspaceName)
	if err := p.deleteAllWorkspaceCredentials(workspaceName, "hmac-auths"); err != nil {
		logger.Errorf("Failed to delete hmac-auth credentials in workspace %s: %v", workspaceName, err)
	}

	logger.Infof("Step 2c: Deleting key-auth credentials from workspace %s", workspaceName)
	if err := p.deleteAllWorkspaceCredentials(workspaceName, "key-auths"); err != nil {
		logger.Errorf("Failed to delete key-auth credentials in workspace %s: %v", workspaceName, err)
	}

	logger.Infof("Step 2d: Deleting JWT credentials from workspace %s", workspaceName)
	if err := p.deleteAllWorkspaceCredentials(workspaceName, "jwts"); err != nil {
		logger.Errorf("Failed to delete JWT credentials in workspace %s: %v", workspaceName, err)
	}

	logger.Infof("Step 2e: Deleting oauth2 credentials from workspace %s", workspaceName)
	if err := p.deleteAllWorkspaceCredentials(workspaceName, "oauth2"); err != nil {
		logger.Errorf("Failed to delete oauth2 credentials in workspace %s: %v", workspaceName, err)
	}

	logger.Infof("Step 2f: Deleting mtls-auth credentials from workspace %s", workspaceName)
	if err := p.deleteAllWorkspaceCredentials(workspaceName, "mtls-auths"); err != nil {
		logger.Errorf("Failed to delete mtls-auth credentials in workspace %s: %v", workspaceName, err)
	}

	// 3. Delete all services (which cascade to routes)
	logger.Infof("Step 3: Deleting services from workspace %s", workspaceName)
	if err := p.deleteAllWorkspaceServices(workspaceName); err != nil {
		logger.Errorf("Failed to delete services in workspace %s: %v", workspaceName, err)
	}

	// 4. Delete all routes (in case any are orphaned)
	logger.Infof("Step 4: Deleting routes from workspace %s", workspaceName)
	if err := p.deleteAllWorkspaceRoutes(workspaceName); err != nil {
		logger.Errorf("Failed to delete routes in workspace %s: %v", workspaceName, err)
	}

	// 5. Delete all consumers
	logger.Infof("Step 5: Deleting consumers from workspace %s", workspaceName)
	if err := p.deleteAllWorkspaceConsumers(workspaceName); err != nil {
		logger.Errorf("Failed to delete consumers in workspace %s: %v", workspaceName, err)
	}

	// 6. Delete all consumer groups
	logger.Infof("Step 6: Deleting consumer groups from workspace %s", workspaceName)
	if err := p.deleteAllWorkspaceConsumerGroups(workspaceName); err != nil {
		logger.Errorf("Failed to delete consumer groups in workspace %s: %v", workspaceName, err)
	}

	// 7. Delete all upstreams
	logger.Infof("Step 7: Deleting upstreams from workspace %s", workspaceName)
	if err := p.deleteAllWorkspaceUpstreams(workspaceName); err != nil {
		logger.Errorf("Failed to delete upstreams in workspace %s: %v", workspaceName, err)
	}

	// 8. Delete all certificates
	logger.Infof("Step 8: Deleting certificates from workspace %s", workspaceName)
	if err := p.deleteAllWorkspaceCertificates(workspaceName); err != nil {
		logger.Errorf("Failed to delete certificates in workspace %s: %v", workspaceName, err)
	}

	// 9. Delete all CA certificates
	logger.Infof("Step 9: Deleting CA certificates from workspace %s", workspaceName)
	if err := p.deleteAllWorkspaceCACertificates(workspaceName); err != nil {
		logger.Errorf("Failed to delete CA certificates in workspace %s: %v", workspaceName, err)
	}

	// 10. Delete all SNIs
	logger.Infof("Step 10: Deleting SNIs from workspace %s", workspaceName)
	if err := p.deleteAllWorkspaceSNIs(workspaceName); err != nil {
		logger.Errorf("Failed to delete SNIs in workspace %s: %v", workspaceName, err)
	}

	// 11. Delete all vaults
	logger.Infof("Step 11: Deleting vaults from workspace %s", workspaceName)
	if err := p.deleteAllWorkspaceVaults(workspaceName); err != nil {
		logger.Errorf("Failed to delete vaults in workspace %s: %v", workspaceName, err)
	}

	// 12. Delete all plugins in the workspace
	logger.Infof("Step 12: Deleting plugins from workspace %s", workspaceName)
	if err := p.deleteAllWorkspacePlugins(workspaceName); err != nil {
		logger.Errorf("Failed to delete plugins in workspace %s: %v", workspaceName, err)
	}

	// 13. Delete all roles in the workspace
	logger.Infof("Step 13: Deleting roles from workspace %s", workspaceName)
	if err := p.deleteWorkspaceRoles(workspaceName); err != nil {
		logger.Errorf("Failed to delete roles in workspace %s: %v", workspaceName, err)
	}

	// 14. Delete all users in the workspace
	logger.Infof("Step 14: Deleting users from workspace %s", workspaceName)
	if err := p.deleteWorkspaceUsers(workspaceName); err != nil {
		logger.Errorf("Failed to delete users in workspace %s: %v", workspaceName, err)
	}

	// 15. Delete all RBAC users in the workspace (workspace-scoped users)
	logger.Infof("Step 15: Deleting RBAC users from workspace %s", workspaceName)
	if err := p.deleteAllWorkspaceRBACUsers(workspaceName); err != nil {
		logger.Errorf("Failed to delete RBAC users in workspace %s: %v", workspaceName, err)
	}

	// Note: Do NOT auto-cleanup group role assignments
	// Users must manually update groups-and-roles.yaml to maintain config integrity
	logger.Warnf("Remember to remove references to workspace %s from groups-and-roles.yaml", workspaceName)

	// Finally, delete the workspace itself using ID
	path := fmt.Sprintf("/workspaces/%s", workspaceID)

	resp, err := p.client.DELETE(path)
	if err != nil {
		// Try to provide more helpful error message
		return fmt.Errorf("failed to delete workspace %s: %w\n  Make sure all child resources (services, routes, etc.) have been deleted first", workspaceName, err)
	}
	defer func() { _ = resp.Body.Close() }()

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
		// Get all roles assigned to this group
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

		// Delete any role assignments that reference this workspace
		for _, roleAssignment := range rolesResult.Data {
			// Extract workspace ID from nested object
			ws, ok := roleAssignment["workspace"].(map[string]interface{})
			if !ok {
				continue
			}

			wsID, ok := ws["id"].(string)
			if !ok || wsID != workspaceID {
				continue
			}

			// Extract RBAC role ID from nested object
			rbacRole, ok := roleAssignment["rbac_role"].(map[string]interface{})
			if !ok {
				continue
			}

			roleID, ok := rbacRole["id"].(string)
			if !ok {
				continue
			}

			// Delete the role assignment using query parameters
			// No ID field exists, use workspace_id and rbac_role_id as identifiers
			params := url.Values{}
			params.Add("workspace_id", workspaceID)
			params.Add("rbac_role_id", roleID)

			rolesPath := fmt.Sprintf("/groups/%s/roles", group.ID)
			if _, err := p.client.DELETEWithParams(rolesPath, params); err != nil {
				logger.Errorf("Failed to delete role assignment from group %s: %v", group.Name, err)
				continue
			}
			logger.Infof("Removed role assignment from group %s (workspace: %s)", group.Name, workspaceName)
			removedCount++
		}
	}

	if removedCount == 0 {
		logger.Debugf("No group-role assignments found for workspace %s", workspaceName)
	} else {
		logger.Infof("Removed %d group-role assignments for workspace %s", removedCount, workspaceName)
	}

	return nil
}

// getWorkspaceID retrieves the ID of a workspace by name
func (p *Processor) getWorkspaceID(workspaceName string) (string, error) {
	var result struct {
		Data []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}

	if err := p.client.GetJSON("/workspaces", nil, &result); err != nil {
		return "", fmt.Errorf("failed to get workspaces: %w", err)
	}

	for _, ws := range result.Data {
		if ws.Name == workspaceName {
			return ws.ID, nil
		}
	}

	return "", fmt.Errorf("workspace %s not found", workspaceName)
}

// deleteWorkspaceRoles deletes all roles in a workspace with pagination
func (p *Processor) deleteWorkspaceRoles(workspaceName string) error {
	path := fmt.Sprintf("/%s/rbac/roles", workspaceName)
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

		if err := p.client.GetJSON(path, params, &result); err != nil {
			return fmt.Errorf("failed to get roles in workspace %s: %w", workspaceName, err)
		}

		// Delete each role
		for _, role := range result.Data {
			rolePath := fmt.Sprintf("/%s/rbac/roles/%s", workspaceName, role.ID)
			resp, err := p.client.DELETE(rolePath)
			if err != nil {
				logger.Errorf("Failed to delete role %s: %v", role.Name, err)
				continue
			}
			_ = resp.Body.Close()
			logger.Infof("Deleted role %s from workspace %s", role.Name, workspaceName)
		}

		// Check if there are more pages
		if result.Offset == "" || len(result.Data) < pageSize {
			break
		}

		offset = result.Offset
	}

	return nil
}

// deleteWorkspaceUsers deletes all users in a workspace with pagination
func (p *Processor) deleteWorkspaceUsers(workspaceName string) error {
	// Get list of users in the workspace
	path := fmt.Sprintf("/%s/admins", workspaceName)
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

		if err := p.client.GetJSON(path, params, &result); err != nil {
			return fmt.Errorf("failed to get users in workspace %s: %w", workspaceName, err)
		}

		// Delete each user
		for _, user := range result.Data {
			if p.dryRun {
				logger.Warnf("DRY-RUN: Would delete user %s from workspace %s", user.Name, workspaceName)
				continue
			}

			userPath := fmt.Sprintf("/%s/admins/%s", workspaceName, user.ID)
			resp, err := p.client.DELETE(userPath)
			if err != nil {
				logger.Errorf("Failed to delete user %s: %v", user.Name, err)
				continue
			}
			_ = resp.Body.Close()
			logger.Infof("Deleted user %s from workspace %s", user.Name, workspaceName)
		}

		// Check if there are more pages
		if result.Offset == "" || len(result.Data) < pageSize {
			break
		}

		offset = result.Offset
	}

	return nil
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

// deleteAllWorkspacePlugins deletes all plugins from a workspace
func (p *Processor) deleteAllWorkspacePlugins(workspaceName string) error {
	// Get all plugins in the workspace
	plugins, err := p.getWorkspacePlugins(workspaceName)
	if err != nil {
		logger.Warnf("Failed to fetch plugins from workspace %s: %v", workspaceName, err)
		return nil // Don't fail the deletion if we can't fetch plugins
	}

	if len(plugins) == 0 {
		logger.Debugf("No plugins found in workspace %s", workspaceName)
		return nil
	}

	logger.Infof("Deleting %d plugins from workspace %s", len(plugins), workspaceName)

	// Delete each plugin
	for _, plugin := range plugins {
		// Get plugin name and ID
		pluginName := "unknown"
		if name, ok := plugin["name"].(string); ok {
			pluginName = name
		}

		pluginID := ""
		if id, ok := plugin["id"].(string); ok {
			pluginID = id
		}

		if pluginID == "" {
			logger.Warnf("Could not find ID for plugin %s in workspace %s", pluginName, workspaceName)
			continue
		}

		// Delete the plugin
		if p.dryRun {
			logger.Infof("[DRY-RUN] Would delete plugin %s (%s) from workspace %s", pluginName, pluginID, workspaceName)
		} else {
			path := fmt.Sprintf("/%s/plugins/%s", workspaceName, pluginID)
			if _, err := p.client.DELETE(path); err != nil {
				logger.Errorf("Failed to delete plugin %s (%s): %v", pluginName, pluginID, err)
				continue
			}
			logger.Infof("Deleted plugin %s from workspace %s", pluginName, workspaceName)
		}
	}

	return nil
}

// deleteAllWorkspaceServices deletes all services from a workspace (cascade deletes routes)
func (p *Processor) deleteAllWorkspaceServices(workspaceName string) error {
	path := fmt.Sprintf("/%s/services", workspaceName)
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

		if err := p.client.GetJSON(path, params, &result); err != nil {
			logger.Warnf("Failed to get services in workspace %s: %v", workspaceName, err)
			return nil // Don't fail the deletion if we can't fetch services
		}

		if len(result.Data) == 0 {
			logger.Debugf("No services found in workspace %s", workspaceName)
			break
		}

		logger.Infof("Deleting %d services from workspace %s", len(result.Data), workspaceName)

		// Delete each service
		for _, service := range result.Data {
			servicePath := fmt.Sprintf("/%s/services/%s", workspaceName, service.ID)
			if _, err := p.client.DELETE(servicePath); err != nil {
				logger.Errorf("Failed to delete service %s: %v", service.Name, err)
				continue
			}
			logger.Infof("Deleted service %s from workspace %s", service.Name, workspaceName)
		}

		// Check if there are more pages
		if result.Offset == "" || len(result.Data) < pageSize {
			break
		}

		offset = result.Offset
	}

	return nil
}

// deleteAllWorkspaceRoutes deletes all routes from a workspace
func (p *Processor) deleteAllWorkspaceRoutes(workspaceName string) error {
	path := fmt.Sprintf("/%s/routes", workspaceName)
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

		if err := p.client.GetJSON(path, params, &result); err != nil {
			logger.Warnf("Failed to get routes in workspace %s: %v", workspaceName, err)
			return nil // Don't fail the deletion if we can't fetch routes
		}

		if len(result.Data) == 0 {
			logger.Debugf("No routes found in workspace %s", workspaceName)
			break
		}

		logger.Infof("Deleting %d routes from workspace %s", len(result.Data), workspaceName)

		// Delete each route
		for _, route := range result.Data {
			routePath := fmt.Sprintf("/%s/routes/%s", workspaceName, route.ID)
			if _, err := p.client.DELETE(routePath); err != nil {
				logger.Errorf("Failed to delete route %s: %v", route.Name, err)
				continue
			}
			logger.Infof("Deleted route %s from workspace %s", route.Name, workspaceName)
		}

		// Check if there are more pages
		if result.Offset == "" || len(result.Data) < pageSize {
			break
		}

		offset = result.Offset
	}

	return nil
}

// deleteAllWorkspaceConsumers deletes all consumers from a workspace
func (p *Processor) deleteAllWorkspaceConsumers(workspaceName string) error {
	path := fmt.Sprintf("/%s/consumers", workspaceName)
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
				ID       string `json:"id"`
				Username string `json:"username"`
			} `json:"data"`
			Offset string `json:"offset"`
		}

		if err := p.client.GetJSON(path, params, &result); err != nil {
			logger.Warnf("Failed to get consumers in workspace %s: %v", workspaceName, err)
			return nil // Don't fail the deletion if we can't fetch consumers
		}

		if len(result.Data) == 0 {
			logger.Debugf("No consumers found in workspace %s", workspaceName)
			break
		}

		logger.Infof("Deleting %d consumers from workspace %s", len(result.Data), workspaceName)

		// Delete each consumer
		for _, consumer := range result.Data {
			consumerPath := fmt.Sprintf("/%s/consumers/%s", workspaceName, consumer.ID)
			if _, err := p.client.DELETE(consumerPath); err != nil {
				logger.Errorf("Failed to delete consumer %s: %v", consumer.Username, err)
				continue
			}
			logger.Infof("Deleted consumer %s from workspace %s", consumer.Username, workspaceName)
		}

		// Check if there are more pages
		if result.Offset == "" || len(result.Data) < pageSize {
			break
		}

		offset = result.Offset
	}

	return nil
}

// deleteAllWorkspaceConsumerGroups deletes all consumer groups from a workspace
func (p *Processor) deleteAllWorkspaceConsumerGroups(workspaceName string) error {
	path := fmt.Sprintf("/%s/consumer_groups", workspaceName)
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

		if err := p.client.GetJSON(path, params, &result); err != nil {
			logger.Debugf("No consumer groups endpoint or failed to get consumer groups in workspace %s: %v", workspaceName, err)
			return nil // Don't fail if endpoint doesn't exist or we can't fetch
		}

		if len(result.Data) == 0 {
			logger.Debugf("No consumer groups found in workspace %s", workspaceName)
			break
		}

		logger.Infof("Deleting %d consumer groups from workspace %s", len(result.Data), workspaceName)

		// Delete each consumer group
		for _, cg := range result.Data {
			cgPath := fmt.Sprintf("/%s/consumer_groups/%s", workspaceName, cg.ID)
			if _, err := p.client.DELETE(cgPath); err != nil {
				logger.Errorf("Failed to delete consumer group %s: %v", cg.Name, err)
				continue
			}
			logger.Infof("Deleted consumer group %s from workspace %s", cg.Name, workspaceName)
		}

		// Check if there are more pages
		if result.Offset == "" || len(result.Data) < pageSize {
			break
		}

		offset = result.Offset
	}

	return nil
}

// deleteAllWorkspaceACLs deletes all ACLs from a workspace
func (p *Processor) deleteAllWorkspaceACLs(workspaceName string) error {
	path := fmt.Sprintf("/%s/acls", workspaceName)
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

		if err := p.client.GetJSON(path, params, &result); err != nil {
			logger.Debugf("No ACLs found or failed to get ACLs in workspace %s: %v", workspaceName, err)
			return nil
		}

		if len(result.Data) == 0 {
			logger.Debugf("No ACLs found in workspace %s", workspaceName)
			break
		}

		logger.Infof("Deleting %d ACLs from workspace %s", len(result.Data), workspaceName)

		for _, acl := range result.Data {
			aclPath := fmt.Sprintf("/%s/acls/%s", workspaceName, acl.ID)
			if _, err := p.client.DELETE(aclPath); err != nil {
				logger.Errorf("Failed to delete ACL %s: %v", acl.Name, err)
				continue
			}
			logger.Infof("Deleted ACL %s from workspace %s", acl.Name, workspaceName)
		}

		if result.Offset == "" || len(result.Data) < pageSize {
			break
		}

		offset = result.Offset
	}

	return nil
}

// deleteAllWorkspaceCredentials deletes all credentials of a specific type from a workspace
func (p *Processor) deleteAllWorkspaceCredentials(workspaceName, credentialType string) error {
	path := fmt.Sprintf("/%s/%s", workspaceName, credentialType)
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

		if err := p.client.GetJSON(path, params, &result); err != nil {
			logger.Debugf("No %s credentials found or failed to get credentials in workspace %s: %v", credentialType, workspaceName, err)
			return nil
		}

		if len(result.Data) == 0 {
			logger.Debugf("No %s credentials found in workspace %s", credentialType, workspaceName)
			break
		}

		logger.Infof("Deleting %d %s credentials from workspace %s", len(result.Data), credentialType, workspaceName)

		for _, cred := range result.Data {
			credPath := fmt.Sprintf("/%s/%s/%s", workspaceName, credentialType, cred.ID)
			if _, err := p.client.DELETE(credPath); err != nil {
				logger.Errorf("Failed to delete %s credential %s: %v", credentialType, cred.Name, err)
				continue
			}
			logger.Infof("Deleted %s credential %s from workspace %s", credentialType, cred.Name, workspaceName)
		}

		if result.Offset == "" || len(result.Data) < pageSize {
			break
		}

		offset = result.Offset
	}

	return nil
}

// deleteAllWorkspaceSNIs deletes all SNIs from a workspace
func (p *Processor) deleteAllWorkspaceSNIs(workspaceName string) error {
	path := fmt.Sprintf("/%s/snis", workspaceName)
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

		if err := p.client.GetJSON(path, params, &result); err != nil {
			logger.Debugf("No SNIs found or failed to get SNIs in workspace %s: %v", workspaceName, err)
			return nil
		}

		if len(result.Data) == 0 {
			logger.Debugf("No SNIs found in workspace %s", workspaceName)
			break
		}

		logger.Infof("Deleting %d SNIs from workspace %s", len(result.Data), workspaceName)

		for _, sni := range result.Data {
			sniPath := fmt.Sprintf("/%s/snis/%s", workspaceName, sni.ID)
			if _, err := p.client.DELETE(sniPath); err != nil {
				logger.Errorf("Failed to delete SNI %s: %v", sni.Name, err)
				continue
			}
			logger.Infof("Deleted SNI %s from workspace %s", sni.Name, workspaceName)
		}

		if result.Offset == "" || len(result.Data) < pageSize {
			break
		}

		offset = result.Offset
	}

	return nil
}

// deleteAllWorkspaceVaults deletes all vaults from a workspace
func (p *Processor) deleteAllWorkspaceVaults(workspaceName string) error {
	path := fmt.Sprintf("/%s/vaults", workspaceName)
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

		if err := p.client.GetJSON(path, params, &result); err != nil {
			logger.Debugf("No vaults found or failed to get vaults in workspace %s: %v", workspaceName, err)
			return nil
		}

		if len(result.Data) == 0 {
			logger.Debugf("No vaults found in workspace %s", workspaceName)
			break
		}

		logger.Infof("Deleting %d vaults from workspace %s", len(result.Data), workspaceName)

		for _, vault := range result.Data {
			vaultPath := fmt.Sprintf("/%s/vaults/%s", workspaceName, vault.ID)
			if _, err := p.client.DELETE(vaultPath); err != nil {
				logger.Errorf("Failed to delete vault %s: %v", vault.Name, err)
				continue
			}
			logger.Infof("Deleted vault %s from workspace %s", vault.Name, workspaceName)
		}

		if result.Offset == "" || len(result.Data) < pageSize {
			break
		}

		offset = result.Offset
	}

	return nil
}

// deleteAllWorkspaceRBACUsers deletes all RBAC users from a workspace
func (p *Processor) deleteAllWorkspaceRBACUsers(workspaceName string) error {
	path := fmt.Sprintf("/%s/rbac/users", workspaceName)
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

		if err := p.client.GetJSON(path, params, &result); err != nil {
			logger.Debugf("No RBAC users found or failed to get RBAC users in workspace %s: %v", workspaceName, err)
			return nil // Don't fail if endpoint doesn't exist or we can't fetch
		}

		if len(result.Data) == 0 {
			logger.Debugf("No RBAC users found in workspace %s", workspaceName)
			break
		}

		logger.Infof("Deleting %d RBAC users from workspace %s", len(result.Data), workspaceName)

		// Delete each RBAC user
		for _, user := range result.Data {
			userPath := fmt.Sprintf("/%s/rbac/users/%s", workspaceName, user.ID)
			if _, err := p.client.DELETE(userPath); err != nil {
				logger.Errorf("Failed to delete RBAC user %s: %v", user.Name, err)
				continue
			}
			logger.Infof("Deleted RBAC user %s from workspace %s", user.Name, workspaceName)
		}

		// Check if there are more pages
		if result.Offset == "" || len(result.Data) < pageSize {
			break
		}

		offset = result.Offset
	}

	return nil
}

// deleteAllWorkspaceUpstreams deletes all upstreams from a workspace
func (p *Processor) deleteAllWorkspaceUpstreams(workspaceName string) error {
	path := fmt.Sprintf("/%s/upstreams", workspaceName)
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

		if err := p.client.GetJSON(path, params, &result); err != nil {
			logger.Debugf("No upstreams endpoint or failed to get upstreams in workspace %s: %v", workspaceName, err)
			return nil // Don't fail if endpoint doesn't exist or we can't fetch
		}

		if len(result.Data) == 0 {
			logger.Debugf("No upstreams found in workspace %s", workspaceName)
			break
		}

		logger.Infof("Deleting %d upstreams from workspace %s", len(result.Data), workspaceName)

		// Delete each upstream
		for _, upstream := range result.Data {
			upstreamPath := fmt.Sprintf("/%s/upstreams/%s", workspaceName, upstream.ID)
			if _, err := p.client.DELETE(upstreamPath); err != nil {
				logger.Errorf("Failed to delete upstream %s: %v", upstream.Name, err)
				continue
			}
			logger.Infof("Deleted upstream %s from workspace %s", upstream.Name, workspaceName)
		}

		// Check if there are more pages
		if result.Offset == "" || len(result.Data) < pageSize {
			break
		}

		offset = result.Offset
	}

	return nil
}

// deleteAllWorkspaceCertificates deletes all certificates from a workspace
func (p *Processor) deleteAllWorkspaceCertificates(workspaceName string) error {
	path := fmt.Sprintf("/%s/certificates", workspaceName)
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
				Cert string `json:"cert"`
			} `json:"data"`
			Offset string `json:"offset"`
		}

		if err := p.client.GetJSON(path, params, &result); err != nil {
			logger.Debugf("No certificates endpoint or failed to get certificates in workspace %s: %v", workspaceName, err)
			return nil // Don't fail if endpoint doesn't exist or we can't fetch
		}

		if len(result.Data) == 0 {
			logger.Debugf("No certificates found in workspace %s", workspaceName)
			break
		}

		logger.Infof("Deleting %d certificates from workspace %s", len(result.Data), workspaceName)

		// Delete each certificate
		for _, cert := range result.Data {
			certPath := fmt.Sprintf("/%s/certificates/%s", workspaceName, cert.ID)
			if _, err := p.client.DELETE(certPath); err != nil {
				logger.Errorf("Failed to delete certificate %s: %v", cert.ID, err)
				continue
			}
			logger.Infof("Deleted certificate %s from workspace %s", cert.ID, workspaceName)
		}

		// Check if there are more pages
		if result.Offset == "" || len(result.Data) < pageSize {
			break
		}

		offset = result.Offset
	}

	return nil
}

// deleteAllWorkspaceCACertificates deletes all CA certificates from a workspace
func (p *Processor) deleteAllWorkspaceCACertificates(workspaceName string) error {
	path := fmt.Sprintf("/%s/ca_certificates", workspaceName)
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
				Cert string `json:"cert"`
			} `json:"data"`
			Offset string `json:"offset"`
		}

		if err := p.client.GetJSON(path, params, &result); err != nil {
			logger.Debugf("No CA certificates endpoint or failed to get CA certificates in workspace %s: %v", workspaceName, err)
			return nil // Don't fail if endpoint doesn't exist or we can't fetch
		}

		if len(result.Data) == 0 {
			logger.Debugf("No CA certificates found in workspace %s", workspaceName)
			break
		}

		logger.Infof("Deleting %d CA certificates from workspace %s", len(result.Data), workspaceName)

		// Delete each CA certificate
		for _, cert := range result.Data {
			certPath := fmt.Sprintf("/%s/ca_certificates/%s", workspaceName, cert.ID)
			if _, err := p.client.DELETE(certPath); err != nil {
				logger.Errorf("Failed to delete CA certificate %s: %v", cert.ID, err)
				continue
			}
			logger.Infof("Deleted CA certificate %s from workspace %s", cert.ID, workspaceName)
		}

		// Check if there are more pages
		if result.Offset == "" || len(result.Data) < pageSize {
			break
		}

		offset = result.Offset
	}

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
