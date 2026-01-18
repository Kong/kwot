package cmd

import (
	"fmt"
	"strings"

	"github.com/Kong/kwot/internal/config"
	"github.com/Kong/kwot/internal/groups"
	"github.com/Kong/kwot/internal/kong"
	"github.com/Kong/kwot/internal/logger"
	"github.com/Kong/kwot/internal/roles"
	"github.com/Kong/kwot/internal/workspace"
	"github.com/spf13/cobra"
)

var diffCmd = &cobra.Command{
	Use:   "diff [resource]",
	Short: "Show drift between Kong and configuration (what would change)",
	Long: `Show differences between your configuration files and what's currently deployed in Kong.
Resources:
  workspaces  - Show workspace drift
  roles       - Show role drift
  groups      - Show group drift
  all         - Show all drifts (default)

Examples:
  kwot diff                           # Show all drifts
  kwot diff workspaces                # Show workspace drift only
  kwot diff roles                     # Show role drift only
  kwot diff groups                    # Show group drift only`,
	RunE: runDiff,
	Args: cobra.MaximumNArgs(1),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return []string{"workspaces", "roles", "groups", "all"}, cobra.ShellCompDirectiveNoFileComp
	},
}

func init() {
	rootCmd.AddCommand(diffCmd)
}

// runDiff handles the diff command
func runDiff(cmd *cobra.Command, args []string) error {
	// Determine which resources to diff
	resource := "all"
	if len(args) > 0 {
		resource = strings.ToLower(args[0])
	}

	// Validate resource
	validResources := map[string]bool{"all": true, "workspaces": true, "roles": true, "groups": true}
	if !validResources[resource] {
		return fmt.Errorf("invalid resource '%s'. Valid options: all, workspaces, roles, groups", resource)
	}

	logger.Info("Starting drift detection (diff mode)...")

	// Load configuration
	cfg := config.GetConfig()

	// Create Kong client
	client, err := kong.NewClient(cfg)
	if err != nil {
		return fmt.Errorf("failed to create Kong client: %w", err)
	}

	// Test connectivity
	if err := client.Ping(); err != nil {
		return fmt.Errorf("failed to connect to Kong Admin API: %w", err)
	}

	logger.Info("Successfully connected to Kong Admin API")
	logger.Info("")

	// Create workspace processor for drift detection
	wsProcessor := workspace.NewProcessor(client, cfg, false)

	// Show diffs
	if resource == "all" || resource == "workspaces" {
		logger.Info("📋 Workspace Drift:")
		if err := compareWorkspaceDrift(wsProcessor); err != nil {
			logger.Warnf("Drift detection for workspaces encountered error: %v", err)
		}
		logger.Info("")
	}

	if resource == "all" || resource == "roles" {
		logger.Info("📋 Role Drift:")
		rolesProcessor := roles.NewProcessor(client, cfg, false)
		if err := compareRoleDrift(rolesProcessor, wsProcessor); err != nil {
			logger.Warnf("Drift detection for roles encountered error: %v", err)
		}
		logger.Info("")
	}

	if resource == "all" || resource == "groups" {
		logger.Info("📋 Group Drift:")
		groupsProcessor := groups.NewProcessor(client, cfg, false)
		if err := compareGroupDrift(groupsProcessor); err != nil {
			logger.Warnf("Drift detection for groups encountered error: %v", err)
		}
		logger.Info("")
	}

	logger.Info("Drift detection complete")
	logger.Info("To apply changes, run: kwot apply")

	return nil
}

// compareWorkspaceDrift compares configured workspaces with Kong to detect orphaned resources
func compareWorkspaceDrift(wsProcessor *workspace.Processor) error {
	// Get configured workspaces from filesystem
	configuredWorkspaces, err := wsProcessor.GetWorkspaceDirs()
	if err != nil {
		return fmt.Errorf("failed to get configured workspaces: %w", err)
	}

	// Get workspaces from Kong
	kongWorkspaces, err := wsProcessor.GetKongWorkspaces()
	if err != nil {
		return fmt.Errorf("failed to get Kong workspaces: %w", err)
	}

	// Build set of configured workspace names
	configSet := make(map[string]bool)
	for _, name := range configuredWorkspaces {
		configSet[name] = true
	}

	// Build set of Kong workspace names
	kongSet := make(map[string]bool)
	for _, name := range kongWorkspaces {
		kongSet[name] = true
	}

	// Find workspaces in config but not in Kong (missing resources)
	var missingInKong []string
	for _, name := range configuredWorkspaces {
		if !kongSet[name] && name != "default" {
			missingInKong = append(missingInKong, name)
		}
	}

	// Find workspaces in Kong but not in config (orphaned resources)
	var orphanedInKong []string
	for _, name := range kongWorkspaces {
		if !configSet[name] && name != "default" {
			orphanedInKong = append(orphanedInKong, name)
		}
	}

	// Report results
	if len(missingInKong) == 0 && len(orphanedInKong) == 0 {
		logger.Infof("✓ No drift detected - Kong is in sync with configuration")
		return nil
	}

	if len(missingInKong) > 0 {
		logger.Warnf("⚠ Missing in Kong (would be created by 'kwot apply'):")
		for _, name := range missingInKong {
			logger.Infof("  → %s", name)
		}
		logger.Info("")
	}

	if len(orphanedInKong) > 0 {
		logger.Warnf("⚠ Orphaned in Kong (not in configuration):")
		for _, name := range orphanedInKong {
			logger.Infof("  ✗ %s", name)
		}
		logger.Info("")
		logger.Infof("These workspaces exist in Kong but are not in your config files.")
		logger.Infof("To delete them, use: kwot delete workspaces -n <workspace-name> --force")
		logger.Info("")
	}

	return nil
}

// compareRoleDrift compares configured roles with Kong to detect orphaned roles
func compareRoleDrift(rolesProcessor *roles.Processor, wsProcessor *workspace.Processor) error {
	// Get configured workspaces
	configuredWorkspaces, err := wsProcessor.GetWorkspaceDirs()
	if err != nil {
		return fmt.Errorf("failed to get configured workspaces: %w", err)
	}

	// Get Kong workspaces to find all workspaces
	kongWorkspaces, err := wsProcessor.GetKongWorkspaces()
	if err != nil {
		return fmt.Errorf("failed to get Kong workspaces: %w", err)
	}

	// Build set of Kong workspace names
	kongSet := make(map[string]bool)
	for _, name := range kongWorkspaces {
		kongSet[name] = true
	}

	// Build set of configured workspace names
	configSet := make(map[string]bool)
	for _, name := range configuredWorkspaces {
		configSet[name] = true
	}

	// Collect missing and orphaned roles
	type role struct {
		workspace string
		role      string
	}
	var missingInKong []role
	var orphanedInKong []role

	// Check roles in configured workspaces that exist in Kong
	for _, wsName := range configuredWorkspaces {
		// Skip workspaces that don't exist in Kong yet
		if !kongSet[wsName] {
			// If workspace doesn't exist, all its configured roles are missing
			wsConfig, err := rolesProcessor.LoadWorkspaceConfig(wsName)
			if err != nil {
				logger.Debugf("Failed to load config for workspace %s: %v", wsName, err)
				continue
			}

			for _, configRole := range wsConfig.RBAC {
				missingInKong = append(missingInKong, role{wsName, configRole.Role})
			}
			continue
		}

		// Get configured roles from workspace config
		wsConfig, err := rolesProcessor.LoadWorkspaceConfig(wsName)
		if err != nil {
			logger.Debugf("Failed to load config for workspace %s: %v", wsName, err)
			continue
		}

		configuredRoleNames := make(map[string]bool)
		for _, configRole := range wsConfig.RBAC {
			configuredRoleNames[configRole.Role] = true
		}

		// Get roles from Kong for this workspace
		kongRoles, err := rolesProcessor.GetAllRolesForWorkspace(wsName)
		if err != nil {
			logger.Warnf("Failed to get roles for workspace %s: %v", wsName, err)
			continue
		}

		// Build set of Kong role names
		kongRoleSet := make(map[string]bool)
		for _, kongRole := range kongRoles {
			kongRoleSet[kongRole.Name] = true
		}

		// Find missing roles (in config but not in Kong)
		for _, configRole := range wsConfig.RBAC {
			if !kongRoleSet[configRole.Role] {
				missingInKong = append(missingInKong, role{wsName, configRole.Role})
			}
		}

		// Find orphaned roles (in Kong but not in config)
		for _, kongRole := range kongRoles {
			if !configuredRoleNames[kongRole.Name] {
				orphanedInKong = append(orphanedInKong, role{wsName, kongRole.Name})
			}
		}
	}

	// Also check for orphaned roles in workspaces that exist in Kong but not in config
	for _, wsName := range kongWorkspaces {
		// Skip default workspace and already configured workspaces
		if wsName == "default" || configSet[wsName] {
			continue
		}

		// Get all roles in this orphaned workspace
		kongRoles, err := rolesProcessor.GetAllRolesForWorkspace(wsName)
		if err != nil {
			logger.Debugf("Failed to get roles for orphaned workspace %s: %v", wsName, err)
			continue
		}

		// All roles in orphaned workspaces are orphaned
		for _, kongRole := range kongRoles {
			orphanedInKong = append(orphanedInKong, role{wsName, kongRole.Name})
		}
	}

	// Report results
	if len(missingInKong) == 0 && len(orphanedInKong) == 0 {
		logger.Infof("✓ No role drift detected - Kong is in sync with configuration")
		return nil
	}

	if len(missingInKong) > 0 {
		logger.Warnf("⚠ Missing in Kong (would be created by 'kwot apply'):")
		for _, m := range missingInKong {
			logger.Infof("  → %s (in workspace '%s')", m.role, m.workspace)
		}
		logger.Info("")
	}

	if len(orphanedInKong) > 0 {
		logger.Warnf("⚠ Orphaned in Kong (not in configuration):")
		for _, o := range orphanedInKong {
			logger.Infof("  ✗ %s (in workspace '%s')", o.role, o.workspace)
		}
		logger.Info("")
	}

	return nil
}

// compareGroupDrift compares configured groups with Kong to detect orphaned groups
func compareGroupDrift(groupsProcessor *groups.Processor) error {
	// Load configured groups
	configuredGroups, err := groupsProcessor.LoadGroupConfig()
	if err != nil {
		return fmt.Errorf("failed to load group config: %w", err)
	}

	// Get groups from Kong
	kongGroups, err := groupsProcessor.GetAllGroups()
	if err != nil {
		return fmt.Errorf("failed to get Kong groups: %w", err)
	}

	// Build set of configured group names
	configSet := make(map[string]bool)
	for _, group := range configuredGroups {
		configSet[group.GroupName] = true
	}

	// Build set of Kong group names
	kongSet := make(map[string]bool)
	for _, group := range kongGroups {
		kongSet[group.Name] = true
	}

	// Find groups in config but not in Kong (missing resources)
	var missingInKong []string
	for _, group := range configuredGroups {
		if !kongSet[group.GroupName] {
			missingInKong = append(missingInKong, group.GroupName)
		}
	}

	// Find groups in Kong but not in config (orphaned resources)
	var orphanedInKong []string
	for _, group := range kongGroups {
		if !configSet[group.Name] {
			orphanedInKong = append(orphanedInKong, group.Name)
		}
	}

	// Report results
	if len(missingInKong) == 0 && len(orphanedInKong) == 0 {
		logger.Infof("✓ No group drift detected - Kong is in sync with configuration")
		return nil
	}

	if len(missingInKong) > 0 {
		logger.Warnf("⚠ Missing in Kong (would be created by 'kwot apply'):")
		for _, name := range missingInKong {
			logger.Infof("  → %s", name)
		}
		logger.Info("")
	}

	if len(orphanedInKong) > 0 {
		logger.Warnf("⚠ Orphaned in Kong (not in configuration):")
		for _, name := range orphanedInKong {
			logger.Infof("  ✗ %s", name)
		}
		logger.Info("")
		logger.Infof("These groups exist in Kong but are not in your config files.")
		logger.Infof("To delete them, use: kwot delete groups -n <group-name> --force")
		logger.Info("")
	}

	return nil
}
