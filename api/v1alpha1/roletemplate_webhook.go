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
	if err := validatePolicyRules(r.Spec.Rules, "spec.rules"); err != nil {
		return nil, err
	}
	return nil, nil
}
