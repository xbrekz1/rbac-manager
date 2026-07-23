package v1alpha1

import (
	"context"
	"strings"
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAccessGrantValidateCreate(t *testing.T) {
	tests := []struct {
		name        string
		ag          *AccessGrant
		wantErr     bool
		errContains string
		wantWarning bool
	}{
		{
			name: "valid with predefined role",
			ag: &AccessGrant{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-grant",
					Namespace: "default",
				},
				Spec: AccessGrantSpec{
					Role:               RoleDeveloper,
					Namespaces:         []string{"test-ns"},
					ServiceAccountName: "test-sa",
				},
			},
			wantErr: false,
		},
		{
			name: "valid with custom rules",
			ag: &AccessGrant{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-grant",
					Namespace: "default",
				},
				Spec: AccessGrantSpec{
					CustomRules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{""},
							Resources: []string{"pods"},
							Verbs:     []string{"get", "list"},
						},
					},
					Namespaces:         []string{"test-ns"},
					ServiceAccountName: "test-sa",
				},
			},
			wantErr: false,
		},
		{
			name: "valid cluster-wide mode",
			ag: &AccessGrant{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-grant",
					Namespace: "default",
				},
				Spec: AccessGrantSpec{
					Role:               RoleClusterAdmin,
					ClusterWide:        true,
					ServiceAccountName: "test-sa",
				},
			},
			wantErr: false,
		},
		{
			name: "valid with roleTemplate",
			ag: &AccessGrant{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-grant",
					Namespace: "default",
				},
				Spec: AccessGrantSpec{
					RoleTemplateName: "my-template",
					Namespaces:       []string{"test-ns"},
				},
			},
			wantErr: false,
		},
		{
			name: "valid with roleTemplate and clusterWide",
			ag: &AccessGrant{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-grant",
					Namespace: "default",
				},
				Spec: AccessGrantSpec{
					RoleTemplateName: "my-template",
					ClusterWide:      true,
				},
			},
			wantErr: false,
		},
		{
			name: "error: both role and customRules",
			ag: &AccessGrant{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-grant",
					Namespace: "default",
				},
				Spec: AccessGrantSpec{
					Role: RoleDeveloper,
					CustomRules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{""},
							Resources: []string{"pods"},
							Verbs:     []string{"get"},
						},
					},
					Namespaces: []string{"test-ns"},
				},
			},
			wantErr:     true,
			errContains: "only one of spec.role, spec.roleTemplate, or spec.customRules may be specified",
		},
		{
			name: "error: roleTemplate and role together",
			ag: &AccessGrant{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-grant",
					Namespace: "default",
				},
				Spec: AccessGrantSpec{
					Role:             RoleDeveloper,
					RoleTemplateName: "my-template",
					Namespaces:       []string{"test-ns"},
				},
			},
			wantErr:     true,
			errContains: "only one of spec.role, spec.roleTemplate, or spec.customRules may be specified",
		},
		{
			name: "error: roleTemplate and customRules together",
			ag: &AccessGrant{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-grant",
					Namespace: "default",
				},
				Spec: AccessGrantSpec{
					RoleTemplateName: "my-template",
					CustomRules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{""},
							Resources: []string{"pods"},
							Verbs:     []string{"get"},
						},
					},
					Namespaces: []string{"test-ns"},
				},
			},
			wantErr:     true,
			errContains: "only one of spec.role, spec.roleTemplate, or spec.customRules may be specified",
		},
		{
			name: "error: neither role nor customRules nor roleTemplate",
			ag: &AccessGrant{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-grant",
					Namespace: "default",
				},
				Spec: AccessGrantSpec{
					Namespaces: []string{"test-ns"},
				},
			},
			wantErr:     true,
			errContains: "one of spec.role, spec.roleTemplate, or spec.customRules must be specified",
		},
		{
			name: "error: roleTemplate without namespaces when not clusterWide",
			ag: &AccessGrant{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-grant",
					Namespace: "default",
				},
				Spec: AccessGrantSpec{
					RoleTemplateName: "my-template",
					ClusterWide:      false,
				},
			},
			wantErr:     true,
			errContains: "spec.namespaces must be specified when clusterWide is false",
		},
		{
			name: "error: unknown role",
			ag: &AccessGrant{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-grant",
					Namespace: "default",
				},
				Spec: AccessGrantSpec{
					Role:       PredefinedRole("unknown-role"),
					Namespaces: []string{"test-ns"},
				},
			},
			wantErr:     true,
			errContains: "unknown predefined role",
		},
		{
			name: "error: customRules with no verbs",
			ag: &AccessGrant{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-grant",
					Namespace: "default",
				},
				Spec: AccessGrantSpec{
					CustomRules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{""},
							Resources: []string{"pods"},
							Verbs:     []string{},
						},
					},
					Namespaces: []string{"test-ns"},
				},
			},
			wantErr:     true,
			errContains: "verbs must be specified",
		},
		{
			name: "error: customRules with no resources or nonResourceURLs",
			ag: &AccessGrant{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-grant",
					Namespace: "default",
				},
				Spec: AccessGrantSpec{
					CustomRules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{""},
							Verbs:     []string{"get"},
						},
					},
					Namespaces: []string{"test-ns"},
				},
			},
			wantErr:     true,
			errContains: "either resources or nonResourceURLs must be specified",
		},
		{
			name: "error: no namespaces when not clusterWide",
			ag: &AccessGrant{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-grant",
					Namespace: "default",
				},
				Spec: AccessGrantSpec{
					Role:        RoleDeveloper,
					ClusterWide: false,
				},
			},
			wantErr:     true,
			errContains: "spec.namespaces must be specified when clusterWide is false",
		},
		{
			name: "error: customRules without namespaces when not clusterWide",
			ag: &AccessGrant{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-grant",
					Namespace: "default",
				},
				Spec: AccessGrantSpec{
					CustomRules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{""},
							Resources: []string{"pods"},
							Verbs:     []string{"get", "list"},
						},
					},
					ClusterWide: false,
					// No Namespaces — should be rejected
				},
			},
			wantErr:     true,
			errContains: "spec.namespaces must be specified when clusterWide is false",
		},
		{
			name: "warning: namespaces with clusterWide",
			ag: &AccessGrant{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-grant",
					Namespace: "default",
				},
				Spec: AccessGrantSpec{
					Role:        RoleViewer,
					ClusterWide: true,
					Namespaces:  []string{"test-ns"},
				},
			},
			wantErr:     false,
			wantWarning: true,
		},
		{
			name: "warning: cluster-admin without clusterWide",
			ag: &AccessGrant{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-grant",
					Namespace: "default",
				},
				Spec: AccessGrantSpec{
					Role:       RoleClusterAdmin,
					Namespaces: []string{"test-ns"},
				},
			},
			wantErr:     false,
			wantWarning: true,
		},
		{
			name: "error: serviceAccountName too long",
			ag: &AccessGrant{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-grant",
					Namespace: "default",
				},
				Spec: AccessGrantSpec{
					Role:               RoleDeveloper,
					Namespaces:         []string{"test-ns"},
					ServiceAccountName: string(make([]byte, 300)),
				},
			},
			wantErr:     true,
			errContains: "too long",
		},
		{
			name: "valid with all roles",
			ag: &AccessGrant{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-grant",
					Namespace: "default",
				},
				Spec: AccessGrantSpec{
					Role:       RoleReader,
					Namespaces: []string{"test-ns"},
				},
			},
			wantErr: false,
		},
		{
			name: "error: customRules with wildcard combination",
			ag: &AccessGrant{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-grant",
					Namespace: "default",
				},
				Spec: AccessGrantSpec{
					CustomRules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{"*"},
							Resources: []string{"*"},
							Verbs:     []string{"*"},
						},
					},
					Namespaces: []string{"test-ns"},
				},
			},
			wantErr:     true,
			errContains: "wildcard apiGroups/resources/verbs is not allowed",
		},
		{
			name: "error: customRules granting clusterroles",
			ag: &AccessGrant{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-grant",
					Namespace: "default",
				},
				Spec: AccessGrantSpec{
					CustomRules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{"rbac.authorization.k8s.io"},
							Resources: []string{"clusterroles"},
							Verbs:     []string{"get", "list"},
						},
					},
					Namespaces: []string{"test-ns"},
				},
			},
			wantErr:     true,
			errContains: "RBAC resources",
		},
		{
			name: "error: customRules granting rolebindings with wildcard apiGroup",
			ag: &AccessGrant{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-grant",
					Namespace: "default",
				},
				Spec: AccessGrantSpec{
					CustomRules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{"*"},
							Resources: []string{"rolebindings"},
							Verbs:     []string{"*"},
						},
					},
					Namespaces: []string{"test-ns"},
				},
			},
			wantErr:     true,
			errContains: "RBAC resources",
		},
		{
			name: "error: customRules granting wildcard resources in RBAC apiGroup",
			ag: &AccessGrant{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-grant",
					Namespace: "default",
				},
				Spec: AccessGrantSpec{
					CustomRules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{"rbac.authorization.k8s.io"},
							Resources: []string{"*"},
							Verbs:     []string{"get"},
						},
					},
					Namespaces: []string{"test-ns"},
				},
			},
			wantErr:     true,
			errContains: "RBAC resources",
		},
		{
			name: "valid: customRules with non-RBAC resources in core group",
			ag: &AccessGrant{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-grant",
					Namespace: "default",
				},
				Spec: AccessGrantSpec{
					CustomRules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{""},
							Resources: []string{"pods", "configmaps"},
							Verbs:     []string{"get", "list"},
						},
					},
					Namespaces: []string{"test-ns"},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warnings, err := (&AccessGrantCustomValidator{}).ValidateCreate(context.Background(), tt.ag)

			if tt.wantErr && err == nil {
				t.Errorf("ValidateCreate() expected error but got none")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("ValidateCreate() unexpected error: %v", err)
			}

			if tt.wantErr && tt.errContains != "" && err != nil {
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("ValidateCreate() error = %v, should contain %q", err, tt.errContains)
				}
			}

			if tt.wantWarning && len(warnings) == 0 {
				t.Errorf("ValidateCreate() expected warnings but got none")
			}

			if !tt.wantWarning && len(warnings) > 0 {
				t.Errorf("ValidateCreate() unexpected warnings: %v", warnings)
			}
		})
	}
}

func TestAccessGrantValidateUpdate(t *testing.T) {
	oldAG := &AccessGrant{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-grant",
			Namespace: "default",
		},
		Spec: AccessGrantSpec{
			Role:       RoleDeveloper,
			Namespaces: []string{"test-ns"},
		},
	}

	tests := []struct {
		name        string
		newAG       *AccessGrant
		wantErr     bool
		errContains string
	}{
		{
			name: "valid update",
			newAG: &AccessGrant{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-grant",
					Namespace: "default",
				},
				Spec: AccessGrantSpec{
					Role:       RoleOperator,
					Namespaces: []string{"test-ns", "test-ns-2"},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid update: both role and customRules",
			newAG: &AccessGrant{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-grant",
					Namespace: "default",
				},
				Spec: AccessGrantSpec{
					Role: RoleDeveloper,
					CustomRules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{""},
							Resources: []string{"pods"},
							Verbs:     []string{"get"},
						},
					},
					Namespaces: []string{"test-ns"},
				},
			},
			wantErr:     true,
			errContains: "only one of spec.role, spec.roleTemplate, or spec.customRules may be specified",
		},
		{
			name: "valid update: switch from role to roleTemplate",
			newAG: &AccessGrant{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-grant",
					Namespace: "default",
				},
				Spec: AccessGrantSpec{
					RoleTemplateName: "my-template",
					Namespaces:       []string{"test-ns"},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := (&AccessGrantCustomValidator{}).ValidateUpdate(context.Background(), oldAG, tt.newAG)

			if tt.wantErr && err == nil {
				t.Errorf("ValidateUpdate() expected error but got none")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("ValidateUpdate() unexpected error: %v", err)
			}

			if tt.wantErr && tt.errContains != "" && err != nil {
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("ValidateUpdate() error = %v, should contain %q", err, tt.errContains)
				}
			}
		})
	}
}

func TestAccessGrantValidateDelete(t *testing.T) {
	ag := &AccessGrant{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-grant",
			Namespace: "default",
		},
		Spec: AccessGrantSpec{
			Role:       RoleDeveloper,
			Namespaces: []string{"test-ns"},
		},
	}

	warnings, err := (&AccessGrantCustomValidator{}).ValidateDelete(context.Background(), ag)
	if err != nil {
		t.Errorf("ValidateDelete() unexpected error: %v", err)
	}

	if len(warnings) > 0 {
		t.Errorf("ValidateDelete() unexpected warnings: %v", warnings)
	}
}

func TestValidateAllPredefinedRoles(t *testing.T) {
	roles := []PredefinedRole{
		RoleReader,
		RoleViewer,
		RoleDeveloper,
		RoleDeveloperExtended,
		RoleDeployer,
		RoleDebugger,
		RoleOperator,
		RoleAuditor,
		RoleMaintainer,
		RoleClusterAdmin,
	}

	for _, role := range roles {
		t.Run(string(role), func(t *testing.T) {
			ag := &AccessGrant{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-grant",
					Namespace: "default",
				},
				Spec: AccessGrantSpec{
					Role:       role,
					Namespaces: []string{"test-ns"},
				},
			}

			_, err := (&AccessGrantCustomValidator{}).ValidateCreate(context.Background(), ag)
			if err != nil {
				t.Errorf("ValidateCreate() for role %q failed: %v", role, err)
			}
		})
	}
}
