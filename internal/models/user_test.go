package models

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestRBACUserYAMLUnmarshal(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		wantToken     string
		wantUserName  string
		wantRoleCount int
	}{
		{
			name: "user with explicit token",
			input: `
name: test-user
user_token: my-static-token
roles:
  - admin
`,
			wantToken:     "my-static-token",
			wantUserName:  "test-user",
			wantRoleCount: 1,
		},
		{
			name: "user without token",
			input: `
name: test-user
roles:
  - admin
  - readonlyrole
`,
			wantToken:     "",
			wantUserName:  "test-user",
			wantRoleCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var u RBACUser
			if err := yaml.Unmarshal([]byte(tt.input), &u); err != nil {
				t.Fatalf("yaml.Unmarshal() error = %v", err)
			}
			if u.Name != tt.wantUserName {
				t.Errorf("Name = %q, want %q", u.Name, tt.wantUserName)
			}
			if u.UserToken != tt.wantToken {
				t.Errorf("UserToken = %q, want %q", u.UserToken, tt.wantToken)
			}
			if len(u.Roles) != tt.wantRoleCount {
				t.Errorf("len(Roles) = %d, want %d", len(u.Roles), tt.wantRoleCount)
			}
		})
	}
}
