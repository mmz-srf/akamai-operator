package akamai

import (
	"fmt"
	"os"
	"strings"

	"github.com/akamai/AkamaiOPEN-edgegrid-golang/v8/pkg/edgegrid"
	"github.com/akamai/AkamaiOPEN-edgegrid-golang/v8/pkg/papi"
	"github.com/akamai/AkamaiOPEN-edgegrid-golang/v8/pkg/session"
)

// Client represents an Akamai API client using the official EdgeGrid client
type Client struct {
	papiClient papi.PAPI
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
