package akamai

import (
	"context"
	"fmt"
	"strings"

	"github.com/akamai/AkamaiOPEN-edgegrid-golang/v8/pkg/papi"
	akamaiV1alpha1 "github.com/mmz-srf/akamai-operator/api/v1alpha1"
)

// CreateEdgeHostname creates a new edge hostname in Akamai
func (c *Client) CreateEdgeHostname(ctx context.Context, spec *akamaiV1alpha1.EdgeHostnameSpec, productID, contractID, groupID string) (string, error) {
	if spec == nil {
		return "", fmt.Errorf("edge hostname spec is nil")
	}

	// Determine if this is a secure edge hostname
	secure := strings.Contains(spec.DomainSuffix, "edgekey") ||
		strings.Contains(spec.DomainSuffix, "akamaized") ||
		spec.SecureNetwork != ""

	// Set default IP version behavior if not specified
	ipVersionBehavior := spec.IPVersionBehavior
	if ipVersionBehavior == "" {
		ipVersionBehavior = "IPV4"
	}

	// Create edge hostname request
	edgeHostnameCreate := papi.EdgeHostnameCreate{
		ProductID:         productID,
		DomainPrefix:      spec.DomainPrefix,
		DomainSuffix:      spec.DomainSuffix,
		Secure:            secure,
		SecureNetwork:     spec.SecureNetwork,
		IPVersionBehavior: ipVersionBehavior,
	}

	createReq := papi.CreateEdgeHostnameRequest{
		ContractID:   contractID,
		GroupID:      groupID,
		EdgeHostname: edgeHostnameCreate,
	}

	// Create the edge hostname
	resp, err := c.papiClient.CreateEdgeHostname(ctx, createReq)
	if err != nil {
		return "", fmt.Errorf("failed to create edge hostname: %w", err)
	}

	if resp == nil || resp.EdgeHostnameID == "" {
		return "", fmt.Errorf("invalid response from create edge hostname API")
	}

	return resp.EdgeHostnameID, nil
}

// GetEdgeHostname retrieves an edge hostname by ID
func (c *Client) GetEdgeHostname(ctx context.Context, edgeHostnameID, contractID, groupID string) (*papi.EdgeHostnameGetItem, error) {
	getReq := papi.GetEdgeHostnameRequest{
		EdgeHostnameID: edgeHostnameID,
		ContractID:     contractID,
		GroupID:        groupID,
	}

	resp, err := c.papiClient.GetEdgeHostname(ctx, getReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get edge hostname: %w", err)
	}

	if resp == nil || len(resp.EdgeHostnames.Items) == 0 {
		return nil, fmt.Errorf("edge hostname not found")
	}

	return &resp.EdgeHostnames.Items[0], nil
}

// ListEdgeHostnames retrieves all edge hostnames for a contract and group
func (c *Client) ListEdgeHostnames(ctx context.Context, contractID, groupID string) ([]papi.EdgeHostnameGetItem, error) {
	listReq := papi.GetEdgeHostnamesRequest{
		ContractID: contractID,
		GroupID:    groupID,
	}

	resp, err := c.papiClient.GetEdgeHostnames(ctx, listReq)
	if err != nil {
		return nil, fmt.Errorf("failed to list edge hostnames: %w", err)
	}

	if resp == nil || resp.EdgeHostnames.Items == nil {
		return []papi.EdgeHostnameGetItem{}, nil
	}

	return resp.EdgeHostnames.Items, nil
}

// FindEdgeHostnameByName searches for an edge hostname by its full name
func (c *Client) FindEdgeHostnameByName(ctx context.Context, edgeHostnameName, contractID, groupID string) (*papi.EdgeHostnameGetItem, error) {
	edgeHostnames, err := c.ListEdgeHostnames(ctx, contractID, groupID)
	if err != nil {
		return nil, err
	}

	for _, eh := range edgeHostnames {
		if eh.Domain == edgeHostnameName {
			return &eh, nil
		}
	}

	return nil, fmt.Errorf("edge hostname %s not found", edgeHostnameName)
}

// GetOrCreateEdgeHostname retrieves an existing edge hostname or creates it if it doesn't exist
func (c *Client) GetOrCreateEdgeHostname(ctx context.Context, spec *akamaiV1alpha1.EdgeHostnameSpec, productID, contractID, groupID string) (string, error) {
	if spec == nil {
		return "", fmt.Errorf("edge hostname spec is nil")
	}

	// Construct the full edge hostname domain
	edgeHostnameDomain := spec.DomainPrefix + "." + spec.DomainSuffix

	// Try to find existing edge hostname
	existingEdgeHostname, err := c.FindEdgeHostnameByName(ctx, edgeHostnameDomain, contractID, groupID)
	if err == nil && existingEdgeHostname != nil {
		// Edge hostname already exists
		return existingEdgeHostname.ID, nil
	}

	// Edge hostname doesn't exist, create it
	edgeHostnameID, err := c.CreateEdgeHostname(ctx, spec, productID, contractID, groupID)
	if err != nil {
		return "", fmt.Errorf("failed to create edge hostname %s: %w", edgeHostnameDomain, err)
	}

	return edgeHostnameID, nil
}

// EnsureEdgeHostnamesExist ensures all edge hostnames referenced in the hostname configuration exist
func (c *Client) EnsureEdgeHostnamesExist(ctx context.Context, hostnames []akamaiV1alpha1.Hostname, edgeHostnameSpec *akamaiV1alpha1.EdgeHostnameSpec, productID, contractID, groupID string) error {
	if len(hostnames) == 0 {
		return nil
	}

	// Get all existing edge hostnames
	existingEdgeHostnames, err := c.ListEdgeHostnames(ctx, contractID, groupID)
	if err != nil {
		return fmt.Errorf("failed to list edge hostnames: %w", err)
	}

	// Create a map of existing edge hostname domains
	existingMap := make(map[string]bool)
	for _, eh := range existingEdgeHostnames {
		existingMap[eh.Domain] = true
	}

	// Check each hostname's CNAMETo target
	uniqueEdgeHostnames := make(map[string]bool)
	for _, h := range hostnames {
		uniqueEdgeHostnames[h.CNAMETo] = true
	}

	// For each unique edge hostname, check if it exists
	for edgeHostname := range uniqueEdgeHostnames {
		if !existingMap[edgeHostname] {
			// Edge hostname doesn't exist
			// If we have an edgeHostnameSpec, use it to create the edge hostname
			if edgeHostnameSpec != nil {
				// Extract prefix and suffix from the edge hostname
				// For example: "example.com.edgesuite.net" -> prefix: "example.com", suffix: "edgesuite.net"
				parts := strings.SplitN(edgeHostname, ".", 2)
				if len(parts) != 2 {
					return fmt.Errorf("invalid edge hostname format: %s", edgeHostname)
				}

				spec := &akamaiV1alpha1.EdgeHostnameSpec{
					DomainPrefix:      parts[0],
					DomainSuffix:      parts[1],
					SecureNetwork:     edgeHostnameSpec.SecureNetwork,
					IPVersionBehavior: edgeHostnameSpec.IPVersionBehavior,
				}

				_, err := c.CreateEdgeHostname(ctx, spec, productID, contractID, groupID)
				if err != nil {
					return fmt.Errorf("failed to create edge hostname %s: %w", edgeHostname, err)
				}
			} else {
				return fmt.Errorf("edge hostname %s does not exist and no edge hostname spec provided to create it", edgeHostname)
			}
		}
	}

	return nil
}
