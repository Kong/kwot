package groups

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Kong/kwot/internal/config"
)

// newTestProcessor creates a Processor using a temp dir as ConfigDir.
// kong.Client is nil — loadGroupConfig never touches it.
func newTestProcessor(t *testing.T, configDir string) *Processor {
	t.Helper()
	return &Processor{
		client: nil,
		cfg:    &config.Config{ConfigDir: configDir},
		dryRun: false,
	}
}

// writeFile creates a file at path with the given content.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("failed to create dir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
}

// --- YAML fixtures ---

// directArrayYAML uses the flat list format (no role_info wrapper).
const directArrayYAML = `
- group_name: demo1-admin-group
  group_comment: demo1 admin group
  roles:
    - workspace: demo1
      role: admin
- group_name: demo1-readonly-group
  group_comment: demo1 readonly group
  roles:
    - workspace: demo1
      role: readonlyrole
`

// structuredYAML uses the role_info + config wrapper format.
const structuredYAML = `
role_info:
  wk_admin: admin
  readonly_role: readonlyrole

config:
  - group_name: demo2-admin-group
    group_comment: demo2 admin group
    roles:
      - workspace: demo2
        role: admin
  - group_name: demo2-readonly-group
    group_comment: demo2 readonly group
    roles:
      - workspace: demo2
        role: readonlyrole
`

const demo3YAML = `
- group_name: demo3-admin-group
  group_comment: demo3 admin group
  roles:
    - workspace: demo3
      role: admin
`

const globalYAML = `
- group_name: global-demo4-admin-group
  group_comment: demo4 admin group (from global)
  roles:
    - workspace: demo4
      role: admin
- group_name: global-demo5-admin-group
  group_comment: demo5 admin group (from global)
  roles:
    - workspace: demo5
      role: admin
`

// --- Tests for specific workspace lookup ---

func TestLoadGroupConfig_SpecificWorkspace_LocalFileTakesPriority(t *testing.T) {
	dir := t.TempDir()
	p := newTestProcessor(t, dir)

	// Local file for demo1
	writeFile(t, filepath.Join(dir, "demo1", groupConfigName), directArrayYAML)
	// Global file also exists — should be ignored for demo1
	writeFile(t, filepath.Join(dir, groupConfigName), globalYAML)

	groups, err := p.loadGroupConfig("demo1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(groups) != 2 {
		t.Fatalf("expected 2 groups from local file, got %d", len(groups))
	}
	if groups[0].GroupName != "demo1-admin-group" {
		t.Errorf("expected demo1-admin-group, got %s", groups[0].GroupName)
	}
}

func TestLoadGroupConfig_SpecificWorkspace_FallsBackToGlobal(t *testing.T) {
	dir := t.TempDir()
	p := newTestProcessor(t, dir)

	// No local file for demo4 — only global exists
	writeFile(t, filepath.Join(dir, groupConfigName), globalYAML)

	groups, err := p.loadGroupConfig("demo4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(groups) != 2 {
		t.Fatalf("expected 2 groups from global file, got %d", len(groups))
	}
}

func TestLoadGroupConfig_SpecificWorkspace_NeitherFileExists(t *testing.T) {
	dir := t.TempDir()
	p := newTestProcessor(t, dir)

	_, err := p.loadGroupConfig("demo1")
	if err == nil {
		t.Fatal("expected error when no config file exists, got nil")
	}
}

// --- Tests for "all" mode ---

func TestLoadGroupConfig_All_PerWorkspaceFilesOnly(t *testing.T) {
	// Customer's setup: each workspace has its own groups-and-roles.yaml, no global file.
	dir := t.TempDir()
	p := newTestProcessor(t, dir)

	writeFile(t, filepath.Join(dir, "demo1", groupConfigName), directArrayYAML) // 2 groups
	writeFile(t, filepath.Join(dir, "demo2", groupConfigName), structuredYAML)  // 2 groups
	writeFile(t, filepath.Join(dir, "demo3", groupConfigName), demo3YAML)       // 1 group

	groups, err := p.loadGroupConfig("all")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(groups) != 5 {
		t.Fatalf("expected 5 groups total (2+2+1), got %d", len(groups))
	}
}

func TestLoadGroupConfig_All_GlobalFileOnly(t *testing.T) {
	// Existing users: only a global groups-and-roles.yaml, no per-workspace files.
	dir := t.TempDir()
	p := newTestProcessor(t, dir)

	// Workspace dirs exist but have no groups-and-roles.yaml
	if err := os.MkdirAll(filepath.Join(dir, "demo4"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, groupConfigName), globalYAML)

	groups, err := p.loadGroupConfig("all")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(groups) != 2 {
		t.Fatalf("expected 2 groups from global file, got %d", len(groups))
	}
}

func TestLoadGroupConfig_All_MixedLocalAndGlobal(t *testing.T) {
	// demo1 has a local file → uses local, ignores global entries for demo1.
	// demo4 and demo5 have no local file → fall back to global entries.
	dir := t.TempDir()
	p := newTestProcessor(t, dir)

	writeFile(t, filepath.Join(dir, "demo1", groupConfigName), directArrayYAML) // 2 groups for demo1
	writeFile(t, filepath.Join(dir, groupConfigName), globalYAML)               // 2 groups for demo4, demo5

	// demo4 dir exists but no local groups file
	if err := os.MkdirAll(filepath.Join(dir, "demo4"), 0o755); err != nil {
		t.Fatal(err)
	}

	groups, err := p.loadGroupConfig("all")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 2 from demo1 local + 2 from global (demo4, demo5 — no local file for them)
	if len(groups) != 4 {
		t.Fatalf("expected 4 groups, got %d", len(groups))
	}

	// Verify global entries for demo1 are NOT included (local file takes precedence)
	for _, g := range groups {
		for _, r := range g.Roles {
			if r.Workspace == "demo1" && g.GroupName != "demo1-admin-group" && g.GroupName != "demo1-readonly-group" {
				t.Errorf("unexpected demo1 group from global file: %s", g.GroupName)
			}
		}
	}
}

func TestLoadGroupConfig_All_NoFilesAnywhere(t *testing.T) {
	dir := t.TempDir()
	p := newTestProcessor(t, dir)

	_, err := p.loadGroupConfig("all")
	if err == nil {
		t.Fatal("expected error when no config files exist, got nil")
	}
}

// --- Tests for YAML format parsing ---

func TestParseGroupConfigFile_DirectArrayFormat(t *testing.T) {
	dir := t.TempDir()
	p := newTestProcessor(t, dir)

	path := filepath.Join(dir, groupConfigName)
	writeFile(t, path, directArrayYAML)

	groups, err := p.parseGroupConfigFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
	if groups[0].GroupName != "demo1-admin-group" {
		t.Errorf("expected demo1-admin-group, got %s", groups[0].GroupName)
	}
	if groups[0].Roles[0].Workspace != "demo1" || groups[0].Roles[0].Role != "admin" {
		t.Errorf("unexpected role assignment: %+v", groups[0].Roles[0])
	}
}

func TestParseGroupConfigFile_StructuredFormat(t *testing.T) {
	dir := t.TempDir()
	p := newTestProcessor(t, dir)

	path := filepath.Join(dir, groupConfigName)
	writeFile(t, path, structuredYAML)

	groups, err := p.parseGroupConfigFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
	if groups[0].GroupName != "demo2-admin-group" {
		t.Errorf("expected demo2-admin-group, got %s", groups[0].GroupName)
	}
}

func TestParseGroupConfigFile_FileNotFound(t *testing.T) {
	dir := t.TempDir()
	p := newTestProcessor(t, dir)

	_, err := p.parseGroupConfigFile(filepath.Join(dir, "nonexistent.yaml"))
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}
