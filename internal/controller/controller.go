package controller

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	rbacmanagerv1alpha1 "github.com/xbrekz1/rbac-manager/api/v1alpha1"
)

const (
	finalizerName      = "rbacmanager.io/cleanup"
	managedByLabel     = "rbacmanager.io/managed-by"
	accessGrantLabel   = "rbacmanager.io/access-grant"
	accessGrantNsLabel = "rbacmanager.io/access-grant-namespace"
	managerValue       = "rbac-manager"

	// Condition types
	ConditionTypeReady              = "Ready"
	ConditionTypeServiceAccountOK   = "ServiceAccountReady"
	ConditionTypeRBACResourcesOK    = "RBACResourcesReady"
	ConditionTypeNamespaceValidated = "NamespacesValidated"

	// Condition reasons
	ReasonReconciliationSucceeded = "ReconciliationSucceeded"
	ReasonReconciliationFailed    = "ReconciliationFailed"
	ReasonServiceAccountCreated   = "ServiceAccountCreated"
	ReasonRBACResourcesCreated    = "RBACResourcesCreated"
	ReasonNamespaceNotFound       = "NamespaceNotFound"
	ReasonCleanupSucceeded        = "CleanupSucceeded"
	ReasonInvalidConfiguration    = "InvalidConfiguration"
)

// AccessGrantReconciler reconciles an AccessGrant object.
type AccessGrantReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=rbacmanager.io,resources=accessgrants,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbacmanager.io,resources=accessgrants/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=rbacmanager.io,resources=accessgrants/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings;clusterroles;clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch;delete

// Reconcile reads the state of the cluster for an AccessGrant object and makes changes based on the state read.
func (r *AccessGrantReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the AccessGrant instance.
	ag := &rbacmanagerv1alpha1.AccessGrant{}
	if err := r.Get(ctx, req.NamespacedName, ag); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Handle deletion.
	if !ag.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(ag, finalizerName) {
			logger.Info("Running cleanup for deleted AccessGrant", "name", ag.Name, "namespace", ag.Namespace)
			r.Recorder.Event(ag, corev1.EventTypeNormal, "Deleting", "Starting cleanup of RBAC resources")

			if err := r.cleanupRBAC(ctx, ag); err != nil {
				logger.Error(err, "Failed to clean up RBAC resources")
				r.Recorder.Event(ag, corev1.EventTypeWarning, "CleanupFailed", fmt.Sprintf("Failed to cleanup: %v", err))
				return ctrl.Result{RequeueAfter: 30 * time.Second}, err
			}

			r.Recorder.Event(ag, corev1.EventTypeNormal, ReasonCleanupSucceeded, "Successfully cleaned up all RBAC resources")
			controllerutil.RemoveFinalizer(ag, finalizerName)
			if err := r.Update(ctx, ag); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Add finalizer if not present.
	if !controllerutil.ContainsFinalizer(ag, finalizerName) {
		controllerutil.AddFinalizer(ag, finalizerName)
		if err := r.Update(ctx, ag); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Update status to Pending.
	ag.Status.Phase = rbacmanagerv1alpha1.PhasePending
	ag.Status.Message = "Reconciling RBAC resources"
	meta.SetStatusCondition(&ag.Status.Conditions, metav1.Condition{
		Type:               ConditionTypeReady,
		Status:             metav1.ConditionFalse,
		Reason:             "Reconciling",
		Message:            "Starting reconciliation",
		ObservedGeneration: ag.Generation,
	})
	if err := r.Status().Update(ctx, ag); err != nil {
		logger.Error(err, "Failed to update status to Pending")
		return ctrl.Result{}, err
	}

	// Perform RBAC reconciliation.
	result, err := r.reconcileRBAC(ctx, ag)
	if err != nil {
		logger.Error(err, "Failed to reconcile RBAC resources")
		r.Recorder.Event(ag, corev1.EventTypeWarning, ReasonReconciliationFailed, fmt.Sprintf("Reconciliation failed: %v", err))

		ag.Status.Phase = rbacmanagerv1alpha1.PhaseFailed
		ag.Status.Message = err.Error()
		ag.Status.ObservedGeneration = ag.Generation
		meta.SetStatusCondition(&ag.Status.Conditions, metav1.Condition{
			Type:               ConditionTypeReady,
			Status:             metav1.ConditionFalse,
			Reason:             ReasonReconciliationFailed,
			Message:            err.Error(),
			ObservedGeneration: ag.Generation,
		})
		if statusErr := r.Status().Update(ctx, ag); statusErr != nil {
			logger.Error(statusErr, "Failed to update status to Failed")
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	// Update status to Active.
	ag.Status.Phase = rbacmanagerv1alpha1.PhaseActive
	ag.Status.ServiceAccount = result.saName
	ag.Status.Namespaces = result.namespaces
	ag.Status.ClusterRole = result.clusterRole
	ag.Status.ObservedGeneration = ag.Generation

	// Determine whether all requested namespaces were reconciled.
	nsCondition := metav1.Condition{
		Type:               ConditionTypeNamespaceValidated,
		ObservedGeneration: ag.Generation,
	}
	if len(result.skippedNamespaces) > 0 {
		ag.Status.Message = fmt.Sprintf("Partially reconciled: %d namespace(s) not yet available: %v",
			len(result.skippedNamespaces), result.skippedNamespaces)
		nsCondition.Status = metav1.ConditionFalse
		nsCondition.Reason = ReasonNamespaceNotFound
		nsCondition.Message = fmt.Sprintf("Namespaces not yet available: %v", result.skippedNamespaces)
		r.Recorder.Event(ag, corev1.EventTypeWarning, ReasonNamespaceNotFound,
			fmt.Sprintf("Namespaces not yet available, will retry: %v", result.skippedNamespaces))
	} else {
		ag.Status.Message = "RBAC resources successfully reconciled"
		nsCondition.Status = metav1.ConditionTrue
		nsCondition.Reason = ReasonReconciliationSucceeded
		nsCondition.Message = "All target namespaces exist and have RBAC resources"
	}
	meta.SetStatusCondition(&ag.Status.Conditions, nsCondition)

	meta.SetStatusCondition(&ag.Status.Conditions, metav1.Condition{
		Type:               ConditionTypeReady,
		Status:             metav1.ConditionTrue,
		Reason:             ReasonReconciliationSucceeded,
		Message:            "RBAC resources reconciled",
		ObservedGeneration: ag.Generation,
	})
	meta.SetStatusCondition(&ag.Status.Conditions, metav1.Condition{
		Type:               ConditionTypeServiceAccountOK,
		Status:             metav1.ConditionTrue,
		Reason:             ReasonServiceAccountCreated,
		Message:            fmt.Sprintf("ServiceAccount %q created", result.saName),
		ObservedGeneration: ag.Generation,
	})
	meta.SetStatusCondition(&ag.Status.Conditions, metav1.Condition{
		Type:               ConditionTypeRBACResourcesOK,
		Status:             metav1.ConditionTrue,
		Reason:             ReasonRBACResourcesCreated,
		Message:            "All Roles and RoleBindings created",
		ObservedGeneration: ag.Generation,
	})

	if err := r.Status().Update(ctx, ag); err != nil {
		logger.Error(err, "Failed to update status to Active")
		return ctrl.Result{}, err
	}

	r.Recorder.Event(ag, corev1.EventTypeNormal, ReasonReconciliationSucceeded,
		fmt.Sprintf("Successfully reconciled RBAC for role %q in %d namespace(s)", ag.Spec.Role, len(result.namespaces)))

	logger.Info("Successfully reconciled AccessGrant",
		"name", ag.Name,
		"namespace", ag.Namespace,
		"serviceAccount", result.saName,
		"namespaces", result.namespaces,
		"skippedNamespaces", result.skippedNamespaces,
	)

	// If some namespaces were missing, requeue sooner to retry (namespace watcher will also trigger).
	if len(result.skippedNamespaces) > 0 {
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Requeue periodically to detect resources deleted externally (e.g. via kubectl delete role).
	return ctrl.Result{RequeueAfter: 10 * time.Minute}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AccessGrantReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Setup event recorder
	r.Recorder = mgr.GetEventRecorderFor("rbac-manager")

	return ctrl.NewControllerManagedBy(mgr).
		For(&rbacmanagerv1alpha1.AccessGrant{}).
		Watches(
			&corev1.Namespace{},
			handler.EnqueueRequestsFromMapFunc(r.findAccessGrantsForNamespace),
		).
		Complete(r)
}

// findAccessGrantsForNamespace returns AccessGrants that reference the given namespace.
// This enables the controller to reconcile AccessGrants when their target namespaces are created.
func (r *AccessGrantReconciler) findAccessGrantsForNamespace(ctx context.Context, obj client.Object) []reconcile.Request {
	namespace := obj.(*corev1.Namespace)

	// List all AccessGrants
	agList := &rbacmanagerv1alpha1.AccessGrantList{}
	if err := r.List(ctx, agList); err != nil {
		return nil
	}

	var requests []reconcile.Request
	for _, ag := range agList.Items {
		// Check if this AccessGrant references the namespace
		for _, ns := range ag.Spec.Namespaces {
			if ns == namespace.Name {
				requests = append(requests, reconcile.Request{
					NamespacedName: client.ObjectKey{
						Name:      ag.Name,
						Namespace: ag.Namespace,
					},
				})
				break
			}
		}
	}

	return requests
}
