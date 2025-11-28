package akamai

import (
	"context"
	"fmt"

	"github.com/akamai/AkamaiOPEN-edgegrid-golang/v8/pkg/papi"
	akamaiV1alpha1 "github.com/mmz-srf/akamai-operator/api/v1alpha1"
)

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

// GetPendingActivationForVersion checks if there's a pending/activating activation for a specific version and network
func (c *Client) GetPendingActivationForVersion(ctx context.Context, propertyID string, version int, network string) (*Activation, error) {
	// Get all activations for the property
	activations, err := c.ListActivations(ctx, propertyID)
	if err != nil {
		return nil, fmt.Errorf("failed to list activations: %w", err)
	}

	// Find any pending/activating activation for the specified version and network
	for _, activation := range activations {
		if activation.PropertyVersion == version &&
			activation.Network == network &&
			(activation.Status == "PENDING" || activation.Status == "ACTIVATING") {
			return &activation, nil
		}
	}

	return nil, nil // No pending activation found
}
