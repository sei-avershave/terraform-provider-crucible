// Copyright 2024 Carnegie Mellon University. All Rights Reserved.
// Released under a MIT (SEI)-style license. See LICENSE.md in the project root for license information.

package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
)

// ProviderConfig holds all configuration needed for the Crucible provider
type ProviderConfig struct {
	Username      string
	Password      string
	AuthURL       string
	TokenURL      string
	VMApiURL      string
	PlayerApiURL  string
	CasterApiURL  string
	ClientID      string
	ClientSecret  string
	ClientScopes  []string
}

// CrucibleClient is a centralized HTTP client for all Crucible API calls
// It handles OAuth2 token caching, automatic refresh, and rich error messages
type CrucibleClient struct {
	config     *ProviderConfig
	token      *oauth2.Token
	tokenMutex sync.RWMutex
	httpClient *http.Client
}

// APIError represents a structured error from the Crucible APIs
type APIError struct {
	StatusCode int
	Message    string
	Body       string
}

// Error implements the error interface for APIError
func (e *APIError) Error() string {
	if e.Body != "" && e.Body != e.Message {
		return fmt.Sprintf("API returned status %d: %s (body: %s)", e.StatusCode, e.Message, e.Body)
	}
	return fmt.Sprintf("API returned status %d: %s", e.StatusCode, e.Message)
}

// NewClient creates a new CrucibleClient with the given configuration
func NewClient(config *ProviderConfig) *CrucibleClient {
	return &CrucibleClient{
		config: config,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetToken returns a valid OAuth2 access token, using cached token if available
// and automatically refreshing if expired
func (c *CrucibleClient) GetToken(ctx context.Context) (string, error) {
	// Fast path: check if we have a valid cached token
	c.tokenMutex.RLock()
	if c.token != nil && c.token.Valid() {
		token := c.token.AccessToken
		c.tokenMutex.RUnlock()
		return token, nil
	}
	c.tokenMutex.RUnlock()

	// Slow path: acquire write lock and fetch new token
	c.tokenMutex.Lock()
	defer c.tokenMutex.Unlock()

	// Double-check in case another goroutine already refreshed
	if c.token != nil && c.token.Valid() {
		return c.token.AccessToken, nil
	}

	// Parse scopes - handle empty string or nil
	scopes := c.config.ClientScopes
	if len(scopes) == 1 && scopes[0] == "" {
		scopes = nil
	}

	// Create OAuth2 config
	oauthConfig := &oauth2.Config{
		ClientID:     c.config.ClientID,
		ClientSecret: c.config.ClientSecret,
		Scopes:       scopes,
		Endpoint: oauth2.Endpoint{
			AuthURL:  c.config.AuthURL,
			TokenURL: c.config.TokenURL,
		},
	}

	// Fetch token using password credentials grant
	token, err := oauthConfig.PasswordCredentialsToken(ctx, c.config.Username, c.config.Password)
	if err != nil {
		return "", fmt.Errorf("failed to obtain OAuth2 token: %w", err)
	}

	c.token = token
	return token.AccessToken, nil
}

// DoRequest performs an HTTP request with automatic authentication
// It handles token injection, retries on auth failures, and returns the response
func (c *CrucibleClient) DoRequest(ctx context.Context, method, url string, body interface{}) (*http.Response, error) {
	// Marshal body if provided
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewBuffer(jsonBody)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Get auth token
	token, err := c.GetToken(ctx)
	if err != nil {
		return nil, err
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}

	// If we get 401, token might have expired - try refreshing once
	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()

		// Invalidate cached token and get a fresh one
		c.tokenMutex.Lock()
		c.token = nil
		c.tokenMutex.Unlock()

		token, err = c.GetToken(ctx)
		if err != nil {
			return nil, err
		}

		// Recreate request with new token
		if body != nil {
			jsonBody, _ := json.Marshal(body)
			bodyReader = bytes.NewBuffer(jsonBody)
		}
		req, err = http.NewRequestWithContext(ctx, method, url, bodyReader)
		if err != nil {
			return nil, fmt.Errorf("failed to create retry HTTP request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		// Retry request
		resp, err = c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("HTTP retry request failed: %w", err)
		}
	}

	return resp, nil
}

// DecodeResponse decodes a JSON response body into the target struct
func (c *CrucibleClient) DecodeResponse(resp *http.Response, target interface{}) error {
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return nil
}

// HandleAPIError extracts error details from an unsuccessful API response
func (c *CrucibleClient) HandleAPIError(resp *http.Response) *APIError {
	defer resp.Body.Close()

	apiErr := &APIError{
		StatusCode: resp.StatusCode,
		Message:    resp.Status,
	}

	// Try to read response body for more context
	bodyBytes, err := io.ReadAll(resp.Body)
	if err == nil && len(bodyBytes) > 0 {
		apiErr.Body = string(bodyBytes)

		// Try to extract error message from JSON response
		var errorResponse map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &errorResponse); err == nil {
			// Check common error message fields
			if msg, ok := errorResponse["message"].(string); ok {
				apiErr.Message = msg
			} else if msg, ok := errorResponse["error"].(string); ok {
				apiErr.Message = msg
			} else if msg, ok := errorResponse["title"].(string); ok {
				apiErr.Message = msg
			}
		}
	}

	return apiErr
}

// GetPlayerAPIURL returns the normalized Player API base URL
func (c *CrucibleClient) GetPlayerAPIURL() string {
	return normalizeAPIURL(c.config.PlayerApiURL)
}

// GetVMAPIURL returns the normalized VM API base URL
func (c *CrucibleClient) GetVMAPIURL() string {
	return normalizeAPIURL(c.config.VMApiURL)
}

// GetCasterAPIURL returns the normalized Caster API base URL
func (c *CrucibleClient) GetCasterAPIURL() string {
	return normalizeAPIURL(c.config.CasterApiURL)
}

// normalizeAPIURL ensures consistent URL formatting
// Strips trailing "/" and "/api", then appends "/api/"
func normalizeAPIURL(url string) string {
	url = strings.TrimSuffix(url, "/")
	url = strings.TrimSuffix(url, "/api")
	return url + "/api/"
}

// DoGet performs a GET request and decodes the response into target
func (c *CrucibleClient) DoGet(ctx context.Context, url string, target interface{}) error {
	resp, err := c.DoRequest(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return c.HandleAPIError(resp)
	}

	return c.DecodeResponse(resp, target)
}

// DoPost performs a POST request with the given body and decodes the response into target
func (c *CrucibleClient) DoPost(ctx context.Context, url string, body interface{}, target interface{}) error {
	resp, err := c.DoRequest(ctx, "POST", url, body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return c.HandleAPIError(resp)
	}

	if target != nil {
		return c.DecodeResponse(resp, target)
	}

	resp.Body.Close()
	return nil
}

// DoPut performs a PUT request with the given body
func (c *CrucibleClient) DoPut(ctx context.Context, url string, body interface{}) error {
	resp, err := c.DoRequest(ctx, "PUT", url, body)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return c.HandleAPIError(resp)
	}

	return nil
}

// DoDelete performs a DELETE request
func (c *CrucibleClient) DoDelete(ctx context.Context, url string) error {
	resp, err := c.DoRequest(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		return c.HandleAPIError(resp)
	}

	return nil
}
