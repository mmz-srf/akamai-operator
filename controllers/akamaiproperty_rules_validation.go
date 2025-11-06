package controllers

import (
	"encoding/json"
	"fmt"
	"strings"

	akamaiV1alpha1 "github.com/mmz-srf/akamai-operator/api/v1alpha1"
)

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
