package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	rbacmanagerv1alpha1 "github.com/xbrekz1/rbac-manager/api/v1alpha1"
	"github.com/xbrekz1/rbac-manager/internal/roles"
)

// reconcileResult holds the output of a successful reconciliation.
type reconcileResult struct {
	namespaces  []string
	saName      string
	clusterRole string
}

// reconcileRBAC orchestrates all RBAC resource creation/updates for an AccessGrant.
func (r *AccessGrantReconciler) reconcileRBAC(ctx context.Context, ag *rbacmanagerv1alpha1.AccessGrant) (*reconcileResult, error) {
	logger := log.FromContext(ctx)

	// Determine the ServiceAccount name.
	saName := ag.Spec.ServiceAccountName
	if saName == "" {
		saName = "rbac-" + ag.Name
	}

	// Ensure ServiceAccount exists in the AccessGrant's own namespace.
	if err := r.reconcileServiceAccount(ctx, ag, saName); err != nil {
		return nil, fmt.Errorf("reconciling ServiceAccount: %w", err)
	}

	policyRules, err := getPolicyRules(ag)
	if err != nil {
		return nil, err
	}

	result := &reconcileResult{
		saName: saName,
	}

	roleName := "rbac-" + ag.Name

	// ClusterWide mode: create ClusterRole + ClusterRoleBinding only.
	if ag.Spec.ClusterWide {
		if err := r.reconcileClusterRole(ctx, ag, roleName, policyRules, saName); err != nil {
			return nil, fmt.Errorf("reconciling ClusterRole: %w", err)
		}
		result.clusterRole = roleName
		return result, nil
	}

	// Namespace-viewer ClusterRole (for viewer and developer-extended).
	if roles.NeedsNamespaceViewer(string(ag.Spec.Role)) {
		nsViewerName := roleName + "-namespace-viewer"
		if err := r.reconcileNamespaceViewerClusterRole(ctx, ag, nsViewerName, saName); err != nil {
			return nil, fmt.Errorf("reconciling namespace-viewer ClusterRole: %w", err)
		}
		result.clusterRole = nsViewerName
	}

	// Namespace-scoped roles.
	var reconciledNamespaces []string
	for _, ns := range ag.Spec.Namespaces {
		// Verify the namespace exists.
		nsObj := &corev1.Namespace{}
		if err := r.Get(ctx, types.NamespacedName{Name: ns}, nsObj); err != nil {
			if errors.IsNotFound(err) {
				logger.Info("Target namespace does not exist, skipping — will retry on next reconcile", "namespace", ns)
				continue
			}
			return nil, fmt.Errorf("checking namespace %s: %w", ns, err)
		}

		if err := r.reconcileRole(ctx, ag, roleName, ns, policyRules); err != nil {
			return nil, fmt.Errorf("reconciling Role in namespace %s: %w", ns, err)
		}

		if err := r.reconcileRoleBinding(ctx, ag, roleName, ns, roleName, saName); err != nil {
			return nil, fmt.Errorf("reconciling RoleBinding in namespace %s: %w", ns, err)
		}

		reconciledNamespaces = append(reconciledNamespaces, ns)
	}

	result.namespaces = reconciledNamespaces
	return result, nil
}

// getPolicyRules resolves the effective RBAC rules for an AccessGrant.
func getPolicyRules(ag *rbacmanagerv1alpha1.AccessGrant) ([]rbacv1.PolicyRule, error) {
	if ag.Spec.Role != "" {
		rules, ok := roles.GetPredefinedRules(string(ag.Spec.Role))
		if !ok {
			return nil, fmt.Errorf("unknown predefined role: %q", ag.Spec.Role)
		}
		return rules, nil
	}
	if len(ag.Spec.CustomRules) > 0 {
		return ag.Spec.CustomRules, nil
	}
	return nil, fmt.Errorf("either spec.role or spec.customRules must be specified")
}

// resourceLabels returns the standard labels for managed RBAC resources, merged with user labels.
func resourceLabels(ag *rbacmanagerv1alpha1.AccessGrant) map[string]string {
	lbls := map[string]string{
		managedByLabel:     managerValue,
		accessGrantLabel:   ag.Name,
		accessGrantNsLabel: ag.Namespace,
	}
	for k, v := range ag.Spec.Labels {
		lbls[k] = v
	}
	return lbls
}

// reconcileServiceAccount creates or updates the ServiceAccount in the AccessGrant's namespace.
func (r *AccessGrantReconciler) reconcileServiceAccount(ctx context.Context, ag *rbacmanagerv1alpha1.AccessGrant, saName string) error {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      saName,
			Namespace: ag.Namespace,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, sa, func() error {
		sa.Labels = resourceLabels(ag)
		sa.Annotations = ag.Spec.Annotations
		return nil
	})
	return err
}

// reconcileRole creates or updates a Role in the given namespace.
func (r *AccessGrantReconciler) reconcileRole(ctx context.Context, ag *rbacmanagerv1alpha1.AccessGrant, name, namespace string, rules []rbacv1.PolicyRule) error {
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, role, func() error {
		role.Labels = resourceLabels(ag)
		role.Annotations = ag.Spec.Annotations
		role.Rules = rules
		return nil
	})
	return err
}

// reconcileRoleBinding creates or updates a RoleBinding in the given namespace.
func (r *AccessGrantReconciler) reconcileRoleBinding(ctx context.Context, ag *rbacmanagerv1alpha1.AccessGrant, name, namespace, roleName, saName string) error {
	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, rb, func() error {
		rb.Labels = resourceLabels(ag)
		rb.Annotations = ag.Spec.Annotations
		rb.RoleRef = rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     roleName,
		}
		rb.Subjects = []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      saName,
				Namespace: ag.Namespace,
			},
		}
		return nil
	})
	return err
}

// reconcileClusterRole creates or updates a ClusterRole and its ClusterRoleBinding.
func (r *AccessGrantReconciler) reconcileClusterRole(ctx context.Context, ag *rbacmanagerv1alpha1.AccessGrant, name string, rules []rbacv1.PolicyRule, saName string) error {
	cr := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, cr, func() error {
		cr.Labels = resourceLabels(ag)
		cr.Annotations = ag.Spec.Annotations
		cr.Rules = rules
		return nil
	}); err != nil {
		return fmt.Errorf("CreateOrUpdate ClusterRole %s: %w", name, err)
	}

	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, crb, func() error {
		crb.Labels = resourceLabels(ag)
		crb.Annotations = ag.Spec.Annotations
		crb.RoleRef = rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     name,
		}
		crb.Subjects = []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      saName,
				Namespace: ag.Namespace,
			},
		}
		return nil
	}); err != nil {
		return fmt.Errorf("CreateOrUpdate ClusterRoleBinding %s: %w", name, err)
	}

	return nil
}

// reconcileNamespaceViewerClusterRole creates or updates a ClusterRole that grants list/get/watch on namespaces,
// and a ClusterRoleBinding for the given ServiceAccount.
func (r *AccessGrantReconciler) reconcileNamespaceViewerClusterRole(ctx context.Context, ag *rbacmanagerv1alpha1.AccessGrant, name, saName string) error {
	nsViewerRules := []rbacv1.PolicyRule{
		{
			APIGroups: []string{""},
			Resources: []string{"namespaces"},
			Verbs:     []string{"get", "list", "watch"},
		},
	}
	return r.reconcileClusterRole(ctx, ag, name, nsViewerRules, saName)
}

// cleanupRBAC deletes all RBAC resources managed by the given AccessGrant using label selectors.
func (r *AccessGrantReconciler) cleanupRBAC(ctx context.Context, ag *rbacmanagerv1alpha1.AccessGrant) error {
	logger := log.FromContext(ctx)

	labelSelector := client.MatchingLabels{
		accessGrantLabel:   ag.Name,
		accessGrantNsLabel: ag.Namespace,
		managedByLabel:     managerValue,
	}

	// Delete ClusterRoleBindings.
	crbList := &rbacv1.ClusterRoleBindingList{}
	if err := r.List(ctx, crbList, labelSelector); err != nil {
		return fmt.Errorf("listing ClusterRoleBindings: %w", err)
	}
	for i := range crbList.Items {
		logger.Info("Deleting ClusterRoleBinding", "name", crbList.Items[i].Name)
		if err := r.Delete(ctx, &crbList.Items[i]); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("deleting ClusterRoleBinding %s: %w", crbList.Items[i].Name, err)
		}
	}

	// Delete ClusterRoles.
	crList := &rbacv1.ClusterRoleList{}
	if err := r.List(ctx, crList, labelSelector); err != nil {
		return fmt.Errorf("listing ClusterRoles: %w", err)
	}
	for i := range crList.Items {
		logger.Info("Deleting ClusterRole", "name", crList.Items[i].Name)
		if err := r.Delete(ctx, &crList.Items[i]); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("deleting ClusterRole %s: %w", crList.Items[i].Name, err)
		}
	}

	// Delete RoleBindings across all namespaces.
	rbList := &rbacv1.RoleBindingList{}
	if err := r.List(ctx, rbList, labelSelector); err != nil {
		return fmt.Errorf("listing RoleBindings: %w", err)
	}
	for i := range rbList.Items {
		logger.Info("Deleting RoleBinding", "name", rbList.Items[i].Name, "namespace", rbList.Items[i].Namespace)
		if err := r.Delete(ctx, &rbList.Items[i]); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("deleting RoleBinding %s/%s: %w", rbList.Items[i].Namespace, rbList.Items[i].Name, err)
		}
	}

	// Delete Roles across all namespaces.
	roleList := &rbacv1.RoleList{}
	if err := r.List(ctx, roleList, labelSelector); err != nil {
		return fmt.Errorf("listing Roles: %w", err)
	}
	for i := range roleList.Items {
		logger.Info("Deleting Role", "name", roleList.Items[i].Name, "namespace", roleList.Items[i].Namespace)
		if err := r.Delete(ctx, &roleList.Items[i]); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("deleting Role %s/%s: %w", roleList.Items[i].Namespace, roleList.Items[i].Name, err)
		}
	}

	// Delete ServiceAccounts.
	saList := &corev1.ServiceAccountList{}
	if err := r.List(ctx, saList, labelSelector); err != nil {
		return fmt.Errorf("listing ServiceAccounts: %w", err)
	}
	for i := range saList.Items {
		logger.Info("Deleting ServiceAccount", "name", saList.Items[i].Name, "namespace", saList.Items[i].Namespace)
		if err := r.Delete(ctx, &saList.Items[i]); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("deleting ServiceAccount %s/%s: %w", saList.Items[i].Namespace, saList.Items[i].Name, err)
		}
	}

	return nil
}
