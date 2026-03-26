package controller

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	rbacmanagerv1alpha1 "github.com/xbrekz1/rbac-manager/api/v1alpha1"
)

const (
	finalizerName      = "rbacmanager.io/cleanup"
	managedByLabel     = "rbacmanager.io/managed-by"
	accessGrantLabel   = "rbacmanager.io/access-grant"
	accessGrantNsLabel = "rbacmanager.io/access-grant-namespace"
	managerValue       = "rbac-manager"
)

// AccessGrantReconciler reconciles an AccessGrant object.
type AccessGrantReconciler struct {
	client.Client
	Scheme *runtime.Scheme
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
			if err := r.cleanupRBAC(ctx, ag); err != nil {
				logger.Error(err, "Failed to clean up RBAC resources")
				return ctrl.Result{RequeueAfter: 30 * time.Second}, err
			}
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
	if err := r.Status().Update(ctx, ag); err != nil {
		logger.Error(err, "Failed to update status to Pending")
		return ctrl.Result{}, err
	}

	// Perform RBAC reconciliation.
	result, err := r.reconcileRBAC(ctx, ag)
	if err != nil {
		logger.Error(err, "Failed to reconcile RBAC resources")
		ag.Status.Phase = rbacmanagerv1alpha1.PhaseFailed
		ag.Status.Message = err.Error()
		ag.Status.ObservedGeneration = ag.Generation
		if statusErr := r.Status().Update(ctx, ag); statusErr != nil {
			logger.Error(statusErr, "Failed to update status to Failed")
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	// Update status to Active.
	ag.Status.Phase = rbacmanagerv1alpha1.PhaseActive
	ag.Status.Message = "RBAC resources successfully reconciled"
	ag.Status.ServiceAccount = result.saName
	ag.Status.Namespaces = result.namespaces
	ag.Status.ClusterRole = result.clusterRole
	ag.Status.ObservedGeneration = ag.Generation
	if err := r.Status().Update(ctx, ag); err != nil {
		logger.Error(err, "Failed to update status to Active")
		return ctrl.Result{}, err
	}

	logger.Info("Successfully reconciled AccessGrant",
		"name", ag.Name,
		"namespace", ag.Namespace,
		"serviceAccount", result.saName,
		"namespaces", result.namespaces,
	)

	// Requeue periodically to detect resources deleted externally (e.g. via kubectl delete role).
	return ctrl.Result{RequeueAfter: 10 * time.Minute}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AccessGrantReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&rbacmanagerv1alpha1.AccessGrant{}).
		Complete(r)
}
