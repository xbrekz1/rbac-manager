package v1alpha1

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var roletemplatelog = logf.Log.WithName("roletemplate-resource")

// SetupWebhookWithManager registers the webhook for RoleTemplate in the manager.
func (r *RoleTemplate) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:path=/validate-rbacmanager-io-v1alpha1-roletemplate,mutating=false,failurePolicy=fail,sideEffects=None,groups=rbacmanager.io,resources=roletemplates,verbs=create;update,versions=v1alpha1,name=vroletemplate.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &RoleTemplate{}

// ValidateCreate implements webhook.Validator.
func (r *RoleTemplate) ValidateCreate() (admission.Warnings, error) {
	roletemplatelog.Info("validate create", "name", r.Name)
	return r.validateRoleTemplate()
}

// ValidateUpdate implements webhook.Validator.
func (r *RoleTemplate) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	roletemplatelog.Info("validate update", "name", r.Name)
	return r.validateRoleTemplate()
}

// ValidateDelete implements webhook.Validator.
func (r *RoleTemplate) ValidateDelete() (admission.Warnings, error) {
	roletemplatelog.Info("validate delete", "name", r.Name)
	return nil, nil
}

func (r *RoleTemplate) validateRoleTemplate() (admission.Warnings, error) {
	if len(r.Spec.Rules) == 0 {
		return nil, fmt.Errorf("spec.rules must contain at least one policy rule")
	}
	for i, rule := range r.Spec.Rules {
		if len(rule.Verbs) == 0 {
			return nil, fmt.Errorf("spec.rules[%d]: verbs must be specified", i)
		}
		if len(rule.Resources) == 0 && len(rule.NonResourceURLs) == 0 {
			return nil, fmt.Errorf("spec.rules[%d]: either resources or nonResourceURLs must be specified", i)
		}
		// Reject wildcard combinations that would grant unrestricted cluster access.
		isAllWildcard := len(rule.Verbs) == 1 && rule.Verbs[0] == "*" &&
			len(rule.Resources) == 1 && rule.Resources[0] == "*" &&
			len(rule.APIGroups) == 1 && rule.APIGroups[0] == "*"
		if isAllWildcard {
			return nil, fmt.Errorf("spec.rules[%d]: wildcard apiGroups/resources/verbs is not allowed; use predefined role 'cluster-admin' instead", i)
		}
		// Reject rules that grant access to RBAC resources (privilege escalation).
		for _, grp := range rule.APIGroups {
			if grp != "rbac.authorization.k8s.io" && grp != "*" {
				continue
			}
			for _, res := range rule.Resources {
				if res == "*" || rbacResources[res] {
					return nil, fmt.Errorf("spec.rules[%d]: RBAC resources (%s) are not allowed; use predefined roles instead", i, res)
				}
			}
		}
	}
	return nil, nil
}
