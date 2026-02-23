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

// CreateVlan allocates a VLAN using the centralized client.
func CreateVlan(ctx context.Context, c *client.CrucibleClient, command *structs.VlanCreateCommand) (*structs.Vlan, error) {
	// Build payload, omitting empty/nil fields
	payload := make(map[string]interface{})

	if command.ProjectId != "" {
		payload["projectId"] = command.ProjectId
	}
	if command.PartitionId != "" {
		payload["partitionId"] = command.PartitionId
	}
	if command.Tag != "" {
		payload["tag"] = command.Tag
	}
	if command.VlanId.Valid {
		payload["vlanId"] = command.VlanId.Int32
	}

	url := c.GetCasterAPIURL() + "vlans/actions/acquire/"
	vlan := new(structs.Vlan)

	if err := c.DoPost(ctx, url, payload, vlan); err != nil {
		return nil, fmt.Errorf("failed to allocate VLAN: %w", err)
	}

	return vlan, nil
}

// ReadVlan reads a VLAN by ID using the centralized client.
func ReadVlan(ctx context.Context, c *client.CrucibleClient, id string) (*structs.Vlan, error) {
	url := c.GetCasterAPIURL() + "vlans/" + id
	vlan := new(structs.Vlan)

	if err := c.DoGet(ctx, url, vlan); err != nil {
		return nil, fmt.Errorf("failed to read VLAN %s: %w", id, err)
	}

	return vlan, nil
}

// DeleteVlan releases a VLAN back to the pool using the centralized client.
func DeleteVlan(ctx context.Context, c *client.CrucibleClient, id string) error {
	url := c.GetCasterAPIURL() + "vlans/" + id + "/actions/release"

	// Use DoPost since the release endpoint is a POST, but we don't need the response
	if err := c.DoPost(ctx, url, nil, nil); err != nil {
		return fmt.Errorf("failed to release VLAN %s: %w", id, err)
	}

	return nil
}

// -------------------- API Wrappers (SDK v1 - legacy) --------------------

// CreateVlan wraps the acquire vlan POST call in caster API
//
// param command: A struct containing info on acquiring a vlan
//
// param m: A map containing configuration info for the provider
//
// Returns the ID of the view and error on failure or nil on success
func CreateVlan(command *structs.VlanCreateCommand, m map[string]string) (*structs.Vlan, error) {
	log.Printf("! At top of API wrapper to create vlan")

	auth, err := util.GetAuth(m)
	if err != nil {
		return nil, err
	}

	// Remove unset fields from payload
	payload := map[string]interface{}{
		"projectId":   util.Ternary(command.ProjectId == "", nil, command.ProjectId),
		"partitionId": util.Ternary(command.PartitionId == "", nil, command.PartitionId),
		"tag":         util.Ternary(command.Tag == "", nil, command.Tag),
		"vlanId":      util.Ternary(!command.VlanId.Valid, nil, command.VlanId.Int32),
	}

	log.Printf("! Creating vlan with payload %+v", payload)

	asJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequest("POST", util.GetCasterApiUrl(m)+"vlans/actions/acquire/", bytes.NewBuffer(asJSON))
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

	status := response.StatusCode
	if status != http.StatusOK {
		return nil, fmt.Errorf("Caster API returned with status code %d when creating vlan", status)
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(response.Body)
	asStr := buf.String()
	defer response.Body.Close()

	vlan := &structs.Vlan{}
	err = json.Unmarshal([]byte(asStr), vlan)

	if err != nil {
		return nil, err
	}

	return vlan, nil
}

// ReadVlan wraps the caster API call to read the fields of a vlan
//
// Param id: the id of the vlan to read
//
// param m: A map containing configuration info for the provider
//
// Returns error on failure or the vlan on success
func ReadVlan(id string, m map[string]string) (*structs.Vlan, error) {
	auth, err := util.GetAuth(m)
	if err != nil {
		return nil, err
	}

	url := util.GetCasterApiUrl(m) + "vlans/" + id
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
		return nil, fmt.Errorf("Caster API returned with status code %d when reading vlan", status)
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(response.Body)
	asStr := buf.String()
	defer response.Body.Close()

	vlan := &structs.Vlan{}

	err = json.Unmarshal([]byte(asStr), vlan)
	if err != nil {
		log.Printf("! Error unmarshaling in read vlan")
		return nil, err
	}

	return vlan, nil
}

// DeleteVlan wraps the caster API release vlan call
//
// Param id: The id of the vlan to release back into the pool
//
// param m: A map containing configuration info for the provider
//
// Returns error on failure or nil on success
func DeleteVlan(id string, m map[string]string) error {
	auth, err := util.GetAuth(m)
	if err != nil {
		return err
	}

	url := util.GetCasterApiUrl(m) + "vlans/" + id + "/actions/release"
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
		return fmt.Errorf("Caster API returned with status code %d when deleting vlan", status)
	}
	return nil
}
