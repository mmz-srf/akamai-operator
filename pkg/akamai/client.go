package akamai

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/akamai/AkamaiOPEN-edgegrid-golang/v8/pkg/edgegrid"
	"github.com/akamai/AkamaiOPEN-edgegrid-golang/v8/pkg/papi"
	"github.com/akamai/AkamaiOPEN-edgegrid-golang/v8/pkg/session"
	akamaiV1alpha1 "github.com/mmz-srf/akamai-operator/api/v1alpha1"
)

// Client represents an Akamai API client using the official EdgeGrid client
type Client struct {
	papiClient papi.PAPI
}

// Property represents an Akamai property
type Property struct {
	PropertyID        string     `json:"propertyId"`
	PropertyName      string     `json:"propertyName"`
	AccountID         string     `json:"accountId"`
	ContractID        string     `json:"contractId"`
	GroupID           string     `json:"groupId"`
	ProductID         string     `json:"productId"`
	LatestVersion     int        `json:"latestVersion"`
	StagingVersion    int        `json:"stagingVersion"`
	ProductionVersion int        `json:"productionVersion"`
	Hostnames         []Hostname `json:"hostnames"`
}

// Hostname represents a hostname configuration
type Hostname struct {
	CNAMEFrom            string `json:"cnameFrom"`
	CNAMETo              string `json:"cnameTo"`
	CertProvisioningType string `json:"certProvisioningType"`
}

// Activation represents an activation status
type Activation struct {
	ActivationID    string   `json:"activationId"`
	PropertyID      string   `json:"propertyId"`
	PropertyVersion int      `json:"propertyVersion"`
	Network         string   `json:"network"`
	Status          string   `json:"status"`
	SubmitDate      string   `json:"submitDate"`
	UpdateDate      string   `json:"updateDate"`
	Note            string   `json:"note"`
	NotifyEmails    []string `json:"notifyEmails"`
	CanFastFallback bool     `json:"canFastFallback"`
	FallbackVersion int      `json:"fallbackVersion,omitempty"`
}

// NewClient creates a new Akamai API client using the official EdgeGrid client
func NewClient() (*Client, error) {
	// Get credentials from environment variables
	host := os.Getenv("AKAMAI_HOST")
	clientToken := os.Getenv("AKAMAI_CLIENT_TOKEN")
	clientSecret := os.Getenv("AKAMAI_CLIENT_SECRET")
	accessToken := os.Getenv("AKAMAI_ACCESS_TOKEN")

	if host == "" || clientToken == "" || clientSecret == "" || accessToken == "" {
		return nil, fmt.Errorf("missing Akamai credentials in environment variables")
	}

	// Validate credential formats
	if len(clientToken) < 20 || len(clientSecret) < 20 || len(accessToken) < 20 {
		return nil, fmt.Errorf("invalid Akamai credentials: tokens appear to be too short")
	}

	// Ensure host format is correct (remove https:// prefix if present, as EdgeGrid client expects just the hostname)
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimPrefix(host, "http://")
	host = strings.TrimSuffix(host, "/")

	// Validate host format
	if !strings.Contains(host, "akamaiapis.net") {
		return nil, fmt.Errorf("invalid Akamai host: must contain 'akamaiapis.net'")
	}

	// Create EdgeGrid configuration
	config := edgegrid.Config{
		Host:         host,
		ClientToken:  clientToken,
		ClientSecret: clientSecret,
		AccessToken:  accessToken,
		MaxBody:      131072, // 128KB
	}

	// Create session with EdgeGrid signer
	sess, err := session.New(
		session.WithSigner(&config),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Create PAPI client
	papiClient := papi.Client(sess)

	return &Client{
		papiClient: papiClient,
	}, nil
}

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

	// Initialize empty hostnames slice for now
	// In a real implementation, you'd get hostnames from the property version
	property.Hostnames = []Hostname{}

	return property, nil
}

// UpdateProperty updates an existing property in Akamai
func (c *Client) UpdateProperty(ctx context.Context, propertyID string, spec *akamaiV1alpha1.AkamaiPropertySpec) (int, error) {
	// First, get the current property to get the latest version
	property, err := c.GetProperty(ctx, propertyID)
	if err != nil {
		return 0, fmt.Errorf("failed to get current property: %w", err)
	}

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
		return 0, fmt.Errorf("failed to create new property version: %w", err)
	}

	if newVersionResp == nil || newVersionResp.VersionLink == "" {
		return 0, fmt.Errorf("invalid response from create property version API")
	}

	newVersion := newVersionResp.VersionLink
	versionNumber, err := extractVersionFromLink(newVersion)
	if err != nil {
		return 0, fmt.Errorf("failed to extract version number: %w", err)
	}

	// TODO: Update property rules, hostnames, etc. based on spec
	// For now, just return the new version number

	return versionNumber, nil
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

// ActivateProperty activates a property version on the specified network
func (c *Client) ActivateProperty(ctx context.Context, propertyID string, version int, activationSpec *akamaiV1alpha1.ActivationSpec, contractID, groupID string) (string, error) {
	// Create activation request
	activationReq := papi.CreateActivationRequest{
		PropertyID: propertyID,
		ContractID: contractID,
		GroupID:    groupID,
		Activation: papi.Activation{
			PropertyVersion:        version,
			Network:                papi.ActivationNetwork(activationSpec.Network),
			Note:                   activationSpec.Note,
			NotifyEmails:           activationSpec.NotifyEmails,
			AcknowledgeAllWarnings: activationSpec.AcknowledgeAllWarnings,
			UseFastFallback:        activationSpec.UseFastFallback,
		},
	}

	// Set optional fields
	if activationSpec.FastPush != nil {
		activationReq.Activation.FastPush = *activationSpec.FastPush
	}
	if activationSpec.IgnoreHttpErrors != nil {
		activationReq.Activation.IgnoreHTTPErrors = *activationSpec.IgnoreHttpErrors
	}

	// Create the activation
	activationResp, err := c.papiClient.CreateActivation(ctx, activationReq)
	if err != nil {
		return "", fmt.Errorf("failed to create activation: %w", err)
	}

	if activationResp == nil || activationResp.ActivationLink == "" {
		return "", fmt.Errorf("invalid response from create activation API")
	}

	// Extract activation ID from the activation link
	activationID := extractActivationIDFromLink(activationResp.ActivationLink)
	return activationID, nil
}

// GetActivation retrieves the status of a property activation
func (c *Client) GetActivation(ctx context.Context, propertyID, activationID string) (*Activation, error) {
	// Get activation details
	getResp, err := c.papiClient.GetActivation(ctx, papi.GetActivationRequest{
		PropertyID:   propertyID,
		ActivationID: activationID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get activation: %w", err)
	}

	if getResp == nil || len(getResp.Activations.Items) == 0 {
		return nil, fmt.Errorf("activation not found")
	}

	papiActivation := getResp.Activations.Items[0]

	// Convert PAPI activation to our Activation structure
	activation := &Activation{
		ActivationID:    papiActivation.ActivationID,
		PropertyID:      papiActivation.PropertyID,
		PropertyVersion: papiActivation.PropertyVersion,
		Network:         string(papiActivation.Network),
		Status:          string(papiActivation.Status),
		SubmitDate:      papiActivation.SubmitDate,
		UpdateDate:      papiActivation.UpdateDate,
		Note:            papiActivation.Note,
		NotifyEmails:    papiActivation.NotifyEmails,
		CanFastFallback: false, // Default value since field doesn't exist in papi.Activation
		FallbackVersion: 0,     // Default value since field doesn't exist in papi.Activation
	}

	return activation, nil
}

// ListActivations lists all activations for a property
func (c *Client) ListActivations(ctx context.Context, propertyID string) ([]Activation, error) {
	// Get activations list
	listResp, err := c.papiClient.GetActivations(ctx, papi.GetActivationsRequest{
		PropertyID: propertyID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list activations: %w", err)
	}

	if listResp == nil {
		return []Activation{}, nil
	}

	// Convert PAPI activations to our Activation structures
	activations := make([]Activation, len(listResp.Activations.Items))
	for i, papiActivation := range listResp.Activations.Items {
		activations[i] = Activation{
			ActivationID:    papiActivation.ActivationID,
			PropertyID:      papiActivation.PropertyID,
			PropertyVersion: papiActivation.PropertyVersion,
			Network:         string(papiActivation.Network),
			Status:          string(papiActivation.Status),
			SubmitDate:      papiActivation.SubmitDate,
			UpdateDate:      papiActivation.UpdateDate,
			Note:            papiActivation.Note,
			NotifyEmails:    papiActivation.NotifyEmails,
			CanFastFallback: false, // Default value since field doesn't exist in papi.Activation
			FallbackVersion: 0,     // Default value since field doesn't exist in papi.Activation
		}
	}

	return activations, nil
}

// Helper functions

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
