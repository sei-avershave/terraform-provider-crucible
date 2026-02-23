// Copyright 2024 Carnegie Mellon University. All Rights Reserved.
// Released under a MIT (SEI)-style license. See LICENSE.md in the project root for license information.

package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

// TestGetToken_Caching verifies that tokens are cached and reused
func TestGetToken_Caching(t *testing.T) {
	callCount := 0

	// Mock OAuth2 token server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		token := map[string]interface{}{
			"access_token": "test-token-" + string(rune(callCount)),
			"token_type":   "Bearer",
			"expires_in":   3600,
		}
		json.NewEncoder(w).Encode(token)
	}))
	defer server.Close()

	config := &ProviderConfig{
		Username:     "test-user",
		Password:     "test-pass",
		AuthURL:      server.URL + "/auth",
		TokenURL:     server.URL + "/token",
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		ClientScopes: []string{"test-scope"},
	}

	client := NewClient(config)
	ctx := context.Background()

	// First call should fetch token
	token1, err := client.GetToken(ctx)
	if err != nil {
		t.Fatalf("GetToken failed: %v", err)
	}
	if callCount != 1 {
		t.Errorf("Expected 1 token server call, got %d", callCount)
	}

	// Second call should use cached token
	token2, err := client.GetToken(ctx)
	if err != nil {
		t.Fatalf("GetToken failed: %v", err)
	}
	if callCount != 1 {
		t.Errorf("Expected cached token to be reused, but server was called %d times", callCount)
	}

	// Tokens should be identical
	if token1 != token2 {
		t.Errorf("Expected cached token to match: %s != %s", token1, token2)
	}
}

// TestGetToken_Expiry verifies that expired tokens are refreshed
func TestGetToken_Expiry(t *testing.T) {
	callCount := 0

	// Mock OAuth2 token server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		token := map[string]interface{}{
			"access_token": "refreshed-token-" + string(rune(callCount)),
			"token_type":   "Bearer",
			"expires_in":   1, // Expire in 1 second
		}
		json.NewEncoder(w).Encode(token)
	}))
	defer server.Close()

	config := &ProviderConfig{
		Username:     "test-user",
		Password:     "test-pass",
		AuthURL:      server.URL + "/auth",
		TokenURL:     server.URL + "/token",
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		ClientScopes: []string{"test-scope"},
	}

	client := NewClient(config)
	ctx := context.Background()

	// First call fetches token
	token1, err := client.GetToken(ctx)
	if err != nil {
		t.Fatalf("GetToken failed: %v", err)
	}
	if callCount != 1 {
		t.Errorf("Expected 1 token server call, got %d", callCount)
	}

	// Wait for token to expire
	time.Sleep(2 * time.Second)

	// Second call should fetch new token
	token2, err := client.GetToken(ctx)
	if err != nil {
		t.Fatalf("GetToken failed: %v", err)
	}
	if callCount != 2 {
		t.Errorf("Expected token to be refreshed (2 calls), got %d calls", callCount)
	}

	// Tokens should be different
	if token1 == token2 {
		t.Errorf("Expected new token after expiry, but got same token: %s", token1)
	}
}

// TestGetToken_Error verifies that authentication errors are properly returned
func TestGetToken_Error(t *testing.T) {
	// Mock OAuth2 token server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"error":             "invalid_grant",
			"error_description": "Invalid username or password",
		})
	}))
	defer server.Close()

	config := &ProviderConfig{
		Username:     "bad-user",
		Password:     "bad-pass",
		AuthURL:      server.URL + "/auth",
		TokenURL:     server.URL + "/token",
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		ClientScopes: []string{"test-scope"},
	}

	client := NewClient(config)
	ctx := context.Background()

	_, err := client.GetToken(ctx)
	if err == nil {
		t.Fatal("Expected GetToken to return error for invalid credentials")
	}

	if !strings.Contains(err.Error(), "failed to obtain OAuth2 token") {
		t.Errorf("Expected error message about OAuth2 failure, got: %v", err)
	}
}

// TestHandleAPIError_JSONError verifies extraction of error messages from JSON responses
func TestHandleAPIError_JSONError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Invalid request: missing required field 'name'",
		})
	}))
	defer server.Close()

	config := &ProviderConfig{
		ClientID:     "test",
		ClientSecret: "test",
		TokenURL:     server.URL,
	}
	client := NewClient(config)

	// Make request
	req, _ := http.NewRequest("GET", server.URL, nil)
	resp, _ := client.httpClient.Do(req)

	// Handle error
	apiErr := client.HandleAPIError(resp)

	if apiErr.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", apiErr.StatusCode)
	}

	if !strings.Contains(apiErr.Message, "Invalid request") {
		t.Errorf("Expected error message to be extracted from JSON, got: %s", apiErr.Message)
	}
}

// TestHandleAPIError_PlainText verifies handling of plain text error responses
func TestHandleAPIError_PlainText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal server error"))
	}))
	defer server.Close()

	config := &ProviderConfig{
		ClientID:     "test",
		ClientSecret: "test",
		TokenURL:     server.URL,
	}
	client := NewClient(config)

	req, _ := http.NewRequest("GET", server.URL, nil)
	resp, _ := client.httpClient.Do(req)

	apiErr := client.HandleAPIError(resp)

	if apiErr.StatusCode != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", apiErr.StatusCode)
	}

	if !strings.Contains(apiErr.Body, "Internal server error") {
		t.Errorf("Expected error body to be included, got: %s", apiErr.Body)
	}
}

// TestDoGet verifies GET requests work correctly
func TestDoGet(t *testing.T) {
	// Mock OAuth2 token server
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "test-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	}))
	defer tokenServer.Close()

	// Mock API server
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify auth header
		if r.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":   "123",
			"name": "Test Resource",
		})
	}))
	defer apiServer.Close()

	config := &ProviderConfig{
		Username:     "test-user",
		Password:     "test-pass",
		TokenURL:     tokenServer.URL,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		ClientScopes: []string{},
	}

	client := NewClient(config)
	ctx := context.Background()

	var result map[string]interface{}
	err := client.DoGet(ctx, apiServer.URL, &result)
	if err != nil {
		t.Fatalf("DoGet failed: %v", err)
	}

	if result["id"] != "123" {
		t.Errorf("Expected id=123, got %v", result["id"])
	}
	if result["name"] != "Test Resource" {
		t.Errorf("Expected name='Test Resource', got %v", result["name"])
	}
}

// TestDoPost verifies POST requests work correctly
func TestDoPost(t *testing.T) {
	// Mock OAuth2 token server
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "test-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	}))
	defer tokenServer.Close()

	// Mock API server
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify method
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		// Verify auth header
		if r.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Verify content type
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		// Read and echo back the body with an ID
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		body["id"] = "new-id-123"

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(body)
	}))
	defer apiServer.Close()

	config := &ProviderConfig{
		Username:     "test-user",
		Password:     "test-pass",
		TokenURL:     tokenServer.URL,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		ClientScopes: []string{},
	}

	client := NewClient(config)
	ctx := context.Background()

	requestBody := map[string]interface{}{
		"name":        "Test Resource",
		"description": "Test Description",
	}

	var result map[string]interface{}
	err := client.DoPost(ctx, apiServer.URL, requestBody, &result)
	if err != nil {
		t.Fatalf("DoPost failed: %v", err)
	}

	if result["id"] != "new-id-123" {
		t.Errorf("Expected id=new-id-123, got %v", result["id"])
	}
	if result["name"] != "Test Resource" {
		t.Errorf("Expected name='Test Resource', got %v", result["name"])
	}
}

// TestDoRequest_401Retry verifies automatic token refresh on 401
func TestDoRequest_401Retry(t *testing.T) {
	tokenCallCount := 0
	apiCallCount := 0

	// Mock OAuth2 token server
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenCallCount++
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "token-" + string(rune(tokenCallCount)),
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	}))
	defer tokenServer.Close()

	// Mock API server that rejects first token but accepts second
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiCallCount++
		auth := r.Header.Get("Authorization")

		// First call with first token fails with 401
		if apiCallCount == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Token expired",
			})
			return
		}

		// Second call with refreshed token succeeds
		if auth == "Bearer token-2" {
			json.NewEncoder(w).Encode(map[string]string{
				"result": "success",
			})
			return
		}

		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer apiServer.Close()

	config := &ProviderConfig{
		Username:     "test-user",
		Password:     "test-pass",
		TokenURL:     tokenServer.URL,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		ClientScopes: []string{},
	}

	client := NewClient(config)
	ctx := context.Background()

	// Pre-cache the first token (which will be rejected)
	client.token = &oauth2.Token{
		AccessToken: "token-1",
		Expiry:      time.Now().Add(1 * time.Hour),
	}

	// Make request - should fail first, then retry with new token
	var result map[string]interface{}
	err := client.DoGet(ctx, apiServer.URL, &result)
	if err != nil {
		t.Fatalf("DoGet failed after retry: %v", err)
	}

	// Should have made 2 API calls (initial + retry)
	if apiCallCount != 2 {
		t.Errorf("Expected 2 API calls (initial + retry), got %d", apiCallCount)
	}

	// Should have fetched 1 new token (the retry token)
	if tokenCallCount != 1 {
		t.Errorf("Expected 1 token refresh, got %d", tokenCallCount)
	}

	if result["result"] != "success" {
		t.Errorf("Expected successful response, got %v", result)
	}
}

// TestNormalizeAPIURL verifies URL normalization logic
func TestNormalizeAPIURL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"https://api.example.com", "https://api.example.com/api/"},
		{"https://api.example.com/", "https://api.example.com/api/"},
		{"https://api.example.com/api", "https://api.example.com/api/"},
		{"https://api.example.com/api/", "https://api.example.com/api/"},
		{"http://localhost:4300", "http://localhost:4300/api/"},
	}

	for _, test := range tests {
		result := normalizeAPIURL(test.input)
		if result != test.expected {
			t.Errorf("normalizeAPIURL(%q) = %q, want %q", test.input, result, test.expected)
		}
	}
}

// TestHandleAPIError_MultipleErrorFormats verifies parsing of different error response formats
func TestHandleAPIError_MultipleErrorFormats(t *testing.T) {
	tests := []struct {
		name           string
		statusCode     int
		responseBody   string
		expectedMsg    string
		expectedInBody string
	}{
		{
			name:           "JSON with message field",
			statusCode:     400,
			responseBody:   `{"message":"Invalid field value"}`,
			expectedMsg:    "Invalid field value",
			expectedInBody: "Invalid field value",
		},
		{
			name:           "JSON with error field",
			statusCode:     500,
			responseBody:   `{"error":"Database connection failed"}`,
			expectedMsg:    "Database connection failed",
			expectedInBody: "Database connection failed",
		},
		{
			name:           "JSON with title field",
			statusCode:     404,
			responseBody:   `{"title":"Resource not found"}`,
			expectedMsg:    "Resource not found",
			expectedInBody: "Resource not found",
		},
		{
			name:           "Plain text error",
			statusCode:     503,
			responseBody:   "Service temporarily unavailable",
			expectedMsg:    "503 Service Unavailable",
			expectedInBody: "Service temporarily unavailable",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(test.statusCode)
				w.Write([]byte(test.responseBody))
			}))
			defer server.Close()

			config := &ProviderConfig{ClientID: "test", ClientSecret: "test", TokenURL: server.URL}
			client := NewClient(config)

			req, _ := http.NewRequest("GET", server.URL, nil)
			resp, _ := client.httpClient.Do(req)

			apiErr := client.HandleAPIError(resp)

			if apiErr.StatusCode != test.statusCode {
				t.Errorf("Expected status %d, got %d", test.statusCode, apiErr.StatusCode)
			}

			if !strings.Contains(apiErr.Message, test.expectedMsg) {
				t.Errorf("Expected message to contain %q, got %q", test.expectedMsg, apiErr.Message)
			}

			if !strings.Contains(apiErr.Body, test.expectedInBody) {
				t.Errorf("Expected body to contain %q, got %q", test.expectedInBody, apiErr.Body)
			}
		})
	}
}
