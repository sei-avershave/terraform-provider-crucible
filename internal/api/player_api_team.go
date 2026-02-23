// Copyright 2022 Carnegie Mellon University. All Rights Reserved.
// Released under a MIT (SEI)-style license. See LICENSE.md in the project root for license information.

package api

import (
	"context"
	"crucible_provider/internal/client"
	"crucible_provider/internal/structs"
	"fmt"
)

// -------------------- Plugin Framework functions (new) --------------------

// CreateTeams creates teams in a view using the centralized client.
func CreateTeams(ctx context.Context, c *client.CrucibleClient, teams *[]structs.TeamInfo, viewID string) error {
	for i := range *teams {
		team := &(*teams)[i]

		payload := map[string]interface{}{
			"name": team.Name,
			"role": team.Role,
		}

		url := c.GetPlayerAPIURL() + "views/" + viewID + "/teams"
		var result map[string]interface{}

		if err := c.DoPost(ctx, url, payload, &result); err != nil {
			return fmt.Errorf("failed to create team '%v': %w", team.Name, err)
		}

		// Store the team ID
		if id, ok := result["id"].(string); ok {
			team.ID = id
		}

		// Add users to team
		if len(team.Users) > 0 {
			for _, user := range team.Users {
				userURL := c.GetPlayerAPIURL() + "teams/" + team.ID.(string) + "/users/" + user.ID
				if err := c.DoPost(ctx, userURL, nil, nil); err != nil {
					return fmt.Errorf("failed to add user to team: %w", err)
				}

				// Set user role if specified
				if userRole, ok := user.Role.(string); ok && userRole != "" {
					if err := setUserRoleInTeam(ctx, c, user.ID, team.ID.(string), viewID, userRole); err != nil {
						return fmt.Errorf("failed to set user role: %w", err)
					}
				}
			}
		}

		// Add app instances to team
		if len(team.AppInstances) > 0 {
			for j, instance := range team.AppInstances {
				// Find the application ID by name
				appID, err := getAppIDByName(ctx, c, viewID, instance.Name)
				if err != nil {
					return fmt.Errorf("failed to find application '%s': %w", instance.Name, err)
				}

				instancePayload := map[string]interface{}{
					"applicationId": appID,
					"displayOrder":  instance.DisplayOrder,
				}

				instURL := c.GetPlayerAPIURL() + "teams/" + team.ID.(string) + "/application-instances"
				var instResult map[string]interface{}

				if err := c.DoPost(ctx, instURL, instancePayload, &instResult); err != nil {
					return fmt.Errorf("failed to create application instance: %w", err)
				}

				// Store the instance ID
				if id, ok := instResult["id"].(string); ok {
					team.AppInstances[j].ID = id
				}
			}
		}
	}

	return nil
}

// AddPermissionsToTeam adds permissions to teams using the centralized client.
func AddPermissionsToTeam(ctx context.Context, c *client.CrucibleClient, teams *[]structs.TeamInfo) error {
	for _, team := range *teams {
		if len(team.Permissions) == 0 {
			continue
		}

		teamID, ok := team.ID.(string)
		if !ok || teamID == "" {
			continue
		}

		for _, permName := range team.Permissions {
			// Look up permission ID by name
			permID, err := getPermissionIDByName(ctx, c, permName)
			if err != nil {
				return fmt.Errorf("failed to lookup permission '%s': %w", permName, err)
			}

			payload := map[string]interface{}{
				"teamId":       teamID,
				"permissionId": permID,
			}

			url := c.GetPlayerAPIURL() + "team-permissions"
			if err := c.DoPost(ctx, url, payload, nil); err != nil {
				return fmt.Errorf("failed to add permission '%s' to team: %w", permName, err)
			}
		}
	}

	return nil
}

// setUserRoleInTeam sets a user's role within a team using the centralized client.
func setUserRoleInTeam(ctx context.Context, c *client.CrucibleClient, userID, teamID, viewID, roleName string) error {
	// Find the membership ID
	membershipURL := c.GetPlayerAPIURL() + "users/" + userID + "/views/" + viewID + "/team-memberships"
	var memberships []map[string]interface{}

	if err := c.DoGet(ctx, membershipURL, &memberships); err != nil {
		return fmt.Errorf("failed to get team memberships: %w", err)
	}

	var membershipID string
	for _, membership := range memberships {
		if membership["teamId"] == teamID {
			membershipID = membership["id"].(string)
			break
		}
	}

	if membershipID == "" {
		return fmt.Errorf("no membership found for user %s in team %s", userID, teamID)
	}

	// Look up role ID
	roleID, err := getRoleByNameWithClient(ctx, c, roleName)
	if err != nil {
		return fmt.Errorf("failed to resolve role '%s': %w", roleName, err)
	}

	// Set the role
	payload := map[string]interface{}{
		"roleId": roleID,
	}

	url := c.GetPlayerAPIURL() + "team-memberships/" + membershipID
	if err := c.DoPut(ctx, url, payload); err != nil {
		return fmt.Errorf("failed to set user role: %w", err)
	}

	return nil
}

// getPermissionIDByName looks up a permission ID by name using the centralized client.
func getPermissionIDByName(ctx context.Context, c *client.CrucibleClient, permName string) (string, error) {
	url := c.GetPlayerAPIURL() + "permissions/name/" + permName

	var result map[string]interface{}
	if err := c.DoGet(ctx, url, &result); err != nil {
		return "", fmt.Errorf("failed to lookup permission '%s': %w", permName, err)
	}

	permID, ok := result["id"].(string)
	if !ok {
		return "", fmt.Errorf("permission ID not found for '%s'", permName)
	}

	return permID, nil
}

// getAppIDByName looks up an application ID by name within a view using the centralized client.
func getAppIDByName(ctx context.Context, c *client.CrucibleClient, viewID, appName string) (string, error) {
	url := c.GetPlayerAPIURL() + "views/" + viewID + "/applications"

	var apps []map[string]interface{}
	if err := c.DoGet(ctx, url, &apps); err != nil {
		return "", fmt.Errorf("failed to get applications: %w", err)
	}

	for _, app := range apps {
		if app["name"] == appName {
			if id, ok := app["id"].(string); ok {
				return id, nil
			}
		}
	}

	return "", fmt.Errorf("application '%s' not found in view %s", appName, viewID)
}

