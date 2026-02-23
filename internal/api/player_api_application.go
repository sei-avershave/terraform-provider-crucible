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

// CreateApps creates applications in a view using the centralized client.
func CreateApps(ctx context.Context, c *client.CrucibleClient, apps *[]structs.AppInfo, viewID string) error {
	for i := range *apps {
		app := &(*apps)[i]
		app.ViewID = viewID

		url := c.GetPlayerAPIURL() + "views/" + viewID + "/applications"
		var result map[string]interface{}

		if err := c.DoPost(ctx, url, app, &result); err != nil {
			return fmt.Errorf("failed to create application '%v': %w", app.Name, err)
		}

		// Store the application ID
		if id, ok := result["id"].(string); ok {
			app.ID = id
		}
	}

	return nil
}

// -------------------- API Wrappers (SDK v1 - legacy) --------------------

// CreateApps creates an application for each of the structs passed.
//
// param apps: a list of structs representing the applications to create
//
// param m: A map containing configuration info for the provider
//
// param viewID: The view to create this app under
//
// Returns some error on failure or nil on success
func CreateApps(apps *[]*structs.AppInfo, m map[string]string, viewID string) error {
	auth, err := util.GetAuth(m)
	if err != nil {
		return err
	}

	// Create a new application for each struct
	for i, app := range *apps {
		asJSON, err := json.Marshal(app)
		if err != nil {
			return err
		}

		url := util.GetPlayerApiUrl(m) + "views/" + viewID + "/applications"
		log.Printf("! creating app. url: %v", url)
		log.Printf("! Payload: %+v", app)
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
		log.Printf("! Response: %v", response)

		status := response.StatusCode
		if status != http.StatusCreated {
			return fmt.Errorf("player API returned with status code %d when creating app. %d apps created before error", status, i)
		}

		// Read and parse response body
		var createdApp struct {
			Id string `json:"id"`
		}
		if err := json.NewDecoder(response.Body).Decode(&createdApp); err != nil {
			return fmt.Errorf("failed to decode response JSON: %w", err)
		}

		// Assign returned ID to app
		app.ID = createdApp.Id
	}
	return nil
}

// UpdateApps updates the applications specified
//
// param apps: a list of structs.AppInfo structs to be updated
//
// param m: A map containing configuration info for the provider
//
// Returns some error on failure or nil on success
func UpdateApps(apps *[]*structs.AppInfo, m map[string]string) error {
	auth, err := util.GetAuth(m)
	if err != nil {
		return err
	}

	// Update each application
	for i, app := range *apps {
		asJSON, err := json.Marshal(app)
		if err != nil {
			return err
		}

		url := util.GetPlayerApiUrl(m) + "applications/" + app.ID
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
			return fmt.Errorf("player API returned with status code %d when updating app. %d apps updated before error", status, i)
		}
	}
	return nil
}

// DeleteApps deletes the applications specified in ids
//
// Returns nil on success or some error on failure
func DeleteApps(ids *[]string, m map[string]string) error {
	auth, err := util.GetAuth(m)
	if err != nil {
		return err
	}

	for i, id := range *ids {
		url := util.GetPlayerApiUrl(m) + "applications/" + id
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
			return fmt.Errorf("player API returned with status code %d when deleting app. %d apps deleted before error", status, i)
		}
	}
	return nil

}

// UpdateAppInstance updates an application instance with new information
//
// param inst: A struct representing the instance to update
//
// param teamID: The ID of the team this instance lives in
//
// param m: A map containing configuration info for the provider
//
// Returns some error on failure or nil on success
func UpdateAppInstance(inst structs.AppInstance, teamID string, m map[string]string) error {
	log.Printf("! In update app instance")

	auth, err := util.GetAuth(m)
	if err != nil {
		return err
	}

	payload := make(map[string]interface{})
	payload["id"] = inst.ID
	payload["teamId"] = teamID
	payload["displayOrder"] = inst.DisplayOrder
	payload["applicationId"] = inst.Parent

	log.Printf("! Updating app instance with payload %+v", payload)

	asJSON, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	url := util.GetPlayerApiUrl(m) + "application-instances/" + inst.ID
	request, err := http.NewRequest("PUT", url, bytes.NewBuffer(asJSON))
	if err != nil {
		return err
	}
	request.Header.Add("Authorization", "Bearer "+auth)
	request.Header.Set("Content-Type", "application/json")
	client := &http.Client{}

	log.Printf("! Request: %+v", request)

	response, err := client.Do(request)
	if err != nil {
		return err
	}

	log.Printf("! Response: %+v", response)

	status := response.StatusCode
	if status != http.StatusOK {
		return fmt.Errorf("player API returned with status code %d when updating app instance", status)
	}

	return nil
}

// AddApplication Adds an application to a team.
//
// param appID: The ID of the application to add
//
// param displayOrder: The displayOrder field to set on this application instance
//
// param teamID: The ID of the team to add to
//
// param m: A map containing configuration info for the provider
//
// returns the ID of the app instance and an error value
func AddApplication(appID, teamID string, displayOrder float64, m map[string]string) (string, error) {
	auth, err := util.GetAuth(m)
	if err != nil {
		return "", err
	}

	payload := make(map[string]interface{})
	payload["teamId"] = teamID
	payload["applicationId"] = appID
	payload["displayOrder"] = displayOrder

	asJSON, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	url := util.GetPlayerApiUrl(m) + "teams/" + teamID + "/application-instances"
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
		return "", fmt.Errorf("Player API returned with status %d when adding application to team", status)
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(response.Body)
	asStr := buf.String()
	defer response.Body.Close()

	body := make(map[string]interface{})

	err = json.Unmarshal([]byte(asStr), &body)
	if err != nil {
		return "", err
	}

	return body["id"].(string), nil
}

// DeleteAppInstances deletes all of the specified application instances
//
// param toDelete: The IDs of the app instances to delete
//
// param m: A map containing configuration info for the provider
//
// Returns some error on failure or nil on success
func DeleteAppInstances(toDelete *[]string, m map[string]string) error {
	log.Printf("! In DeleteAppInstances")

	auth, err := util.GetAuth(m)
	if err != nil {
		return err
	}

	for i, id := range *toDelete {
		url := util.GetPlayerApiUrl(m) + "application-instances/" + id
		request, err := http.NewRequest("DELETE", url, nil)
		if err != nil {
			return err
		}
		request.Header.Add("Authorization", "Bearer "+auth)
		client := &http.Client{}

		log.Printf("! Request: %+v", request)

		response, err := client.Do(request)
		if err != nil {
			return err
		}

		log.Printf("! Response: %+v", response)

		status := response.StatusCode
		if status != http.StatusNoContent {
			return fmt.Errorf("player API returned with status code %d when deleting app instance. %d instances deleted before error", status, i)
		}
	}
	return nil
}

// -------------------- Helper functions --------------------

// Reads the Applications for a given view.
//
// param id: the if of the view to consider
//
// Returns a: list of structs.AppInfo structs and an error value which is nil on success and some value on failure
func readApps(id string, m map[string]string) (*[]structs.AppInfo, error) {
	auth, err := util.GetAuth(m)
	if err != nil {
		return nil, err
	}

	url := util.GetPlayerApiUrl(m) + "views/" + id + "/applications"
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
		return nil, fmt.Errorf("Player API returned with status %d when retreiving application info", status)
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(response.Body)
	asStr := buf.String()
	defer response.Body.Close()

	apps := new([]structs.AppInfo)

	err = json.Unmarshal([]byte(asStr), apps)
	if err != nil {
		return nil, err
	}

	return apps, nil
}

// Returns all the application instances for a given team
func getTeamAppInstances(teamID string, m map[string]string) (*[]structs.AppInstance, error) {
	auth, err := util.GetAuth(m)
	if err != nil {
		return nil, err
	}

	url := util.GetPlayerApiUrl(m) + "teams/" + teamID + "/application-instances"
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
		return nil, fmt.Errorf("Player API returned with status %d when retreiving application info", status)
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(response.Body)
	asStr := buf.String()
	defer response.Body.Close()

	instances := new([]structs.AppInstance)

	err = json.Unmarshal([]byte(asStr), instances)
	if err != nil {
		return nil, err
	}

	return instances, nil
}
