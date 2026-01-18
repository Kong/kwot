#!/usr/bin/env bash
set -euo pipefail

# Script to generate workspace configuration structure for performance testing
# Usage: bash generate-workspaces.sh <number_of_workspaces> [config_dir_name]
# Example: bash generate-workspaces.sh 50
# Example: bash generate-workspaces.sh 50 config-perf

if [[ $# -lt 1 ]]; then
    echo "Usage: bash generate-workspaces.sh <number_of_workspaces> [config_dir_name]"
    echo "Example: bash generate-workspaces.sh 50"
    echo "Example: bash generate-workspaces.sh 50 config-perf"
    exit 1
fi

NUM_WORKSPACES=$1
CONFIG_DIR_NAME="${2:-config-perf-test}"

# Validate input
if ! [[ "$NUM_WORKSPACES" =~ ^[0-9]+$ ]] || [ "$NUM_WORKSPACES" -lt 1 ]; then
    echo "Error: number_of_workspaces must be a positive integer"
    exit 1
fi

# Get the script directory and navigate to project root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
CONFIG_DIR="$PROJECT_ROOT/$CONFIG_DIR_NAME"
ORIGINAL_CONFIG_DIR="$PROJECT_ROOT/config"
SKELETON_DIR="$PROJECT_ROOT/perf-test/skeleton"

echo "========================================="
echo "Workspace Configuration Generator"
echo "========================================="
echo "Project root: $PROJECT_ROOT"
echo "Config directory: ./$CONFIG_DIR_NAME"
echo "Skeleton directory: $SKELETON_DIR"
echo "Number of workspaces to generate: $NUM_WORKSPACES"
echo ""

# Verify skeleton directory exists
if [[ ! -d "$SKELETON_DIR" ]]; then
    echo "Error: Skeleton directory not found at $SKELETON_DIR"
    exit 1
fi

# Verify required skeleton files
required_files=("workspace.yaml")
for file in "${required_files[@]}"; do
    if [[ ! -f "$SKELETON_DIR/$file" ]]; then
        echo "Error: Required skeleton file not found: $SKELETON_DIR/$file"
        exit 1
    fi
done

# Create config directory if it doesn't exist
mkdir -p "$CONFIG_DIR"

# Copy RBAC files to config root if not already present
echo "Setting up root-level RBAC configuration..."

# Create config directory if it doesn't exist
mkdir -p "$CONFIG_DIR"

# Copy root workspace config
if [[ -f "$SKELETON_DIR/root-workspace.yaml" ]]; then
    cp "$SKELETON_DIR/root-workspace.yaml" "$CONFIG_DIR/root-workspace.yaml"
    echo "✓ Copied root-workspace.yaml"
fi

# Copy RBAC role permission files if they exist in skeleton
for rbac_file in tenant-user.yaml tenant-release-user.yaml tenant-devportal-approver.yaml; do
    if [[ -f "$SKELETON_DIR/$rbac_file" ]]; then
        cp "$SKELETON_DIR/$rbac_file" "$CONFIG_DIR/$rbac_file"
        echo "✓ Copied $rbac_file"
    fi
done

# Copy groups-and-roles.yaml from original config or skeleton
if [[ -f "$ORIGINAL_CONFIG_DIR/groups-and-roles.yaml" ]]; then
    cp "$ORIGINAL_CONFIG_DIR/groups-and-roles.yaml" "$CONFIG_DIR/groups-and-roles.yaml"
    echo "✓ Copied groups-and-roles.yaml"
elif [[ -f "$SKELETON_DIR/groups-and-roles.yaml" ]]; then
    cp "$SKELETON_DIR/groups-and-roles.yaml" "$CONFIG_DIR/groups-and-roles.yaml"
    echo "✓ Copied groups-and-roles.yaml"
fi

echo ""
echo "Generating groups and roles configuration for $NUM_WORKSPACES workspaces..."

# Generate groups-and-roles.yaml with dynamic workspace entries
generate_groups_and_roles() {
    local num_workspaces=$1
    local output_file=$2
    
    cat > "$output_file" << 'EOF'
---
role_info:
  wk_admin: &wk_admin admin
  readonly_role: &readonly_role readonlyrole
  tenant_user_role: &tenant_user_role Tenant-User
  tenant_release_user_role: &tenant_release_user_role Tenant-Release-User
  tenant_devportal_role: &tenant_devportal_role Tenant-Devportal-Approver

config:
  # Performance Test Workspaces
  # Each workspace has 3 groups for tenant roles: tenant-user, tenant-release, tenant-devportal
  # Note: admin and readonlyrole are built-in system roles not created per-workspace
EOF

    # Generate entries for each workspace
    for ((i=1; i<=num_workspaces; i++)); do
        # Use Tenant-Release-User for all workspaces (the actual role that gets created)
        release_role="*tenant_release_user_role"
        
        cat >> "$output_file" << EOF

  - group_name: ws-${i}-tenant-user-group
    group_comment: Workspace ws-${i} tenant user group
    roles:
      - workspace: ws-${i}
        role: *tenant_user_role

  - group_name: ws-${i}-tenant-release-group
    group_comment: Workspace ws-${i} tenant release group
    roles:
      - workspace: ws-${i}
        role: ${release_role}

  - group_name: ws-${i}-tenant-devportal-group
    group_comment: Workspace ws-${i} tenant devportal group
    roles:
      - workspace: ws-${i}
        role: *tenant_devportal_role
EOF
    done
}

# Generate the groups-and-roles.yaml file
generate_groups_and_roles "$NUM_WORKSPACES" "$CONFIG_DIR/groups-and-roles.yaml"
echo "✓ Generated groups-and-roles.yaml with $NUM_WORKSPACES workspaces"

echo ""
echo "Generating $NUM_WORKSPACES workspace folders..."

# Generate workspace folders
for ((i=1; i<=NUM_WORKSPACES; i++)); do
    workspace_name="ws-$i"
    workspace_dir="$CONFIG_DIR/$workspace_name"
    
    # Create workspace directory
    mkdir -p "$workspace_dir"
    
    # Copy skeleton files to workspace directory
    cp "$SKELETON_DIR/workspace.yaml" "$workspace_dir/workspace.yaml"
    
    # Progress indicator
    if (( i % 10 == 0 )); then
        echo "  ✓ Generated workspace folders 1-$i"
    fi
done

echo ""
echo "========================================="
echo "✓ Successfully generated $NUM_WORKSPACES workspace folders"
echo "========================================="
echo ""
echo "Configuration directory: ./$CONFIG_DIR_NAME"
echo ""
echo "To use this configuration:"
echo "  1. Update CONFIG_DIR environment variable to:"
echo "     export CONFIG_DIR=./$CONFIG_DIR_NAME"
echo ""
echo "  2. Or run kwot with:"
echo "     CONFIG_DIR=./$CONFIG_DIR_NAME ./bin/kwot apply"
echo ""
echo "Generated structure:"
echo "  $CONFIG_DIR_NAME/"
echo "    ├── groups-and-roles.yaml"
echo "    ├── root-workspace.yaml"
echo "    ├── tenant-user.yaml"
echo "    ├── tenant-release-user.yaml"
echo "    ├── tenant-devportal-approver.yaml"
echo "    ├── ws-1/"
echo "    │   └── workspace.yaml"
echo "    ├── ws-2/"
echo "    └── ..."
echo "    └── ws-$NUM_WORKSPACES/"
echo ""
echo "Total files created: $NUM_WORKSPACES workspace YAML files"
