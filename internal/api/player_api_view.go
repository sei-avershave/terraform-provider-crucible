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

// -------------------- Plugin Framework functions (new) --------------------

// CreateView creates a view using the centralized client.
func CreateView(ctx context.Context, c *client.CrucibleClient, view *structs.ViewInfo) (string, error) {
	payload := map[string]interface{}{
		"name":            view.Name,
		"createAdminTeam": view.CreateAdminTeam,
	}

	if view.Description != "" {
		payload["description"] = view.Description
	}
	if view.Status != "" {
		payload["status"] = view.Status
	} else {
		payload["status"] = "Active"
	}

	url := c.GetPlayerAPIURL() + "views"
	var result map[string]interface{}

	if err := c.DoPost(ctx, url, payload, &result); err != nil {
		return "", fmt.Errorf("failed to create view: %w", err)
	}

	id, ok := result["id"].(string)
	if !ok {
		return "", fmt.Errorf("view ID not found in API response")
	}

	return id, nil
}

// ReadView reads a view by ID using the centralized client.
func ReadView(ctx context.Context, c *client.CrucibleClient, id string) (*structs.ViewInfo, error) {
	url := c.GetPlayerAPIURL() + "views/" + id
	view := new(structs.ViewInfo)

	if err := c.DoGet(ctx, url, view); err != nil {
		return nil, fmt.Errorf("failed to read view %s: %w", id, err)
	}

	return view, nil
}

// UpdateView updates a view's metadata using the centralized client.
func UpdateView(ctx context.Context, c *client.CrucibleClient, id string, view *structs.ViewInfo) error {
	payload := map[string]interface{}{
		"name":   view.Name,
		"status": view.Status,
	}

	if view.Description != "" {
		payload["description"] = view.Description
	}

	url := c.GetPlayerAPIURL() + "views/" + id
	if err := c.DoPut(ctx, url, payload); err != nil {
		return fmt.Errorf("failed to update view: %w", err)
	}

	return nil
}

// DeleteView deletes a view using the centralized client.
func DeleteView(ctx context.Context, c *client.CrucibleClient, id string) error {
	url := c.GetPlayerAPIURL() + "views/" + id
	if err := c.DoDelete(ctx, url); err != nil {
		return fmt.Errorf("failed to delete view %s: %w", id, err)
	}
	return nil
}

// ViewExists checks if a view exists using the centralized client.
func ViewExists(ctx context.Context, c *client.CrucibleClient, id string) (bool, error) {
	url := c.GetPlayerAPIURL() + "views/" + id
	resp, err := c.DoRequest(ctx, "GET", url, nil)
	if err != nil {
		return false, fmt.Errorf("failed to check view existence: %w", err)
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}
