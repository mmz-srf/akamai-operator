package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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
				r.updateStatus(ctx, akamaiProperty, PhaseError, "ActivationFailed", "Check activation logs")
				return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
			} else {
				// Still in progress
				logger.Info("Activation in progress", "network", activationSpec.Network, "status", activation.Status)
				r.updateStatus(ctx, akamaiProperty, PhaseActivating, "ActivationInProgress", fmt.Sprintf("Status: %s", activation.Status))
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
		r.updateStatus(ctx, akamaiProperty, PhaseActivating, "StartingActivation", fmt.Sprintf("Activating version %d on %s", versionToActivate, activationSpec.Network))

		activationID, err := r.AkamaiClient.ActivateProperty(ctx, akamaiProperty.Status.PropertyID, versionToActivate, activationSpec, akamaiProperty.Spec.ContractID, akamaiProperty.Spec.GroupID)
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

// updateRulesIfNeeded checks if rules need to be updated and updates them if necessary
func (r *AkamaiPropertyReconciler) updateRulesIfNeeded(ctx context.Context, akamaiProperty *akamaiV1alpha1.AkamaiProperty) (bool, error) {
	logger := log.FromContext(ctx)

	// Validate the rules configuration first
	if err := r.validatePropertyRules(akamaiProperty.Spec.Rules); err != nil {
		return false, fmt.Errorf("rule validation failed: %w", err)
	}

	// Get current rules from Akamai
	currentRules, err := r.AkamaiClient.GetPropertyRules(ctx,
		akamaiProperty.Status.PropertyID,
		akamaiProperty.Status.LatestVersion,
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

	logger.Info("Property rules need updating", "propertyID", akamaiProperty.Status.PropertyID)
	r.updateStatus(ctx, akamaiProperty, PhaseUpdating, "UpdatingPropertyRules", "")

	// Convert our PropertyRules to the format expected by Akamai
	rulesInterface, err := r.convertRulesToAkamaiFormat(akamaiProperty.Spec.Rules)
	if err != nil {
		return false, fmt.Errorf("failed to convert rules to Akamai format: %w", err)
	}

	// Update the rules
	updatedRules, err := r.AkamaiClient.UpdatePropertyRules(ctx,
		akamaiProperty.Status.PropertyID,
		akamaiProperty.Status.LatestVersion,
		akamaiProperty.Spec.ContractID,
		akamaiProperty.Spec.GroupID,
		rulesInterface,
		currentRules.Etag)
	if err != nil {
		return false, fmt.Errorf("failed to update property rules: %w", err)
	}

	logger.Info("Successfully updated property rules",
		"propertyID", akamaiProperty.Status.PropertyID,
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

// normalizeCurrentRules converts Akamai API response rules to our PropertyRules structure
func (r *AkamaiPropertyReconciler) normalizeCurrentRules(current interface{}) (*akamaiV1alpha1.PropertyRules, error) {
	// Marshal to JSON and then unmarshal to our structure
	// This normalizes the structure and removes fields we don't care about
	currentBytes, err := json.Marshal(current)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal current rules: %w", err)
	}

	var currentRules akamaiV1alpha1.PropertyRules
	if err := json.Unmarshal(currentBytes, &currentRules); err != nil {
		return nil, fmt.Errorf("failed to unmarshal current rules: %w", err)
	}

	// Clean up Akamai-generated fields that shouldn't affect comparison
	r.cleanRulesForComparison(&currentRules)

	return &currentRules, nil
}

// cleanRulesForComparison removes or normalizes fields that shouldn't affect rule comparison
func (r *AkamaiPropertyReconciler) cleanRulesForComparison(rules *akamaiV1alpha1.PropertyRules) {
	// Remove or reset fields that are auto-generated by Akamai and don't represent meaningful changes
	// Reset UUID as it's auto-generated
	rules.UUID = ""

	// Normalize empty/default values for top-level fields
	// Empty criteriaMustSatisfy should be treated same as "all" (Akamai default)
	if rules.CriteriaMustSatisfy == "" {
		rules.CriteriaMustSatisfy = "all"
	}

	// Normalize options field - empty object or null should be treated the same
	if rules.Options.Raw != nil {
		var optionsMap map[string]interface{}
		if err := json.Unmarshal(rules.Options.Raw, &optionsMap); err == nil {
			if len(optionsMap) == 0 {
				// Empty options object, set to nil for consistency
				rules.Options.Raw = nil
			}
		}
	}

	// Normalize customOverride - null should be treated same as not present
	if rules.CustomOverride.Raw != nil {
		var customOverrideVal interface{}
		if err := json.Unmarshal(rules.CustomOverride.Raw, &customOverrideVal); err == nil {
			if customOverrideVal == nil {
				rules.CustomOverride.Raw = nil
			}
		}
	}

	// Clean up behaviors
	for i := range rules.Behaviors {
		r.cleanBehaviorForComparison(&rules.Behaviors[i])
	}

	// Clean up criteria
	for i := range rules.Criteria {
		r.cleanCriteriaForComparison(&rules.Criteria[i])
	}

	// Recursively clean child rules
	for i := range rules.Children {
		var childRule akamaiV1alpha1.PropertyRules
		if err := json.Unmarshal(rules.Children[i].Raw, &childRule); err == nil {
			r.cleanRulesForComparison(&childRule)
			// Marshal it back
			if cleanBytes, err := json.Marshal(childRule); err == nil {
				rules.Children[i].Raw = cleanBytes
			}
		}
	}
}

// cleanBehaviorForComparison cleans up behavior for comparison
func (r *AkamaiPropertyReconciler) cleanBehaviorForComparison(behavior *akamaiV1alpha1.RuleBehavior) {
	// Reset auto-generated fields
	behavior.UUID = ""

	// Normalize options by removing empty values and Akamai-generated fields
	if behavior.Options.Raw != nil {
		var options map[string]interface{}
		if err := json.Unmarshal(behavior.Options.Raw, &options); err == nil {
			r.normalizeOptionsMap(options)
			if cleanBytes, err := json.Marshal(options); err == nil {
				behavior.Options.Raw = cleanBytes
			}
		}
	}
}

// cleanCriteriaForComparison cleans up criteria for comparison
func (r *AkamaiPropertyReconciler) cleanCriteriaForComparison(criteria *akamaiV1alpha1.RuleCriteria) {
	// Reset auto-generated fields
	criteria.UUID = ""

	// Normalize options
	if criteria.Options.Raw != nil {
		var options map[string]interface{}
		if err := json.Unmarshal(criteria.Options.Raw, &options); err == nil {
			r.normalizeOptionsMap(options)
			if cleanBytes, err := json.Marshal(options); err == nil {
				criteria.Options.Raw = cleanBytes
			}
		}
	}
}

// normalizeOptionsMap normalizes an options map by removing irrelevant fields
func (r *AkamaiPropertyReconciler) normalizeOptionsMap(options map[string]interface{}) {
	// Remove fields that are commonly auto-generated or irrelevant for comparison
	fieldsToRemove := []string{
		"uuid",
		"templateUuid",
		"lastModified",
		"created",
		"etag",
		"ruleFormat",
	}

	for _, field := range fieldsToRemove {
		delete(options, field)
	}

	// Remove empty string values that might be defaults
	for key, value := range options {
		if str, ok := value.(string); ok && str == "" {
			delete(options, key)
		}
		// Remove empty maps/objects
		if mapVal, ok := value.(map[string]interface{}); ok && len(mapVal) == 0 {
			delete(options, key)
		}
		// Remove empty arrays/slices
		if sliceVal, ok := value.([]interface{}); ok && len(sliceVal) == 0 {
			delete(options, key)
		}
	}
}

// compareRulesDeep performs a deep comparison of two PropertyRules structures
func (r *AkamaiPropertyReconciler) compareRulesDeep(desired, current *akamaiV1alpha1.PropertyRules) bool {
	// Create clean copies for comparison
	desiredClean := r.copyAndCleanRules(desired)
	currentClean := r.copyAndCleanRules(current)

	// Convert both to normalized JSON for comparison
	desiredBytes, err := json.Marshal(desiredClean)
	if err != nil {
		return true // If we can't marshal, assume they're different
	}

	currentBytes, err := json.Marshal(currentClean)
	if err != nil {
		return true // If we can't marshal, assume they're different
	}

	// Normalize JSON by unmarshaling and marshaling again to ensure consistent ordering
	var desiredNormalized, currentNormalized map[string]interface{}

	if err := json.Unmarshal(desiredBytes, &desiredNormalized); err != nil {
		return true
	}
	if err := json.Unmarshal(currentBytes, &currentNormalized); err != nil {
		return true
	}

	// Final normalization pass to handle null vs empty object/array equivalence
	r.normalizeMapForComparison(desiredNormalized)
	r.normalizeMapForComparison(currentNormalized)

	desiredFinal, _ := json.Marshal(desiredNormalized)
	currentFinal, _ := json.Marshal(currentNormalized)

	// Compare normalized JSON
	different := string(desiredFinal) != string(currentFinal)

	if different {
		logger := log.FromContext(context.Background())
		logger.V(1).Info("Rules differ",
			"desired", string(desiredFinal),
			"current", string(currentFinal))
	}

	return different
}

// normalizeMapForComparison recursively normalizes a map to handle null/empty equivalence
func (r *AkamaiPropertyReconciler) normalizeMapForComparison(m map[string]interface{}) {
	for key, value := range m {
		switch v := value.(type) {
		case map[string]interface{}:
			// Recursively normalize nested maps
			r.normalizeMapForComparison(v)
			// Remove empty maps (treat as null/not present)
			if len(v) == 0 {
				delete(m, key)
			}
		case []interface{}:
			// Recursively normalize array elements
			for _, item := range v {
				if itemMap, ok := item.(map[string]interface{}); ok {
					r.normalizeMapForComparison(itemMap)
				}
			}
			// Remove empty arrays (treat as null/not present)
			if len(v) == 0 {
				delete(m, key)
			}
		case string:
			// Remove empty strings
			if v == "" {
				delete(m, key)
			}
		case nil:
			// Remove null values
			delete(m, key)
		}
	}
}

// copyAndCleanRules creates a clean copy of rules for comparison
func (r *AkamaiPropertyReconciler) copyAndCleanRules(rules *akamaiV1alpha1.PropertyRules) *akamaiV1alpha1.PropertyRules {
	if rules == nil {
		return nil
	}

	// Deep copy via JSON marshal/unmarshal
	rulesBytes, err := json.Marshal(rules)
	if err != nil {
		return rules // Return original if copy fails
	}

	var rulesCopy akamaiV1alpha1.PropertyRules
	if err := json.Unmarshal(rulesBytes, &rulesCopy); err != nil {
		return rules // Return original if copy fails
	}

	// Clean the copy
	r.cleanRulesForComparison(&rulesCopy)

	return &rulesCopy
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

// validatePropertyRules validates the structure and content of property rules
func (r *AkamaiPropertyReconciler) validatePropertyRules(rules *akamaiV1alpha1.PropertyRules) error {
	if rules == nil {
		return nil // Rules are optional
	}

	// Validate required fields for top-level rule
	if rules.Name == "" {
		return fmt.Errorf("top-level rule must have a name (typically 'default')")
	}

	// For top-level rule, name should be "default"
	if rules.Name != "default" {
		return fmt.Errorf("top-level rule name should be 'default', got '%s'", rules.Name)
	}

	// Validate behaviors
	for i, behavior := range rules.Behaviors {
		if err := r.validateRuleBehavior(&behavior, fmt.Sprintf("behavior[%d]", i)); err != nil {
			return fmt.Errorf("invalid behavior at index %d: %w", i, err)
		}
	}

	// Validate criteria
	for i, criterion := range rules.Criteria {
		if err := r.validateRuleCriteria(&criterion, fmt.Sprintf("criteria[%d]", i)); err != nil {
			return fmt.Errorf("invalid criteria at index %d: %w", i, err)
		}
	}

	// Validate variables
	variableNames := make(map[string]bool)
	for i, variable := range rules.Variables {
		if err := r.validateRuleVariable(&variable, fmt.Sprintf("variable[%d]", i)); err != nil {
			return fmt.Errorf("invalid variable at index %d: %w", i, err)
		}

		// Check for duplicate variable names
		if variableNames[variable.Name] {
			return fmt.Errorf("duplicate variable name '%s' at index %d", variable.Name, i)
		}
		variableNames[variable.Name] = true
	}

	// Recursively validate child rules
	for i, childRaw := range rules.Children {
		// Unmarshal the raw child into a PropertyRules struct for validation
		var child akamaiV1alpha1.PropertyRules
		if err := json.Unmarshal(childRaw.Raw, &child); err != nil {
			return fmt.Errorf("invalid child rule at index %d: failed to parse child rule: %w", i, err)
		}

		if err := r.validatePropertyRules(&child); err != nil {
			return fmt.Errorf("invalid child rule at index %d: %w", i, err)
		}
	}

	return nil
}

// validateRuleBehavior validates a single rule behavior
func (r *AkamaiPropertyReconciler) validateRuleBehavior(behavior *akamaiV1alpha1.RuleBehavior, path string) error {
	if behavior.Name == "" {
		return fmt.Errorf("%s: behavior name is required", path)
	}

	// Basic validation for common behaviors
	switch behavior.Name {
	case "origin":
		// Origin behavior should have hostname
		if behavior.Options.Raw == nil {
			return fmt.Errorf("%s: origin behavior requires options", path)
		}
	case "caching":
		// Caching behavior validation
		if behavior.Options.Raw == nil {
			return fmt.Errorf("%s: caching behavior requires options", path)
		}
	case "compress":
		// Compression behavior validation
		// Options are optional for this behavior
	default:
		// For unknown behaviors, just ensure they have a name
		// The Akamai API will validate the specific behavior options
	}

	return nil
}

// validateRuleCriteria validates a single rule criteria
func (r *AkamaiPropertyReconciler) validateRuleCriteria(criteria *akamaiV1alpha1.RuleCriteria, path string) error {
	if criteria.Name == "" {
		return fmt.Errorf("%s: criteria name is required", path)
	}

	// Basic validation for common criteria
	switch criteria.Name {
	case "hostname":
		// Hostname criteria should have values
		if criteria.Options.Raw == nil {
			return fmt.Errorf("%s: hostname criteria requires options with values", path)
		}
	case "path":
		// Path criteria should have values
		if criteria.Options.Raw == nil {
			return fmt.Errorf("%s: path criteria requires options with values", path)
		}
	case "requestMethod":
		// Request method criteria validation
		if criteria.Options.Raw == nil {
			return fmt.Errorf("%s: requestMethod criteria requires options", path)
		}
	default:
		// For unknown criteria, just ensure they have a name
		// The Akamai API will validate the specific criteria options
	}

	return nil
}

// validateRuleVariable validates a single rule variable
func (r *AkamaiPropertyReconciler) validateRuleVariable(variable *akamaiV1alpha1.RuleVariable, path string) error {
	if variable.Name == "" {
		return fmt.Errorf("%s: variable name is required", path)
	}

	// Variable name should be uppercase and follow conventions
	if variable.Name != strings.ToUpper(variable.Name) {
		return fmt.Errorf("%s: variable name '%s' should be uppercase", path, variable.Name)
	}

	// Variable name should not contain spaces
	if strings.Contains(variable.Name, " ") {
		return fmt.Errorf("%s: variable name '%s' should not contain spaces", path, variable.Name)
	}

	return nil
}

// needsUpdate checks if the property needs to be updated
func (r *AkamaiPropertyReconciler) needsUpdate(desired *akamaiV1alpha1.AkamaiProperty, current *akamai.Property) bool {
	logger := log.FromContext(context.Background())

	// Compare property name
	if desired.Spec.PropertyName != current.PropertyName {
		logger.V(1).Info("Property name differs", "desired", desired.Spec.PropertyName, "current", current.PropertyName)
		return true
	}

	// For now, don't compare hostnames as they might be managed separately
	// In a real implementation, you would fetch and compare actual property configuration
	// like rules, hostnames, etc. from the property version

	// Since we're not implementing full property configuration management yet,
	// we'll only update if the basic property metadata differs
	logger.V(1).Info("Property is up to date", "propertyName", current.PropertyName)
	return false
}

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

		// Update the status on the latest version
		now := metav1.NewTime(time.Now())
		latest.Status.Phase = phase
		latest.Status.LastUpdated = &now

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
		updated := false
		for i, existingCondition := range latest.Status.Conditions {
			if existingCondition.Type == condition.Type {
				latest.Status.Conditions[i] = condition
				updated = true
				break
			}
		}
		if !updated {
			latest.Status.Conditions = append(latest.Status.Conditions, condition)
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

// SetupWithManager sets up the controller with the Manager.
func (r *AkamaiPropertyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&akamaiV1alpha1.AkamaiProperty{}).
		Complete(r)
}
