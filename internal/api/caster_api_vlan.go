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
