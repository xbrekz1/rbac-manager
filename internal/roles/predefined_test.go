package roles

import (
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
)

func TestGetPredefinedRules(t *testing.T) {
	tests := []struct {
		name           string
		role           string
		expectFound    bool
		expectRules    bool
		expectNonEmpty bool
	}{
		{
			name:           "reader role exists",
			role:           "reader",
			expectFound:    true,
			expectRules:    true,
			expectNonEmpty: true,
		},
		{
			name:           "viewer role exists",
			role:           "viewer",
			expectFound:    true,
			expectRules:    true,
			expectNonEmpty: true,
		},
		{
			name:           "developer role exists",
			role:           "developer",
			expectFound:    true,
			expectRules:    true,
			expectNonEmpty: true,
		},
		{
			name:           "developer-extended role exists",
			role:           "developer-extended",
			expectFound:    true,
			expectRules:    true,
			expectNonEmpty: true,
		},
		{
			name:           "deployer role exists",
			role:           "deployer",
			expectFound:    true,
			expectRules:    true,
			expectNonEmpty: true,
		},
		{
			name:           "debugger role exists",
			role:           "debugger",
			expectFound:    true,
			expectRules:    true,
			expectNonEmpty: true,
		},
		{
			name:           "operator role exists",
			role:           "operator",
			expectFound:    true,
			expectRules:    true,
			expectNonEmpty: true,
		},
		{
			name:           "auditor role exists",
			role:           "auditor",
			expectFound:    true,
			expectRules:    true,
			expectNonEmpty: true,
		},
		{
			name:           "maintainer role exists",
			role:           "maintainer",
			expectFound:    true,
			expectRules:    true,
			expectNonEmpty: true,
		},
		{
			name:           "cluster-admin role exists",
			role:           "cluster-admin",
			expectFound:    true,
			expectRules:    true,
			expectNonEmpty: true,
		},
		{
			name:        "unknown role returns not found",
			role:        "unknown-role",
			expectFound: false,
		},
		{
			name:        "empty role name",
			role:        "",
			expectFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules, found := GetPredefinedRules(tt.role)

			if found != tt.expectFound {
				t.Errorf("GetPredefinedRules(%q) found = %v, want %v", tt.role, found, tt.expectFound)
			}

			if tt.expectRules && rules == nil {
				t.Errorf("GetPredefinedRules(%q) returned nil rules, expected non-nil", tt.role)
			}

			if tt.expectNonEmpty && len(rules) == 0 {
				t.Errorf("GetPredefinedRules(%q) returned empty rules, expected non-empty", tt.role)
			}

			if !tt.expectFound && rules != nil {
				t.Errorf("GetPredefinedRules(%q) returned rules for non-existent role", tt.role)
			}
		})
	}
}

func TestNeedsNamespaceViewer(t *testing.T) {
	tests := []struct {
		name   string
		role   string
		expect bool
	}{
		{
			name:   "viewer needs namespace viewer",
			role:   "viewer",
			expect: true,
		},
		{
			name:   "developer-extended needs namespace viewer",
			role:   "developer-extended",
			expect: true,
		},
		{
			name:   "developer does not need namespace viewer",
			role:   "developer",
			expect: false,
		},
		{
			name:   "reader does not need namespace viewer",
			role:   "reader",
			expect: false,
		},
		{
			name:   "deployer does not need namespace viewer",
			role:   "deployer",
			expect: false,
		},
		{
			name:   "unknown role does not need namespace viewer",
			role:   "unknown",
			expect: false,
		},
		{
			name:   "empty role name",
			role:   "",
			expect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NeedsNamespaceViewer(tt.role)
			if result != tt.expect {
				t.Errorf("NeedsNamespaceViewer(%q) = %v, want %v", tt.role, result, tt.expect)
			}
		})
	}
}

func TestRoleHierarchy(t *testing.T) {
	// Test that higher-privilege roles include all permissions from lower-privilege roles

	// Reader permissions
	readerRules, _ := GetPredefinedRules("reader")
	if !hasGetListWatchOnPods(readerRules) {
		t.Error("reader should have get/list/watch on pods")
	}
	if hasExecOnPods(readerRules) {
		t.Error("reader should NOT have exec on pods")
	}
	if hasAccessToSecrets(readerRules) {
		t.Error("reader should NOT have access to secrets")
	}

	// Viewer permissions
	viewerRules, _ := GetPredefinedRules("viewer")
	if !hasGetListWatchOnPods(viewerRules) {
		t.Error("viewer should have get/list/watch on pods")
	}
	if !hasAccessToPodLogs(viewerRules) {
		t.Error("viewer should have access to pod logs")
	}
	if hasExecOnPods(viewerRules) {
		t.Error("viewer should NOT have exec on pods")
	}

	// Developer permissions
	devRules, _ := GetPredefinedRules("developer")
	if !hasGetListWatchOnPods(devRules) {
		t.Error("developer should have get/list/watch on pods")
	}
	if !hasAccessToPodLogs(devRules) {
		t.Error("developer should have access to pod logs")
	}
	if !hasExecOnPods(devRules) {
		t.Error("developer should have exec on pods")
	}
	if !hasAccessToSecrets(devRules) {
		t.Error("developer should have access to secrets (read)")
	}

	// Operator permissions
	opRules, _ := GetPredefinedRules("operator")
	if !hasGetListWatchOnPods(opRules) {
		t.Error("operator should have get/list/watch on pods")
	}
	if !hasExecOnPods(opRules) {
		t.Error("operator should have exec on pods")
	}
	if !hasAccessToSecrets(opRules) {
		t.Error("operator should have access to secrets")
	}
	if !hasUpdatePatchOnDeployments(opRules) {
		t.Error("operator should have update/patch on deployments")
	}

	// Maintainer permissions (full access)
	maintainerRules, _ := GetPredefinedRules("maintainer")
	if !hasFullAccess(maintainerRules) {
		t.Error("maintainer should have full access (*/*)")
	}

	// Cluster-admin permissions
	clusterAdminRules, _ := GetPredefinedRules("cluster-admin")
	if !hasFullAccess(clusterAdminRules) {
		t.Error("cluster-admin should have full access")
	}
	if !hasNonResourceURLAccess(clusterAdminRules) {
		t.Error("cluster-admin should have access to non-resource URLs")
	}
}

func TestDeployerPermissions(t *testing.T) {
	deployerRules, _ := GetPredefinedRules("deployer")

	if !hasUpdatePatchOnDeployments(deployerRules) {
		t.Error("deployer should have update/patch on deployments")
	}
	if hasExecOnPods(deployerRules) {
		t.Error("deployer should NOT have exec on pods")
	}
	if hasAccessToSecrets(deployerRules) {
		t.Error("deployer should NOT have access to secrets")
	}
	if !hasAccessToPodLogs(deployerRules) {
		t.Error("deployer should have access to pod logs")
	}
}

func TestDebuggerPermissions(t *testing.T) {
	debuggerRules, _ := GetPredefinedRules("debugger")

	if !hasExecOnPods(debuggerRules) {
		t.Error("debugger should have exec on pods")
	}
	if !hasPortForwardOnPods(debuggerRules) {
		t.Error("debugger should have port-forward on pods")
	}
	if hasUpdatePatchOnDeployments(debuggerRules) {
		t.Error("debugger should NOT have update/patch on deployments")
	}
	if hasAccessToSecrets(debuggerRules) {
		t.Error("debugger should NOT have access to secrets")
	}
}

func TestAuditorPermissions(t *testing.T) {
	auditorRules, _ := GetPredefinedRules("auditor")

	if !hasAccessToSecrets(auditorRules) {
		t.Error("auditor should have access to secrets (for audit)")
	}
	if !hasGetListWatchOnPods(auditorRules) {
		t.Error("auditor should have get/list/watch on pods")
	}
	if hasUpdatePatchOnDeployments(auditorRules) {
		t.Error("auditor should NOT have update/patch on deployments (read-only)")
	}
	if hasExecOnPods(auditorRules) {
		t.Error("auditor should NOT have exec on pods (read-only)")
	}
}

// Helper functions to check permissions

func hasGetListWatchOnPods(rules []rbacv1.PolicyRule) bool {
	for _, rule := range rules {
		if containsResource(rule, "pods") && containsVerbs(rule, "get", "list", "watch") {
			return true
		}
	}
	return false
}

func hasAccessToPodLogs(rules []rbacv1.PolicyRule) bool {
	for _, rule := range rules {
		if containsResource(rule, "pods/log") && (containsVerb(rule, "get") || containsVerb(rule, "list")) {
			return true
		}
	}
	return false
}

func hasExecOnPods(rules []rbacv1.PolicyRule) bool {
	for _, rule := range rules {
		if containsResource(rule, "pods/exec") && containsVerb(rule, "create") {
			return true
		}
	}
	return false
}

func hasPortForwardOnPods(rules []rbacv1.PolicyRule) bool {
	for _, rule := range rules {
		if containsResource(rule, "pods/portforward") && containsVerb(rule, "create") {
			return true
		}
	}
	return false
}

func hasAccessToSecrets(rules []rbacv1.PolicyRule) bool {
	for _, rule := range rules {
		if containsResource(rule, "secrets") && (containsVerb(rule, "get") || containsVerb(rule, "list")) {
			return true
		}
	}
	return false
}

func hasUpdatePatchOnDeployments(rules []rbacv1.PolicyRule) bool {
	for _, rule := range rules {
		for _, apiGroup := range rule.APIGroups {
			if apiGroup == "apps" || apiGroup == "*" {
				if containsResource(rule, "deployments") && containsVerbs(rule, "update", "patch") {
					return true
				}
			}
		}
	}
	return false
}

func hasFullAccess(rules []rbacv1.PolicyRule) bool {
	for _, rule := range rules {
		if containsWildcard(rule.APIGroups) && containsWildcard(rule.Resources) && containsWildcard(rule.Verbs) {
			return true
		}
	}
	return false
}

func hasNonResourceURLAccess(rules []rbacv1.PolicyRule) bool {
	for _, rule := range rules {
		if len(rule.NonResourceURLs) > 0 && containsWildcard(rule.NonResourceURLs) && containsWildcard(rule.Verbs) {
			return true
		}
	}
	return false
}

func containsResource(rule rbacv1.PolicyRule, resource string) bool {
	for _, r := range rule.Resources {
		if r == resource || r == "*" {
			return true
		}
	}
	return false
}

func containsVerb(rule rbacv1.PolicyRule, verb string) bool {
	for _, v := range rule.Verbs {
		if v == verb || v == "*" {
			return true
		}
	}
	return false
}

func containsVerbs(rule rbacv1.PolicyRule, verbs ...string) bool {
	for _, verb := range verbs {
		if !containsVerb(rule, verb) {
			return false
		}
	}
	return true
}

func containsWildcard(slice []string) bool {
	for _, item := range slice {
		if item == "*" {
			return true
		}
	}
	return false
}

func TestAllPredefinedRolesHaveValidRules(t *testing.T) {
	expectedRoles := []string{
		"reader",
		"viewer",
		"developer",
		"developer-extended",
		"deployer",
		"debugger",
		"operator",
		"auditor",
		"maintainer",
		"cluster-admin",
	}

	for _, role := range expectedRoles {
		t.Run(role, func(t *testing.T) {
			rules, found := GetPredefinedRules(role)
			if !found {
				t.Fatalf("Role %q should exist in predefinedRoles map", role)
			}

			if len(rules) == 0 {
				t.Fatalf("Role %q should have at least one policy rule", role)
			}

			// Validate that each rule has required fields
			for i, rule := range rules {
				if len(rule.Verbs) == 0 {
					t.Errorf("Rule %d in role %q has no verbs", i, role)
				}

				// Either Resources or NonResourceURLs should be specified
				if len(rule.Resources) == 0 && len(rule.NonResourceURLs) == 0 {
					t.Errorf("Rule %d in role %q has neither resources nor nonResourceURLs", i, role)
				}
			}
		})
	}
}
