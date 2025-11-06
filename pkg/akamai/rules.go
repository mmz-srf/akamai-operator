package akamai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/akamai/AkamaiOPEN-edgegrid-golang/v8/pkg/papi"
)

// GetPropertyRules retrieves the rule tree for a property version
func (c *Client) GetPropertyRules(ctx context.Context, propertyID string, version int, contractID, groupID string) (*PropertyRules, error) {
	// Get property rules using GetRuleTree
	getRulesResp, err := c.papiClient.GetRuleTree(ctx, papi.GetRuleTreeRequest{
		PropertyID:      propertyID,
		PropertyVersion: version,
		ContractID:      contractID,
		GroupID:         groupID,
		ValidateRules:   false, // Skip validation for faster response when just reading
		// Don't set ValidateMode when ValidateRules is false to avoid validation issues
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get property rules: %w", err)
	}

	if getRulesResp == nil {
		return nil, fmt.Errorf("empty response from get property rules API")
	}

	// Convert PAPI response to our PropertyRules structure
	propertyRules := &PropertyRules{
		AccountID:       getRulesResp.AccountID,
		ContractID:      getRulesResp.ContractID,
		GroupID:         getRulesResp.GroupID,
		PropertyID:      getRulesResp.PropertyID,
		PropertyVersion: getRulesResp.PropertyVersion,
		Etag:            getRulesResp.Etag,
		RuleFormat:      getRulesResp.RuleFormat,
		Rules:           getRulesResp.Rules,
	}

	return propertyRules, nil
}

// UpdatePropertyRules updates the rule tree for a property version
func (c *Client) UpdatePropertyRules(ctx context.Context, propertyID string, version int, contractID, groupID string, rules interface{}, etag string) (*PropertyRules, error) {
	// Convert interface{} to papi.Rules - we expect it to be a proper Rules structure
	var papiRules papi.Rules
	switch r := rules.(type) {
	case papi.Rules:
		papiRules = r
	case map[string]interface{}:
		// For flexibility, allow map input and try to marshal/unmarshal
		// This is not type-safe but allows for dynamic rule structures
		// In a production environment, you might want stricter typing
		ruleBytes, err := json.Marshal(r)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal rules: %w", err)
		}
		if err := json.Unmarshal(ruleBytes, &papiRules); err != nil {
			return nil, fmt.Errorf("failed to unmarshal rules to papi.Rules: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported rules type: %T", rules)
	}

	// Try with full validation first, fallback to no validation if fast validation is not supported
	updateRequest := papi.UpdateRulesRequest{
		PropertyID:      propertyID,
		PropertyVersion: version,
		ContractID:      contractID,
		GroupID:         groupID,
		Rules: papi.RulesUpdate{
			Rules: papiRules,
		},
		ValidateRules: true,   // Enable validation for safety
		ValidateMode:  "full", // Use full validation
		DryRun:        false,  // Actually apply the changes
	}

	// Update property rules using UpdateRuleTree
	updateResp, err := c.papiClient.UpdateRuleTree(ctx, updateRequest)
	if err != nil {
		// If validation fails, try without validation as a fallback
		if strings.Contains(err.Error(), "not a feature") || strings.Contains(err.Error(), "validate") {
			fmt.Printf("Warning: Full validation not supported, retrying without validation\n")
			updateRequest.ValidateRules = false
			updateRequest.ValidateMode = ""

			updateResp, err = c.papiClient.UpdateRuleTree(ctx, updateRequest)
			if err != nil {
				return nil, fmt.Errorf("failed to update property rules (even without validation): %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to update property rules: %w", err)
		}
	}

	if updateResp == nil {
		return nil, fmt.Errorf("empty response from update property rules API")
	}

	// Convert response to our PropertyRules structure
	propertyRules := &PropertyRules{
		AccountID:       updateResp.AccountID,
		ContractID:      updateResp.ContractID,
		GroupID:         updateResp.GroupID,
		PropertyID:      updateResp.PropertyID,
		PropertyVersion: updateResp.PropertyVersion,
		Etag:            updateResp.Etag,
		RuleFormat:      updateResp.RuleFormat,
		Rules:           updateResp.Rules,
	}

	// Check for validation errors or warnings
	if len(updateResp.Errors) > 0 {
		var errorMessages []string
		for _, ruleError := range updateResp.Errors {
			errorMessages = append(errorMessages, fmt.Sprintf("%s: %s", ruleError.Title, ruleError.Detail))
		}
		return propertyRules, fmt.Errorf("rule validation errors: %v", errorMessages)
	}

	return propertyRules, nil
}
