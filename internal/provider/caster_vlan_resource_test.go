// Copyright 2024 Carnegie Mellon University. All Rights Reserved.
// Released under a MIT (SEI)-style license. See LICENSE.md in the project root for license information.

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccVlanResource_WithPartition(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccVlanResourceConfigPartition("550e8400-e29b-41d4-a716-446655440003", "test-tag"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("crucible_vlan.test", "partition_id", "550e8400-e29b-41d4-a716-446655440003"),
					resource.TestCheckResourceAttr("crucible_vlan.test", "tag", "test-tag"),
					resource.TestCheckResourceAttrSet("crucible_vlan.test", "id"),
					resource.TestCheckResourceAttrSet("crucible_vlan.test", "vlan_id"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "crucible_vlan.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccVlanResource_WithProject(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccVlanResourceConfigProject("550e8400-e29b-41d4-a716-446655440004", "project-tag"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("crucible_vlan.test", "project_id", "550e8400-e29b-41d4-a716-446655440004"),
					resource.TestCheckResourceAttr("crucible_vlan.test", "tag", "project-tag"),
					resource.TestCheckResourceAttrSet("crucible_vlan.test", "id"),
				),
			},
		},
	})
}

func TestAccVlanResource_WithPool(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccVlanResourceConfigPool("550e8400-e29b-41d4-a716-446655440005"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("crucible_vlan.test", "pool_id", "550e8400-e29b-41d4-a716-446655440005"),
					resource.TestCheckResourceAttrSet("crucible_vlan.test", "id"),
				),
			},
		},
	})
}

func testAccVlanResourceConfigPartition(partitionID, tag string) string {
	return fmt.Sprintf(`
provider "crucible" {}

resource "crucible_vlan" "test" {
  partition_id = %[1]q
  tag          = %[2]q
}
`, partitionID, tag)
}

func testAccVlanResourceConfigProject(projectID, tag string) string {
	return fmt.Sprintf(`
provider "crucible" {}

resource "crucible_vlan" "test" {
  project_id = %[1]q
  tag        = %[2]q
}
`, projectID, tag)
}

func testAccVlanResourceConfigPool(poolID string) string {
	return fmt.Sprintf(`
provider "crucible" {}

resource "crucible_vlan" "test" {
  pool_id = %[1]q
}
`, poolID)
}
