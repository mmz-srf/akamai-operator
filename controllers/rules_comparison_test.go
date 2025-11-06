package controllers

import (
	"encoding/json"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"

	akamaiV1alpha1 "github.com/mmz-srf/akamai-operator/api/v1alpha1"
)

func TestRulesNeedUpdate(t *testing.T) {
	reconciler := &AkamaiPropertyReconciler{}

	tests := []struct {
		name     string
		desired  *akamaiV1alpha1.PropertyRules
		current  interface{}
		expected bool
	}{
		{
			name:     "nil desired rules",
			desired:  nil,
			current:  map[string]interface{}{},
			expected: false,
		},
		{
			name: "identical rules",
			desired: &akamaiV1alpha1.PropertyRules{
				Name: "default",
				Behaviors: []akamaiV1alpha1.RuleBehavior{
					{
						Name: "origin",
						Options: runtime.RawExtension{
							Raw: []byte(`{"hostname":"example.com","httpPort":80}`),
						},
					},
				},
			},
			current: map[string]interface{}{
				"name": "default",
				"behaviors": []map[string]interface{}{
					{
						"name": "origin",
						"options": map[string]interface{}{
							"hostname": "example.com",
							"httpPort": 80,
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "different behavior options",
			desired: &akamaiV1alpha1.PropertyRules{
				Name: "default",
				Behaviors: []akamaiV1alpha1.RuleBehavior{
					{
						Name: "origin",
						Options: runtime.RawExtension{
							Raw: []byte(`{"hostname":"example.com","httpPort":80}`),
						},
					},
				},
			},
			current: map[string]interface{}{
				"name": "default",
				"behaviors": []map[string]interface{}{
					{
						"name": "origin",
						"options": map[string]interface{}{
							"hostname": "different.com",
							"httpPort": 80,
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "ignore auto-generated UUID differences",
			desired: &akamaiV1alpha1.PropertyRules{
				Name: "default",
				UUID: "", // No UUID in desired
				Behaviors: []akamaiV1alpha1.RuleBehavior{
					{
						Name: "origin",
						Options: runtime.RawExtension{
							Raw: []byte(`{"hostname":"example.com","httpPort":80}`),
						},
					},
				},
			},
			current: map[string]interface{}{
				"name": "default",
				"uuid": "auto-generated-uuid-12345", // UUID present in current
				"behaviors": []map[string]interface{}{
					{
						"name": "origin",
						"uuid": "behavior-uuid-67890",
						"options": map[string]interface{}{
							"hostname": "example.com",
							"httpPort": 80,
						},
					},
				},
			},
			expected: false, // Should be considered identical after cleaning UUIDs
		},
		{
			name: "ignore empty string values",
			desired: &akamaiV1alpha1.PropertyRules{
				Name: "default",
				Behaviors: []akamaiV1alpha1.RuleBehavior{
					{
						Name: "origin",
						Options: runtime.RawExtension{
							Raw: []byte(`{"hostname":"example.com","httpPort":80}`),
						},
					},
				},
			},
			current: map[string]interface{}{
				"name": "default",
				"behaviors": []map[string]interface{}{
					{
						"name": "origin",
						"options": map[string]interface{}{
							"hostname":   "example.com",
							"httpPort":   80,
							"emptyField": "", // Empty field should be ignored
							"otherEmpty": "",
						},
					},
				},
			},
			expected: false, // Should be considered identical after cleaning empty fields
		},
		{
			name: "different criteria",
			desired: &akamaiV1alpha1.PropertyRules{
				Name: "default",
				Criteria: []akamaiV1alpha1.RuleCriteria{
					{
						Name: "hostname",
						Options: runtime.RawExtension{
							Raw: []byte(`{"values":["example.com"],"matchOperator":"IS_ONE_OF"}`),
						},
					},
				},
			},
			current: map[string]interface{}{
				"name": "default",
				"criteria": []map[string]interface{}{
					{
						"name": "hostname",
						"options": map[string]interface{}{
							"values":        []string{"different.com"},
							"matchOperator": "IS_ONE_OF",
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "null options vs empty object options",
			desired: &akamaiV1alpha1.PropertyRules{
				Name:                "default",
				CriteriaMustSatisfy: "all",
				Options: runtime.RawExtension{
					Raw: nil, // null options
				},
				Behaviors: []akamaiV1alpha1.RuleBehavior{
					{
						Name: "origin",
						Options: runtime.RawExtension{
							Raw: []byte(`{"hostname":"example.com","originType":"CUSTOMER"}`),
						},
					},
				},
			},
			current: map[string]interface{}{
				"name":    "default",
				"options": map[string]interface{}{}, // empty object
				// Note: criteriaMustSatisfy not present in current
				"behaviors": []map[string]interface{}{
					{
						"name": "origin",
						"options": map[string]interface{}{
							"hostname":   "example.com",
							"originType": "CUSTOMER",
						},
					},
				},
			},
			expected: false, // Should be considered identical
		},
		{
			name: "missing criteriaMustSatisfy vs all",
			desired: &akamaiV1alpha1.PropertyRules{
				Name:                "default",
				CriteriaMustSatisfy: "all",
				Behaviors: []akamaiV1alpha1.RuleBehavior{
					{
						Name: "caching",
						Options: runtime.RawExtension{
							Raw: []byte(`{"behavior":"NO_STORE"}`),
						},
					},
				},
			},
			current: map[string]interface{}{
				"name": "default",
				// criteriaMustSatisfy not present
				"behaviors": []map[string]interface{}{
					{
						"name": "caching",
						"options": map[string]interface{}{
							"behavior": "NO_STORE",
						},
					},
				},
			},
			expected: false, // Should be considered identical (default is "all")
		},
		{
			name: "customOverride null handling",
			desired: &akamaiV1alpha1.PropertyRules{
				Name: "default",
				CustomOverride: runtime.RawExtension{
					Raw: nil,
				},
				Behaviors: []akamaiV1alpha1.RuleBehavior{
					{
						Name: "origin",
						Options: runtime.RawExtension{
							Raw: []byte(`{"hostname":"example.com"}`),
						},
					},
				},
			},
			current: map[string]interface{}{
				"name":           "default",
				"customOverride": nil,
				"behaviors": []map[string]interface{}{
					{
						"name": "origin",
						"options": map[string]interface{}{
							"hostname": "example.com",
						},
					},
				},
			},
			expected: false, // Should be considered identical
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := reconciler.rulesNeedUpdate(tt.desired, tt.current)
			if err != nil {
				t.Errorf("rulesNeedUpdate() error = %v", err)
				return
			}
			if result != tt.expected {
				t.Errorf("rulesNeedUpdate() = %v, expected %v", result, tt.expected)

				// For debugging, show what was actually compared
				if tt.desired != nil {
					currentRules, _ := reconciler.normalizeCurrentRules(tt.current)
					desiredClean := reconciler.copyAndCleanRules(tt.desired)
					currentClean := reconciler.copyAndCleanRules(currentRules)

					desiredJSON, _ := json.MarshalIndent(desiredClean, "", "  ")
					currentJSON, _ := json.MarshalIndent(currentClean, "", "  ")

					t.Logf("Desired (cleaned):\n%s", string(desiredJSON))
					t.Logf("Current (cleaned):\n%s", string(currentJSON))
				}
			}
		})
	}
}

func TestNormalizeCurrentRules(t *testing.T) {
	reconciler := &AkamaiPropertyReconciler{}

	// Test that Akamai API response format gets normalized correctly
	akamaiResponse := map[string]interface{}{
		"name": "default",
		"uuid": "auto-generated-uuid",
		"behaviors": []map[string]interface{}{
			{
				"name": "origin",
				"uuid": "behavior-uuid",
				"options": map[string]interface{}{
					"hostname":     "example.com",
					"httpPort":     80,
					"lastModified": "2023-01-01T00:00:00Z",
					"emptyField":   "",
				},
			},
		},
		"criteria": []map[string]interface{}{
			{
				"name": "hostname",
				"uuid": "criteria-uuid",
				"options": map[string]interface{}{
					"values":        []string{"example.com"},
					"matchOperator": "IS_ONE_OF",
					"templateUuid":  "template-uuid",
				},
			},
		},
	}

	normalized, err := reconciler.normalizeCurrentRules(akamaiResponse)
	if err != nil {
		t.Fatalf("normalizeCurrentRules() error = %v", err)
	}

	// Verify that auto-generated fields are cleaned
	if normalized.UUID != "" {
		t.Errorf("Expected UUID to be cleaned, got %s", normalized.UUID)
	}

	if len(normalized.Behaviors) > 0 && normalized.Behaviors[0].UUID != "" {
		t.Errorf("Expected behavior UUID to be cleaned, got %s", normalized.Behaviors[0].UUID)
	}

	if len(normalized.Criteria) > 0 && normalized.Criteria[0].UUID != "" {
		t.Errorf("Expected criteria UUID to be cleaned, got %s", normalized.Criteria[0].UUID)
	}

	// Verify that meaningful data is preserved
	if normalized.Name != "default" {
		t.Errorf("Expected name to be preserved, got %s", normalized.Name)
	}

	if len(normalized.Behaviors) == 0 || normalized.Behaviors[0].Name != "origin" {
		t.Errorf("Expected behavior to be preserved")
	}

	if len(normalized.Criteria) == 0 || normalized.Criteria[0].Name != "hostname" {
		t.Errorf("Expected criteria to be preserved")
	}
}
