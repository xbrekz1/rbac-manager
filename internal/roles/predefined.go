package roles

import (
	rbacv1 "k8s.io/api/rbac/v1"
)

// predefinedRoles maps role names to their policy rules.
//
// This package provides 10 predefined RBAC roles covering common access patterns
// in Kubernetes environments. Roles are organized into a hierarchy for standard
// development workflows, plus specialized roles for CI/CD, debugging, and auditing.
//
// Role hierarchy (least → most privileged):
//
//	reader → viewer → developer → developer-extended → operator → maintainer
//
// Specialized roles (orthogonal to hierarchy):
//
//	deployer  (CI/CD pipelines)
//	debugger  (runtime debugging, incident response)
//	auditor   (security review, compliance)
//	cluster-admin (full cluster access)
//
// Each role is designed with the principle of least privilege, granting only
// the permissions necessary for its intended use case.
var predefinedRoles = map[string][]rbacv1.PolicyRule{

	// reader — minimal read-only access.
	// Can see workload resources, but NOT logs, NOT exec, NOT secrets.
	// Use case: product managers, stakeholders, external auditors.
	"reader": {
		{
			APIGroups: []string{""},
			Resources: []string{"pods", "services", "endpoints", "persistentvolumeclaims"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"configmaps"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{"apps"},
			Resources: []string{"deployments", "statefulsets", "daemonsets", "replicasets"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"events"},
			Verbs:     []string{"get", "list", "watch"},
		},
	},

	// viewer — view pods and logs.
	// Use case: monitoring teams, on-call without intervention rights.
	"viewer": {
		{
			APIGroups: []string{""},
			Resources: []string{"pods", "services", "endpoints"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"pods/log"},
			Verbs:     []string{"get", "list"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"configmaps"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{"apps"},
			Resources: []string{"deployments", "statefulsets", "daemonsets", "replicasets"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"events"},
			Verbs:     []string{"get", "list", "watch"},
		},
	},

	// developer — debug applications within a namespace.
	// viewer + pod exec + read secrets.
	// Use case: developers, QA engineers.
	"developer": {
		{
			APIGroups: []string{""},
			Resources: []string{"pods", "services", "endpoints"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"pods/log"},
			Verbs:     []string{"get", "list"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"pods/exec"},
			Verbs:     []string{"create"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"configmaps", "secrets"},
			Verbs:     []string{"get", "list"},
		},
		{
			APIGroups: []string{"apps"},
			Resources: []string{"deployments", "statefulsets", "daemonsets", "replicasets"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"events"},
			Verbs:     []string{"get", "list", "watch"},
		},
	},

	// developer-extended — extended developer access.
	// Same as developer, but additionally receives a ClusterRole granting
	// namespace listing — required for OpenLens sidebar navigation
	// (without it, the namespace list in the left panel is empty).
	// Can also restart deployments via kubectl rollout restart.
	"developer-extended": {
		{
			APIGroups: []string{""},
			Resources: []string{"pods", "services", "endpoints"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"pods/log"},
			Verbs:     []string{"get", "list"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"pods/exec"},
			Verbs:     []string{"create"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"configmaps", "secrets"},
			Verbs:     []string{"get", "list"},
		},
		{
			APIGroups: []string{"apps"},
			Resources: []string{"deployments"},
			Verbs:     []string{"get", "list", "watch", "update", "patch"},
		},
		{
			APIGroups: []string{"apps"},
			Resources: []string{"statefulsets", "daemonsets", "replicasets"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"events", "persistentvolumeclaims"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{"networking.k8s.io"},
			Resources: []string{"ingresses"},
			Verbs:     []string{"get", "list", "watch"},
		},
	},

	// deployer — application deployment (CI/CD).
	// Can update workload resources, but cannot exec or read secrets.
	// Use case: GitLab CI, GitHub Actions, ArgoCD service accounts.
	"deployer": {
		{
			APIGroups: []string{"apps"},
			Resources: []string{"deployments", "statefulsets", "daemonsets"},
			Verbs:     []string{"get", "list", "watch", "create", "update", "patch"},
		},
		{
			APIGroups: []string{"apps"},
			Resources: []string{"replicasets"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"pods"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"pods/log"},
			Verbs:     []string{"get", "list"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"services", "endpoints"},
			Verbs:     []string{"get", "list", "watch", "create", "update", "patch"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"configmaps"},
			Verbs:     []string{"get", "list", "watch", "create", "update", "patch"},
		},
		{
			APIGroups: []string{"batch"},
			Resources: []string{"jobs", "cronjobs"},
			Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
		},
		{
			APIGroups: []string{"networking.k8s.io"},
			Resources: []string{"ingresses"},
			Verbs:     []string{"get", "list", "watch", "create", "update", "patch"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"events"},
			Verbs:     []string{"get", "list", "watch"},
		},
	},

	// debugger — deep runtime debugging.
	// Can exec into pods, view logs, port-forward — without permission to change anything.
	// Use case: SRE during an incident, temporary access for an external specialist.
	"debugger": {
		{
			APIGroups: []string{""},
			Resources: []string{"pods"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"pods/log"},
			Verbs:     []string{"get", "list"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"pods/exec"},
			Verbs:     []string{"create"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"pods/portforward"},
			Verbs:     []string{"create"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"services", "endpoints"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"configmaps"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{"apps"},
			Resources: []string{"deployments", "statefulsets", "daemonsets", "replicasets"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"events"},
			Verbs:     []string{"get", "list", "watch"},
		},
	},

	// operator — manage workloads without access to cluster secrets.
	// Can restart pods, update deployments, manage services.
	// Use case: SRE, platform team, on-call engineers.
	"operator": {
		{
			APIGroups: []string{""},
			Resources: []string{"pods"},
			Verbs:     []string{"get", "list", "watch", "delete"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"pods/log"},
			Verbs:     []string{"get", "list"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"pods/exec"},
			Verbs:     []string{"create"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"pods/portforward"},
			Verbs:     []string{"create"},
		},
		{
			APIGroups: []string{"apps"},
			Resources: []string{"deployments", "statefulsets", "daemonsets"},
			Verbs:     []string{"get", "list", "watch", "update", "patch"},
		},
		{
			APIGroups: []string{"apps"},
			Resources: []string{"replicasets"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"services", "endpoints"},
			Verbs:     []string{"get", "list", "watch", "create", "update", "patch"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"configmaps"},
			Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"secrets"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"persistentvolumeclaims"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{"networking.k8s.io"},
			Resources: []string{"ingresses"},
			Verbs:     []string{"get", "list", "watch", "create", "update", "patch"},
		},
		{
			APIGroups: []string{"autoscaling"},
			Resources: []string{"horizontalpodautoscalers"},
			Verbs:     []string{"get", "list", "watch", "update", "patch"},
		},
		{
			APIGroups: []string{"batch"},
			Resources: []string{"jobs", "cronjobs"},
			Verbs:     []string{"get", "list", "watch", "create", "delete"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"events"},
			Verbs:     []string{"get", "list", "watch"},
		},
	},

	// auditor — read-only audit of the entire namespace.
	// Sees everything including secrets — for security audits.
	// Recommended to grant temporarily via an expires-at annotation.
	// Use case: security team, compliance auditors, external reviews.
	"auditor": {
		{
			APIGroups: []string{""},
			Resources: []string{"pods", "services", "endpoints", "configmaps", "secrets",
				"serviceaccounts", "persistentvolumeclaims", "events", "resourcequotas", "limitranges"},
			Verbs: []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"pods/log"},
			Verbs:     []string{"get", "list"},
		},
		{
			APIGroups: []string{"apps"},
			Resources: []string{"deployments", "statefulsets", "daemonsets", "replicasets"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{"batch"},
			Resources: []string{"jobs", "cronjobs"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{"networking.k8s.io"},
			Resources: []string{"ingresses", "networkpolicies"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{"rbac.authorization.k8s.io"},
			Resources: []string{"roles", "rolebindings"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{"autoscaling"},
			Resources: []string{"horizontalpodautoscalers"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{"policy"},
			Resources: []string{"poddisruptionbudgets"},
			Verbs:     []string{"get", "list", "watch"},
		},
	},

	// maintainer — full access to everything in the namespace.
	// Use case: tech leads, platform engineers, service owners.
	"maintainer": {
		{
			APIGroups: []string{"*"},
			Resources: []string{"*"},
			Verbs:     []string{"*"},
		},
	},

	// cluster-admin — full access to the entire cluster.
	// Equivalent to Kubernetes' built-in cluster-admin role.
	// IMPORTANT: always use with clusterWide: true, otherwise access is scoped to one namespace.
	// Use case: platform engineers, emergency access, deploying complex Helm charts.
	"cluster-admin": {
		{
			APIGroups: []string{"*"},
			Resources: []string{"*"},
			Verbs:     []string{"*"},
		},
		{
			NonResourceURLs: []string{"*"},
			Verbs:           []string{"*"},
		},
	},
}

// rolesNeedingNamespaceViewer lists roles that require an additional ClusterRole
// granting get/list/watch on namespaces. This is needed for tools like OpenLens
// that rely on namespace listing for sidebar navigation.
var rolesNeedingNamespaceViewer = map[string]bool{
	"viewer":             true,
	"developer-extended": true,
}

// GetPredefinedRules returns the RBAC policy rules for a given predefined role name.
//
// This function looks up a role by name and returns its associated PolicyRules.
// The rules define what Kubernetes API operations (verbs) can be performed on
// which resources (apiGroups and resources).
//
// Parameters:
//   - role: The name of the predefined role (e.g., "developer", "viewer", "maintainer")
//
// Returns:
//   - []rbacv1.PolicyRule: The policy rules for the role
//   - bool: true if the role exists, false otherwise
//
// Example:
//
//	rules, ok := GetPredefinedRules("developer")
//	if !ok {
//	    return fmt.Errorf("unknown role")
//	}
//	// Use rules to create a Role or ClusterRole
func GetPredefinedRules(role string) ([]rbacv1.PolicyRule, bool) {
	rules, ok := predefinedRoles[role]
	if !ok {
		return nil, false
	}
	return rules, true
}

// NeedsNamespaceViewer returns true if the given role requires additional ClusterRole
// permissions to list/get/watch namespaces.
//
// Some roles (viewer, developer-extended) need cluster-level permissions to view
// the list of namespaces, which is required by tools like OpenLens for proper
// sidebar navigation. When this function returns true, the controller will create
// an additional ClusterRole with namespace viewing permissions.
//
// Parameters:
//   - role: The name of the role to check
//
// Returns:
//   - bool: true if the role needs namespace viewer permissions, false otherwise
//
// Example:
//
//	if NeedsNamespaceViewer("developer-extended") {
//	    // Create additional ClusterRole for namespace visibility
//	}
func NeedsNamespaceViewer(role string) bool {
	return rolesNeedingNamespaceViewer[role]
}
