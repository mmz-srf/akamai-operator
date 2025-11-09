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
	var currentActivationID, currentActivationStatus, lastActivationNote string
	if activationSpec.Network == "STAGING" {
		currentActivationID = akamaiProperty.Status.StagingActivationID
		currentActivationStatus = akamaiProperty.Status.StagingActivationStatus
		lastActivationNote = akamaiProperty.Status.StagingActivationNote
	} else if activationSpec.Network == "PRODUCTION" {
		currentActivationID = akamaiProperty.Status.ProductionActivationID
		currentActivationStatus = akamaiProperty.Status.ProductionActivationStatus
		lastActivationNote = akamaiProperty.Status.ProductionActivationNote
	}

	// Check if activation note has changed - this is the trigger for new activation
	activationNoteChanged := activationSpec.Note != lastActivationNote

	// Check if we need to start a new activation
	needsActivation := false
	if currentActivationID == "" {
		// No previous activation
		needsActivation = true
		logger.Info("No previous activation found, will activate", "network", activationSpec.Network, "version", versionToActivate)
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

			// Check if the in-progress activation is for an older version
			if activation.PropertyVersion < versionToActivate {
				logger.Info("Found activation for older version, will activate newer version after current completes",
					"currentActivationVersion", activation.PropertyVersion,
					"latestVersion", versionToActivate,
					"activationStatus", activation.Status)
				// If the old activation is still pending/activating, wait for it to complete
				// before starting a new one to avoid conflicts
				if activation.Status == "PENDING" || activation.Status == "ACTIVATING" {
					logger.Info("Waiting for older activation to complete before activating newer version",
						"network", activationSpec.Network,
						"oldVersion", activation.PropertyVersion,
						"newVersion", versionToActivate)
					return ctrl.Result{RequeueAfter: time.Minute * 2, Requeue: true}, nil
				}
				// Old activation completed (ACTIVE/FAILED/etc)
				// Only activate if the note has changed (to prevent auto-activation loops)
				if activationNoteChanged {
					logger.Info("Old activation complete and note changed, will activate new version",
						"network", activationSpec.Network,
						"oldVersion", activation.PropertyVersion,
						"newVersion", versionToActivate)
					needsActivation = true
				} else {
					logger.Info("Old activation complete but note unchanged, skipping activation",
						"network", activationSpec.Network,
						"latestVersion", versionToActivate,
						"activeVersion", activation.PropertyVersion)
				}
			} else if activation.PropertyVersion == versionToActivate && (activation.Status == "PENDING" || activation.Status == "ACTIVATING") {
				// Activation already in progress for current version, just monitor it
				logger.Info("Activation in progress for current version", "network", activationSpec.Network, "status", activation.Status, "version", versionToActivate)
				r.updateStatus(ctx, akamaiProperty, PhaseActivating, "ActivationInProgress", fmt.Sprintf("Status: %s", activation.Status))
				return ctrl.Result{RequeueAfter: time.Minute * 2, Requeue: true}, nil
			} else if activation.Status == "ACTIVE" {
				logger.Info("Activation completed successfully", "network", activationSpec.Network, "version", activation.PropertyVersion)
				return ctrl.Result{}, nil
			} else if activation.Status == "FAILED" {
				logger.Error(nil, "Activation failed", "network", activationSpec.Network, "activationID", currentActivationID)
				r.updateStatus(ctx, akamaiProperty, PhaseError, "ActivationFailed", "Check activation logs")
				return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
			} else {
				// Still in progress for current version
				logger.Info("Activation in progress", "network", activationSpec.Network, "status", activation.Status)
				r.updateStatus(ctx, akamaiProperty, PhaseActivating, "ActivationInProgress", fmt.Sprintf("Status: %s", activation.Status))
				return ctrl.Result{RequeueAfter: time.Minute * 2, Requeue: true}, nil
			}
		} else {
			// Check if we need to activate a newer version based on note change
			var currentActiveVersion int
			if activationSpec.Network == "STAGING" {
				currentActiveVersion = akamaiProperty.Status.StagingVersion
			} else {
				currentActiveVersion = akamaiProperty.Status.ProductionVersion
			}

			// Only activate if:
			// 1. The activation note has changed (user explicitly wants new activation), OR
			// 2. There's a newer version AND no active version yet (initial activation)
			if activationNoteChanged {
				logger.Info("Activation note changed, will activate latest version",
					"network", activationSpec.Network,
					"latestVersion", versionToActivate,
					"currentActiveVersion", currentActiveVersion,
					"newNote", activationSpec.Note,
					"oldNote", lastActivationNote)
				needsActivation = true
			} else if versionToActivate > currentActiveVersion && currentActiveVersion == 0 {
				// Initial activation case - no active version yet
				logger.Info("No active version yet, will activate latest version",
					"network", activationSpec.Network,
					"version", versionToActivate)
				needsActivation = true
			} else {
				logger.V(1).Info("Activation not needed - note unchanged and version already active",
					"network", activationSpec.Network,
					"latestVersion", versionToActivate,
					"activeVersion", currentActiveVersion)
			}
		}
	}

	if needsActivation {
		logger.Info("Starting property activation", "network", activationSpec.Network, "version", versionToActivate, "note", activationSpec.Note)
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
