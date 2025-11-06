package akamai

import (
	"fmt"
	"strconv"
	"strings"
)

// extractPropertyIDFromLink extracts the property ID from a property link
func extractPropertyIDFromLink(propertyLink string) string {
	// Property link format: /papi/v1/properties/prp_123456?contractId=ctr_xxx&groupId=grp_xxx
	parts := strings.Split(propertyLink, "/")
	for i, part := range parts {
		if part == "properties" && i+1 < len(parts) {
			propertyIDWithQuery := parts[i+1]
			// Remove query parameters
			if idx := strings.Index(propertyIDWithQuery, "?"); idx != -1 {
				return propertyIDWithQuery[:idx]
			}
			return propertyIDWithQuery
		}
	}
	return ""
}

// extractActivationIDFromLink extracts the activation ID from an activation link
func extractActivationIDFromLink(activationLink string) string {
	// Activation link format: /papi/v1/properties/prp_123456/activations/atv_123456?contractId=ctr_xxx&groupId=grp_xxx
	parts := strings.Split(activationLink, "/")
	for i, part := range parts {
		if part == "activations" && i+1 < len(parts) {
			activationIDWithQuery := parts[i+1]
			// Remove query parameters
			if idx := strings.Index(activationIDWithQuery, "?"); idx != -1 {
				return activationIDWithQuery[:idx]
			}
			return activationIDWithQuery
		}
	}
	return ""
}

// extractVersionFromLink extracts the version number from a version link
func extractVersionFromLink(versionLink string) (int, error) {
	// Version link format: /papi/v1/properties/prp_123456/versions/1?contractId=ctr_xxx&groupId=grp_xxx
	parts := strings.Split(versionLink, "/")
	for i, part := range parts {
		if part == "versions" && i+1 < len(parts) {
			versionWithQuery := parts[i+1]
			// Remove query parameters
			if idx := strings.Index(versionWithQuery, "?"); idx != -1 {
				versionWithQuery = versionWithQuery[:idx]
			}
			return strconv.Atoi(versionWithQuery)
		}
	}
	return 0, fmt.Errorf("could not extract version from link: %s", versionLink)
}
