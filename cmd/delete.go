package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Kong/kwot/internal/config"
	"github.com/Kong/kwot/internal/groups"
	"github.com/Kong/kwot/internal/kong"
	"github.com/Kong/kwot/internal/logger"
	"github.com/Kong/kwot/internal/roles"
	"github.com/Kong/kwot/internal/validation"
	"github.com/Kong/kwot/internal/workspace"
	"github.com/spf13/cobra"
)

var (
	deleteName string
)

var deleteCmd = &cobra.Command{
	Use:   "delete [command]",
	Short: "Delete Kong Gateway resources",
	Long: `Delete Kong Gateway resources (workspaces, roles, groups, or all).

This command requires the --force flag as a safety measure to prevent accidental deletion.

Resource types:
  all        - Delete ALL configured resources (groups, roles, workspaces) - REQUIRES FEATURE_DELETE_ALL_ENABLED=true
  workspaces - Delete a Kong workspace (also deletes its roles and RBAC users)
  roles      - Delete a specific role from a workspace (use -n flag for role name)
  groups     - Delete a Kong group or all groups (use -n flag for group name)

Examples:
  kwot delete --force                                      # Delete ALL resources (defaults to 'all', with 5s confirmation)
  kwot delete all --force                                  # Delete ALL resources (with 5s confirmation)
  kwot delete workspaces -n demo1 --force                    # Delete workspace
  kwot delete roles -n admin-role -w demo1 --force           # Delete role from workspace
  kwot delete roles -n admin-role --force                    # Delete role from all workspaces
  kwot delete groups -n admin-group --force                  # Delete specific group
	kwot delete groups --force                                 # Delete all groups`,
	Args: requireKnownSubcommand,
	RunE: func(cmd *cobra.Command, args []string) error {
		// If no subcommand is provided, default to 'delete all'
		if len(args) == 0 {
			return runDeleteAll(cmd, args)
		}
		_ = cmd.Help()
		return nil
	},
}

var deleteWorkspacesCmd = &cobra.Command{
	Use:   "workspaces [flags]",
	Short: "Delete Kong workspaces",
	Long: `Delete Kong workspaces and all associated resources.

REQUIREMENTS:
- FEATURE_FORCE_WIPE_WORKSPACE must be set to 'true' in .env
- --force flag is required as a safety measure
- -n/--name flag is required to specify the workspace to delete

This will delete:
- The workspace itself
- All roles defined in the workspace
- All RBAC users in the workspace

Examples:
  kwot delete workspaces -n demo1 --force                # Delete demo1 workspace
  kwot delete workspaces -n demo1 --dry-run --force      # Preview what would be deleted`,
	Args: requireNoArgs,
	RunE: runDeleteWorkspaces,
}

var deleteRolesCmd = &cobra.Command{
	Use:   "roles [flags]",
	Short: "Delete RBAC roles",
	Long: `Delete a specific RBAC role from a workspace.

The -n/--name flag specifies the role to delete.
The -w/--workspace flag specifies the target workspace (default: all).

Examples:
  kwot delete roles -n admin-role -w demo1 --force
  kwot delete roles -n viewer-role -w demo1 --dry-run --force
  kwot delete roles -n admin-role --force                 # Delete from all workspaces`,
	Args: cobra.NoArgs,
	RunE: runDeleteRoles,
}

var deleteGroupsCmd = &cobra.Command{
	Use:   "groups [flags]",
	Short: "Delete Kong groups",
	Long: `Delete Kong groups.

IMPORTANT: Groups are global resources in Kong and do not belong to specific workspaces.
The -w/--workspace flag is ignored for group operations.

Use -n/--name flag to delete a specific group, or omit it to delete all groups.

Examples:
  kwot delete groups -n admin-group --force                # Delete specific group
  kwot delete groups --force                               # Delete ALL groups (global)
  kwot delete groups -n developers --dry-run --force       # Preview deletion`,
	Args: cobra.NoArgs,
	RunE: runDeleteGroups,
}

var deleteAllCmd = &cobra.Command{
	Use:   "all [flags]",
	Short: "Delete ALL configured resources",
	Long: `Delete ALL configured resources (groups, roles, and workspaces) in reverse order.

⚠️  DANGER: This is a destructive operation that will delete everything configured in kwot.

REQUIREMENTS:
- FEATURE_DELETE_ALL_ENABLED must be set to 'true' in .env
- --force flag is required as a safety measure
- A 5-second countdown confirmation will be shown before deletion proceeds

Deletion order (reverse of apply):
  1. Groups - Delete all groups
  2. Roles - Delete all roles from all workspaces
  3. Workspaces - Delete all workspaces

Examples:
  kwot delete all --force                    # Delete all resources (with 5s confirmation)
  kwot delete all --dry-run --force          # Preview what would be deleted`,
	Args: requireNoArgs,
	RunE: runDeleteAll,
}

func init() {
	rootCmd.AddCommand(deleteCmd)

	// Add subcommands
	deleteCmd.AddCommand(deleteAllCmd)
	deleteCmd.AddCommand(deleteWorkspacesCmd)
	deleteCmd.AddCommand(deleteRolesCmd)
	deleteCmd.AddCommand(deleteGroupsCmd)

	// Add -n/--name flag to subcommands
	deleteWorkspacesCmd.Flags().StringVarP(&deleteName, "name", "n", "", "name of the resource to delete")

	deleteRolesCmd.Flags().StringVarP(&deleteName, "name", "n", "", "name of the role to delete")

	deleteGroupsCmd.Flags().StringVarP(&deleteName, "name", "n", "", "name of the group to delete")

	// Force flag is required
	deleteCmd.PersistentFlags().BoolVar(&force, "force", false, "required flag to confirm deletion (safety measure)")
}

// Delete workspaces
func runDeleteWorkspaces(cmd *cobra.Command, args []string) error {
	if deleteName == "" {
		return fmt.Errorf("-n/--name flag is required to specify the workspace to delete")
	}

	// Validate that we're not trying to delete the default workspace
	if err := validation.ValidateWorkspaceNameForDeletion(deleteName); err != nil {
		return err
	}

	// Get config
	cfg := config.GetConfig()

	// Check if workspace deletion is enabled
	if !cfg.FeatureForceWipeWorkspace {
		return fmt.Errorf("workspace deletion is disabled (FEATURE_FORCE_WIPE_WORKSPACE=false). Set FEATURE_FORCE_WIPE_WORKSPACE=true in .env to enable workspace deletion")
	}

	// Require --force flag for safety
	if !force {
		return fmt.Errorf("deletion requires --force flag to prevent accidents")
	}

	logger.Infof("Preparing to delete workspace '%s'...", deleteName)

	if dryRun {
		logger.Infof("[DRY-RUN] Would delete workspace '%s' and all its resources", deleteName)
		return nil
	}

	// Create Kong client
	client, err := kong.NewClient(cfg)
	if err != nil {
		return fmt.Errorf("failed to create Kong client: %w", err)
	}

	// Create workspace processor
	wsProcessor := workspace.NewProcessor(client, cfg, dryRun)

	// Delete the workspace
	if err := wsProcessor.DeleteWorkspace(deleteName); err != nil {
		return fmt.Errorf("failed to delete workspace '%s': %w", deleteName, err)
	}

	logger.Infof("✓ Workspace '%s' deleted successfully", deleteName)
	return nil
}

// Delete roles
func runDeleteRoles(cmd *cobra.Command, args []string) error {
	if deleteName == "" {
		return fmt.Errorf("-n/--name flag is required to specify the role to delete")
	}

	if !force {
		return fmt.Errorf("deletion requires --force flag to prevent accidents")
	}

	roleName := deleteName
	ws := workspaceName

	// If workspace is "all", delete role from all workspaces
	if ws == "all" {
		logger.Infof("Preparing to delete role '%s' from all workspaces...", roleName)

		if dryRun {
			logger.Infof("[DRY-RUN] Would delete role '%s' from all workspaces", roleName)
			return nil
		}

		// Get config and create Kong client
		cfg := config.GetConfig()
		client, err := kong.NewClient(cfg)
		if err != nil {
			return fmt.Errorf("failed to create Kong client: %w", err)
		}

		// Create roles processor
		rolesProcessor := roles.NewProcessor(client, cfg, dryRun)

		// Get all workspaces by scanning the config directory
		dirs, err := os.ReadDir(cfg.ConfigDir)
		if err != nil {
			return fmt.Errorf("failed to read config directory: %w", err)
		}

		var allWorkspaces []string
		for _, dir := range dirs {
			if dir.IsDir() && dir.Name() != "." && dir.Name() != ".." {
				allWorkspaces = append(allWorkspaces, dir.Name())
			}
		}

		var deleteErrors []error
		var successCount int
		var notFoundCount int
		for _, workspace := range allWorkspaces {
			if err := rolesProcessor.DeleteRole(workspace, roleName); err != nil {
				errMsg := err.Error()
				// If role doesn't exist, log it but don't count as error
				if strings.Contains(errMsg, "not found") {
					logger.Infof("Role '%s' does not exist in workspace '%s', skipping deletion", roleName, workspace)
					notFoundCount++
					continue
				}
				logger.Warnf("Failed to delete role '%s' from workspace '%s': %v", roleName, workspace, err)
				deleteErrors = append(deleteErrors, fmt.Errorf("workspace %s: %w", workspace, err))
			} else {
				logger.Infof("✓ Role '%s' deleted from workspace '%s'", roleName, workspace)
				successCount++
			}
		}

		if len(deleteErrors) > 0 {
			return fmt.Errorf("failed to delete role from %d workspace(s)", len(deleteErrors))
		}

		if successCount == 0 && notFoundCount == len(allWorkspaces) {
			logger.Warnf("Role '%s' was not found in any workspace", roleName)
		} else if successCount > 0 {
			logger.Infof("✓ Role '%s' deleted successfully from %d workspace(s)", roleName, successCount)
		}
		return nil
	}

	// Delete from specific workspace
	logger.Infof("Preparing to delete role '%s' from workspace '%s'...", roleName, ws)

	if dryRun {
		logger.Infof("[DRY-RUN] Would delete role '%s' from workspace '%s'", roleName, ws)
		return nil
	}

	// Get config and create Kong client
	cfg := config.GetConfig()
	client, err := kong.NewClient(cfg)
	if err != nil {
		return fmt.Errorf("failed to create Kong client: %w", err)
	}

	// Create roles processor
	rolesProcessor := roles.NewProcessor(client, cfg, dryRun)

	// Delete the role
	if err := rolesProcessor.DeleteRole(ws, roleName); err != nil {
		return fmt.Errorf("failed to delete role '%s' from workspace '%s': %w", roleName, ws, err)
	}

	logger.Infof("✓ Role '%s' deleted successfully from workspace '%s'", roleName, ws)
	return nil
}

// Delete groups
func runDeleteGroups(cmd *cobra.Command, args []string) error {
	if !force {
		return fmt.Errorf("deletion requires --force flag to prevent accidents")
	}

	// Get config and create Kong client
	cfg := config.GetConfig()
	client, err := kong.NewClient(cfg)
	if err != nil {
		return fmt.Errorf("failed to create Kong client: %w", err)
	}

	// Create groups processor
	groupsProcessor := groups.NewProcessor(client, cfg, dryRun)

	// If no group name provided via -n flag, delete all groups
	if deleteName == "" {
		logger.Infof("Preparing to delete all groups...")

		if dryRun {
			logger.Infof("[DRY-RUN] Would delete all groups")
			return nil
		}

		// Delete all groups
		if err := groupsProcessor.DeleteAllGroups(); err != nil {
			return fmt.Errorf("failed to delete groups: %w", err)
		}

		logger.Infof("✓ All groups deleted successfully")
		return nil
	}

	// Delete specific group
	groupName := deleteName
	logger.Infof("Preparing to delete group '%s'...", groupName)

	if dryRun {
		logger.Infof("[DRY-RUN] Would delete group '%s'", groupName)
		return nil
	}

	// Delete the group
	if err := groupsProcessor.DeleteGroup(groupName); err != nil {
		return fmt.Errorf("failed to delete group '%s': %w", groupName, err)
	}

	logger.Infof("✓ Group '%s' deleted successfully", groupName)
	return nil
}

// Delete all resources (opposite of apply all)
func runDeleteAll(cmd *cobra.Command, args []string) error {
	// Check if feature is enabled
	cfg := config.GetConfig()
	if !cfg.FeatureDeleteAllEnabled {
		return fmt.Errorf("delete all is disabled (FEATURE_DELETE_ALL_ENABLED=false). Set FEATURE_DELETE_ALL_ENABLED=true in .env to enable bulk deletion")
	}

	if !force {
		return fmt.Errorf("deletion requires --force flag to prevent accidents")
	}

	logger.Warnf("⚠️  WARNING: You are about to delete ALL configured resources!")
	logger.Warnf("This will delete in the following order:")
	logger.Warnf("  1. All groups")
	logger.Warnf("  2. All roles from all workspaces")
	logger.Warnf("  3. All workspaces")
	logger.Warnf("")

	if dryRun {
		logger.Infof("[DRY-RUN] Would delete all resources in the order above")
		return nil
	}

	// Show countdown confirmation
	logger.Infof("Starting 5-second countdown... Press Ctrl+C to cancel")
	for i := 5; i > 0; i-- {
		logger.Infof("Deleting in %d second(s)...", i)
		time.Sleep(1 * time.Second)
	}

	logger.Infof("🗑️  Proceeding with deletion...")

	// Create Kong client
	client, err := kong.NewClient(cfg)
	if err != nil {
		return fmt.Errorf("failed to create Kong client: %w", err)
	}

	// Initialize processors
	groupsProcessor := groups.NewProcessor(client, cfg, dryRun)
	rolesProcessor := roles.NewProcessor(client, cfg, dryRun)
	wsProcessor := workspace.NewProcessor(client, cfg, dryRun)

	// Step 1: Delete all groups
	logger.Infof("Step 1/3: Deleting all groups...")
	if err := groupsProcessor.DeleteAllGroups(); err != nil {
		return fmt.Errorf("failed to delete groups: %w", err)
	}
	logger.Infof("✓ All groups deleted")

	// Step 2: Delete all roles from all workspaces
	logger.Infof("Step 2/3: Deleting all roles from all workspaces...")
	dirs, err := os.ReadDir(cfg.ConfigDir)
	if err != nil {
		return fmt.Errorf("failed to read config directory: %w", err)
	}

	var allWorkspaces []string
	for _, dir := range dirs {
		if dir.IsDir() && dir.Name() != "." && dir.Name() != ".." {
			allWorkspaces = append(allWorkspaces, dir.Name())
		}
	}

	var roleDeleteErrors []error
	var roleDeleteCount int
	for _, wsName := range allWorkspaces {
		// Get all roles for this workspace
		rolesInWorkspace, err := rolesProcessor.GetAllRolesForWorkspace(wsName)
		if err != nil {
			logger.Warnf("Warning: Could not get roles for workspace '%s': %v", wsName, err)
			continue
		}

		for _, role := range rolesInWorkspace {
			if err := rolesProcessor.DeleteRole(wsName, role.Name); err != nil {
				if !strings.Contains(err.Error(), "not found") {
					logger.Warnf("Failed to delete role '%s' from workspace '%s': %v", role.Name, wsName, err)
					roleDeleteErrors = append(roleDeleteErrors, err)
					continue
				}
			}
			roleDeleteCount++
		}
	}

	if roleDeleteCount > 0 {
		logger.Infof("✓ Deleted %d roles from workspaces", roleDeleteCount)
	} else {
		logger.Infof("✓ No roles to delete")
	}

	if len(roleDeleteErrors) > 0 {
		return fmt.Errorf("encountered %d error(s) while deleting roles", len(roleDeleteErrors))
	}

	// Step 3: Delete all workspaces
	logger.Infof("Step 3/3: Deleting all workspaces...")
	var wsDeleteErrors []error
	var wsDeleteCount int
	for _, wsName := range allWorkspaces {
		if err := wsProcessor.DeleteWorkspace(wsName); err != nil {
			errMsg := err.Error()
			// Skip if workspace doesn't exist
			if strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "Workspace") {
				logger.Infof("Workspace '%s' does not exist in Kong, skipping", wsName)
				continue
			}
			logger.Warnf("Failed to delete workspace '%s': %v", wsName, err)
			wsDeleteErrors = append(wsDeleteErrors, err)
			continue
		}
		wsDeleteCount++
	}

	if wsDeleteCount > 0 {
		logger.Infof("✓ Deleted %d workspaces", wsDeleteCount)
	} else {
		logger.Infof("✓ No workspaces to delete")
	}

	if len(wsDeleteErrors) > 0 {
		return fmt.Errorf("encountered %d error(s) while deleting workspaces", len(wsDeleteErrors))
	}

	logger.Infof("✅ Successfully deleted all configured resources!")
	return nil
}
