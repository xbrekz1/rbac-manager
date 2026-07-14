package v1alpha1

import (
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/xbrekz1/rbac-manager/internal/roles"
)

var accessgrantlog = logf.Log.WithName("accessgrant-resource")

// rbacResources are resources that grant RBAC management capabilities.
// Blocking these in customRules prevents privilege escalation to cluster-admin.
var rbacResources = map[string]bool{
	"roles": true, "rolebindings": true,
	"clusterroles": true, "clusterrolebindings": true,
}

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
		if _, ok := roles.GetPredefinedRules(string(r.Spec.Role)); !ok {
			return fmt.Errorf("unknown predefined role: %q", r.Spec.Role)
		}
	}
	return nil
}

func validateCustomRules(r *AccessGrant) error {
	return validatePolicyRules(r.Spec.CustomRules, "customRules")
}

// validatePolicyRules validates a slice of RBAC policy rules for safety.
// Shared by AccessGrant customRules and RoleTemplate rules.
func validatePolicyRules(rules []rbacv1.PolicyRule, pathPrefix string) error {
	for i, rule := range rules {
		if err := validateSingleRule(rule, pathPrefix, i); err != nil {
			return err
		}
	}
	return nil
}

func validateSingleRule(rule rbacv1.PolicyRule, pathPrefix string, i int) error {
	if len(rule.Verbs) == 0 {
		return fmt.Errorf("%s[%d]: verbs must be specified", pathPrefix, i)
	}
	if len(rule.Resources) == 0 && len(rule.NonResourceURLs) == 0 {
		return fmt.Errorf("%s[%d]: either resources or nonResourceURLs must be specified", pathPrefix, i)
	}
	if isFullWildcard(rule) {
		return fmt.Errorf("%s[%d]: wildcard apiGroups/resources/verbs is not allowed; use predefined role 'cluster-admin' instead", pathPrefix, i)
	}
	if res := rbacResourceConflict(rule); res != "" {
		return fmt.Errorf("%s[%d]: RBAC resources (%s) are not allowed; use predefined roles instead", pathPrefix, i, res)
	}
	return nil
}

func isFullWildcard(rule rbacv1.PolicyRule) bool {
	return len(rule.Verbs) == 1 && rule.Verbs[0] == "*" &&
		len(rule.Resources) == 1 && rule.Resources[0] == "*" &&
		len(rule.APIGroups) == 1 && rule.APIGroups[0] == "*"
}

// rbacResourceConflict returns the first resource name that would grant RBAC
// management access, or an empty string if the rule is safe.
func rbacResourceConflict(rule rbacv1.PolicyRule) string {
	for _, grp := range rule.APIGroups {
		if grp != "rbac.authorization.k8s.io" && grp != "*" {
			continue
		}
		for _, res := range rule.Resources {
			if res == "*" || rbacResources[res] {
				return res
			}
		}
	}
	return ""
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

// reservedServiceAccountNames are Kubernetes built-in service accounts that
// must not be overwritten by the operator.
var reservedServiceAccountNames = map[string]bool{
	"default": true,
}

func validateNames(r *AccessGrant) error {
	if len(r.Spec.ServiceAccountName) > 253 {
		return fmt.Errorf("spec.serviceAccountName is too long (max 253 characters)")
	}
	if r.Spec.ServiceAccountName != "" && reservedServiceAccountNames[r.Spec.ServiceAccountName] {
		return fmt.Errorf("spec.serviceAccountName %q is reserved and cannot be used", r.Spec.ServiceAccountName)
	}
	const maxK8sName = 253
	if len("rbac-"+r.Name)+len("-namespace-viewer") > maxK8sName {
		return fmt.Errorf("AccessGrant name %q is too long: generated role names would exceed %d characters", r.Name, maxK8sName)
	}
	return nil
}
