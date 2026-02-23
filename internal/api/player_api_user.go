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

// ---------------------- Public functions (SDK v1 - legacy) ----------------------

// RemoveUsers removes the specified users from the specified teams
//
// param teamsToUsers: Maps each team to the users that should be removed from it
//
// param m: A map containing configuration info for the provider
//
// Returns some error on failure or nil on success
func RemoveUsers(teamsToUsers map[string][]string, m map[string]string) error {
	auth, err := util.GetAuth(m)
	if err != nil {
		return err
	}

	for team := range teamsToUsers {
		for _, user := range teamsToUsers[team] {
			url := util.GetPlayerApiUrl(m) + "teams/" + team + "/users/" + user
			request, err := http.NewRequest("DELETE", url, nil)
			request.Header.Add("Authorization", "Bearer "+auth)
			client := &http.Client{}

			response, err := client.Do(request)
			if err != nil {
				return err
			}

			status := response.StatusCode
			if status != http.StatusOK {
				return fmt.Errorf("player API returned with status code %d when removing user from team", status)
			}
		}
	}
	return nil
}

// AddUsersToTeam adds the specified users to the specified team
//
// param users: The IDs of the users to add
//
// param team: The ID of the team to add the users to
//
// param m: A map containing configuration info for the provider
//
// Returns some error on failure or nil on success
func AddUsersToTeam(users *[]string, team string, m map[string]string) error {
	auth, err := util.GetAuth(m)
	if err != nil {
		return err
	}

	for _, user := range *users {
		url := util.GetPlayerApiUrl(m) + "teams/" + team + "/users/" + user
		request, err := http.NewRequest("POST", url, nil)
		request.Header.Add("Authorization", "Bearer "+auth)
		client := http.Client{}

		response, err := client.Do(request)
		if err != nil {
			return err
		}

		status := response.StatusCode
		if status != http.StatusOK {
			return fmt.Errorf("player API returned with status code %d when getting users from team", status)
		}
	}

	return nil
}

// SetUserRole sets this user's role within their team
//
// param teamID: The team to set a user's role within
//
// param viewID: The view in which the team where the role is being set lives
//
// user: The user to set a role for
//
// param m: A map containing configuration info for the provider
//
// Returns some error on failure or nil on success
func SetUserRole(teamID, viewID string, user structs.UserInfo, m map[string]string) error {
	auth, err := util.GetAuth(m)
	if err != nil {
		return err
	}

	// Find the ID of the relevant TeamMembership
	url := util.GetPlayerApiUrl(m) + "users/" + user.ID + "/views/" + viewID + "/team-memberships"
	id, err := findMembershipID(url, teamID, auth)
	if err != nil {
		return err
	}

	// Look up the role by name
	role, err := getRoleByName(user.Role.(string), auth, m)
	if err != nil {
		return err
	}

	// Set the role
	payload, err := json.Marshal(map[string]interface{}{
		"roleId": role,
	})
	if err != nil {
		return err
	}

	url = util.GetPlayerApiUrl(m) + "team-memberships/" + id
	request, err := http.NewRequest("PUT", url, bytes.NewBuffer(payload))
	request.Header.Add("Authorization", "Bearer "+auth)
	request.Header.Set("Content-Type", "application/json")
	client := http.Client{}

	response, err := client.Do(request)
	if err != nil {
		return err
	}

	status := response.StatusCode
	if status != http.StatusOK {
		return fmt.Errorf("player API returned with status code %d when setting user role", status)
	}

	return nil
}

// CreateUser creates a new player user. Called whenever a new identity account is created.
//
// param user a struct representing the user to create
//
// param name the name of the user
//
// param m: A map containing configuration info for the provider
//
// returns nil on success or some error on failure
func CreateUser(user structs.PlayerUser, m map[string]string) error {
	auth, err := util.GetAuth(m)
	if err != nil {
		return err
	}

	// If a role was set, find its ID. Otherwise set role field to nil
	var roleID interface{} = nil
	if user.Role != "" {
		role, err := getRoleByName(user.Role.(string), auth, m)
		if err != nil {
			return err
		}
		roleID = role
	}
	user.Role = roleID

	payload, err := json.Marshal(user)
	if err != nil {
		return err
	}

	url := util.GetPlayerApiUrl(m) + "users"
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(payload))
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

	if response.StatusCode != http.StatusCreated {
		return fmt.Errorf("Error creating user in Player. API returned status code %d", response.StatusCode)
	}

	return nil
}

// ReadUser returns a struct representing a given user
//
// param id: The ID of the user to consider
//
// param m: A map containing configuration info for the provider
//
// Returns the user struct and an optional error value
func ReadUser(id string, m map[string]string) (*structs.PlayerUser, error) {
	response, err := getUserByID(id, m)
	if err != nil {
		return nil, err
	}

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Error deleting user in Player. API returned status code %d", response.StatusCode)
	}

	// Read response body into struct
	user := new(structs.PlayerUser)
	buf := new(bytes.Buffer)
	buf.ReadFrom(response.Body)
	asStr := buf.String()
	defer response.Body.Close()

	err = json.Unmarshal([]byte(asStr), user)
	if err != nil {
		return nil, err
	}

	return user, nil
}

// UserExists returns true if a user exists and false otherwise
//
// param id: The ID of the user to consider
//
// param m: A map containing configuration info for the provider
//
// Returns whether the user exists and an optional error value
func UserExists(id string, m map[string]string) (bool, error) {
	resp, err := getUserByID(id, m)
	if err != nil {
		return false, nil
	}

	return resp.StatusCode != 404, nil
}

// UpdateUser updates a user in Player.
//
// param user a struct representing the user to update
//
// param name the name of the user
//
// param m: A map containing configuration info for the provider
//
// returns nil on success or some error on failure
func UpdateUser(user structs.PlayerUser, m map[string]string) error {
	auth, err := util.GetAuth(m)
	if err != nil {
		return err
	}

	// If a role was set, find its ID. Otherwise set role field to nil
	var roleID interface{} = nil
	if user.Role.(string) != "" {
		role, err := getRoleByName(user.Role.(string), auth, m)
		if err != nil {
			return err
		}
		roleID = role
	}
	user.Role = roleID

	payload, err := json.Marshal(user)
	if err != nil {
		return err
	}

	url := util.GetPlayerApiUrl(m) + "users/" + user.ID
	request, err := http.NewRequest(http.MethodPut, url, bytes.NewBuffer(payload))
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

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("Error updating user in Player. API returned status code %d", response.StatusCode)
	}

	return nil
}

// DeleteUser deletes the user with the given id.
//
// param id: The ID of the user to delete
//
// param m: A map containing configuration info for the provider
//
// returns nil on success or some error on failure
func DeleteUser(id string, m map[string]string) error {
	auth, err := util.GetAuth(m)
	if err != nil {
		return err
	}

	url := util.GetPlayerApiUrl(m) + "users/" + id
	request, err := http.NewRequest(http.MethodDelete, url, nil)
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

	if response.StatusCode != http.StatusNoContent {
		return fmt.Errorf("Error deleting user in Player. API returned status code %d", response.StatusCode)
	}

	return nil
}

// ---------------------- Private functions ----------------------

// adds the specified user to the specified team
//
// param users: A slice of UserInfo struct pointers representing the users to be created
//
// param m: A map containing configuration info for the provider
//
// Returns some error on failure or nil on success
func addUser(userID, teamID string, m map[string]string) error {
	auth, err := util.GetAuth(m)
	if err != nil {
		return err
	}

	// Add the user to their team
	url := util.GetPlayerApiUrl(m) + "teams/" + teamID + "/users/" + userID
	request, err := http.NewRequest("POST", url, nil)
	request.Header.Add("Authorization", "Bearer "+auth)
	client := &http.Client{}

	response, err := client.Do(request)
	if err != nil {
		return err
	}

	status := response.StatusCode
	if status != http.StatusOK {
		return fmt.Errorf("player API returned with status code %d when adding user to team", status)
	}

	return nil
}

// Find the ID of the relevant TeamMembership
func findMembershipID(url, teamID, auth string) (string, error) {
	request, err := http.NewRequest("GET", url, nil)
	request.Header.Add("Authorization", "Bearer "+auth)
	client := http.Client{}

	response, err := client.Do(request)
	if err != nil {
		return "", err
	}

	status := response.StatusCode
	if status != http.StatusOK {
		return "", fmt.Errorf("player API returned with status code %d when looking for teamMembership id", status)
	}

	// Unmarshal response into map slice to look for ID
	asMap := new([]map[string]interface{})
	buf := new(bytes.Buffer)
	buf.ReadFrom(response.Body)
	asStr := buf.String()
	defer response.Body.Close()

	err = json.Unmarshal([]byte(asStr), asMap)
	if err != nil {
		return "", err
	}

	// Look for ID of the relevant membership
	for _, membership := range *asMap {
		if membership["teamId"] == teamID {
			return membership["id"].(string), nil
		}
	}

	return "", fmt.Errorf("no membership found for the given user and view")
}

// Returns the teamMembership with the given id
func getMembership(id, auth string, m map[string]string) (string, error) {
	url := util.GetPlayerApiUrl(m) + "team-memberships/" + id

	request, err := http.NewRequest("GET", url, nil)
	request.Header.Add("Authorization", "Bearer "+auth)
	client := http.Client{}

	response, err := client.Do(request)
	if err != nil {
		return "", err
	}

	status := response.StatusCode
	if status != http.StatusOK {
		return "", fmt.Errorf("player API returned with status code %d when getting TeamMembership", status)
	}

	asMap := make(map[string]interface{})
	buf := new(bytes.Buffer)
	buf.ReadFrom(response.Body)
	asStr := buf.String()
	defer response.Body.Close()

	err = json.Unmarshal([]byte(asStr), &asMap)
	if err != nil {
		return "", err
	}

	role := asMap["roleName"]
	return util.Ternary(role == nil, "", role).(string), nil
}

// Returns all users in the given team
func getUsersInTeam(teamID, viewID string, m map[string]string) ([]structs.UserInfo, error) {
	auth, err := util.GetAuth(m)
	if err != nil {
		return nil, err
	}

	url := util.GetPlayerApiUrl(m) + "teams/" + teamID + "/users"
	request, err := http.NewRequest("GET", url, nil)
	request.Header.Add("Authorization", "Bearer "+auth)
	client := http.Client{}

	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}

	status := response.StatusCode
	if status != http.StatusOK {
		return nil, fmt.Errorf("player API returned with status code %d when getting users from team", status)
	}

	// Read the response body
	buf := new(bytes.Buffer)
	buf.ReadFrom(response.Body)
	asStr := buf.String()
	defer response.Body.Close()

	users := new([]map[string]interface{})
	err = json.Unmarshal([]byte(asStr), users)
	if err != nil {
		return nil, err
	}

	userStructs := new([]structs.UserInfo)

	for _, user := range *users {
		userID := user["id"].(string)
		// Get team membership by id, assign it to RoleID field
		url = util.GetPlayerApiUrl(m) + "users/" + userID + "/views/" + viewID + "/team-memberships"
		id, err := findMembershipID(url, teamID, auth)
		if err != nil {
			return nil, err
		}
		role, err := getMembership(id, auth, m)
		if err != nil {
			return nil, err
		}
		log.Printf("! Role of current user: %s", role)

		curr := &structs.UserInfo{
			ID:   userID,
			Role: role,
		}
		*userStructs = append(*userStructs, *curr)
	}

	return *userStructs, nil
}

func getUserByID(id string, m map[string]string) (*http.Response, error) {
	auth, err := util.GetAuth(m)
	if err != nil {
		return nil, err
	}

	url := util.GetPlayerApiUrl(m) + "users/" + id
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Add("Authorization", "Bearer "+auth)
	request.Header.Set("Content-Type", "application/json")
	client := &http.Client{}

	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}

	return response, nil
}
