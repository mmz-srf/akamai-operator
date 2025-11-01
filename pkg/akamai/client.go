package akamai

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	akamaiV1alpha1 "github.com/mmz-srf/akamai-operator/api/v1alpha1"
)

// Client represents an Akamai API client
type Client struct {
	BaseURL     string
	HTTPClient  *http.Client
	Credentials *Credentials
}

// Credentials holds the Akamai API credentials
type Credentials struct {
	Host         string
	ClientToken  string
	ClientSecret string
	AccessToken  string
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

// CreatePropertyRequest represents the request to create a property
type CreatePropertyRequest struct {
	PropertyName string `json:"propertyName"`
	ProductID    string `json:"productId"`
	GroupID      string `json:"groupId"`
	ContractID   string `json:"contractId"`
}

// CreatePropertyResponse represents the response from creating a property
type CreatePropertyResponse struct {
	PropertyLink string `json:"propertyLink"`
}

// ActivationRequest represents the request to activate a property
type ActivationRequest struct {
	PropertyVersion        int      `json:"propertyVersion"`
	Network                string   `json:"network"`
	Note                   string   `json:"note,omitempty"`
	NotifyEmails           []string `json:"notifyEmails"`
	AcknowledgeAllWarnings bool     `json:"acknowledgeAllWarnings,omitempty"`
	UseFastFallback        bool     `json:"useFastFallback,omitempty"`
	FastPush               *bool    `json:"fastPush,omitempty"`
	IgnoreHttpErrors       *bool    `json:"ignoreHttpErrors,omitempty"`
}

// ActivationResponse represents the response from activating a property
type ActivationResponse struct {
	ActivationLink string `json:"activationLink"`
}

// Activation represents an activation status
type Activation struct {
	ActivationID    string    `json:"activationId"`
	PropertyID      string    `json:"propertyId"`
	PropertyVersion int       `json:"propertyVersion"`
	Network         string    `json:"network"`
	Status          string    `json:"status"`
	SubmitDate      time.Time `json:"submitDate"`
	UpdateDate      time.Time `json:"updateDate"`
	Note            string    `json:"note"`
	NotifyEmails    []string  `json:"notifyEmails"`
	CanFastFallback bool      `json:"canFastFallback"`
	FallbackVersion int       `json:"fallbackVersion,omitempty"`
}

// GetPropertyResponse represents the response from getting a property
type GetPropertyResponse struct {
	Properties struct {
		Items []Property `json:"items"`
	} `json:"properties"`
}

// NewClient creates a new Akamai API client
func NewClient() (*Client, error) {
	// Get credentials from environment variables
	host := os.Getenv("AKAMAI_HOST")
	clientToken := os.Getenv("AKAMAI_CLIENT_TOKEN")
	clientSecret := os.Getenv("AKAMAI_CLIENT_SECRET")
	accessToken := os.Getenv("AKAMAI_ACCESS_TOKEN")

	if host == "" || clientToken == "" || clientSecret == "" || accessToken == "" {
		return nil, fmt.Errorf("missing Akamai credentials in environment variables")
	}

	// Ensure host has https:// prefix
	if !strings.HasPrefix(host, "https://") {
		host = "https://" + host
	}

	return &Client{
		BaseURL: host,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		Credentials: &Credentials{
			Host:         host,
			ClientToken:  clientToken,
			ClientSecret: clientSecret,
			AccessToken:  accessToken,
		},
	}, nil
}

// CreateProperty creates a new property in Akamai
func (c *Client) CreateProperty(ctx context.Context, spec *akamaiV1alpha1.AkamaiPropertySpec) (string, error) {
	createReq := CreatePropertyRequest{
		PropertyName: spec.PropertyName,
		ProductID:    spec.ProductID,
		GroupID:      spec.GroupID,
		ContractID:   spec.ContractID,
	}

	reqBody, err := json.Marshal(createReq)
	if err != nil {
		return "", fmt.Errorf("failed to marshal create property request: %w", err)
	}

	url := fmt.Sprintf("%s/papi/v1/properties?contractId=%s&groupId=%s",
		c.BaseURL, spec.ContractID, spec.GroupID)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("PAPI-Use-Prefixes", "true")

	// Sign the request
	if err := c.signRequest(req, reqBody); err != nil {
		return "", fmt.Errorf("failed to sign request: %w", err)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make HTTP request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var createResp CreatePropertyResponse
	if err := json.Unmarshal(respBody, &createResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Extract property ID from the property link
	parts := strings.Split(createResp.PropertyLink, "/")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid property link format: %s", createResp.PropertyLink)
	}

	propertyID := parts[len(parts)-1]
	if strings.Contains(propertyID, "?") {
		propertyID = strings.Split(propertyID, "?")[0]
	}

	return propertyID, nil
}

// GetProperty retrieves a property from Akamai
func (c *Client) GetProperty(ctx context.Context, propertyID string) (*Property, error) {
	url := fmt.Sprintf("%s/papi/v1/properties/%s", c.BaseURL, propertyID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("PAPI-Use-Prefixes", "true")

	// Sign the request
	if err := c.signRequest(req, nil); err != nil {
		return nil, fmt.Errorf("failed to sign request: %w", err)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make HTTP request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var getResp GetPropertyResponse
	if err := json.Unmarshal(respBody, &getResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(getResp.Properties.Items) == 0 {
		return nil, fmt.Errorf("property not found")
	}

	return &getResp.Properties.Items[0], nil
}

// UpdateProperty updates an existing property in Akamai
func (c *Client) UpdateProperty(ctx context.Context, propertyID string, spec *akamaiV1alpha1.AkamaiPropertySpec) (int, error) {
	// First, get the current property to get the latest version
	property, err := c.GetProperty(ctx, propertyID)
	if err != nil {
		return 0, fmt.Errorf("failed to get current property: %w", err)
	}

	// Create a new version based on the latest version
	newVersion := property.LatestVersion + 1

	// For this example, we'll just return the new version number
	// In a real implementation, you would update the property rules, hostnames, etc.
	return newVersion, nil
}

// DeleteProperty deletes a property from Akamai
func (c *Client) DeleteProperty(ctx context.Context, propertyID string) error {
	// Note: Akamai doesn't typically allow deleting properties via API
	// This is a placeholder implementation
	// In reality, you might want to deactivate the property instead

	url := fmt.Sprintf("%s/papi/v1/properties/%s", c.BaseURL, propertyID)

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("PAPI-Use-Prefixes", "true")

	// Sign the request
	if err := c.signRequest(req, nil); err != nil {
		return fmt.Errorf("failed to sign request: %w", err)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// ActivateProperty activates a property version on the specified network
func (c *Client) ActivateProperty(ctx context.Context, propertyID string, version int, activationSpec *akamaiV1alpha1.ActivationSpec) (string, error) {
	activationReq := ActivationRequest{
		PropertyVersion:        version,
		Network:                activationSpec.Network,
		Note:                   activationSpec.Note,
		NotifyEmails:           activationSpec.NotifyEmails,
		AcknowledgeAllWarnings: activationSpec.AcknowledgeAllWarnings,
		UseFastFallback:        activationSpec.UseFastFallback,
		FastPush:               activationSpec.FastPush,
		IgnoreHttpErrors:       activationSpec.IgnoreHttpErrors,
	}

	reqBody, err := json.Marshal(activationReq)
	if err != nil {
		return "", fmt.Errorf("failed to marshal activation request: %w", err)
	}

	url := fmt.Sprintf("%s/papi/v1/properties/%s/activations", c.BaseURL, propertyID)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("PAPI-Use-Prefixes", "true")

	// Sign the request
	if err := c.signRequest(req, reqBody); err != nil {
		return "", fmt.Errorf("failed to sign request: %w", err)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make HTTP request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var activationResp ActivationResponse
	if err := json.Unmarshal(respBody, &activationResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal activation response: %w", err)
	}

	// Extract activation ID from the activation link
	activationID := extractActivationID(activationResp.ActivationLink)
	return activationID, nil
}

// GetActivation retrieves the status of a property activation
func (c *Client) GetActivation(ctx context.Context, propertyID, activationID string) (*Activation, error) {
	url := fmt.Sprintf("%s/papi/v1/properties/%s/activations/%s", c.BaseURL, propertyID, activationID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("PAPI-Use-Prefixes", "true")

	// Sign the request
	if err := c.signRequest(req, nil); err != nil {
		return nil, fmt.Errorf("failed to sign request: %w", err)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make HTTP request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var activation Activation
	if err := json.Unmarshal(respBody, &activation); err != nil {
		return nil, fmt.Errorf("failed to unmarshal activation response: %w", err)
	}

	return &activation, nil
}

// ListActivations lists all activations for a property
func (c *Client) ListActivations(ctx context.Context, propertyID string) ([]Activation, error) {
	url := fmt.Sprintf("%s/papi/v1/properties/%s/activations", c.BaseURL, propertyID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("PAPI-Use-Prefixes", "true")

	// Sign the request
	if err := c.signRequest(req, nil); err != nil {
		return nil, fmt.Errorf("failed to sign request: %w", err)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make HTTP request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	type ActivationsResponse struct {
		Activations struct {
			Items []Activation `json:"items"`
		} `json:"activations"`
	}

	var activationsResp ActivationsResponse
	if err := json.Unmarshal(respBody, &activationsResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal activations response: %w", err)
	}

	return activationsResp.Activations.Items, nil
}

// extractActivationID extracts the activation ID from the activation link
func extractActivationID(activationLink string) string {
	// Parse the activation link to extract the activation ID
	// Example: /papi/v0/properties/prp_173136/activations/atv_67037?contractId=ctr_K-0N7RAK7&groupId=grp_15225
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

// signRequest signs an HTTP request using Akamai EdgeGrid authentication
func (c *Client) signRequest(req *http.Request, body []byte) error {
	timestamp := time.Now().UTC().Format("20060102T15:04:05+0000")
	nonce := generateNonce()

	// Create the auth header
	authHeader := fmt.Sprintf("EG1-HMAC-SHA256 client_token=%s;access_token=%s;timestamp=%s;nonce=%s;",
		c.Credentials.ClientToken, c.Credentials.AccessToken, timestamp, nonce)

	// Create the string to sign
	parsedURL, err := url.Parse(req.URL.String())
	if err != nil {
		return fmt.Errorf("failed to parse URL: %w", err)
	}

	path := parsedURL.Path
	if parsedURL.RawQuery != "" {
		path += "?" + parsedURL.RawQuery
	}

	// Create content hash if body exists
	contentHash := ""
	if body != nil && len(body) > 0 {
		hasher := sha256.New()
		hasher.Write(body)
		contentHash = base64.StdEncoding.EncodeToString(hasher.Sum(nil))
	}

	stringToSign := fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s",
		req.Method, parsedURL.Scheme, parsedURL.Host, path, authHeader, contentHash)

	// Create the signing key
	signingKey := hmac.New(sha256.New, []byte(c.Credentials.ClientSecret))
	signingKey.Write([]byte(timestamp))
	signingKeyBytes := signingKey.Sum(nil)

	// Create the signature
	signature := hmac.New(sha256.New, signingKeyBytes)
	signature.Write([]byte(stringToSign))
	signatureB64 := base64.StdEncoding.EncodeToString(signature.Sum(nil))

	// Set the authorization header
	authHeader += "signature=" + signatureB64
	req.Header.Set("Authorization", authHeader)

	return nil
}

// generateNonce generates a random nonce for the request
func generateNonce() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// normalizeHeaders normalizes HTTP headers for signing
func normalizeHeaders(headers http.Header, headersToSign []string) string {
	var normalized []string
	headerMap := make(map[string]string)

	// Convert headers to lowercase map
	for key, values := range headers {
		headerMap[strings.ToLower(key)] = strings.Join(values, ",")
	}

	// Sort headers to sign
	sort.Strings(headersToSign)

	// Build normalized header string
	for _, header := range headersToSign {
		if value, exists := headerMap[strings.ToLower(header)]; exists {
			normalized = append(normalized, fmt.Sprintf("%s:%s", strings.ToLower(header), strings.TrimSpace(value)))
		}
	}

	return strings.Join(normalized, "\t")
}
