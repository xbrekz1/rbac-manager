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
	if err := validateRoleSpec(r); err != nil {
		return nil, err
	}
	if err := validateCustomRules(r); err != nil {
		return nil, err
	}
	warnings, err := validateNamespacingSpec(r)
	if err != nil {
		return nil, err
	}
	if err := validateNames(r); err != nil {
		return nil, err
	}
	return warnings, nil
}

func validateRoleSpec(r *AccessGrant) error {
	set := 0
	if r.Spec.Role != "" {
		set++
	}
	if len(r.Spec.CustomRules) > 0 {
		set++
	}
	if r.Spec.RoleTemplateName != "" {
		set++
	}
	if set > 1 {
		return fmt.Errorf("only one of spec.role, spec.roleTemplate, or spec.customRules may be specified")
	}
	if set == 0 {
		return fmt.Errorf("one of spec.role, spec.roleTemplate, or spec.customRules must be specified")
	}
	if r.Spec.Role != "" {
		validRoles := map[PredefinedRole]bool{
			RoleReader: true, RoleViewer: true, RoleDeveloper: true,
			RoleDeveloperExtended: true, RoleDeployer: true, RoleDebugger: true,
			RoleOperator: true, RoleAuditor: true, RoleMaintainer: true, RoleClusterAdmin: true,
		}
		if !validRoles[r.Spec.Role] {
			return fmt.Errorf("unknown predefined role: %q", r.Spec.Role)
		}
	}
	return nil
}

func validateCustomRules(r *AccessGrant) error {
	for i, rule := range r.Spec.CustomRules {
		if len(rule.Verbs) == 0 {
			return fmt.Errorf("customRules[%d]: verbs must be specified", i)
		}
		if len(rule.Resources) == 0 && len(rule.NonResourceURLs) == 0 {
			return fmt.Errorf("customRules[%d]: either resources or nonResourceURLs must be specified", i)
		}
		// Reject wildcard combinations that would grant unrestricted cluster access.
		isAllWildcard := len(rule.Verbs) == 1 && rule.Verbs[0] == "*" &&
			len(rule.Resources) == 1 && rule.Resources[0] == "*" &&
			len(rule.APIGroups) == 1 && rule.APIGroups[0] == "*"
		if isAllWildcard {
			return fmt.Errorf("customRules[%d]: wildcard apiGroups/resources/verbs is not allowed; use predefined role 'cluster-admin' with clusterWide: true instead", i)
		}
		// Reject rules that grant access to RBAC resources (privilege escalation).
		rbacResources := map[string]bool{
			"roles": true, "rolebindings": true,
			"clusterroles": true, "clusterrolebindings": true,
		}
		for _, grp := range rule.APIGroups {
			if grp != "rbac.authorization.k8s.io" && grp != "*" {
				continue
			}
			for _, res := range rule.Resources {
				if rbacResources[res] {
					return fmt.Errorf("customRules[%d]: RBAC resources (%s) are not allowed in customRules; use predefined roles instead", i, res)
				}
			}
		}
	}
	return nil
}

func validateNamespacingSpec(r *AccessGrant) (admission.Warnings, error) {
	var warnings admission.Warnings
	if r.Spec.ClusterWide {
		if len(r.Spec.Namespaces) > 0 {
			warnings = append(warnings, "spec.namespaces is ignored when spec.clusterWide is true")
		}
	} else if r.Spec.Role == RoleClusterAdmin {
		warnings = append(warnings, "role 'cluster-admin' is recommended to be used with clusterWide: true for full cluster access")
	}
	if !r.Spec.ClusterWide && len(r.Spec.Namespaces) == 0 {
		return nil, fmt.Errorf("spec.namespaces must be specified when clusterWide is false")
	}
	return warnings, nil
}

func validateNames(r *AccessGrant) error {
	if len(r.Spec.ServiceAccountName) > 253 {
		return fmt.Errorf("spec.serviceAccountName is too long (max 253 characters)")
	}
	const maxK8sName = 253
	if len("rbac-"+r.Name)+len("-namespace-viewer") > maxK8sName {
		return fmt.Errorf("AccessGrant name %q is too long: generated role names would exceed %d characters", r.Name, maxK8sName)
	}
	return nil
}
