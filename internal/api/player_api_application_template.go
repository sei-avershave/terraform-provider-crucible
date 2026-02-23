// Copyright 2022 Carnegie Mellon University. All Rights Reserved.
// Released under a MIT (SEI)-style license. See LICENSE.md in the project root for license information.

package api

import (
	"context"
	"crucible_provider/internal/client"
	"crucible_provider/internal/structs"
	"fmt"
	"net/http"
)

// --------------------- Plugin Framework functions (new) ---------------------

// CreateAppTemplate creates an application template using the centralized client.
func CreateAppTemplate(ctx context.Context, c *client.CrucibleClient, template *structs.AppTemplate) (string, error) {
	// Build payload, handling empty strings as nil
	payload := map[string]interface{}{
		"name":             template.Name,
		"embeddable":       template.Embeddable,
		"loadInBackground": template.LoadInBackground,
	}

	// Only include optional fields if they're set
	if template.URL != "" {
		payload["url"] = template.URL
	}
	if template.Icon != "" {
		payload["icon"] = template.Icon
	}

	url := c.GetPlayerAPIURL() + "application-templates"
	var result map[string]interface{}

	if err := c.DoPost(ctx, url, payload, &result); err != nil {
		return "", fmt.Errorf("failed to create application template: %w", err)
	}

	id, ok := result["id"].(string)
	if !ok {
		return "", fmt.Errorf("application template ID not found in API response")
	}

	return id, nil
}

// AppTemplateRead reads an application template by ID using the centralized client.
func AppTemplateRead(ctx context.Context, c *client.CrucibleClient, id string) (*structs.AppTemplate, error) {
	url := c.GetPlayerAPIURL() + "application-templates/" + id
	template := new(structs.AppTemplate)

	if err := c.DoGet(ctx, url, template); err != nil {
		return nil, fmt.Errorf("failed to read application template %s: %w", id, err)
	}

	return template, nil
}

// AppTemplateUpdate updates an existing application template using the centralized client.
func AppTemplateUpdate(ctx context.Context, c *client.CrucibleClient, id string, template *structs.AppTemplate) error {
	// Build payload
	payload := map[string]interface{}{
		"name":             template.Name,
		"embeddable":       template.Embeddable,
		"loadInBackground": template.LoadInBackground,
	}

	// Only include optional fields if they're set
	if template.URL != "" {
		payload["url"] = template.URL
	}
	if template.Icon != "" {
		payload["icon"] = template.Icon
	}

	url := c.GetPlayerAPIURL() + "application-templates/" + id
	if err := c.DoPut(ctx, url, payload); err != nil {
		return fmt.Errorf("failed to update application template: %w", err)
	}

	return nil
}

// DeleteAppTemplate deletes an application template by ID using the centralized client.
func DeleteAppTemplate(ctx context.Context, c *client.CrucibleClient, id string) error {
	url := c.GetPlayerAPIURL() + "application-templates/" + id
	if err := c.DoDelete(ctx, url); err != nil {
		return fmt.Errorf("failed to delete application template %s: %w", id, err)
	}
	return nil
}

// AppTemplateExists checks if an application template exists using the centralized client.
func AppTemplateExists(ctx context.Context, c *client.CrucibleClient, id string) (bool, error) {
	url := c.GetPlayerAPIURL() + "application-templates/" + id
	resp, err := c.DoRequest(ctx, "GET", url, nil)
	if err != nil {
		return false, fmt.Errorf("failed to check application template existence: %w", err)
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}

