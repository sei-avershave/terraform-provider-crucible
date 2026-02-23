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

// ---------------------- Plugin Framework functions (new) ----------------------

// CreateUser creates a new player user using the centralized client.
func CreateUser(ctx context.Context, c *client.CrucibleClient, user *structs.PlayerUser) error {
	// If a role was set, find its ID. Otherwise set role field to nil
	var roleID interface{} = nil
	if roleStr, ok := user.Role.(string); ok && roleStr != "" {
		role, err := getRoleByNameWithClient(ctx, c, roleStr)
		if err != nil {
			return fmt.Errorf("failed to resolve role '%s': %w", roleStr, err)
		}
		roleID = role
	}
	user.Role = roleID

	url := c.GetPlayerAPIURL() + "users"
	var result map[string]interface{}
	if err := c.DoPost(ctx, url, user, &result); err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

// ReadUser reads a player user by ID using the centralized client.
func ReadUser(ctx context.Context, c *client.CrucibleClient, id string) (*structs.PlayerUser, error) {
	url := c.GetPlayerAPIURL() + "users/" + id
	user := new(structs.PlayerUser)

	if err := c.DoGet(ctx, url, user); err != nil {
		return nil, fmt.Errorf("failed to read user %s: %w", id, err)
	}

	return user, nil
}

// UpdateUser updates an existing player user using the centralized client.
func UpdateUser(ctx context.Context, c *client.CrucibleClient, user *structs.PlayerUser) error {
	// If a role was set, find its ID. Otherwise set role field to nil
	var roleID interface{} = nil
	if roleStr, ok := user.Role.(string); ok && roleStr != "" {
		role, err := getRoleByNameWithClient(ctx, c, roleStr)
		if err != nil {
			return fmt.Errorf("failed to resolve role '%s': %w", roleStr, err)
		}
		roleID = role
	}
	user.Role = roleID

	url := c.GetPlayerAPIURL() + "users/" + user.ID
	if err := c.DoPut(ctx, url, user); err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	return nil
}

// DeleteUser deletes a player user by ID using the centralized client.
func DeleteUser(ctx context.Context, c *client.CrucibleClient, id string) error {
	url := c.GetPlayerAPIURL() + "users/" + id
	if err := c.DoDelete(ctx, url); err != nil {
		return fmt.Errorf("failed to delete user %s: %w", id, err)
	}
	return nil
}

// UserExists checks if a player user exists using the centralized client.
func UserExists(ctx context.Context, c *client.CrucibleClient, id string) (bool, error) {
	url := c.GetPlayerAPIURL() + "users/" + id
	resp, err := c.DoRequest(ctx, "GET", url, nil)
	if err != nil {
		return false, fmt.Errorf("failed to check user existence: %w", err)
	}
	defer resp.Body.Close()

	// User exists if we get 200, doesn't exist if 404
	return resp.StatusCode == http.StatusOK, nil
}

// getRoleByNameWithClient looks up a role ID by name using the centralized client.
func getRoleByNameWithClient(ctx context.Context, c *client.CrucibleClient, roleName string) (string, error) {
	url := c.GetPlayerAPIURL() + "roles/name/" + roleName

	var result map[string]interface{}
	if err := c.DoGet(ctx, url, &result); err != nil {
		return "", fmt.Errorf("failed to lookup role '%s': %w", roleName, err)
	}

	roleID, ok := result["id"].(string)
	if !ok {
		return "", fmt.Errorf("role ID not found in response for role '%s'", roleName)
	}

	return roleID, nil
}

// GetRoleByID returns the name of the role with the given ID using the centralized client.
func GetRoleByID(ctx context.Context, c *client.CrucibleClient, roleID string) (string, error) {
	url := c.GetPlayerAPIURL() + "roles/" + roleID

	var result map[string]interface{}
	if err := c.DoGet(ctx, url, &result); err != nil {
		return "", fmt.Errorf("failed to lookup role by ID '%s': %w", roleID, err)
	}

	roleName, ok := result["name"].(string)
	if !ok {
		return "", fmt.Errorf("role name not found in response for role ID '%s'", roleID)
	}

	return roleName, nil
}

