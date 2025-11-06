package controllers

import (
	"context"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	akamaiV1alpha1 "github.com/mmz-srf/akamai-operator/api/v1alpha1"
	"github.com/mmz-srf/akamai-operator/pkg/akamai"
)

// reconcileProperty handles the main reconciliation logic
func (r *AkamaiPropertyReconciler) reconcileProperty(ctx context.Context, akamaiProperty *akamaiV1alpha1.AkamaiProperty) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Check if property exists in Akamai
	if akamaiProperty.Status.PropertyID == "" {
		// Property doesn't exist, create it
		logger.Info("Creating new Akamai property", "propertyName", akamaiProperty.Spec.PropertyName)
		r.updateStatus(ctx, akamaiProperty, PhaseCreating, "CreatingAkamaiProperty", "")

		propertyID, err := r.AkamaiClient.CreateProperty(ctx, &akamaiProperty.Spec)
		if err != nil {
			logger.Error(err, "Failed to create Akamai property")
			r.updateStatus(ctx, akamaiProperty, PhaseError, "FailedToCreateProperty", err.Error())
			return ctrl.Result{RequeueAfter: time.Minute * 2}, nil
		}

		akamaiProperty.Status.PropertyID = propertyID
		akamaiProperty.Status.LatestVersion = 1
		akamaiProperty.Status.Phase = PhaseReady

		if err := r.updateStatusWithRetry(ctx, akamaiProperty); err != nil {
			return ctrl.Result{}, err
		}

		// Update hostnames if specified after property creation
		if len(akamaiProperty.Spec.Hostnames) > 0 {
			err = r.AkamaiClient.SetPropertyHostnames(ctx, propertyID,
				akamaiProperty.Spec.ContractID,
				akamaiProperty.Spec.GroupID,
				1, // Initial version is 1
				akamaiProperty.Spec.Hostnames)
			if err != nil {
				logger.Error(err, "Failed to set initial hostnames")
				r.updateStatus(ctx, akamaiProperty, PhaseError, "FailedToSetInitialHostnames", err.Error())
				return ctrl.Result{RequeueAfter: time.Minute * 2}, nil
			}
			logger.Info("Successfully set initial hostnames", "count", len(akamaiProperty.Spec.Hostnames))
		}

		logger.Info("Successfully created Akamai property", "propertyID", propertyID)
		r.updateStatus(ctx, akamaiProperty, PhaseReady, "PropertyCreatedSuccessfully", "")
		return ctrl.Result{RequeueAfter: time.Minute * 10}, nil
	}

	// Property exists, check if it needs to be updated
	currentProperty, err := r.AkamaiClient.GetProperty(ctx, akamaiProperty.Status.PropertyID)
	if err != nil {
		logger.Error(err, "Failed to get Akamai property")
		r.updateStatus(ctx, akamaiProperty, PhaseError, "FailedToRetrieveProperty", err.Error())
		return ctrl.Result{RequeueAfter: time.Minute * 2}, nil
	}

	// Check if property needs to be updated
	if r.needsUpdate(akamaiProperty, currentProperty) {
		logger.Info("Updating Akamai property", "propertyID", akamaiProperty.Status.PropertyID)
		r.updateStatus(ctx, akamaiProperty, PhaseUpdating, "UpdatingAkamaiProperty", "")

		newVersion, err := r.AkamaiClient.UpdateProperty(ctx, akamaiProperty.Status.PropertyID, &akamaiProperty.Spec)
		if err != nil {
			logger.Error(err, "Failed to update Akamai property")
			r.updateStatus(ctx, akamaiProperty, PhaseError, "FailedToUpdateProperty", err.Error())
			return ctrl.Result{RequeueAfter: time.Minute * 2}, nil
		}

		akamaiProperty.Status.LatestVersion = newVersion
		if err := r.updateStatusWithRetry(ctx, akamaiProperty); err != nil {
			return ctrl.Result{}, err
		}

		logger.Info("Successfully updated Akamai property", "propertyID", akamaiProperty.Status.PropertyID, "version", newVersion)
	}

	// Check if rules need to be updated
	if akamaiProperty.Spec.Rules != nil {
		rulesUpdated, err := r.updateRulesIfNeeded(ctx, akamaiProperty)
		if err != nil {
			logger.Error(err, "Failed to update property rules")
			r.updateStatus(ctx, akamaiProperty, PhaseError, "FailedToUpdateRules", err.Error())
			return ctrl.Result{RequeueAfter: time.Minute * 2}, nil
		}
		if rulesUpdated {
			logger.Info("Successfully updated property rules", "propertyID", akamaiProperty.Status.PropertyID)
		}
	} else {
		logger.V(1).Info("Property is up to date, no update needed", "propertyID", akamaiProperty.Status.PropertyID)
	}

	// Handle activation if specified
	if akamaiProperty.Spec.Activation != nil {
		activationResult, err := r.handleActivation(ctx, akamaiProperty)
		if err != nil {
			logger.Error(err, "Failed to handle activation")
			r.updateStatus(ctx, akamaiProperty, PhaseError, "FailedToHandleActivation", err.Error())
			return ctrl.Result{RequeueAfter: time.Minute * 2}, nil
		}
		if activationResult.Requeue {
			return activationResult, nil
		}
	}

	r.updateStatus(ctx, akamaiProperty, PhaseReady, "PropertyIsReady", "")
	return ctrl.Result{RequeueAfter: time.Minute * 30}, nil
}

// handleDeletion handles the deletion of the AkamaiProperty resource
func (r *AkamaiPropertyReconciler) handleDeletion(ctx context.Context, akamaiProperty *akamaiV1alpha1.AkamaiProperty) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if controllerutil.ContainsFinalizer(akamaiProperty, FinalizerName) {
		// Update status to indicate deletion is in progress
		r.updateStatus(ctx, akamaiProperty, PhaseDeleting, "DeletingAkamaiProperty", "")

		// Delete the property from Akamai if it exists
		if akamaiProperty.Status.PropertyID != "" {
			logger.Info("Deleting Akamai property", "propertyID", akamaiProperty.Status.PropertyID)

			err := r.AkamaiClient.DeleteProperty(ctx, akamaiProperty.Status.PropertyID)
			if err != nil {
				logger.Error(err, "Failed to delete Akamai property")
				r.updateStatus(ctx, akamaiProperty, PhaseError, "FailedToDeleteProperty", err.Error())
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

// needsUpdate checks if the property needs to be updated
func (r *AkamaiPropertyReconciler) needsUpdate(desired *akamaiV1alpha1.AkamaiProperty, current *akamai.Property) bool {
	logger := log.FromContext(context.Background())

	// Compare property name
	if desired.Spec.PropertyName != current.PropertyName {
		logger.V(1).Info("Property name differs", "desired", desired.Spec.PropertyName, "current", current.PropertyName)
		return true
	}

	// Compare hostnames if specified in the desired state
	if len(desired.Spec.Hostnames) > 0 {
		if akamai.CompareHostnames(desired.Spec.Hostnames, current.Hostnames) {
			logger.V(1).Info("Hostnames differ, update needed",
				"desiredCount", len(desired.Spec.Hostnames),
				"currentCount", len(current.Hostnames))
			return true
		}
	}

	// Property is up to date
	logger.V(1).Info("Property is up to date", "propertyName", current.PropertyName)
	return false
}
