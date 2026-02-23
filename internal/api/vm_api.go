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

// CreateVM creates a new virtual machine using the centralized client.
func CreateVM(ctx context.Context, c *client.CrucibleClient, vmInfo *structs.VMInfo) error {
	url := c.GetVMAPIURL() + "vms"
	var result map[string]interface{}

	if err := c.DoPost(ctx, url, vmInfo, &result); err != nil {
		return fmt.Errorf("failed to create VM: %w", err)
	}

	// Check if API returned a default URL
	if defaultURL, ok := result["defaultUrl"].(bool); ok {
		vmInfo.DefaultURL = defaultURL
	}

	return nil
}

// GetVMInfo reads a VM's information using the centralized client.
func GetVMInfo(ctx context.Context, c *client.CrucibleClient, id string) (*structs.VMInfo, error) {
	url := c.GetVMAPIURL() + "vms/" + id
	vmInfo := new(structs.VMInfo)

	if err := c.DoGet(ctx, url, vmInfo); err != nil {
		return nil, fmt.Errorf("failed to read VM %s: %w", id, err)
	}

	return vmInfo, nil
}

// UpdateVM updates an existing virtual machine using the centralized client.
func UpdateVM(ctx context.Context, c *client.CrucibleClient, vmInfo *structs.VMInfo) error {
	url := c.GetVMAPIURL() + "vms/" + vmInfo.ID

	if err := c.DoPut(ctx, url, vmInfo); err != nil {
		return fmt.Errorf("failed to update VM: %w", err)
	}

	return nil
}

// DeleteVM deletes a virtual machine using the centralized client.
func DeleteVM(ctx context.Context, c *client.CrucibleClient, id string) error {
	url := c.GetVMAPIURL() + "vms/" + id

	if err := c.DoDelete(ctx, url); err != nil {
		return fmt.Errorf("failed to delete VM %s: %w", id, err)
	}

	return nil
}

// VMExists checks if a virtual machine exists using the centralized client.
func VMExists(ctx context.Context, c *client.CrucibleClient, id string) (bool, error) {
	url := c.GetVMAPIURL() + "vms/" + id
	resp, err := c.DoRequest(ctx, "GET", url, nil)
	if err != nil {
		return false, fmt.Errorf("failed to check VM existence: %w", err)
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}

// AddVMToTeams adds a VM to multiple teams using the centralized client.
func AddVMToTeams(ctx context.Context, c *client.CrucibleClient, vmID string, teamIDs []string) error {
	for _, teamID := range teamIDs {
		url := c.GetVMAPIURL() + "teams/" + teamID + "/vms/" + vmID
		if err := c.DoPost(ctx, url, nil, nil); err != nil {
			return fmt.Errorf("failed to add VM %s to team %s: %w", vmID, teamID, err)
		}
	}
	return nil
}

// RemoveVMFromTeams removes a VM from multiple teams using the centralized client.
func RemoveVMFromTeams(ctx context.Context, c *client.CrucibleClient, vmID string, teamIDs []string) error {
	for _, teamID := range teamIDs {
		url := c.GetVMAPIURL() + "teams/" + teamID + "/vms/" + vmID
		if err := c.DoDelete(ctx, url); err != nil {
			return fmt.Errorf("failed to remove VM %s from team %s: %w", vmID, teamID, err)
		}
	}
	return nil
}

