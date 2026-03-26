package v1alpha1

import (
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PredefinedRole defines the name of a built-in role.
type PredefinedRole string

const (
	RoleReader            PredefinedRole = "reader"
	RoleViewer            PredefinedRole = "viewer"
	RoleDeveloper         PredefinedRole = "developer"
	RoleDeveloperExtended PredefinedRole = "developer-extended"
	RoleDeployer          PredefinedRole = "deployer"
	RoleDebugger          PredefinedRole = "debugger"
	RoleOperator          PredefinedRole = "operator"
	RoleAuditor           PredefinedRole = "auditor"
	RoleMaintainer        PredefinedRole = "maintainer"
	RoleClusterAdmin      PredefinedRole = "cluster-admin"
)

// Phase represents the current lifecycle phase of an AccessGrant.
type Phase string

const (
	PhasePending Phase = "Pending"
	PhaseActive  Phase = "Active"
	PhaseFailed  Phase = "Failed"
)

// AccessGrantSpec defines the desired state of AccessGrant.
type AccessGrantSpec struct {
	// Namespaces where RBAC resources will be created.
	// +optional
	Namespaces []string `json:"namespaces,omitempty"`

	// Role is the name of a predefined role.
	// +kubebuilder:validation:Enum=reader;viewer;developer;developer-extended;deployer;debugger;operator;auditor;maintainer;cluster-admin
	// +optional
	Role PredefinedRole `json:"role,omitempty"`

	// CustomRules are custom RBAC policy rules. Used when Role is not specified.
	// +optional
	CustomRules []rbacv1.PolicyRule `json:"customRules,omitempty"`

	// ClusterWide creates a ClusterRole/ClusterRoleBinding instead of namespace-scoped resources.
	// +optional
	ClusterWide bool `json:"clusterWide,omitempty"`

	// ServiceAccountName is the name of the ServiceAccount to create.
	// Defaults to "rbac-<accessgrant-name>".
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// Labels are extra labels added to all managed resources.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations are extra annotations added to all managed resources.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// AccessGrantStatus defines the observed state of AccessGrant.
type AccessGrantStatus struct {
	// Phase is the current lifecycle state.
	Phase Phase `json:"phase,omitempty"`

	// ServiceAccount is the name of the managed ServiceAccount.
	ServiceAccount string `json:"serviceAccount,omitempty"`

	// Namespaces lists the namespaces where RBAC resources were successfully created.
	Namespaces []string `json:"namespaces,omitempty"`

	// ClusterRole is the name of the managed ClusterRole (when ClusterWide=true or role needs namespace visibility).
	ClusterRole string `json:"clusterRole,omitempty"`

	// Message is a human-readable description of the current state.
	Message string `json:"message,omitempty"`

	// Conditions represent detailed status of the resource.
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the last generation reconciled.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=ag,scope=Namespaced
// +kubebuilder:printcolumn:name="Role",type=string,JSONPath=`.spec.role`
// +kubebuilder:printcolumn:name="ServiceAccount",type=string,JSONPath=`.status.serviceAccount`
// +kubebuilder:printcolumn:name="Namespaces",type=string,JSONPath=`.status.namespaces`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// AccessGrant defines a desired RBAC access configuration for a ServiceAccount.
type AccessGrant struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AccessGrantSpec   `json:"spec,omitempty"`
	Status AccessGrantStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AccessGrantList contains a list of AccessGrant.
type AccessGrantList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AccessGrant `json:"items"`
}
