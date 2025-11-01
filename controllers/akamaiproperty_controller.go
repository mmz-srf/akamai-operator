package controllers

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

const (
	// FinalizerName is the finalizer added to AkamaiProperty resources
	FinalizerName = "akamai.com/finalizer"

	// Condition types
	ConditionTypeReady       = "Ready"
	ConditionTypeAvailable   = "Available"
	ConditionTypeProgressing = "Progressing"

	// Phase constants
	PhaseCreating   = "Creating"
	PhaseReady      = "Ready"
	PhaseUpdating   = "Updating"
	PhaseActivating = "Activating"
	PhaseError      = "Error"
	PhaseDeleting   = "Deleting"
)

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
			r.updateStatus(ctx, &akamaiProperty, PhaseError, "Failed to initialize Akamai client", err.Error())
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

// reconcileProperty handles the main reconciliation logic
func (r *AkamaiPropertyReconciler) reconcileProperty(ctx context.Context, akamaiProperty *akamaiV1alpha1.AkamaiProperty) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Check if property exists in Akamai
	if akamaiProperty.Status.PropertyID == "" {
		// Property doesn't exist, create it
		logger.Info("Creating new Akamai property", "propertyName", akamaiProperty.Spec.PropertyName)
		r.updateStatus(ctx, akamaiProperty, PhaseCreating, "Creating Akamai property", "")

		propertyID, err := r.AkamaiClient.CreateProperty(ctx, &akamaiProperty.Spec)
		if err != nil {
			logger.Error(err, "Failed to create Akamai property")
			r.updateStatus(ctx, akamaiProperty, PhaseError, "Failed to create property", err.Error())
			return ctrl.Result{RequeueAfter: time.Minute * 2}, nil
		}

		akamaiProperty.Status.PropertyID = propertyID
		akamaiProperty.Status.LatestVersion = 1
		akamaiProperty.Status.Phase = PhaseReady

		if err := r.Status().Update(ctx, akamaiProperty); err != nil {
			return ctrl.Result{}, err
		}

		logger.Info("Successfully created Akamai property", "propertyID", propertyID)
		r.updateStatus(ctx, akamaiProperty, PhaseReady, "Property created successfully", "")
		return ctrl.Result{RequeueAfter: time.Minute * 10}, nil
	}

	// Property exists, check if it needs to be updated
	currentProperty, err := r.AkamaiClient.GetProperty(ctx, akamaiProperty.Status.PropertyID)
	if err != nil {
		logger.Error(err, "Failed to get Akamai property")
		r.updateStatus(ctx, akamaiProperty, PhaseError, "Failed to retrieve property", err.Error())
		return ctrl.Result{RequeueAfter: time.Minute * 2}, nil
	}

	// Check if property needs to be updated
	if r.needsUpdate(akamaiProperty, currentProperty) {
		logger.Info("Updating Akamai property", "propertyID", akamaiProperty.Status.PropertyID)
		r.updateStatus(ctx, akamaiProperty, PhaseUpdating, "Updating Akamai property", "")

		newVersion, err := r.AkamaiClient.UpdateProperty(ctx, akamaiProperty.Status.PropertyID, &akamaiProperty.Spec)
		if err != nil {
			logger.Error(err, "Failed to update Akamai property")
			r.updateStatus(ctx, akamaiProperty, PhaseError, "Failed to update property", err.Error())
			return ctrl.Result{RequeueAfter: time.Minute * 2}, nil
		}

		akamaiProperty.Status.LatestVersion = newVersion
		if err := r.Status().Update(ctx, akamaiProperty); err != nil {
			return ctrl.Result{}, err
		}

		logger.Info("Successfully updated Akamai property", "propertyID", akamaiProperty.Status.PropertyID, "version", newVersion)
	}

	// Handle activation if specified
	if akamaiProperty.Spec.Activation != nil {
		activationResult, err := r.handleActivation(ctx, akamaiProperty)
		if err != nil {
			logger.Error(err, "Failed to handle activation")
			r.updateStatus(ctx, akamaiProperty, PhaseError, "Failed to handle activation", err.Error())
			return ctrl.Result{RequeueAfter: time.Minute * 2}, nil
		}
		if activationResult.Requeue {
			return activationResult, nil
		}
	}

	r.updateStatus(ctx, akamaiProperty, PhaseReady, "Property is ready", "")
	return ctrl.Result{RequeueAfter: time.Minute * 30}, nil
}

// handleDeletion handles the deletion of the AkamaiProperty resource
func (r *AkamaiPropertyReconciler) handleDeletion(ctx context.Context, akamaiProperty *akamaiV1alpha1.AkamaiProperty) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if controllerutil.ContainsFinalizer(akamaiProperty, FinalizerName) {
		// Update status to indicate deletion is in progress
		r.updateStatus(ctx, akamaiProperty, PhaseDeleting, "Deleting Akamai property", "")

		// Delete the property from Akamai if it exists
		if akamaiProperty.Status.PropertyID != "" {
			logger.Info("Deleting Akamai property", "propertyID", akamaiProperty.Status.PropertyID)

			err := r.AkamaiClient.DeleteProperty(ctx, akamaiProperty.Status.PropertyID)
			if err != nil {
				logger.Error(err, "Failed to delete Akamai property")
				r.updateStatus(ctx, akamaiProperty, PhaseError, "Failed to delete property", err.Error())
				return ctrl.Result{RequeueAfter: time.Minute * 2}, nil
			}

			logger.Info("Successfully deleted Akamai property", "propertyID", akamaiProperty.Status.PropertyID)
		}

		// Remove the finalizer
		controllerutil.RemoveFinalizer(akamaiProperty, FinalizerName)
		if err := r.Update(ctx, akamaiProperty); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// handleActivation handles the activation of the property
func (r *AkamaiPropertyReconciler) handleActivation(ctx context.Context, akamaiProperty *akamaiV1alpha1.AkamaiProperty) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	activationSpec := akamaiProperty.Spec.Activation

	// Determine which version to activate (use latest version)
	versionToActivate := akamaiProperty.Status.LatestVersion

	// Check current activation status for the target network
	var currentActivationID, currentActivationStatus string
	if activationSpec.Network == "STAGING" {
		currentActivationID = akamaiProperty.Status.StagingActivationID
		currentActivationStatus = akamaiProperty.Status.StagingActivationStatus
	} else if activationSpec.Network == "PRODUCTION" {
		currentActivationID = akamaiProperty.Status.ProductionActivationID
		currentActivationStatus = akamaiProperty.Status.ProductionActivationStatus
	}

	// Check if we need to start a new activation
	needsActivation := false
	if currentActivationID == "" {
		// No previous activation
		needsActivation = true
	} else {
		// Check if there's already an activation in progress
		if currentActivationStatus == "PENDING" || currentActivationStatus == "ACTIVATING" {
			// Check the current status of the activation
			activation, err := r.AkamaiClient.GetActivation(ctx, akamaiProperty.Status.PropertyID, currentActivationID)
			if err != nil {
				logger.Error(err, "Failed to get activation status")
				return ctrl.Result{RequeueAfter: time.Minute * 2}, nil
			}

			// Update the status based on the current activation
			r.updateActivationStatus(akamaiProperty, activationSpec.Network, activation)

			if activation.Status == "ACTIVE" {
				logger.Info("Activation completed successfully", "network", activationSpec.Network, "version", activation.PropertyVersion)
				return ctrl.Result{}, nil
			} else if activation.Status == "FAILED" {
				logger.Error(nil, "Activation failed", "network", activationSpec.Network, "activationID", currentActivationID)
				r.updateStatus(ctx, akamaiProperty, PhaseError, "Activation failed", "Check activation logs")
				return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
			} else {
				// Still in progress
				logger.Info("Activation in progress", "network", activationSpec.Network, "status", activation.Status)
				r.updateStatus(ctx, akamaiProperty, PhaseActivating, "Activation in progress", fmt.Sprintf("Status: %s", activation.Status))
				return ctrl.Result{RequeueAfter: time.Minute * 2, Requeue: true}, nil
			}
		} else {
			// Check if we need to activate a newer version
			var currentActiveVersion int
			if activationSpec.Network == "STAGING" {
				currentActiveVersion = akamaiProperty.Status.StagingVersion
			} else {
				currentActiveVersion = akamaiProperty.Status.ProductionVersion
			}

			if versionToActivate > currentActiveVersion {
				needsActivation = true
			}
		}
	}

	if needsActivation {
		logger.Info("Starting property activation", "network", activationSpec.Network, "version", versionToActivate)
		r.updateStatus(ctx, akamaiProperty, PhaseActivating, "Starting activation", fmt.Sprintf("Activating version %d on %s", versionToActivate, activationSpec.Network))

		activationID, err := r.AkamaiClient.ActivateProperty(ctx, akamaiProperty.Status.PropertyID, versionToActivate, activationSpec)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to activate property: %w", err)
		}

		// Update the activation ID and status
		if activationSpec.Network == "STAGING" {
			akamaiProperty.Status.StagingActivationID = activationID
			akamaiProperty.Status.StagingActivationStatus = "PENDING"
		} else {
			akamaiProperty.Status.ProductionActivationID = activationID
			akamaiProperty.Status.ProductionActivationStatus = "PENDING"
		}

		if err := r.Status().Update(ctx, akamaiProperty); err != nil {
			return ctrl.Result{}, err
		}

		logger.Info("Successfully started activation", "activationID", activationID, "network", activationSpec.Network)
		return ctrl.Result{RequeueAfter: time.Minute * 2, Requeue: true}, nil
	}

	return ctrl.Result{}, nil
}

// updateActivationStatus updates the activation status in the AkamaiProperty resource
func (r *AkamaiPropertyReconciler) updateActivationStatus(akamaiProperty *akamaiV1alpha1.AkamaiProperty, network string, activation *akamai.Activation) {
	if network == "STAGING" {
		akamaiProperty.Status.StagingActivationStatus = activation.Status
		if activation.Status == "ACTIVE" {
			akamaiProperty.Status.StagingVersion = activation.PropertyVersion
		}
	} else if network == "PRODUCTION" {
		akamaiProperty.Status.ProductionActivationStatus = activation.Status
		if activation.Status == "ACTIVE" {
			akamaiProperty.Status.ProductionVersion = activation.PropertyVersion
		}
	}
}

// needsUpdate checks if the property needs to be updated
func (r *AkamaiPropertyReconciler) needsUpdate(desired *akamaiV1alpha1.AkamaiProperty, current *akamai.Property) bool {
	// This is a simplified comparison - in a real implementation, you would
	// compare the relevant fields between the desired spec and current state
	return desired.Spec.PropertyName != current.PropertyName ||
		len(desired.Spec.Hostnames) != len(current.Hostnames)
}

// updateStatus updates the status of the AkamaiProperty resource
func (r *AkamaiPropertyReconciler) updateStatus(ctx context.Context, akamaiProperty *akamaiV1alpha1.AkamaiProperty, phase, reason, message string) {
	now := metav1.NewTime(time.Now())
	akamaiProperty.Status.Phase = phase
	akamaiProperty.Status.LastUpdated = &now

	// Update conditions
	condition := metav1.Condition{
		Type:               ConditionTypeReady,
		Status:             metav1.ConditionFalse,
		LastTransitionTime: now,
		Reason:             reason,
		Message:            message,
	}

	if phase == PhaseReady {
		condition.Status = metav1.ConditionTrue
	}

	// Update or add the condition
	updated := false
	for i, existingCondition := range akamaiProperty.Status.Conditions {
		if existingCondition.Type == condition.Type {
			akamaiProperty.Status.Conditions[i] = condition
			updated = true
			break
		}
	}
	if !updated {
		akamaiProperty.Status.Conditions = append(akamaiProperty.Status.Conditions, condition)
	}

	// Update the status
	if err := r.Status().Update(ctx, akamaiProperty); err != nil {
		log.FromContext(ctx).Error(err, "Failed to update status")
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *AkamaiPropertyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&akamaiV1alpha1.AkamaiProperty{}).
		Complete(r)
}
