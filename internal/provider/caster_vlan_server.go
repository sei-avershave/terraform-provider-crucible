// Copyright 2022 Carnegie Mellon University. All Rights Reserved.
// Released under a MIT (SEI)-style license. See LICENSE.md in the project root for license information.

package provider

import (
	"crucible_provider/internal/api"
	"crucible_provider/internal/structs"
	"database/sql"
	"fmt"
	"log"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

func casterVlan() *schema.Resource {
	return &schema.Resource{
		Create: casterVlanCreate,
		Read:   casterVlanRead,
		Delete: casterVlanDelete,

		Schema: map[string]*schema.Schema{
			"partition_id": {
				Type:          schema.TypeString,
				Optional:      true,
				ConflictsWith: []string{"project_id"},
				ForceNew:      true,
				Computed:      true,
			},
			"pool_id": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"project_id": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"tag": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
				Computed: true,
			},
			"vlan_id": {
				Type:     schema.TypeInt,
				Optional: true,
				Computed: true,
				ForceNew: true,
			},
		},
	}
}

// Get view properties from d
// Call API to create view
// If no error, set local state
// Call read to make sure everything worked
func casterVlanCreate(d *schema.ResourceData, m interface{}) error {
	if m == nil {
		return fmt.Errorf("error configuring provider")
	}

	vlanCreateCommand := &structs.VlanCreateCommand{
		ProjectId:   d.Get("project_id").(string),
		PartitionId: d.Get("partition_id").(string),
		Tag:         d.Get("tag").(string),
	}

	vlanId, vlanIdExists := d.GetOk("vlan_id")

	if vlanIdExists {
		vlanCreateCommand.VlanId = sql.NullInt32{Int32: int32(vlanId.(int)), Valid: true}
	}

	casted := m.(map[string]string)
	vlan, err := api.CreateVlan(vlanCreateCommand, casted)
	if err != nil {
		return err
	}

	d.SetId(vlan.Id)

	// Set local state
	err = d.Set("vlan_id", vlan.VlanId)
	if err != nil {
		return err
	}

	err = d.Set("pool_id", vlan.PoolId)
	if err != nil {
		return err
	}

	err = d.Set("partition_id", vlan.PartitionId)
	if err != nil {
		return err
	}

	err = d.Set("tag", vlan.Tag)
	if err != nil {
		return err
	}

	log.Printf("! Vlan created with ID %s", d.Id())
	return nil
}

// Check if vlan exists. If not, set id to "" and return nil
// Read vlan info from API
// Use it to update local state
func casterVlanRead(d *schema.ResourceData, m interface{}) error {
	id := d.Id()
	casted := m.(map[string]string)

	// Call API to read state of the vlan
	vlan, err := api.ReadVlan(id, casted)
	if err != nil {
		return err
	}

	if !vlan.InUse {
		d.SetId("")
		return nil
	}

	// Set local state
	err = d.Set("vlan_id", vlan.VlanId)
	if err != nil {
		return err
	}

	err = d.Set("pool_id", vlan.PoolId)
	if err != nil {
		return err
	}

	err = d.Set("partition_id", vlan.PartitionId)
	if err != nil {
		return err
	}

	err = d.Set("tag", vlan.Tag)
	if err != nil {
		return err
	}

	return nil
}

// Delete vlan
// Call API release function. Return nil on success or some error on failure
func casterVlanDelete(d *schema.ResourceData, m interface{}) error {
	if m == nil {
		return fmt.Errorf("error configuring provider")
	}

	id := d.Id()
	casted := m.(map[string]string)

	return api.DeleteVlan(id, casted)
}
