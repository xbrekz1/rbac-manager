package v1alpha1

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var accessgrantlog = logf.Log.WithName("accessgrant-resource")

// SetupWebhookWithManager registers the webhook for AccessGrant in the manager.
func (r *AccessGrant) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:path=/validate-rbacmanager-io-v1alpha1-accessgrant,mutating=false,failurePolicy=fail,sideEffects=None,groups=rbacmanager.io,resources=accessgrants,verbs=create;update,versions=v1alpha1,name=vaccessgrant.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &AccessGrant{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *AccessGrant) ValidateCreate() (admission.Warnings, error) {
	accessgrantlog.Info("validate create", "name", r.Name)

	return r.validateAccessGrant()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *AccessGrant) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	accessgrantlog.Info("validate update", "name", r.Name)

	return r.validateAccessGrant()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *AccessGrant) ValidateDelete() (admission.Warnings, error) {
	accessgrantlog.Info("validate delete", "name", r.Name)

	// No validation needed for delete
	return nil, nil
}

// validateAccessGrant contains the validation logic for AccessGrant.
func (r *AccessGrant) validateAccessGrant() (admission.Warnings, error) {
	var warnings admission.Warnings

	// Validate that either Role or CustomRules is specified, but not both
	if r.Spec.Role != "" && len(r.Spec.CustomRules) > 0 {
		return warnings, fmt.Errorf("either spec.role or spec.customRules must be specified, but not both")
	}

	if r.Spec.Role == "" && len(r.Spec.CustomRules) == 0 {
		return warnings, fmt.Errorf("either spec.role or spec.customRules must be specified")
	}

	// Validate predefined role exists
	if r.Spec.Role != "" {
		validRoles := map[PredefinedRole]bool{
			RoleReader:            true,
			RoleViewer:            true,
			RoleDeveloper:         true,
			RoleDeveloperExtended: true,
			RoleDeployer:          true,
			RoleDebugger:          true,
			RoleOperator:          true,
			RoleAuditor:           true,
			RoleMaintainer:        true,
			RoleClusterAdmin:      true,
		}

		if !validRoles[r.Spec.Role] {
			return warnings, fmt.Errorf("unknown predefined role: %q", r.Spec.Role)
		}
	}

	// Validate CustomRules
	if len(r.Spec.CustomRules) > 0 {
		for i, rule := range r.Spec.CustomRules {
			if len(rule.Verbs) == 0 {
				return warnings, fmt.Errorf("customRules[%d]: verbs must be specified", i)
			}

			if len(rule.Resources) == 0 && len(rule.NonResourceURLs) == 0 {
				return warnings, fmt.Errorf("customRules[%d]: either resources or nonResourceURLs must be specified", i)
			}

			// Reject wildcard combinations that would grant unrestricted cluster access.
			hasWildcardVerbs := len(rule.Verbs) == 1 && rule.Verbs[0] == "*"
			hasWildcardResources := len(rule.Resources) == 1 && rule.Resources[0] == "*"
			hasWildcardGroups := len(rule.APIGroups) == 1 && rule.APIGroups[0] == "*"
			if hasWildcardVerbs && hasWildcardResources && hasWildcardGroups {
				return warnings, fmt.Errorf("customRules[%d]: wildcard apiGroups/resources/verbs is not allowed; use predefined role 'cluster-admin' with clusterWide: true instead", i)
			}
		}
	}

	// Validate ClusterWide mode
	if r.Spec.ClusterWide {
		// Warn if namespaces are specified with ClusterWide
		if len(r.Spec.Namespaces) > 0 {
			warnings = append(warnings, "spec.namespaces is ignored when spec.clusterWide is true")
		}

		// Warn if using cluster-admin without ClusterWide
	} else if r.Spec.Role == RoleClusterAdmin {
		warnings = append(warnings, "role 'cluster-admin' is recommended to be used with clusterWide: true for full cluster access")
	}

	// Validate namespace list
	if !r.Spec.ClusterWide && len(r.Spec.Namespaces) == 0 && r.Spec.Role != "" {
		return warnings, fmt.Errorf("spec.namespaces must be specified when clusterWide is false")
	}

	// Validate ServiceAccount name length.
	if r.Spec.ServiceAccountName != "" {
		if len(r.Spec.ServiceAccountName) > 253 {
			return warnings, fmt.Errorf("spec.serviceAccountName is too long (max 253 characters)")
		}
	}

	// Validate that the generated role name (rbac-<name>) fits within the 253-char limit.
	// Also account for the "-namespace-viewer" suffix used for viewer/developer-extended roles.
	const maxK8sName = 253
	generatedRoleName := "rbac-" + r.Name
	if len(generatedRoleName)+len("-namespace-viewer") > maxK8sName {
		return warnings, fmt.Errorf("AccessGrant name %q is too long: generated role names would exceed %d characters", r.Name, maxK8sName)
	}

	return warnings, nil
}
