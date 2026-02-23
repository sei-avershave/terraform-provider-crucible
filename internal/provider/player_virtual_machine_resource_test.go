// Copyright 2024 Carnegie Mellon University. All Rights Reserved.
// Released under a MIT (SEI)-style license. See LICENSE.md in the project root for license information.

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccVMResource_Basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccVMResourceConfigBasic("550e8400-e29b-41d4-a716-446655440010", "Test VM"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("crucible_player_virtual_machine.test", "vm_id", "550e8400-e29b-41d4-a716-446655440010"),
					resource.TestCheckResourceAttr("crucible_player_virtual_machine.test", "name", "Test VM"),
					resource.TestCheckResourceAttrSet("crucible_player_virtual_machine.test", "id"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "crucible_player_virtual_machine.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccVMResource_WithTeams(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccVMResourceConfigWithTeams("550e8400-e29b-41d4-a716-446655440011", "VM With Teams", []string{
					"550e8400-e29b-41d4-a716-446655440020",
					"550e8400-e29b-41d4-a716-446655440021",
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("crucible_player_virtual_machine.test", "name", "VM With Teams"),
					resource.TestCheckResourceAttr("crucible_player_virtual_machine.test", "team_ids.#", "2"),
				),
			},
			// Update to add another team
			{
				Config: testAccVMResourceConfigWithTeams("550e8400-e29b-41d4-a716-446655440011", "VM With Teams", []string{
					"550e8400-e29b-41d4-a716-446655440020",
					"550e8400-e29b-41d4-a716-446655440021",
					"550e8400-e29b-41d4-a716-446655440022",
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("crucible_player_virtual_machine.test", "team_ids.#", "3"),
				),
			},
		},
	})
}

func TestAccVMResource_WithConsoleConnection(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccVMResourceConfigWithConsole("550e8400-e29b-41d4-a716-446655440012", "VM With Console"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("crucible_player_virtual_machine.test", "console_connection_info.hostname", "console.example.com"),
					resource.TestCheckResourceAttr("crucible_player_virtual_machine.test", "console_connection_info.port", "5900"),
					resource.TestCheckResourceAttr("crucible_player_virtual_machine.test", "console_connection_info.protocol", "vnc"),
				),
			},
		},
	})
}

func TestAccVMResource_WithProxmox(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccVMResourceConfigWithProxmox("550e8400-e29b-41d4-a716-446655440013", "VM With Proxmox"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("crucible_player_virtual_machine.test", "proxmox_vm_info.id", "100"),
					resource.TestCheckResourceAttr("crucible_player_virtual_machine.test", "proxmox_vm_info.node", "pve-node1"),
					resource.TestCheckResourceAttr("crucible_player_virtual_machine.test", "proxmox_vm_info.type", "qemu"),
				),
			},
		},
	})
}

func testAccVMResourceConfigBasic(vmID, name string) string {
	return fmt.Sprintf(`
provider "crucible" {}

resource "crucible_player_virtual_machine" "test" {
  vm_id = %[1]q
  name  = %[2]q
}
`, vmID, name)
}

func testAccVMResourceConfigWithTeams(vmID, name string, teamIDs []string) string {
	config := fmt.Sprintf(`
provider "crucible" {}

resource "crucible_player_virtual_machine" "test" {
  vm_id    = %[1]q
  name     = %[2]q
  team_ids = [
`, vmID, name)

	for i, teamID := range teamIDs {
		if i > 0 {
			config += ",\n"
		}
		config += fmt.Sprintf("    %q", teamID)
	}

	config += "\n  ]\n}\n"
	return config
}

func testAccVMResourceConfigWithConsole(vmID, name string) string {
	return fmt.Sprintf(`
provider "crucible" {}

resource "crucible_player_virtual_machine" "test" {
  vm_id = %[1]q
  name  = %[2]q

  console_connection_info {
    hostname = "console.example.com"
    port     = "5900"
    protocol = "vnc"
    username = "testuser"
    password = "testpass"
  }
}
`, vmID, name)
}

func testAccVMResourceConfigWithProxmox(vmID, name string) string {
	return fmt.Sprintf(`
provider "crucible" {}

resource "crucible_player_virtual_machine" "test" {
  vm_id = %[1]q
  name  = %[2]q

  proxmox_vm_info {
    id   = 100
    node = "pve-node1"
    type = "qemu"
  }
}
`, vmID, name)
}
