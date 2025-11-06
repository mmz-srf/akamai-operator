package akamai

import (
	"context"
	"fmt"

	"github.com/akamai/AkamaiOPEN-edgegrid-golang/v8/pkg/papi"
	akamaiV1alpha1 "github.com/mmz-srf/akamai-operator/api/v1alpha1"
)

// GetPropertyHostnames retrieves hostnames for a specific property version
func (c *Client) GetPropertyHostnames(ctx context.Context, propertyID, contractID, groupID string, version int) ([]Hostname, error) {
	getHostnamesReq := papi.GetPropertyVersionHostnamesRequest{
		PropertyID:      propertyID,
		PropertyVersion: version,
		ContractID:      contractID,
		GroupID:         groupID,
	}

	resp, err := c.papiClient.GetPropertyVersionHostnames(ctx, getHostnamesReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get property hostnames: %w", err)
	}

	if resp == nil || resp.Hostnames.Items == nil {
		return []Hostname{}, nil
	}

	// Convert PAPI hostnames to our Hostname structure
	hostnames := make([]Hostname, 0, len(resp.Hostnames.Items))
	for _, h := range resp.Hostnames.Items {
		hostname := Hostname{
			CNAMEFrom:            h.CnameFrom,
			CNAMETo:              h.CnameTo,
			CertProvisioningType: h.CertProvisioningType,
		}
		hostnames = append(hostnames, hostname)
	}

	return hostnames, nil
}

// UpdatePropertyHostnames updates the hostnames for a property version
// This uses PATCH to add/update hostnames without affecting existing ones
func (c *Client) UpdatePropertyHostnames(ctx context.Context, propertyID, contractID, groupID string, version int, hostnames []akamaiV1alpha1.Hostname) error {
	if len(hostnames) == 0 {
		// Nothing to update
		return nil
	}

	// Convert spec hostnames to PAPI format
	papiHostnames := make([]papi.Hostname, 0, len(hostnames))
	for _, h := range hostnames {
		papiHostname := papi.Hostname{
			CnameType:            papi.HostnameCnameTypeEdgeHostname,
			CnameFrom:            h.CNAMEFrom,
			CnameTo:              h.CNAMETo,
			CertProvisioningType: h.CertProvisioningType,
		}
		papiHostnames = append(papiHostnames, papiHostname)
	}

	// Use UpdatePropertyVersionHostnames (which uses PATCH internally)
	updateReq := papi.UpdatePropertyVersionHostnamesRequest{
		PropertyID:      propertyID,
		PropertyVersion: version,
		ContractID:      contractID,
		GroupID:         groupID,
		Hostnames:       papiHostnames,
	}

	_, err := c.papiClient.UpdatePropertyVersionHostnames(ctx, updateReq)
	if err != nil {
		return fmt.Errorf("failed to update property hostnames: %w", err)
	}

	return nil
}

// SetPropertyHostnames replaces all hostnames for a property version
// This is different from UpdatePropertyHostnames which patches existing hostnames
func (c *Client) SetPropertyHostnames(ctx context.Context, propertyID, contractID, groupID string, version int, hostnames []akamaiV1alpha1.Hostname) error {
	// Convert spec hostnames to PAPI format
	papiHostnames := make([]papi.Hostname, 0, len(hostnames))
	for _, h := range hostnames {
		papiHostname := papi.Hostname{
			CnameType:            papi.HostnameCnameTypeEdgeHostname,
			CnameFrom:            h.CNAMEFrom,
			CnameTo:              h.CNAMETo,
			CertProvisioningType: h.CertProvisioningType,
		}
		papiHostnames = append(papiHostnames, papiHostname)
	}

	// Use UpdatePropertyVersionHostnames to set hostnames
	updateReq := papi.UpdatePropertyVersionHostnamesRequest{
		PropertyID:      propertyID,
		PropertyVersion: version,
		ContractID:      contractID,
		GroupID:         groupID,
		Hostnames:       papiHostnames,
	}

	_, err := c.papiClient.UpdatePropertyVersionHostnames(ctx, updateReq)
	if err != nil {
		return fmt.Errorf("failed to set property hostnames: %w", err)
	}

	return nil
}

// CompareHostnames compares two sets of hostnames and returns true if they differ
func CompareHostnames(desired []akamaiV1alpha1.Hostname, current []Hostname) bool {
	if len(desired) != len(current) {
		return true
	}

	// Create a map for easier comparison
	currentMap := make(map[string]Hostname)
	for _, h := range current {
		currentMap[h.CNAMEFrom] = h
	}

	// Check if all desired hostnames exist with the same configuration
	for _, dh := range desired {
		ch, exists := currentMap[dh.CNAMEFrom]
		if !exists {
			return true
		}
		if dh.CNAMETo != ch.CNAMETo {
			return true
		}
		if dh.CertProvisioningType != "" && dh.CertProvisioningType != ch.CertProvisioningType {
			return true
		}
	}

	return false
}
