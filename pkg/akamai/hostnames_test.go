package akamai

import (
	"testing"

	akamaiV1alpha1 "github.com/mmz-srf/akamai-operator/api/v1alpha1"
)

func TestCompareHostnames(t *testing.T) {
	tests := []struct {
		name     string
		desired  []akamaiV1alpha1.Hostname
		current  []Hostname
		expected bool // true if they differ
	}{
		{
			name: "identical hostnames",
			desired: []akamaiV1alpha1.Hostname{
				{
					CNAMEFrom:            "www.example.com",
					CNAMETo:              "example.com.edgesuite.net",
					CertProvisioningType: "CPS_MANAGED",
				},
			},
			current: []Hostname{
				{
					CNAMEFrom:            "www.example.com",
					CNAMETo:              "example.com.edgesuite.net",
					CertProvisioningType: "CPS_MANAGED",
				},
			},
			expected: false,
		},
		{
			name: "different count",
			desired: []akamaiV1alpha1.Hostname{
				{
					CNAMEFrom: "www.example.com",
					CNAMETo:   "example.com.edgesuite.net",
				},
				{
					CNAMEFrom: "api.example.com",
					CNAMETo:   "example.com.edgesuite.net",
				},
			},
			current: []Hostname{
				{
					CNAMEFrom: "www.example.com",
					CNAMETo:   "example.com.edgesuite.net",
				},
			},
			expected: true,
		},
		{
			name: "different cnameTo",
			desired: []akamaiV1alpha1.Hostname{
				{
					CNAMEFrom: "www.example.com",
					CNAMETo:   "example.com.edgekey.net",
				},
			},
			current: []Hostname{
				{
					CNAMEFrom: "www.example.com",
					CNAMETo:   "example.com.edgesuite.net",
				},
			},
			expected: true,
		},
		{
			name: "different cnameFrom",
			desired: []akamaiV1alpha1.Hostname{
				{
					CNAMEFrom: "api.example.com",
					CNAMETo:   "example.com.edgesuite.net",
				},
			},
			current: []Hostname{
				{
					CNAMEFrom: "www.example.com",
					CNAMETo:   "example.com.edgesuite.net",
				},
			},
			expected: true,
		},
		{
			name: "different certProvisioningType",
			desired: []akamaiV1alpha1.Hostname{
				{
					CNAMEFrom:            "www.example.com",
					CNAMETo:              "example.com.edgesuite.net",
					CertProvisioningType: "DEFAULT",
				},
			},
			current: []Hostname{
				{
					CNAMEFrom:            "www.example.com",
					CNAMETo:              "example.com.edgesuite.net",
					CertProvisioningType: "CPS_MANAGED",
				},
			},
			expected: true,
		},
		{
			name: "empty desired certProvisioningType matches any",
			desired: []akamaiV1alpha1.Hostname{
				{
					CNAMEFrom: "www.example.com",
					CNAMETo:   "example.com.edgesuite.net",
				},
			},
			current: []Hostname{
				{
					CNAMEFrom:            "www.example.com",
					CNAMETo:              "example.com.edgesuite.net",
					CertProvisioningType: "CPS_MANAGED",
				},
			},
			expected: false,
		},
		{
			name: "multiple hostnames in different order",
			desired: []akamaiV1alpha1.Hostname{
				{
					CNAMEFrom: "api.example.com",
					CNAMETo:   "example.com.edgesuite.net",
				},
				{
					CNAMEFrom: "www.example.com",
					CNAMETo:   "example.com.edgesuite.net",
				},
			},
			current: []Hostname{
				{
					CNAMEFrom: "www.example.com",
					CNAMETo:   "example.com.edgesuite.net",
				},
				{
					CNAMEFrom: "api.example.com",
					CNAMETo:   "example.com.edgesuite.net",
				},
			},
			expected: false,
		},
		{
			name:     "both empty",
			desired:  []akamaiV1alpha1.Hostname{},
			current:  []Hostname{},
			expected: false,
		},
		{
			name:    "desired empty current has hostnames",
			desired: []akamaiV1alpha1.Hostname{},
			current: []Hostname{
				{
					CNAMEFrom: "www.example.com",
					CNAMETo:   "example.com.edgesuite.net",
				},
			},
			expected: true,
		},
		{
			name: "current empty desired has hostnames",
			desired: []akamaiV1alpha1.Hostname{
				{
					CNAMEFrom: "www.example.com",
					CNAMETo:   "example.com.edgesuite.net",
				},
			},
			current:  []Hostname{},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CompareHostnames(tt.desired, tt.current)
			if result != tt.expected {
				t.Errorf("CompareHostnames() = %v, want %v", result, tt.expected)
				t.Logf("Desired: %+v", tt.desired)
				t.Logf("Current: %+v", tt.current)
			}
		})
	}
}

func TestCompareHostnamesWithMultipleHostnames(t *testing.T) {
	desired := []akamaiV1alpha1.Hostname{
		{
			CNAMEFrom:            "www.example.com",
			CNAMETo:              "example.com.edgesuite.net",
			CertProvisioningType: "CPS_MANAGED",
		},
		{
			CNAMEFrom:            "api.example.com",
			CNAMETo:              "example.com.edgekey.net",
			CertProvisioningType: "CPS_MANAGED",
		},
		{
			CNAMEFrom:            "static.example.com",
			CNAMETo:              "example.com.akamaized.net",
			CertProvisioningType: "CPS_MANAGED",
		},
	}

	current := []Hostname{
		{
			CNAMEFrom:            "www.example.com",
			CNAMETo:              "example.com.edgesuite.net",
			CertProvisioningType: "CPS_MANAGED",
		},
		{
			CNAMEFrom:            "api.example.com",
			CNAMETo:              "example.com.edgekey.net",
			CertProvisioningType: "CPS_MANAGED",
		},
		{
			CNAMEFrom:            "static.example.com",
			CNAMETo:              "example.com.akamaized.net",
			CertProvisioningType: "CPS_MANAGED",
		},
	}

	// Should be identical
	if CompareHostnames(desired, current) {
		t.Error("Expected hostnames to be identical, but CompareHostnames returned true (different)")
	}

	// Modify one hostname
	current[1].CNAMETo = "example.com.edgesuite.net"
	if !CompareHostnames(desired, current) {
		t.Error("Expected hostnames to be different, but CompareHostnames returned false (same)")
	}
}
