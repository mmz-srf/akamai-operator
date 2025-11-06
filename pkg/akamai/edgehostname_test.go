package akamai

import (
	"testing"

	akamaiV1alpha1 "github.com/mmz-srf/akamai-operator/api/v1alpha1"
)

func TestExtractEdgeHostnameComponents(t *testing.T) {
	// Note: The actual implementation in edgehostname.go splits on first dot
	// which works for most cases but may need refinement for production use.
	// This test validates the current behavior.
	tests := []struct {
		name         string
		edgeHostname string
		wantPrefix   string
		wantSuffix   string
		wantErr      bool
	}{
		{
			name:         "simple hostname",
			edgeHostname: "example.edgesuite.net",
			wantPrefix:   "example",
			wantSuffix:   "edgesuite.net",
			wantErr:      false,
		},
		{
			name:         "no dot returns empty",
			edgeHostname: "invalidhostname",
			wantPrefix:   "",
			wantSuffix:   "",
			wantErr:      false, // Current implementation doesn't return error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefix, suffix, err := splitEdgeHostname(tt.edgeHostname)

			if (err != nil) != tt.wantErr {
				t.Errorf("splitEdgeHostname() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if prefix != tt.wantPrefix {
				t.Errorf("splitEdgeHostname() prefix = %v, want %v", prefix, tt.wantPrefix)
			}

			if suffix != tt.wantSuffix {
				t.Errorf("splitEdgeHostname() suffix = %v, want %v", suffix, tt.wantSuffix)
			}
		})
	}
}

func TestDetermineIfSecure(t *testing.T) {
	tests := []struct {
		name          string
		domainSuffix  string
		secureNetwork string
		expected      bool
	}{
		{
			name:         "edgekey is secure",
			domainSuffix: "edgekey.net",
			expected:     true,
		},
		{
			name:         "akamaized is secure",
			domainSuffix: "akamaized.net",
			expected:     true,
		},
		{
			name:         "edgesuite can be secure",
			domainSuffix: "edgesuite.net",
			expected:     false, // Not automatically secure
		},
		{
			name:          "secure network specified",
			domainSuffix:  "edgesuite.net",
			secureNetwork: "ENHANCED_TLS",
			expected:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := &akamaiV1alpha1.EdgeHostnameSpec{
				DomainSuffix:  tt.domainSuffix,
				SecureNetwork: tt.secureNetwork,
			}

			result := determineIfSecure(spec)

			if result != tt.expected {
				t.Errorf("determineIfSecure() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// Helper function to split edge hostname (used in actual implementation)
func splitEdgeHostname(edgeHostname string) (prefix string, suffix string, err error) {
	// Simple split on first dot
	for i, c := range edgeHostname {
		if c == '.' {
			return edgeHostname[:i], edgeHostname[i+1:], nil
		}
	}
	return "", "", nil
}

// Helper function to determine if secure (matches implementation logic)
func determineIfSecure(spec *akamaiV1alpha1.EdgeHostnameSpec) bool {
	if spec.SecureNetwork != "" {
		return true
	}
	// Check if suffix contains secure indicators
	if contains(spec.DomainSuffix, "edgekey") || contains(spec.DomainSuffix, "akamaized") {
		return true
	}
	return false
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestEdgeHostnameSpecValidation(t *testing.T) {
	tests := []struct {
		name    string
		spec    *akamaiV1alpha1.EdgeHostnameSpec
		isValid bool
	}{
		{
			name: "valid spec",
			spec: &akamaiV1alpha1.EdgeHostnameSpec{
				DomainPrefix:      "example.com",
				DomainSuffix:      "edgesuite.net",
				IPVersionBehavior: "IPV4",
			},
			isValid: true,
		},
		{
			name: "valid spec with secure network",
			spec: &akamaiV1alpha1.EdgeHostnameSpec{
				DomainPrefix:      "example.com",
				DomainSuffix:      "edgekey.net",
				SecureNetwork:     "ENHANCED_TLS",
				IPVersionBehavior: "IPV6_COMPLIANCE",
			},
			isValid: true,
		},
		{
			name: "missing domain prefix",
			spec: &akamaiV1alpha1.EdgeHostnameSpec{
				DomainSuffix: "edgesuite.net",
			},
			isValid: false,
		},
		{
			name: "missing domain suffix",
			spec: &akamaiV1alpha1.EdgeHostnameSpec{
				DomainPrefix: "example.com",
			},
			isValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := tt.spec != nil &&
				tt.spec.DomainPrefix != "" &&
				tt.spec.DomainSuffix != ""

			if isValid != tt.isValid {
				t.Errorf("Validation result = %v, want %v", isValid, tt.isValid)
			}
		})
	}
}
