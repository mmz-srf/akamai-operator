package controllers

import (
	"encoding/json"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"

	akamaiV1alpha1 "github.com/mmz-srf/akamai-operator/api/v1alpha1"
)

// TestRealWorldComparison tests the exact scenario from the user's YAML
func TestRealWorldComparison(t *testing.T) {
	reconciler := &AkamaiPropertyReconciler{}

	// This is the desired state from the YAML (simplified structure)
	desired := &akamaiV1alpha1.PropertyRules{
		Name:                "default",
		CriteriaMustSatisfy: "all",
		Comments:            "Identify your main traffic segments so you can granularly zoom in your traffic statistics like hits, bandwidth, offloa",
		Criteria:            []akamaiV1alpha1.RuleCriteria{},
		Children:            []runtime.RawExtension{},
		Options: runtime.RawExtension{
			Raw: nil, // null in YAML
		},
		Behaviors: []akamaiV1alpha1.RuleBehavior{
			{
				Name: "origin",
				Options: runtime.RawExtension{
					Raw: []byte(`{"originType":"CUSTOMER","hostname":"origin.my-website.com","forwardHostHeader":"REQUEST_HOST_HEADER"}`),
				},
			},
			{
				Name: "cpCode",
				Options: runtime.RawExtension{
					Raw: []byte(`{"value":{"id":20717,"description":"srf.ch","products":["Fresca","Site_Del"],"createdDate":1237288269000,"name":"srf.chd, response codes, and errors."}}`),
				},
			},
			{
				Name: "caching",
				Options: runtime.RawExtension{
					Raw: []byte(`{"behavior":"NO_STORE"}`),
				},
			},
		},
		CustomOverride: runtime.RawExtension{
			Raw: nil,
		},
	}

	// This is what comes back from Akamai (based on the diff you provided)
	current := map[string]interface{}{
		"name":     "default",
		"comments": "Identify your main traffic segments so you can granularly zoom in your traffic statistics like hits, bandwidth, offloa",
		"options":  map[string]interface{}{}, // empty object instead of null
		// Note: criteriaMustSatisfy is not present (defaults to "all")
		"customOverride": nil,
		"behaviors": []interface{}{
			map[string]interface{}{
				"name": "origin",
				"options": map[string]interface{}{
					"forwardHostHeader": "REQUEST_HOST_HEADER",
					"hostname":          "origin.my-website.com",
					"originType":        "CUSTOMER",
				},
			},
			map[string]interface{}{
				"name": "cpCode",
				"options": map[string]interface{}{
					"value": map[string]interface{}{
						"createdDate": float64(1237288269000),
						"description": "srf.ch",
						"id":          float64(20717),
						"name":        "srf.chd, response codes, and errors.",
						"products":    []interface{}{"Fresca", "Site_Del"},
					},
				},
			},
			map[string]interface{}{
				"name": "caching",
				"options": map[string]interface{}{
					"behavior": "NO_STORE",
				},
			},
		},
	}

	needsUpdate, err := reconciler.rulesNeedUpdate(desired, current)
	if err != nil {
		t.Fatalf("rulesNeedUpdate() error = %v", err)
	}

	if needsUpdate {
		t.Error("rulesNeedUpdate() returned true, but these rules should be considered identical")

		// Debug output
		currentRules, _ := reconciler.normalizeCurrentRules(current)
		desiredClean := reconciler.copyAndCleanRules(desired)
		currentClean := reconciler.copyAndCleanRules(currentRules)

		desiredJSON, _ := json.MarshalIndent(desiredClean, "", "  ")
		currentJSON, _ := json.MarshalIndent(currentClean, "", "  ")

		t.Logf("Desired (cleaned):\n%s", string(desiredJSON))
		t.Logf("Current (cleaned):\n%s", string(currentJSON))
	} else {
		t.Log("✓ Rules are correctly identified as identical")
	}
}

// TestEmptyArraysAndObjects tests various empty vs null scenarios
func TestEmptyArraysAndObjects(t *testing.T) {
	reconciler := &AkamaiPropertyReconciler{}

	tests := []struct {
		name     string
		desired  *akamaiV1alpha1.PropertyRules
		current  interface{}
		expected bool
	}{
		{
			name: "empty criteria array vs no criteria",
			desired: &akamaiV1alpha1.PropertyRules{
				Name:     "default",
				Criteria: []akamaiV1alpha1.RuleCriteria{}, // empty array
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
				"name": "default",
				// criteria field not present
				"behaviors": []interface{}{
					map[string]interface{}{
						"name": "origin",
						"options": map[string]interface{}{
							"hostname": "example.com",
						},
					},
				},
			},
			expected: false, // Should be identical
		},
		{
			name: "empty children array vs no children",
			desired: &akamaiV1alpha1.PropertyRules{
				Name:     "default",
				Children: []runtime.RawExtension{}, // empty array
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
				"name": "default",
				// children field not present
				"behaviors": []interface{}{
					map[string]interface{}{
						"name": "origin",
						"options": map[string]interface{}{
							"hostname": "example.com",
						},
					},
				},
			},
			expected: false, // Should be identical
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

				// Debug output
				currentRules, _ := reconciler.normalizeCurrentRules(tt.current)
				desiredClean := reconciler.copyAndCleanRules(tt.desired)
				currentClean := reconciler.copyAndCleanRules(currentRules)

				desiredJSON, _ := json.MarshalIndent(desiredClean, "", "  ")
				currentJSON, _ := json.MarshalIndent(currentClean, "", "  ")

				t.Logf("Desired (cleaned):\n%s", string(desiredJSON))
				t.Logf("Current (cleaned):\n%s", string(currentJSON))
			}
		})
	}
}
