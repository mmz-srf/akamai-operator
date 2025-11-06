package controllers

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	akamaiV1alpha1 "github.com/mmz-srf/akamai-operator/api/v1alpha1"
)

// updateStatusWithRetry updates the status with retry logic for resource conflicts
func (r *AkamaiPropertyReconciler) updateStatusWithRetry(ctx context.Context, akamaiProperty *akamaiV1alpha1.AkamaiProperty) error {
	const maxRetries = 3
	logger := log.FromContext(ctx)

	for attempt := 0; attempt < maxRetries; attempt++ {
		// Get the latest version of the resource to avoid conflicts
		var latest akamaiV1alpha1.AkamaiProperty
		if err := r.Get(ctx, client.ObjectKeyFromObject(akamaiProperty), &latest); err != nil {
			logger.Error(err, "Failed to get latest resource version", "attempt", attempt+1)
			return err
		}

		// Update the status on the latest version, preserving other fields
		latest.Status.PropertyID = akamaiProperty.Status.PropertyID
		latest.Status.LatestVersion = akamaiProperty.Status.LatestVersion
		latest.Status.StagingVersion = akamaiProperty.Status.StagingVersion
		latest.Status.ProductionVersion = akamaiProperty.Status.ProductionVersion
		latest.Status.StagingActivationID = akamaiProperty.Status.StagingActivationID
		latest.Status.ProductionActivationID = akamaiProperty.Status.ProductionActivationID
		latest.Status.StagingActivationStatus = akamaiProperty.Status.StagingActivationStatus
		latest.Status.ProductionActivationStatus = akamaiProperty.Status.ProductionActivationStatus
		latest.Status.Phase = akamaiProperty.Status.Phase
		latest.Status.LastUpdated = akamaiProperty.Status.LastUpdated
		latest.Status.Conditions = akamaiProperty.Status.Conditions

		// Try to update the status
		if err := r.Status().Update(ctx, &latest); err != nil {
			logger.Error(err, "Failed to update status", "attempt", attempt+1)
			if attempt == maxRetries-1 {
				return fmt.Errorf("failed to update status after %d retries: %w", maxRetries, err)
			}
			// Wait a bit before retrying
			time.Sleep(time.Millisecond * 100 * time.Duration(attempt+1))
			continue
		}

		// Success - update the original object with the latest status for future use
		akamaiProperty.Status = latest.Status
		akamaiProperty.ObjectMeta.ResourceVersion = latest.ObjectMeta.ResourceVersion
		logger.V(1).Info("Successfully updated status")
		return nil
	}

	return fmt.Errorf("failed to update status after %d retries", maxRetries)
}

// updateStatus updates the status of the AkamaiProperty resource with retry logic
func (r *AkamaiPropertyReconciler) updateStatus(ctx context.Context, akamaiProperty *akamaiV1alpha1.AkamaiProperty, phase, reason, message string) {
	const maxRetries = 3
	logger := log.FromContext(ctx)

	for attempt := 0; attempt < maxRetries; attempt++ {
		// Get the latest version of the resource to avoid conflicts
		var latest akamaiV1alpha1.AkamaiProperty
		if err := r.Get(ctx, client.ObjectKeyFromObject(akamaiProperty), &latest); err != nil {
			logger.Error(err, "Failed to get latest resource version", "attempt", attempt+1)
			continue
		}

		// Check if status actually needs to be updated
		statusChanged := false

		// Check if phase changed
		if latest.Status.Phase != phase {
			statusChanged = true
		}

		// Update the status on the latest version
		now := metav1.NewTime(time.Now())
		latest.Status.Phase = phase

		// Only update LastUpdated timestamp if status actually changed
		if statusChanged {
			latest.Status.LastUpdated = &now
		}

		// Preserve existing status fields that might have been set elsewhere
		if latest.Status.PropertyID == "" && akamaiProperty.Status.PropertyID != "" {
			latest.Status.PropertyID = akamaiProperty.Status.PropertyID
		}
		if latest.Status.LatestVersion == 0 && akamaiProperty.Status.LatestVersion != 0 {
			latest.Status.LatestVersion = akamaiProperty.Status.LatestVersion
		}
		if latest.Status.StagingVersion == 0 && akamaiProperty.Status.StagingVersion != 0 {
			latest.Status.StagingVersion = akamaiProperty.Status.StagingVersion
		}
		if latest.Status.ProductionVersion == 0 && akamaiProperty.Status.ProductionVersion != 0 {
			latest.Status.ProductionVersion = akamaiProperty.Status.ProductionVersion
		}
		if latest.Status.StagingActivationID == "" && akamaiProperty.Status.StagingActivationID != "" {
			latest.Status.StagingActivationID = akamaiProperty.Status.StagingActivationID
		}
		if latest.Status.ProductionActivationID == "" && akamaiProperty.Status.ProductionActivationID != "" {
			latest.Status.ProductionActivationID = akamaiProperty.Status.ProductionActivationID
		}
		if latest.Status.StagingActivationStatus == "" && akamaiProperty.Status.StagingActivationStatus != "" {
			latest.Status.StagingActivationStatus = akamaiProperty.Status.StagingActivationStatus
		}
		if latest.Status.ProductionActivationStatus == "" && akamaiProperty.Status.ProductionActivationStatus != "" {
			latest.Status.ProductionActivationStatus = akamaiProperty.Status.ProductionActivationStatus
		}

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
		conditionChanged := false
		updated := false
		for i, existingCondition := range latest.Status.Conditions {
			if existingCondition.Type == condition.Type {
				// Check if condition actually changed
				if existingCondition.Status != condition.Status ||
					existingCondition.Reason != condition.Reason ||
					existingCondition.Message != condition.Message {
					conditionChanged = true
					condition.LastTransitionTime = now
				} else {
					// Preserve the existing LastTransitionTime if nothing changed
					condition.LastTransitionTime = existingCondition.LastTransitionTime
				}
				latest.Status.Conditions[i] = condition
				updated = true
				break
			}
		}
		if !updated {
			latest.Status.Conditions = append(latest.Status.Conditions, condition)
			conditionChanged = true
		}

		// If nothing changed, skip the update
		if !statusChanged && !conditionChanged {
			logger.V(1).Info("Status unchanged, skipping update", "phase", phase, "reason", reason)
			// Still update the in-memory object for consistency
			akamaiProperty.Status = latest.Status
			akamaiProperty.ObjectMeta.ResourceVersion = latest.ObjectMeta.ResourceVersion
			return
		}

		// Try to update the status
		if err := r.Status().Update(ctx, &latest); err != nil {
			logger.Error(err, "Failed to update status", "attempt", attempt+1)
			if attempt == maxRetries-1 {
				logger.Error(err, "Failed to update status after all retries")
				return
			}
			// Wait a bit before retrying to allow other operations to complete
			time.Sleep(time.Millisecond * 100 * time.Duration(attempt+1))
			continue
		}

		// Success - update the original object with the latest status for future use
		akamaiProperty.Status = latest.Status
		akamaiProperty.ObjectMeta.ResourceVersion = latest.ObjectMeta.ResourceVersion
		logger.V(1).Info("Successfully updated status", "phase", phase, "reason", reason)
		return
	}
}
