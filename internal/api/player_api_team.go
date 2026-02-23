// Copyright 2022 Carnegie Mellon University. All Rights Reserved.
// Released under a MIT (SEI)-style license. See LICENSE.md in the project root for license information.

package api

import (
	"bytes"
	"context"
	"crucible_provider/internal/client"
	"crucible_provider/internal/structs"
	"crucible_provider/internal/util"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
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

// -------------------- API Wrappers (SDK v1 - legacy) --------------------
//
// param teams the teams to create
//
// param viewID: the view to create the teams within
//
// param m map: containing provider config info
//
// Returns some error on failure or nil on success
func CreateTeams(teams *[]*structs.TeamInfo, viewID string, m map[string]string) error {
	log.Printf("! At top of API wrapper to create teams")

	auth, err := util.GetAuth(m)
	if err != nil {
		return err
	}

	// Create a new team for each entry in the slice of structs
	for i, team := range *teams {
		// We don't want the ID field in the request, so make struct into map and remove that key
		asMap := team.ToMap()
		delete(asMap, "id")

		// Need to look up the role by name
		role := asMap["role"]
		delete(asMap, "role")

		log.Printf("! Team's role: %v", role)
		if role.(string) != "" {
			roleID, err := getTeamRoleByName(role.(string), auth, m)
			if err != nil {
				return err
			}

			// API wasn't seeing role_id, rename to roleId
			asMap["roleId"] = roleID
		}

		asJSON, err := json.Marshal(asMap)
		if err != nil {
			return err
		}

		log.Printf("! Team being created: %+v", asMap)

		url := util.GetPlayerApiUrl(m) + "views/" + viewID + "/teams"
		request, err := http.NewRequest("POST", url, bytes.NewBuffer(asJSON))
		if err != nil {
			return err
		}
		request.Header.Add("Authorization", "Bearer "+auth)
		request.Header.Set("Content-Type", "application/json")
		client := &http.Client{}

		response, err := client.Do(request)
		if err != nil {
			return err
		}

		status := response.StatusCode
		if status != http.StatusCreated {
			return fmt.Errorf("player API returned with status code %d when creating team. %d teams created before error", status, i)
		}

		// Get the id of the team from the response
		body := make(map[string]interface{})
		err = json.NewDecoder(response.Body).Decode(&body)
		if err != nil {
			return err
		}
		teamID := body["id"].(string)
		(*teams)[i].ID = teamID

		log.Printf("! Team creation response body: %+v", body)
		// Add each user to this team
		for _, user := range team.Users {
			err := addUser(user.ID, teamID, m)
			if err != nil {
				return err
			}
			log.Printf("! User's role: %v", user.Role)
			if user.Role.(string) != "" {
				err = SetUserRole(teamID, viewID, user, m)
				if err != nil {
					return err
				}
			}
		}

		// Add each application to this team
		for i, app := range team.AppInstances {
			id, err := AddApplication(app.Parent, teamID, app.DisplayOrder, m)
			if err != nil {
				return err
			}
			app.ID = id
			team.AppInstances[i] = app
		}
	}
	return nil
}

// UpdateTeams updates the specified teams.
//
// Param teams: the teams to update.
//
// param m map: containing provider config info.
//
// Returns some error on failure or nil on success.
func UpdateTeams(teams *[]*structs.TeamInfo, m map[string]string) error {
	log.Printf("! At top of API wrapper for updating team")
	auth, err := util.GetAuth(m)
	if err != nil {
		return err
	}

	// Update each team
	for i, team := range *teams {
		log.Printf("! Team loop")
		// Set up payload for PUT request
		roleID, err := getTeamRoleByName(team.Role.(string), auth, m)
		if err != nil {
			return err
		}

		asJSON, err := json.Marshal(map[string]interface{}{
			"id":     team.ID,
			"name":   team.Name,
			"roleId": roleID,
		})
		if err != nil {
			return err
		}

		url := util.GetPlayerApiUrl(m) + "teams/" + team.ID.(string)
		log.Printf("! Updating team. URL: %v", url)
		log.Printf("! Updating team. Payload: %+v", team)
		request, err := http.NewRequest("PUT", url, bytes.NewBuffer(asJSON))
		if err != nil {
			return err
		}
		request.Header.Add("Authorization", "Bearer "+auth)
		request.Header.Set("Content-Type", "application/json")
		client := &http.Client{}

		response, err := client.Do(request)
		if err != nil {
			return err
		}

		status := response.StatusCode
		if status != http.StatusOK {
			return fmt.Errorf("player API returned with status code %d when updating team. %d teams updated before error", status, i)
		}
	}
	return nil
}

// DeleteTeams deletes the teams specified.
//
// param ids: the IDs of the teams to delete
//
// param m map: containing provider config info
//
// Returns some error on failure or nil on success
func DeleteTeams(ids *[]string, m map[string]string) error {
	auth, err := util.GetAuth(m)
	if err != nil {
		return err
	}

	for i, id := range *ids {
		url := util.GetPlayerApiUrl(m) + "teams/" + id
		request, err := http.NewRequest("DELETE", url, nil)
		if err != nil {
			return err
		}
		request.Header.Add("Authorization", "Bearer "+auth)
		client := &http.Client{}

		response, err := client.Do(request)
		if err != nil {
			return err
		}

		status := response.StatusCode
		if status != http.StatusNoContent {
			return fmt.Errorf("player API returned with status code %d when deleting team. %d teams deleted before error", status, i)
		}
	}
	return nil
}

// AddPermissionsToTeam adds each team's specified permissions to that team
//
// param teams: A slice of structs representing the teams
//
// param m map: containing provider config info
//
// Returns some error on failure or nil on success
func AddPermissionsToTeam(teams *[]*structs.TeamInfo, m map[string]string) error {
	log.Printf("! At top of API wrapper to add permissions to team")
	auth, err := util.GetAuth(m)
	if err != nil {
		return err
	}

	for _, team := range *teams {
		log.Printf("! Adding permission to team %+v", team)
		for _, perm := range team.Permissions {
			url := util.GetPlayerApiUrl(m) + "teams/" + team.ID.(string) + "/permissions/" + perm
			request, err := http.NewRequest("POST", url, nil)
			if err != nil {
				return err
			}
			request.Header.Add("Authorization", "Bearer "+auth)

			client := &http.Client{}
			response, err := client.Do(request)
			if err != nil {
				return err
			}

			status := response.StatusCode
			if status != http.StatusOK {
				return fmt.Errorf("player API returned with status code %d when adding permission to team", status)
			}

		}
	}

	return nil
}

// UpdateTeamPermissions adds and removes the permissions specified from the teams specified
//
// param toAdd: map corresponding teams with lists of permissions to add
//
// param toRemove: map corresponding teams with lists of permissions to remove
//
// param m map: containing provider config info
//
// Returns some error on failure or nil on success
func UpdateTeamPermissions(toAdd, toRemove map[string][]string, m map[string]string) error {
	log.Printf("! At top of API wrapper to update a team's permissions")
	auth, err := util.GetAuth(m)
	if err != nil {
		return err
	}

	// Add permissions
	for team := range toAdd {
		for _, perm := range toAdd[team] {
			url := util.GetPlayerApiUrl(m) + "teams/" + team + "/permissions/" + perm
			request, err := http.NewRequest("POST", url, nil)
			if err != nil {
				return err
			}

			request.Header.Add("Authorization", "Bearer "+auth)

			client := &http.Client{}
			response, err := client.Do(request)
			if err != nil {
				return err
			}

			status := response.StatusCode
			if status != http.StatusOK {
				return fmt.Errorf("player API returned with status code %d when adding permission to team", status)
			}
		}
	}

	// Remove permissions
	for team := range toRemove {
		for _, perm := range toRemove[team] {
			url := util.GetPlayerApiUrl(m) + "teams/" + team + "/permissions/" + perm
			request, err := http.NewRequest("DELETE", url, nil)
			if err != nil {
				return err
			}

			request.Header.Add("Authorization", "Bearer "+auth)

			client := &http.Client{}
			response, err := client.Do(request)
			if err != nil {
				return err
			}

			status := response.StatusCode
			if status != http.StatusOK {
				return fmt.Errorf("player API returned with status code %d when adding permission to team", status)
			}
		}
	}

	return nil
}

// GetRoleByID returns the name of the role with the given ID
func GetRoleByID(role string, m map[string]string) (string, error) {
	auth, err := util.GetAuth(m)
	if err != nil {
		return "", err
	}

	url := util.GetPlayerApiUrl(m) + "roles/" + role
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	request.Header.Add("Authorization", "Bearer "+auth)

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}

	status := response.StatusCode
	if status != http.StatusOK {
		return "", fmt.Errorf("player API returned with status code %d looking for role %v", status, role)
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(response.Body)
	asStr := buf.String()
	defer response.Body.Close()

	asMap := make(map[string]interface{})
	err = json.Unmarshal([]byte(asStr), &asMap)
	if err != nil {
		return "", err
	}

	return asMap["name"].(string), nil
}

// -------------------- Helper functions --------------------

// Reads information for all teams in a view.
//
// param viewID: the view to look under
//
// Returns a list of teamInfo structs and an error value
func readTeams(viewID string, m map[string]string) (*[]structs.TeamInfo, error) {
	log.Printf("! At top of API wrapper to read teams")

	auth, err := util.GetAuth(m)
	if err != nil {
		return nil, err
	}

	url := util.GetPlayerApiUrl(m) + "views/" + viewID + "/teams"
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Add("Authorization", "Bearer "+auth)

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}

	status := response.StatusCode
	if status != http.StatusOK {
		return nil, fmt.Errorf("player API returned with status code %d when reading teams", status)
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(response.Body)
	asStr := buf.String()
	defer response.Body.Close()

	asMap := new([]map[string]interface{})
	teams := new([]structs.TeamInfo)

	// Unmarshal into a map instead of team struct so we can handle the permissions field
	err = json.Unmarshal([]byte(asStr), asMap)
	if err != nil {
		return nil, err
	}

	log.Printf("! Remote team state as map: %+v", asMap)
	for _, team := range *asMap {
		permissions := new([]string)
		permissionsMaps := team["permissions"].([]interface{})
		for _, perm := range permissionsMaps {
			permMap := perm.(map[string]interface{})
			*permissions = append(*permissions, permMap["id"].(string))
		}

		*teams = append(*teams, structs.TeamInfo{
			ID:          team["id"],
			Name:        team["name"],
			Role:        team["roleName"],
			Permissions: *permissions,
		})
	}

	if err != nil {
		return nil, err
	}

	// Read the users for each team
	for i, team := range *teams {
		id := team.ID.(string)
		users, err := getUsersInTeam(id, viewID, m)
		if err != nil {
			return nil, err
		}
		team.Users = users
		(*teams)[i] = team
	}

	// Read the app instances for each team
	for i, team := range *teams {
		id := team.ID.(string)
		instances, err := getTeamAppInstances(id, m)
		if err != nil {
			return nil, err
		}
		team.AppInstances = *instances
		(*teams)[i] = team
	}

	log.Printf("! Returning from api, team structs are: %+v", teams)
	return teams, nil
}

// Returns the ID of the role with the given name
func getRoleByName(role, auth string, m map[string]string) (string, error) {
	url := util.GetPlayerApiUrl(m) + "roles/name/" + role
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	request.Header.Add("Authorization", "Bearer "+auth)

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}

	status := response.StatusCode
	if status != http.StatusOK {
		return "", fmt.Errorf("player API returned with status code %d looking for role %v", status, role)
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(response.Body)
	asStr := buf.String()
	defer response.Body.Close()

	asMap := make(map[string]interface{})
	err = json.Unmarshal([]byte(asStr), &asMap)
	if err != nil {
		return "", err
	}

	return asMap["id"].(string), nil
}

// Returns the ID of the team role with the given name
func getTeamRoleByName(roleName, auth string, m map[string]string) (string, error) {
	url := util.GetPlayerApiUrl(m) + "team-roles"
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	request.Header.Add("Authorization", "Bearer "+auth)

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}

	status := response.StatusCode
	if status != http.StatusOK {
		return "", fmt.Errorf("player API returned with status code %d looking for role %v", status, roleName)
	}

	var roles []map[string]interface{}
	if err := json.NewDecoder(response.Body).Decode(&roles); err != nil {
		return "", err
	}

	for _, role := range roles {
		if name, ok := role["name"].(string); ok && name == roleName {
			if id, ok := role["id"].(string); ok {
				return id, nil
			}
			return "", fmt.Errorf("role %q found but has no valid id", roleName)
		}
	}

	return "", fmt.Errorf("role %q not found in returned list", roleName)
}
