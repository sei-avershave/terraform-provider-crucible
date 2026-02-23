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
