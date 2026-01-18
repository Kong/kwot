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

var applyName string

var allCmd = &cobra.Command{
	Use:   "apply [command]",
	Short: "Apply configuration to Kong Gateway",
	Long: `Apply configuration declarations from your config files.

By default, this applies everything in the correct order:
  1. Workspaces - Create/update Kong workspaces
  2. Roles - Define RBAC roles and permissions
  3. Groups - Create groups and assign workspace-specific roles

You can also apply specific resources individually:
  kwot apply all        - Apply all resources
  kwot apply roles      - Apply only roles (use -n flag for specific role)
  kwot apply groups     - Apply only groups
  kwot apply workspaces - Apply only workspaces

Examples:
  kwot apply all                                # Apply all resources
  kwot apply roles                              # Apply all roles
  kwot apply roles -n admin -w demo1            # Apply specific role to workspace
  kwot apply groups -w demo1                    # Apply groups to demo1
  kwot apply workspaces                         # Apply all workspaces
  kwot apply --dry-run                          # Preview changes (no apply)`,
	Args: requireKnownSubcommand,
	RunE: func(cmd *cobra.Command, args []string) error {
		// If no subcommand is provided, default to 'apply all'
		if len(args) == 0 {
			return applyResources("", "", "")
		}
		_ = cmd.Help()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(allCmd)

	// Add subcommands to apply
	applyAllCmd := &cobra.Command{
		Use:   "all [flags]",
		Short: "Apply all resources",
		Long: `Apply all resources in the correct order: workspaces, roles, then groups.

Examples:
  kwot apply all                    # Apply all resources
  kwot apply all -w demo1           # Apply all resources for workspace demo1
  kwot apply all --dry-run          # Preview changes without applying`,
		Args: requireNoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return applyResources("", "", "")
		},
	}

	applyRolesCmd := &cobra.Command{
		Use:   "roles [flags]",
		Short: "Apply RBAC roles",
		Long: `Apply RBAC roles and their permissions.

Use -n/--name flag to apply a specific role, or omit it to apply all roles.
This allows you to update permissions for a single role without recreating others.

Examples:
  kwot apply roles                       # Apply all roles
  kwot apply roles -n admin              # Apply specific role (all workspaces)
  kwot apply roles -n admin -w demo1     # Apply specific role to workspace
  kwot apply roles -w demo1              # Roles for specific workspace
  kwot apply roles --dry-run             # Preview changes without applying`,
		Args: requireNoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return applyResources("roles", applyName, "")
		},
	}
	applyRolesCmd.Flags().StringVarP(&applyName, "name", "n", "", "name of specific role to apply (optional)")

	applyGroupsCmd := &cobra.Command{
		Use:   "groups [flags]",
		Short: "Apply groups",
		Long: `Apply groups and role assignments.

Groups are global resources in Kong that can have workspace-specific role assignments.

Examples:
  kwot apply groups                   # Apply all groups
  kwot apply groups -w demo1          # Groups assigned to specific workspace
  kwot apply groups --dry-run         # Preview changes without applying`,
		Args: requireNoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return applyResources("groups", "", "")
		},
	}

	applyWorkspacesCmd := &cobra.Command{
		Use:   "workspaces [flags]",
		Short: "Apply workspaces",
		Long: `Apply workspaces.

Use -n/--name flag to apply a specific workspace by name, or omit it to apply all workspaces.

Examples:
  kwot apply workspaces                   # Apply all workspaces
  kwot apply workspaces -n demo1          # Apply specific workspace
  kwot apply workspaces --dry-run         # Preview changes without applying`,
		Args: requireNoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return applyResources("workspaces", applyName, "")
		},
	}
	applyWorkspacesCmd.Flags().StringVarP(&applyName, "name", "n", "", "name of specific workspace to apply (optional)")

	allCmd.AddCommand(applyAllCmd)
	allCmd.AddCommand(applyRolesCmd)
	allCmd.AddCommand(applyGroupsCmd)
	allCmd.AddCommand(applyWorkspacesCmd)
}

func applyResources(resourceType string, roleName string, _ string) error {
	// Determine target workspace
	// For workspace resources: use -n flag if provided, else use -w flag (default "all")
	// For other resources: use -w flag (default "all")
	var targetWs string
	if resourceType == "workspaces" && roleName != "" {
		// When applying specific workspace with -n
		targetWs = roleName
	} else {
		// Use global -w flag (default is "all")
		targetWs = workspaceName
	}

	// Handle dry-run mode
	if dryRun {
		separator := strings.Repeat("=", 60)
		logger.Info(separator)
		logger.Info("DRY-RUN MODE: No changes will be applied")
		logger.Info(separator)
	}

	logger.Info("Starting configuration apply process...")
	logger.Info(fmt.Sprintf("Target: %s", targetWs))
	if resourceType != "" {
		logger.Info(fmt.Sprintf("Resource: %s", resourceType))
	}
	if roleName != "" {
		logger.Info(fmt.Sprintf("Role: %s", roleName))
	}

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

	wsProcessor := workspace.NewProcessor(client, cfg, dryRun)
	roleProcessor := roles.NewProcessor(client, cfg, dryRun)
	groupProcessor := groups.NewProcessor(client, cfg, dryRun)

	// Determine what to process
	applyWorkspaces := resourceType == "" || resourceType == "workspaces"
	applyRoles := resourceType == "" || resourceType == "roles"
	applyGroups := resourceType == "" || resourceType == "groups"

	// Step 1: Process workspaces
	if applyWorkspaces {
		logger.Info("Step 1/3: Processing workspaces...")
		if err := wsProcessor.ProcessWorkspaces(targetWs); err != nil {
			return fmt.Errorf("failed to process workspaces: %w", err)
		}
	}

	// Step 2: Process roles
	if applyRoles {
		step := "2/3"
		if !applyWorkspaces {
			step = "1/2"
		}
		logger.Info(fmt.Sprintf("Step %s: Processing roles...", step))
		if err := roleProcessor.ProcessRoles(targetWs, roleName); err != nil {
			return fmt.Errorf("failed to process roles: %w", err)
		}

		// Step 2b: Apply RBAC users (after roles to ensure they work for both new and existing workspaces)
		// Skip RBAC user application if only a specific role is being applied
		if roleName == "" {
			logger.Info("Applying RBAC users...")
			if err := wsProcessor.ApplyRBACUsersForWorkspaces(targetWs); err != nil {
				return fmt.Errorf("failed to apply RBAC users: %w", err)
			}
		}
	}

	// Step 3: Process groups
	if applyGroups && roleName == "" {
		step := "3/3"
		if !applyWorkspaces && !applyRoles {
			step = "1/1"
		} else if !applyWorkspaces || !applyRoles {
			step = "2/2"
		}
		logger.Info(fmt.Sprintf("Step %s: Processing groups...", step))
		if err := groupProcessor.ProcessGroups(targetWs); err != nil {
			return fmt.Errorf("failed to process groups: %w", err)
		}
	}

	if dryRun {
		separator := strings.Repeat("=", 60)
		logger.Info(separator)
		logger.Info("Preview mode complete. Review changes above.")
		logger.Info("To apply changes, run again without --dry-run flag")
		logger.Info(separator)
	} else {
		logger.Info("Complete configuration process finished successfully")
	}

	return nil
}
