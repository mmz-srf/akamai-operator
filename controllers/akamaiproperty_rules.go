package controllers

import (
	"context"
	"encoding/json"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/log"

	akamaiV1alpha1 "github.com/mmz-srf/akamai-operator/api/v1alpha1"
)

// updateRulesIfNeeded checks if rules need to be updated and updates them if necessary
func (r *AkamaiPropertyReconciler) updateRulesIfNeeded(ctx context.Context, akamaiProperty *akamaiV1alpha1.AkamaiProperty) (bool, error) {
	logger := log.FromContext(ctx)

	// Validate the rules configuration first
	if err := r.validatePropertyRules(akamaiProperty.Spec.Rules); err != nil {
		return false, fmt.Errorf("rule validation failed: %w", err)
	}

	// Check if the latest version is published on staging or production
	isPublished, network, err := r.AkamaiClient.IsVersionPublished(ctx, akamaiProperty.Status.PropertyID, akamaiProperty.Status.LatestVersion)
	if err != nil {
		return false, fmt.Errorf("failed to check if version is published: %w", err)
	}

	versionToUpdate := akamaiProperty.Status.LatestVersion

	if isPublished {
		// The latest version is published, we need to create a new version
		logger.Info("Latest version is published, creating new version for rules update",
			"currentVersion", akamaiProperty.Status.LatestVersion,
			"network", network)

		newVersion, err := r.AkamaiClient.UpdateProperty(ctx, akamaiProperty.Status.PropertyID, &akamaiProperty.Spec)
		if err != nil {
			return false, fmt.Errorf("failed to create new property version for rules update: %w", err)
		}

		versionToUpdate = newVersion
		akamaiProperty.Status.LatestVersion = newVersion

		// Update the status to reflect the new version
		if err := r.updateStatusWithRetry(ctx, akamaiProperty); err != nil {
			return false, fmt.Errorf("failed to update status with new version: %w", err)
		}

		logger.Info("Created new version for rules update", "newVersion", newVersion)
	}

	// Get current rules from Akamai for the version we're updating
	currentRules, err := r.AkamaiClient.GetPropertyRules(ctx,
		akamaiProperty.Status.PropertyID,
		versionToUpdate,
		akamaiProperty.Spec.ContractID,
		akamaiProperty.Spec.GroupID)
	if err != nil {
		return false, fmt.Errorf("failed to get current property rules: %w", err)
	}

	// Check if rules need updating by comparing desired vs current
	needsUpdate, err := r.rulesNeedUpdate(akamaiProperty.Spec.Rules, currentRules.Rules)
	if err != nil {
		return false, fmt.Errorf("failed to compare rules: %w", err)
	}

	if !needsUpdate {
		logger.V(1).Info("Property rules are up to date", "propertyID", akamaiProperty.Status.PropertyID)
		return false, nil
	}

	logger.Info("Property rules need updating", "propertyID", akamaiProperty.Status.PropertyID, "version", versionToUpdate)
	r.updateStatus(ctx, akamaiProperty, PhaseUpdating, "UpdatingPropertyRules", "")

	// Convert our PropertyRules to the format expected by Akamai
	rulesInterface, err := r.convertRulesToAkamaiFormat(akamaiProperty.Spec.Rules)
	if err != nil {
		return false, fmt.Errorf("failed to convert rules to Akamai format: %w", err)
	}

	// Update the rules
	updatedRules, err := r.AkamaiClient.UpdatePropertyRules(ctx,
		akamaiProperty.Status.PropertyID,
		versionToUpdate,
		akamaiProperty.Spec.ContractID,
		akamaiProperty.Spec.GroupID,
		rulesInterface,
		currentRules.Etag)
	if err != nil {
		return false, fmt.Errorf("failed to update property rules: %w", err)
	}

	logger.Info("Successfully updated property rules",
		"propertyID", akamaiProperty.Status.PropertyID,
		"version", versionToUpdate,
		"newEtag", updatedRules.Etag)

	return true, nil
}

// rulesNeedUpdate compares desired rules with current rules to determine if an update is needed
func (r *AkamaiPropertyReconciler) rulesNeedUpdate(desired *akamaiV1alpha1.PropertyRules, current interface{}) (bool, error) {
	if desired == nil {
		return false, nil
	}

	// Convert current rules to our PropertyRules structure for comparison
	currentRules, err := r.normalizeCurrentRules(current)
	if err != nil {
		return false, fmt.Errorf("failed to normalize current rules: %w", err)
	}

	// Compare the meaningful parts of the rules
	return r.compareRulesDeep(desired, currentRules), nil
}

// convertRulesToAkamaiFormat converts our PropertyRules to the format expected by Akamai API
func (r *AkamaiPropertyReconciler) convertRulesToAkamaiFormat(rules *akamaiV1alpha1.PropertyRules) (interface{}, error) {
	if rules == nil {
		return nil, fmt.Errorf("rules cannot be nil")
	}

	// Convert our custom rule structure to a map that can be marshaled to Akamai format
	// This is a simplified conversion - you might need more sophisticated logic

	// First, marshal our rules to JSON
	ruleBytes, err := json.Marshal(rules)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal rules: %w", err)
	}

	// Then unmarshal to a generic interface{} that can be used with the Akamai API
	var rulesMap map[string]interface{}
	if err := json.Unmarshal(ruleBytes, &rulesMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal rules: %w", err)
	}

	return rulesMap, nil
}
