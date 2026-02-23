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

// --------------------- Public functions (SDK v1 - legacy) ---------------------

// CreateAppTemplate creates an application template with the specified fields.
//
// Param template: Struct representing the app template to create
//
// param m: A map containing configuration info for the provider
//
// Returns the ID of the template and an error value
func CreateAppTemplate(template *structs.AppTemplate, m map[string]string) (string, error) {
	auth, err := util.GetAuth(m)
	if err != nil {
		return "", err
	}

	// Need to ignore unset string fields in http request
	payload := map[string]interface{}{
		"name":             template.Name,
		"url":              util.Ternary(template.URL == "", nil, template.URL),
		"icon":             util.Ternary(template.Icon == "", nil, template.Icon),
		"embeddable":       template.Embeddable,
		"loadInBackground": template.LoadInBackground,
	}

	asJSON, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	log.Printf("! Creating template with payload %+v", payload)
	// Create the template
	url := util.GetPlayerApiUrl(m) + "application-templates"
	request, err := http.NewRequest("POST", url, bytes.NewBuffer(asJSON))
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
		return "", fmt.Errorf("Player API returned with status code %d when creating template", status)
	}

	// Read the ID field from the response
	body := make(map[string]interface{})
	err = json.NewDecoder(response.Body).Decode(&body)
	defer response.Body.Close()

	if err != nil {
		return "", err
	}

	return body["id"].(string), nil
}

// AppTemplateRead returns an AppTemplate struct representing the remote state of the specified application template
//
// Param id: The id of the template to read
//
// param m: A map containing configuration info for the provider
//
// Returns the struct representing the template and an error value
func AppTemplateRead(id string, m map[string]string) (*structs.AppTemplate, error) {
	response, err := getAppTemplateByID(id, m)
	if err != nil {
		return nil, err
	}

	status := response.StatusCode
	if status != http.StatusOK {
		return nil, fmt.Errorf("Player API returned with status code %d when reading template", status)
	}

	// Read the response body into a struct
	buf := new(bytes.Buffer)
	buf.ReadFrom(response.Body)
	asStr := buf.String()
	defer response.Body.Close()

	template := &structs.AppTemplate{}

	err = json.Unmarshal([]byte(asStr), template)
	if err != nil {
		return nil, err
	}

	return template, nil
}

// AppTemplateUpdate updates the specified application template with the specified values
//
// Param id: The ID of the template to update
//
// Param template: A struct representing the updated template
//
// param m: A map containing configuration info for the provider
//
// Returns nil on success or some error on failure
func AppTemplateUpdate(id string, template *structs.AppTemplate, m map[string]string) error {
	auth, err := util.GetAuth(m)
	if err != nil {
		return err
	}

	asJSON, err := json.Marshal(template)
	if err != nil {
		return err
	}

	// Update the template
	url := util.GetPlayerApiUrl(m) + "application-templates/" + id
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
		return fmt.Errorf("Player API returned with status code %d when updating template", status)
	}

	return nil
}

// DeleteAppTemplate deletes the specified app template
//
// Param id: The id of the template to delete
//
// param m: A map containing configuration info for the provider
//
// Returns nil on success or some error on failure
func DeleteAppTemplate(id string, m map[string]string) error {
	auth, err := util.GetAuth(m)
	if err != nil {
		return err
	}

	url := util.GetPlayerApiUrl(m) + "application-templates/" + id
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
		return fmt.Errorf("Player API returned with status code %d when reading template", status)
	}

	return nil
}

// AppTemplateExists returns whether a template exists along with an error value
func AppTemplateExists(id string, m map[string]string) (bool, error) {
	resp, err := getAppTemplateByID(id, m)
	if err != nil {
		return false, err
	}

	return (resp.StatusCode != http.StatusNotFound), nil
}

// --------------------- Private helper functions ---------------------

// Gets an app template by its ID and returns the HTTP response
func getAppTemplateByID(id string, m map[string]string) (*http.Response, error) {
	auth, err := util.GetAuth(m)
	if err != nil {
		return nil, err
	}

	url := util.GetPlayerApiUrl(m) + "application-templates/" + id
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
