package controllers

import (
	"context"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	akamaiV1alpha1 "github.com/mmz-srf/akamai-operator/api/v1alpha1"
	"github.com/mmz-srf/akamai-operator/pkg/akamai"
)

// AkamaiPropertyReconciler reconciles a AkamaiProperty object
type AkamaiPropertyReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	AkamaiClient *akamai.Client
}

//+kubebuilder:rbac:groups=akamai.com,resources=akamaiproperties,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=akamai.com,resources=akamaiproperties/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=akamai.com,resources=akamaiproperties/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *AkamaiPropertyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the AkamaiProperty instance
	var akamaiProperty akamaiV1alpha1.AkamaiProperty
	if err := r.Get(ctx, req.NamespacedName, &akamaiProperty); err != nil {
		if apierrors.IsNotFound(err) {
			// Object not found, return. Created objects are automatically garbage collected.
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	// Initialize Akamai client if not already done
	if r.AkamaiClient == nil {
		akamaiClient, err := akamai.NewClient()
		if err != nil {
			logger.Error(err, "Failed to create Akamai client")
			r.updateStatus(ctx, &akamaiProperty, PhaseError, "FailedToInitializeAkamaiClient", err.Error())
			return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
		}
		r.AkamaiClient = akamaiClient
	}

	// Handle deletion
	if akamaiProperty.ObjectMeta.DeletionTimestamp != nil {
		return r.handleDeletion(ctx, &akamaiProperty)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(&akamaiProperty, FinalizerName) {
		controllerutil.AddFinalizer(&akamaiProperty, FinalizerName)
		if err := r.Update(ctx, &akamaiProperty); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Reconcile the property
	return r.reconcileProperty(ctx, &akamaiProperty)
}

// SetupWithManager sets up the controller with the Manager.
func (r *AkamaiPropertyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&akamaiV1alpha1.AkamaiProperty{}).
		Complete(r)
}
