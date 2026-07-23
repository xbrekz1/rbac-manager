package v1alpha1

import (
	"context"
	"strings"
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestRoleTemplateValidateCreate(t *testing.T) {
	tests := []struct {
		name        string
		rt          *RoleTemplate
		wantErr     bool
		errContains string
	}{
		{
			name: "valid rules",
			rt: &RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{Name: "test-rt", Namespace: "default"},
				Spec: RoleTemplateSpec{
					Rules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{""},
							Resources: []string{"pods"},
							Verbs:     []string{"get", "list"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid with needsNamespaceViewer",
			rt: &RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{Name: "test-rt", Namespace: "default"},
				Spec: RoleTemplateSpec{
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
			wantErr: false,
		},
		{
			name: "error: no rules",
			rt: &RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{Name: "test-rt", Namespace: "default"},
				Spec:       RoleTemplateSpec{},
			},
			wantErr:     true,
			errContains: "spec.rules must contain at least one policy rule",
		},
		{
			name: "error: rule with no verbs",
			rt: &RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{Name: "test-rt", Namespace: "default"},
				Spec: RoleTemplateSpec{
					Rules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{""},
							Resources: []string{"pods"},
							Verbs:     []string{},
						},
					},
				},
			},
			wantErr:     true,
			errContains: "verbs must be specified",
		},
		{
			name: "error: rule with no resources or nonResourceURLs",
			rt: &RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{Name: "test-rt", Namespace: "default"},
				Spec: RoleTemplateSpec{
					Rules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{""},
							Verbs:     []string{"get"},
						},
					},
				},
			},
			wantErr:     true,
			errContains: "either resources or nonResourceURLs must be specified",
		},
		{
			name: "error: wildcard combination",
			rt: &RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{Name: "test-rt", Namespace: "default"},
				Spec: RoleTemplateSpec{
					Rules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{"*"},
							Resources: []string{"*"},
							Verbs:     []string{"*"},
						},
					},
				},
			},
			wantErr:     true,
			errContains: "wildcard apiGroups/resources/verbs is not allowed",
		},
		{
			name: "error: RBAC resources clusterroles",
			rt: &RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{Name: "test-rt", Namespace: "default"},
				Spec: RoleTemplateSpec{
					Rules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{"rbac.authorization.k8s.io"},
							Resources: []string{"clusterroles"},
							Verbs:     []string{"get", "list"},
						},
					},
				},
			},
			wantErr:     true,
			errContains: "RBAC resources",
		},
		{
			name: "error: RBAC resources with wildcard apiGroup",
			rt: &RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{Name: "test-rt", Namespace: "default"},
				Spec: RoleTemplateSpec{
					Rules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{"*"},
							Resources: []string{"rolebindings"},
							Verbs:     []string{"*"},
						},
					},
				},
			},
			wantErr:     true,
			errContains: "RBAC resources",
		},
		{
			name: "error: wildcard resources in RBAC apiGroup",
			rt: &RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{Name: "test-rt", Namespace: "default"},
				Spec: RoleTemplateSpec{
					Rules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{"rbac.authorization.k8s.io"},
							Resources: []string{"*"},
							Verbs:     []string{"get"},
						},
					},
				},
			},
			wantErr:     true,
			errContains: "RBAC resources",
		},
		{
			name: "valid: non-RBAC resources with multiple rules",
			rt: &RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{Name: "test-rt", Namespace: "default"},
				Spec: RoleTemplateSpec{
					Rules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{""},
							Resources: []string{"pods", "services"},
							Verbs:     []string{"get", "list"},
						},
						{
							APIGroups: []string{"apps"},
							Resources: []string{"deployments"},
							Verbs:     []string{"get", "list", "patch"},
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := (&RoleTemplateCustomValidator{}).ValidateCreate(context.Background(), tt.rt)

			if tt.wantErr && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tt.wantErr && tt.errContains != "" && err != nil {
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error = %v, should contain %q", err, tt.errContains)
				}
			}
		})
	}
}

func TestRoleTemplateValidateUpdate(t *testing.T) {
	old := &RoleTemplate{
		ObjectMeta: metav1.ObjectMeta{Name: "test-rt", Namespace: "default"},
		Spec: RoleTemplateSpec{
			Rules: []rbacv1.PolicyRule{
				{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"get"}},
			},
		},
	}

	t.Run("valid update", func(t *testing.T) {
		newRT := &RoleTemplate{
			ObjectMeta: metav1.ObjectMeta{Name: "test-rt", Namespace: "default"},
			Spec: RoleTemplateSpec{
				Rules: []rbacv1.PolicyRule{
					{APIGroups: []string{""}, Resources: []string{"pods", "services"}, Verbs: []string{"get", "list"}},
				},
			},
		}
		if _, err := (&RoleTemplateCustomValidator{}).ValidateUpdate(context.Background(), old, newRT); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("invalid update: RBAC resources", func(t *testing.T) {
		newRT := &RoleTemplate{
			ObjectMeta: metav1.ObjectMeta{Name: "test-rt", Namespace: "default"},
			Spec: RoleTemplateSpec{
				Rules: []rbacv1.PolicyRule{
					{APIGroups: []string{"rbac.authorization.k8s.io"}, Resources: []string{"clusterroles"}, Verbs: []string{"*"}},
				},
			},
		}
		if _, err := (&RoleTemplateCustomValidator{}).ValidateUpdate(context.Background(), old, newRT); err == nil {
			t.Error("expected error but got none")
		}
	})
}

func TestRoleTemplateValidateDelete(t *testing.T) {
	rt := &RoleTemplate{
		ObjectMeta: metav1.ObjectMeta{Name: "test-rt", Namespace: "default"},
		Spec: RoleTemplateSpec{
			Rules: []rbacv1.PolicyRule{
				{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"get"}},
			},
		},
	}
	if _, err := (&RoleTemplateCustomValidator{}).ValidateDelete(context.Background(), rt); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
