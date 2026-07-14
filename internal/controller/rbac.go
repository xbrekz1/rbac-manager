package controller

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	rbacmanagerv1alpha1 "github.com/xbrekz1/rbac-manager/api/v1alpha1"
	"github.com/xbrekz1/rbac-manager/internal/roles"
)

// reconcileResult holds the output of a successful reconciliation.
// It contains information about created resources that will be stored in the AccessGrant status.
type reconcileResult struct {
	namespaces        []string // List of namespaces where RBAC resources were created
	skippedNamespaces []string // List of namespaces that were skipped because they don't exist yet
	saName            string   // Name of the created ServiceAccount
	clusterRole       string   // Name of the ClusterRole (if created)
}

// reconcileRBAC orchestrates all RBAC resource creation/updates for an AccessGrant.
// It handles both namespace-scoped and cluster-wide RBAC configurations.
//
// For namespace-scoped mode (clusterWide: false):
//   - Creates a ServiceAccount in the AccessGrant's namespace
//   - Creates Role and RoleBinding in each target namespace
//   - Optionally creates a ClusterRole for namespace visibility (viewer, developer-extended)
//
// For cluster-wide mode (clusterWide: true):
//   - Creates a ServiceAccount in the AccessGrant's namespace
//   - Creates a ClusterRole and ClusterRoleBinding with cluster-wide permissions
//
// Returns reconcileResult containing created resource names, or an error if reconciliation failed.
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

	policyRules, needsNsViewer, err := r.resolveRole(ctx, ag)
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

	// Namespace-viewer ClusterRole (for viewer, developer-extended, or RoleTemplate with flag set).
	if needsNsViewer {
		nsViewerName := roleName + "-namespace-viewer"
		if err := r.reconcileNamespaceViewerClusterRole(ctx, ag, nsViewerName, saName); err != nil {
			return nil, fmt.Errorf("reconciling namespace-viewer ClusterRole: %w", err)
		}
		result.clusterRole = nsViewerName
	}

	// Namespace-scoped roles.
	var reconciledNamespaces, skippedNamespaces []string
	for _, ns := range ag.Spec.Namespaces {
		// Verify the namespace exists.
		nsObj := &corev1.Namespace{}
		if err := r.Get(ctx, types.NamespacedName{Name: ns}, nsObj); err != nil {
			if apierrors.IsNotFound(err) {
				logger.Info("Target namespace does not exist, skipping — will retry on next reconcile", "namespace", ns)
				skippedNamespaces = append(skippedNamespaces, ns)
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
	result.skippedNamespaces = skippedNamespaces
	return result, nil
}

// resolveRole resolves the effective RBAC rules and namespace-viewer flag for an AccessGrant.
// Priority: spec.roleTemplate → spec.role → spec.customRules.
func (r *AccessGrantReconciler) resolveRole(ctx context.Context, ag *rbacmanagerv1alpha1.AccessGrant) ([]rbacv1.PolicyRule, bool, error) {
	if ag.Spec.RoleTemplateName != "" {
		rt := &rbacmanagerv1alpha1.RoleTemplate{}
		if err := r.Get(ctx, types.NamespacedName{Name: ag.Spec.RoleTemplateName, Namespace: ag.Namespace}, rt); err != nil {
			return nil, false, fmt.Errorf("fetching RoleTemplate %q: %w", ag.Spec.RoleTemplateName, err)
		}
		return rt.Spec.Rules, rt.Spec.NeedsNamespaceViewer, nil
	}
	if ag.Spec.Role != "" {
		rules, ok := roles.GetPredefinedRules(string(ag.Spec.Role))
		if !ok {
			return nil, false, fmt.Errorf("unknown predefined role: %q", ag.Spec.Role)
		}
		return rules, roles.NeedsNamespaceViewer(string(ag.Spec.Role)), nil
	}
	if len(ag.Spec.CustomRules) > 0 {
		return ag.Spec.CustomRules, false, nil
	}
	return nil, false, fmt.Errorf("one of spec.role, spec.roleTemplate, or spec.customRules must be specified")
}

// resourceLabels returns the standard labels for managed RBAC resources, merged with user labels.
// These labels are applied to all resources created by the operator (ServiceAccounts, Roles, etc.)
// and are used for resource cleanup when the AccessGrant is deleted.
//
// Standard labels include:
//   - rbacmanager.io/managed-by: identifies resources managed by this operator
//   - rbacmanager.io/access-grant: name of the parent AccessGrant
//   - rbacmanager.io/access-grant-namespace: namespace of the parent AccessGrant
//
// User-defined labels from spec.labels are merged in first; system labels are written last
// and always take precedence over user labels with the same key.
func resourceLabels(ag *rbacmanagerv1alpha1.AccessGrant) map[string]string {
	lbls := make(map[string]string, len(ag.Spec.Labels)+3)
	// User labels first so system labels always take precedence.
	for k, v := range ag.Spec.Labels {
		lbls[k] = v
	}
	lbls[managedByLabel] = managerValue
	lbls[accessGrantLabel] = ag.Name
	lbls[accessGrantNsLabel] = ag.Namespace
	return lbls
}

// reconcileServiceAccount creates or updates the ServiceAccount in the AccessGrant's namespace.
// The ServiceAccount is always created in the same namespace as the AccessGrant, regardless of
// which namespaces it will have permissions in.
//
// Owner references are set to enable automatic garbage collection when the AccessGrant is deleted.
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
		// Set owner reference for automatic garbage collection
		return controllerutil.SetControllerReference(ag, sa, r.Scheme)
	})
	return err
}

// reconcileRole creates or updates a Role in the given namespace.
// The Role contains the RBAC policy rules that define what actions the ServiceAccount can perform.
//
// Note: Owner references are NOT set on namespace-scoped resources in different namespaces
// because Kubernetes doesn't support cross-namespace owner references. Instead, cleanup
// is handled via label selectors in the cleanupRBAC function.
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
// The RoleBinding connects the Role to the ServiceAccount, granting the ServiceAccount
// the permissions defined in the Role.
//
// Note: Like Roles, cross-namespace owner references are not supported, so cleanup
// is handled via label selectors.
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
// This is used when clusterWide: true is set, granting cluster-wide permissions.
//
// ClusterRoles and ClusterRoleBindings are cluster-scoped resources, so they cannot have
// owner references to namespace-scoped AccessGrants. Cleanup is handled via label selectors.
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
//
// This is needed for certain roles (viewer, developer-extended) to enable namespace listing
// in tools like OpenLens, which require get/list/watch permissions on namespaces to display
// the namespace sidebar correctly.
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
// It attempts to delete all resources even if individual deletions fail, returning all errors.
func (r *AccessGrantReconciler) cleanupRBAC(ctx context.Context, ag *rbacmanagerv1alpha1.AccessGrant) error {
	logger := log.FromContext(ctx)
	selector := client.MatchingLabels{
		accessGrantLabel:   ag.Name,
		accessGrantNsLabel: ag.Namespace,
		managedByLabel:     managerValue,
	}

	crbList := &rbacv1.ClusterRoleBindingList{}
	crList := &rbacv1.ClusterRoleList{}
	rbList := &rbacv1.RoleBindingList{}
	roleList := &rbacv1.RoleList{}
	saList := &corev1.ServiceAccountList{}

	type step struct {
		kind string
		list client.ObjectList
		objs func() []client.Object
	}
	steps := []step{
		{"ClusterRoleBinding", crbList, func() []client.Object { return asObjects[rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBinding](crbList.Items) }},
		{"ClusterRole", crList, func() []client.Object { return asObjects[rbacv1.ClusterRole, *rbacv1.ClusterRole](crList.Items) }},
		{"RoleBinding", rbList, func() []client.Object { return asObjects[rbacv1.RoleBinding, *rbacv1.RoleBinding](rbList.Items) }},
		{"Role", roleList, func() []client.Object { return asObjects[rbacv1.Role, *rbacv1.Role](roleList.Items) }},
		{"ServiceAccount", saList, func() []client.Object { return asObjects[corev1.ServiceAccount, *corev1.ServiceAccount](saList.Items) }},
	}

	var errs []error
	for _, s := range steps {
		if err := r.List(ctx, s.list, selector); err != nil {
			errs = append(errs, fmt.Errorf("listing %s: %w", s.kind, err))
			continue
		}
		for _, obj := range s.objs() {
			ns, name := obj.GetNamespace(), obj.GetName()
			if ns != "" {
				logger.Info("Deleting "+s.kind, "name", name, "namespace", ns)
			} else {
				logger.Info("Deleting "+s.kind, "name", name)
			}
			if err := r.Delete(ctx, obj); client.IgnoreNotFound(err) != nil {
				id := name
				if ns != "" {
					id = ns + "/" + name
				}
				errs = append(errs, fmt.Errorf("deleting %s %s: %w", s.kind, id, err))
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("cleanup encountered %d error(s): %w", len(errs), errors.Join(errs...))
	}
	return nil
}

// asObjects converts a typed slice into []client.Object by taking the address of each element.
// P must be *T and implement client.Object.
func asObjects[T any, P interface {
	*T
	client.Object
}](items []T) []client.Object {
	objs := make([]client.Object, len(items))
	for i := range items {
		objs[i] = P(&items[i])
	}
	return objs
}
