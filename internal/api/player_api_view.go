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

// -------------------- API Wrappers (SDK v1 - legacy) --------------------

// CreateView wraps the create view POST call in player API
//
// param view: A struct containing info on the view to be created
//
// param m: A map containing configuration info for the provider
//
// Returns the ID of the view and error on failure or nil on success
func CreateView(view *structs.ViewInfo, m map[string]string) (string, error) {
	log.Printf("! At top of API wrapper to create view")

	auth, err := util.GetAuth(m)
	if err != nil {
		return "", err
	}

	// Remove unset fields from payload
	payload := map[string]interface{}{
		"name":            view.Name,
		"description":     util.Ternary(view.Description == "", nil, view.Description),
		"status":          util.Ternary(view.Status == "", "Active", view.Status),
		"createAdminTeam": view.CreateAdminTeam,
	}

	log.Printf("! Creating view with payload %+v", payload)

	asJSON, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	request, err := http.NewRequest("POST", util.GetPlayerApiUrl(m)+"views", bytes.NewBuffer(asJSON))
	if err != nil {
		return "", err
	}
	request.Header.Add("Authorization", "Bearer "+auth)
	request.Header.Set("Content-Type", "application/json")
	client := &http.Client{}

	response, err := client.Do(request)
	if err != nil {
		return "", err
	}

	status := response.StatusCode
	if status != http.StatusCreated {
		return "", fmt.Errorf("Player API returned with status code %d when creating view", status)
	}

	// Get the id of the view from the response
	body := make(map[string]interface{})
	err = json.NewDecoder(response.Body).Decode(&body)
	defer response.Body.Close()

	if err != nil {
		return "", err
	}

	return body["id"].(string), nil
}

// ReadView wraps the player API call to read the fields of a view
//
// Param id: the id of the view to read
//
// param m: A map containing configuration info for the provider
//
// Returns error on failure or nil on success
func ReadView(id string, m map[string]string) (*structs.ViewInfo, error) {
	response, err := getViewByID(id, m)
	if err != nil {
		return nil, err
	}

	status := response.StatusCode
	if status != http.StatusOK {
		return nil, fmt.Errorf("Player API returned with status code %d when reading view", status)
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(response.Body)
	asStr := buf.String()
	defer response.Body.Close()

	view := &structs.ViewInfo{}

	err = json.Unmarshal([]byte(asStr), view)
	if err != nil {
		log.Printf("! Error unmarshaling in read view")
		return nil, err
	}

	apps, err := readApps(id, m)
	if err != nil {
		return nil, err
	}
	teams, err := readTeams(id, m)
	if err != nil {
		return nil, err
	}

	view.Applications = *apps
	view.Teams = *teams
	return view, nil
}

// UpdateView wraps the update view player API call
//
// param view: A struct containing info on the view to be created
//
// param m: A map containing configuration info for the provider
//
// param id: The id of the view to update
//
// Returns error on failure or nil on success
func UpdateView(view *structs.ViewInfo, m map[string]string, id string) error {
	log.Printf("! At top of API wrapper to update view")

	auth, err := util.GetAuth(m)
	if err != nil {
		return err
	}

	// This API call requires the ID of the view to be supplied
	asMap := view.ToMap()
	asMap["id"] = id

	asJSON, err := json.Marshal(asMap)
	if err != nil {
		return err
	}

	url := util.GetPlayerApiUrl(m) + "views/" + id
	log.Printf("! url: %v", url)
	request, err := http.NewRequest("PUT", url, bytes.NewBuffer(asJSON))
	if err != nil {
		return err
	}
	request.Header.Add("Authorization", "Bearer "+auth)
	request.Header.Set("Content-Type", "application/json")
	client := &http.Client{}

	log.Printf("! View before update api call %+v", asMap)
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	log.Printf("! Response: %+v", response)

	status := response.StatusCode
	if status != http.StatusOK {
		return fmt.Errorf("Player API returned with status code %d when updating view", status)
	}

	return nil
}

// DeleteView wraps the player API delete view call
//
// Param id: The id of the view to delete
//
// param m: A map containing configuration info for the provider
//
// Returns error on failure or nil on success
func DeleteView(id string, m map[string]string) error {
	auth, err := util.GetAuth(m)
	if err != nil {
		return err
	}

	url := util.GetPlayerApiUrl(m) + "views/" + id
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
		return fmt.Errorf("Player API returned with status code %d when deleting view", status)
	}
	return nil
}

// ViewExists returns true if a view with a given id exists
//
// param id: The ID of the view under consideration
//
// param m: A map containing configuration info for the provider
func ViewExists(id string, m map[string]string) (bool, error) {
	response, err := getViewByID(id, m)
	if err != nil {
		return false, err
	}
	return (response.StatusCode != http.StatusNotFound), nil
}

// -------------------- Helper functions --------------------

func getViewByID(id string, m map[string]string) (*http.Response, error) {
	auth, err := util.GetAuth(m)
	if err != nil {
		return nil, err
	}

	url := util.GetPlayerApiUrl(m) + "views/" + id
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

	return response, nil
}
