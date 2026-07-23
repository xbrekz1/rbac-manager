package v1alpha1

import (
	"context"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var roletemplatelog = logf.Log.WithName("roletemplate-resource")

// SetupWebhookWithManager registers the webhook for RoleTemplate in the manager.
func (r *RoleTemplate) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &RoleTemplate{}).
		WithValidator(&RoleTemplateCustomValidator{}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-rbacmanager-io-v1alpha1-roletemplate,mutating=false,failurePolicy=fail,sideEffects=None,groups=rbacmanager.io,resources=roletemplates,verbs=create;update,versions=v1alpha1,name=vroletemplate.kb.io,admissionReviewVersions=v1

// RoleTemplateCustomValidator validates RoleTemplate resources at admission time.
type RoleTemplateCustomValidator struct{}

var _ admission.Validator[*RoleTemplate] = &RoleTemplateCustomValidator{}

// ValidateCreate implements admission.Validator.
func (v *RoleTemplateCustomValidator) ValidateCreate(_ context.Context, obj *RoleTemplate) (admission.Warnings, error) {
	roletemplatelog.Info("validate create", "name", obj.Name)
	return obj.validateRoleTemplate()
}

// ValidateUpdate implements admission.Validator.
func (v *RoleTemplateCustomValidator) ValidateUpdate(_ context.Context, _, newObj *RoleTemplate) (admission.Warnings, error) {
	roletemplatelog.Info("validate update", "name", newObj.Name)
	return newObj.validateRoleTemplate()
}

// ValidateDelete implements admission.Validator.
func (v *RoleTemplateCustomValidator) ValidateDelete(_ context.Context, obj *RoleTemplate) (admission.Warnings, error) {
	roletemplatelog.Info("validate delete", "name", obj.Name)
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
