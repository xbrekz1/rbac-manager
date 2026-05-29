package controller

import (
	"context"
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	rbacmanagerv1alpha1 "github.com/xbrekz1/rbac-manager/api/v1alpha1"
)

func newTestReconciler(objs ...client.Object) *AccessGrantReconciler {
	s := runtime.NewScheme()
	_ = rbacmanagerv1alpha1.AddToScheme(s)
	fakeClient := fake.NewClientBuilder().WithScheme(s).WithObjects(objs...).Build()
	return &AccessGrantReconciler{Client: fakeClient, Scheme: s}
}

func TestResolveRole(t *testing.T) {
	tests := []struct {
		name           string
		ag             *rbacmanagerv1alpha1.AccessGrant
		extraObjs      []client.Object
		expectError    bool
		expectRules    int
		expectNsViewer bool
	}{
		{
			name: "predefined developer role returns 6 rules",
			ag: &rbacmanagerv1alpha1.AccessGrant{
				Spec: rbacmanagerv1alpha1.AccessGrantSpec{
					Role: rbacmanagerv1alpha1.RoleDeveloper,
				},
			},
			expectRules:    6,
			expectNsViewer: false,
		},
		{
			name: "predefined viewer role sets needsNamespaceViewer",
			ag: &rbacmanagerv1alpha1.AccessGrant{
				Spec: rbacmanagerv1alpha1.AccessGrantSpec{
					Role: rbacmanagerv1alpha1.RoleViewer,
				},
			},
			expectNsViewer: true,
		},
		{
			name: "predefined developer-extended role sets needsNamespaceViewer",
			ag: &rbacmanagerv1alpha1.AccessGrant{
				Spec: rbacmanagerv1alpha1.AccessGrantSpec{
					Role: rbacmanagerv1alpha1.RoleDeveloperExtended,
				},
			},
			expectNsViewer: true,
		},
		{
			name: "custom rules returned as-is with no namespace viewer",
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
			expectRules:    1,
			expectNsViewer: false,
		},
		{
			name: "roleTemplate found returns its rules",
			ag: &rbacmanagerv1alpha1.AccessGrant{
				ObjectMeta: metav1.ObjectMeta{Namespace: "rbac-manager"},
				Spec: rbacmanagerv1alpha1.AccessGrantSpec{
					RoleTemplateName: "my-template",
				},
			},
			extraObjs: []client.Object{
				&rbacmanagerv1alpha1.RoleTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-template",
						Namespace: "rbac-manager",
					},
					Spec: rbacmanagerv1alpha1.RoleTemplateSpec{
						Rules: []rbacv1.PolicyRule{
							{
								APIGroups: []string{"apps"},
								Resources: []string{"deployments"},
								Verbs:     []string{"get", "list", "patch"},
							},
							{
								APIGroups: []string{""},
								Resources: []string{"pods"},
								Verbs:     []string{"get", "list"},
							},
						},
					},
				},
			},
			expectRules:    2,
			expectNsViewer: false,
		},
		{
			name: "roleTemplate with needsNamespaceViewer=true propagates flag",
			ag: &rbacmanagerv1alpha1.AccessGrant{
				ObjectMeta: metav1.ObjectMeta{Namespace: "rbac-manager"},
				Spec: rbacmanagerv1alpha1.AccessGrantSpec{
					RoleTemplateName: "viewer-template",
				},
			},
			extraObjs: []client.Object{
				&rbacmanagerv1alpha1.RoleTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "viewer-template",
						Namespace: "rbac-manager",
					},
					Spec: rbacmanagerv1alpha1.RoleTemplateSpec{
						NeedsNamespaceViewer: true,
						Rules: []rbacv1.PolicyRule{
							{
								APIGroups: []string{""},
								Resources: []string{"pods"},
								Verbs:     []string{"get", "list"},
							},
						},
					},
				},
			},
			expectRules:    1,
			expectNsViewer: true,
		},
		{
			name: "roleTemplate not found returns error",
			ag: &rbacmanagerv1alpha1.AccessGrant{
				ObjectMeta: metav1.ObjectMeta{Namespace: "rbac-manager"},
				Spec: rbacmanagerv1alpha1.AccessGrantSpec{
					RoleTemplateName: "missing-template",
				},
			},
			expectError: true,
		},
		{
			name: "unknown predefined role returns error",
			ag: &rbacmanagerv1alpha1.AccessGrant{
				Spec: rbacmanagerv1alpha1.AccessGrantSpec{
					Role: rbacmanagerv1alpha1.PredefinedRole("unknown-role"),
				},
			},
			expectError: true,
		},
		{
			name: "nothing set returns error",
			ag: &rbacmanagerv1alpha1.AccessGrant{
				Spec: rbacmanagerv1alpha1.AccessGrantSpec{},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestReconciler(tt.extraObjs...)
			rules, nsViewer, err := r.resolveRole(context.Background(), tt.ag)

			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !tt.expectError && tt.expectRules > 0 && len(rules) != tt.expectRules {
				t.Errorf("expected %d rules, got %d", tt.expectRules, len(rules))
			}
			if !tt.expectError && nsViewer != tt.expectNsViewer {
				t.Errorf("expected needsNamespaceViewer=%v, got %v", tt.expectNsViewer, nsViewer)
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
				managedByLabel:     managerValue, // system label wins even when user tries to override
				accessGrantLabel:   "test-grant",
				accessGrantNsLabel: "test-ns",
				"custom-label":     "custom-value",
			},
			checkManagedBy: true,
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
