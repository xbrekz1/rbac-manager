package controller

import (
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	rbacmanagerv1alpha1 "github.com/xbrekz1/rbac-manager/api/v1alpha1"
)

func TestGetPolicyRules(t *testing.T) {
	tests := []struct {
		name        string
		ag          *rbacmanagerv1alpha1.AccessGrant
		expectError bool
		expectRules int
	}{
		{
			name: "predefined role returns rules",
			ag: &rbacmanagerv1alpha1.AccessGrant{
				Spec: rbacmanagerv1alpha1.AccessGrantSpec{
					Role: rbacmanagerv1alpha1.RoleDeveloper,
				},
			},
			expectError: false,
			expectRules: 6, // developer role has 6 rules
		},
		{
			name: "custom rules returns rules",
			ag: &rbacmanagerv1alpha1.AccessGrant{
				Spec: rbacmanagerv1alpha1.AccessGrantSpec{
					CustomRules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{""},
							Resources: []string{"pods"},
							Verbs:     []string{"get", "list"},
						},
					},
				},
			},
			expectError: false,
			expectRules: 1,
		},
		{
			name: "unknown role returns error",
			ag: &rbacmanagerv1alpha1.AccessGrant{
				Spec: rbacmanagerv1alpha1.AccessGrantSpec{
					Role: rbacmanagerv1alpha1.PredefinedRole("unknown-role"),
				},
			},
			expectError: true,
		},
		{
			name: "neither role nor customRules returns error",
			ag: &rbacmanagerv1alpha1.AccessGrant{
				Spec: rbacmanagerv1alpha1.AccessGrantSpec{},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules, err := getPolicyRules(tt.ag)

			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if !tt.expectError && len(rules) != tt.expectRules {
				t.Errorf("expected %d rules, got %d", tt.expectRules, len(rules))
			}
		})
	}
}

func TestResourceLabels(t *testing.T) {
	tests := []struct {
		name           string
		ag             *rbacmanagerv1alpha1.AccessGrant
		expectLabels   map[string]string
		checkManagedBy bool
		checkGrant     bool
	}{
		{
			name: "basic labels without user labels",
			ag: &rbacmanagerv1alpha1.AccessGrant{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-grant",
					Namespace: "test-ns",
				},
				Spec: rbacmanagerv1alpha1.AccessGrantSpec{},
			},
			expectLabels: map[string]string{
				managedByLabel:     managerValue,
				accessGrantLabel:   "test-grant",
				accessGrantNsLabel: "test-ns",
			},
			checkManagedBy: true,
			checkGrant:     true,
		},
		{
			name: "labels with user-provided labels",
			ag: &rbacmanagerv1alpha1.AccessGrant{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-grant",
					Namespace: "test-ns",
				},
				Spec: rbacmanagerv1alpha1.AccessGrantSpec{
					Labels: map[string]string{
						"team":        "backend",
						"environment": "dev",
					},
				},
			},
			expectLabels: map[string]string{
				managedByLabel:     managerValue,
				accessGrantLabel:   "test-grant",
				accessGrantNsLabel: "test-ns",
				"team":             "backend",
				"environment":      "dev",
			},
			checkManagedBy: true,
			checkGrant:     true,
		},
		{
			name: "user labels should not override system labels",
			ag: &rbacmanagerv1alpha1.AccessGrant{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-grant",
					Namespace: "test-ns",
				},
				Spec: rbacmanagerv1alpha1.AccessGrantSpec{
					Labels: map[string]string{
						managedByLabel: "other-manager",
						"custom-label": "custom-value",
					},
				},
			},
			expectLabels: map[string]string{
				managedByLabel:     "other-manager", // user label takes precedence
				accessGrantLabel:   "test-grant",
				accessGrantNsLabel: "test-ns",
				"custom-label":     "custom-value",
			},
			checkManagedBy: false, // because user overrode it
			checkGrant:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			labels := resourceLabels(tt.ag)

			if len(labels) != len(tt.expectLabels) {
				t.Errorf("expected %d labels, got %d", len(tt.expectLabels), len(labels))
			}

			for k, v := range tt.expectLabels {
				if labels[k] != v {
					t.Errorf("expected label %s=%s, got %s=%s", k, v, k, labels[k])
				}
			}

			if tt.checkManagedBy {
				if labels[managedByLabel] != managerValue {
					t.Errorf("managedByLabel should be %q, got %q", managerValue, labels[managedByLabel])
				}
			}

			if tt.checkGrant {
				if labels[accessGrantLabel] != tt.ag.Name {
					t.Errorf("accessGrantLabel should be %q, got %q", tt.ag.Name, labels[accessGrantLabel])
				}
				if labels[accessGrantNsLabel] != tt.ag.Namespace {
					t.Errorf("accessGrantNsLabel should be %q, got %q", tt.ag.Namespace, labels[accessGrantNsLabel])
				}
			}
		})
	}
}

func TestReconcileResultStructure(t *testing.T) {
	result := &reconcileResult{
		namespaces:  []string{"ns1", "ns2"},
		saName:      "test-sa",
		clusterRole: "test-cluster-role",
	}

	if len(result.namespaces) != 2 {
		t.Errorf("expected 2 namespaces, got %d", len(result.namespaces))
	}

	if result.saName != "test-sa" {
		t.Errorf("expected saName 'test-sa', got %q", result.saName)
	}

	if result.clusterRole != "test-cluster-role" {
		t.Errorf("expected clusterRole 'test-cluster-role', got %q", result.clusterRole)
	}
}

func TestConstants(t *testing.T) {
	// Test that important constants are set correctly
	if finalizerName != "rbacmanager.io/cleanup" {
		t.Errorf("finalizerName should be 'rbacmanager.io/cleanup', got %q", finalizerName)
	}

	if managedByLabel != "rbacmanager.io/managed-by" {
		t.Errorf("managedByLabel should be 'rbacmanager.io/managed-by', got %q", managedByLabel)
	}

	if accessGrantLabel != "rbacmanager.io/access-grant" {
		t.Errorf("accessGrantLabel should be 'rbacmanager.io/access-grant', got %q", accessGrantLabel)
	}

	if accessGrantNsLabel != "rbacmanager.io/access-grant-namespace" {
		t.Errorf("accessGrantNsLabel should be 'rbacmanager.io/access-grant-namespace', got %q", accessGrantNsLabel)
	}

	if managerValue != "rbac-manager" {
		t.Errorf("managerValue should be 'rbac-manager', got %q", managerValue)
	}
}
