package controllers

import (
	"context"
	"fmt"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	akamaiV1alpha1 "github.com/mmz-srf/akamai-operator/api/v1alpha1"
	"github.com/mmz-srf/akamai-operator/pkg/akamai"
)

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

			if err := r.updateStatusWithRetry(ctx, akamaiProperty); err != nil {
				return ctrl.Result{}, err
			}

			if activation.Status == "ACTIVE" {
				logger.Info("Activation completed successfully", "network", activationSpec.Network, "version", activation.PropertyVersion)
				// Don't return yet - fall through to check if a newer version needs activation
			} else if activation.Status == "FAILED" {
				logger.Error(nil, "Activation failed", "network", activationSpec.Network, "activationID", currentActivationID)
				r.updateStatus(ctx, akamaiProperty, PhaseError, "ActivationFailed", "Check activation logs")
				return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
			} else {
				// Still in progress
				logger.Info("Activation in progress", "network", activationSpec.Network, "status", activation.Status)
				r.updateStatus(ctx, akamaiProperty, PhaseActivating, "ActivationInProgress", fmt.Sprintf("Status: %s", activation.Status))
				return ctrl.Result{RequeueAfter: time.Minute * 2, Requeue: true}, nil
			}
		}

		// Check if we need to activate a newer version or if note has changed
		var currentActiveVersion int
		var lastActivationNote string
		if activationSpec.Network == "STAGING" {
			currentActiveVersion = akamaiProperty.Status.StagingVersion
			lastActivationNote = akamaiProperty.Status.StagingActivationNote
		} else {
			currentActiveVersion = akamaiProperty.Status.ProductionVersion
			lastActivationNote = akamaiProperty.Status.ProductionActivationNote
		}

		// Check if version is newer
		versionChanged := versionToActivate > currentActiveVersion
		// Check if note has changed
		noteChanged := activationSpec.Note != lastActivationNote

		// Trigger activation ONLY if BOTH conditions are met:
		// 1. Latest version is newer than currently active version AND
		// 2. Activation note has changed
		if versionChanged && noteChanged {
			logger.Info("Activation needed: newer version and note has changed",
				"currentVersion", currentActiveVersion,
				"targetVersion", versionToActivate,
				"oldNote", lastActivationNote,
				"newNote", activationSpec.Note)
			needsActivation = true
		} else if versionChanged && !noteChanged {
			logger.Info("Activation skipped: newer version available but note has not changed",
				"currentVersion", currentActiveVersion,
				"targetVersion", versionToActivate,
				"note", activationSpec.Note)
		} else if !versionChanged && noteChanged {
			logger.Info("Activation skipped: note has changed but no new version available",
				"version", versionToActivate,
				"oldNote", lastActivationNote,
				"newNote", activationSpec.Note)
		}
	}

	if needsActivation {
		logger.Info("Starting property activation", "network", activationSpec.Network, "version", versionToActivate)
		r.updateStatus(ctx, akamaiProperty, PhaseActivating, "StartingActivation", fmt.Sprintf("Activating version %d on %s", versionToActivate, activationSpec.Network))

		activationID, err := r.AkamaiClient.ActivateProperty(ctx, akamaiProperty.Status.PropertyID, versionToActivate, activationSpec, akamaiProperty.Spec.ContractID, akamaiProperty.Spec.GroupID)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to activate property: %w", err)
		}

		// Update the activation ID, status, and note
		if activationSpec.Network == "STAGING" {
			akamaiProperty.Status.StagingActivationID = activationID
			akamaiProperty.Status.StagingActivationStatus = "PENDING"
			akamaiProperty.Status.StagingActivationNote = activationSpec.Note
		} else {
			akamaiProperty.Status.ProductionActivationID = activationID
			akamaiProperty.Status.ProductionActivationStatus = "PENDING"
			akamaiProperty.Status.ProductionActivationNote = activationSpec.Note
		}

		if err := r.updateStatusWithRetry(ctx, akamaiProperty); err != nil {
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
