package akamai

import (
	"context"
	"fmt"

	"github.com/akamai/AkamaiOPEN-edgegrid-golang/v8/pkg/papi"
	akamaiV1alpha1 "github.com/mmz-srf/akamai-operator/api/v1alpha1"
)

// CreateProperty creates a new property in Akamai
func (c *Client) CreateProperty(ctx context.Context, spec *akamaiV1alpha1.AkamaiPropertySpec) (string, error) {
	// Create property request
	createReq := papi.CreatePropertyRequest{
		ContractID: spec.ContractID,
		GroupID:    spec.GroupID,
		Property: papi.PropertyCreate{
			PropertyName: spec.PropertyName,
			ProductID:    spec.ProductID,
			RuleFormat:   "v2023-01-05", // Use a recent rule format
		},
	}

	// Create the property
	createResp, err := c.papiClient.CreateProperty(ctx, createReq)
	if err != nil {
		return "", fmt.Errorf("failed to create property: %w", err)
	}

	if createResp == nil || createResp.PropertyLink == "" {
		return "", fmt.Errorf("invalid response from create property API")
	}

	// Extract property ID from the property link
	propertyID := extractPropertyIDFromLink(createResp.PropertyLink)
	if propertyID == "" {
		return "", fmt.Errorf("failed to extract property ID from link: %s", createResp.PropertyLink)
	}

	return propertyID, nil
}

// GetProperty retrieves a property from Akamai
func (c *Client) GetProperty(ctx context.Context, propertyID string) (*Property, error) {
	// Get property details
	getResp, err := c.papiClient.GetProperty(ctx, papi.GetPropertyRequest{
		PropertyID: propertyID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get property: %w", err)
	}

	if getResp == nil || len(getResp.Properties.Items) == 0 {
		return nil, fmt.Errorf("property not found")
	}

	papiProperty := getResp.Properties.Items[0]

	// Convert PAPI property to our Property structure
	property := &Property{
		PropertyID:    papiProperty.PropertyID,
		PropertyName:  papiProperty.PropertyName,
		AccountID:     papiProperty.AccountID,
		ContractID:    papiProperty.ContractID,
		GroupID:       papiProperty.GroupID,
		ProductID:     papiProperty.ProductID,
		LatestVersion: papiProperty.LatestVersion,
	}

	// Handle optional fields that might be nil
	if papiProperty.StagingVersion != nil {
		property.StagingVersion = *papiProperty.StagingVersion
	}
	if papiProperty.ProductionVersion != nil {
		property.ProductionVersion = *papiProperty.ProductionVersion
	}

	// Get hostnames for the latest version
	if property.LatestVersion > 0 {
		hostnames, err := c.GetPropertyHostnames(ctx, propertyID, papiProperty.ContractID, papiProperty.GroupID, property.LatestVersion)
		if err != nil {
			// Log the error but don't fail the entire operation
			// Hostnames might not be configured yet
			property.Hostnames = []Hostname{}
		} else {
			property.Hostnames = hostnames
		}
	} else {
		property.Hostnames = []Hostname{}
	}

	return property, nil
}

// IsVersionPublished checks if a specific property version is published on staging or production
func (c *Client) IsVersionPublished(ctx context.Context, propertyID string, version int) (bool, string, error) {
	// Get property details to check published versions
	property, err := c.GetProperty(ctx, propertyID)
	if err != nil {
		return false, "", fmt.Errorf("failed to get property: %w", err)
	}

	// Check if the version is published on staging
	if property.StagingVersion == version {
		return true, "STAGING", nil
	}

	// Check if the version is published on production
	if property.ProductionVersion == version {
		return true, "PRODUCTION", nil
	}

	return false, "", nil
}

// GetOrCreateUnpublishedVersion returns the latest version if it's not published,
// or creates a new version if the latest is published
func (c *Client) GetOrCreateUnpublishedVersion(ctx context.Context, propertyID, contractID, groupID string) (int, bool, error) {
	// Get property details
	property, err := c.GetProperty(ctx, propertyID)
	if err != nil {
		return 0, false, fmt.Errorf("failed to get property: %w", err)
	}

	// Check if the latest version is published
	isPublished, _, err := c.IsVersionPublished(ctx, propertyID, property.LatestVersion)
	if err != nil {
		return 0, false, fmt.Errorf("failed to check if version is published: %w", err)
	}

	if !isPublished {
		// Latest version is not published, we can use it
		return property.LatestVersion, false, nil
	}

	// Latest version is published, create a new one
	newVersionReq := papi.CreatePropertyVersionRequest{
		PropertyID: propertyID,
		ContractID: contractID,
		GroupID:    groupID,
		Version: papi.PropertyVersionCreate{
			CreateFromVersion: property.LatestVersion,
		},
	}

	newVersionResp, err := c.papiClient.CreatePropertyVersion(ctx, newVersionReq)
	if err != nil {
		return 0, false, fmt.Errorf("failed to create new property version: %w", err)
	}

	if newVersionResp == nil || newVersionResp.VersionLink == "" {
		return 0, false, fmt.Errorf("invalid response from create property version API")
	}

	newVersion := newVersionResp.VersionLink
	versionNumber, err := extractVersionFromLink(newVersion)
	if err != nil {
		return 0, false, fmt.Errorf("failed to extract version number: %w", err)
	}

	return versionNumber, true, nil
}

// UpdateProperty updates an existing property in Akamai
func (c *Client) UpdateProperty(ctx context.Context, propertyID string, spec *akamaiV1alpha1.AkamaiPropertySpec) (int, error) {
	// First, get the current property to get the latest version
	property, err := c.GetProperty(ctx, propertyID)
	if err != nil {
		return 0, fmt.Errorf("failed to get current property: %w", err)
	}

	// Check if the latest version is published on staging or production
	isPublished, network, err := c.IsVersionPublished(ctx, propertyID, property.LatestVersion)
	if err != nil {
		return 0, fmt.Errorf("failed to check if version is published: %w", err)
	}

	var versionToUpdate int
	if isPublished {
		// The latest version is published, we need to create a new version
		// Create a new version of the property
		newVersionReq := papi.CreatePropertyVersionRequest{
			PropertyID: propertyID,
			ContractID: spec.ContractID,
			GroupID:    spec.GroupID,
			Version: papi.PropertyVersionCreate{
				CreateFromVersion: property.LatestVersion,
			},
		}

		newVersionResp, err := c.papiClient.CreatePropertyVersion(ctx, newVersionReq)
		if err != nil {
			return 0, fmt.Errorf("failed to create new property version (latest version %d is published on %s): %w", property.LatestVersion, network, err)
		}

		if newVersionResp == nil || newVersionResp.VersionLink == "" {
			return 0, fmt.Errorf("invalid response from create property version API")
		}

		newVersion := newVersionResp.VersionLink
		versionNumber, err := extractVersionFromLink(newVersion)
		if err != nil {
			return 0, fmt.Errorf("failed to extract version number: %w", err)
		}

		versionToUpdate = versionNumber
	} else {
		// The latest version is not published, we can update it directly
		versionToUpdate = property.LatestVersion
	}

	// Update hostnames if specified in spec
	if len(spec.Hostnames) > 0 {
		err = c.SetPropertyHostnames(ctx, propertyID, spec.ContractID, spec.GroupID, versionToUpdate, spec.Hostnames)
		if err != nil {
			return 0, fmt.Errorf("failed to update property hostnames: %w", err)
		}
	}

	// TODO: Update property rules if needed
	// Rules are handled separately by the controller

	return versionToUpdate, nil
}

// DeleteProperty deletes a property from Akamai
func (c *Client) DeleteProperty(ctx context.Context, propertyID string) error {
	// Use the RemoveProperty API
	removeReq := papi.RemovePropertyRequest{
		PropertyID: propertyID,
	}

	_, err := c.papiClient.RemoveProperty(ctx, removeReq)
	if err != nil {
		return fmt.Errorf("failed to remove property: %w", err)
	}

	return nil
}
