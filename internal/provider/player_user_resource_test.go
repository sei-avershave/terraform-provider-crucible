// Copyright 2024 Carnegie Mellon University. All Rights Reserved.
// Released under a MIT (SEI)-style license. See LICENSE.md in the project root for license information.

package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccPlayerUserResource_Basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccPlayerUserResourceConfig("550e8400-e29b-41d4-a716-446655440001", "Test User", ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("crucible_player_user.test", "user_id", "550e8400-e29b-41d4-a716-446655440001"),
					resource.TestCheckResourceAttr("crucible_player_user.test", "name", "Test User"),
					resource.TestCheckResourceAttrSet("crucible_player_user.test", "id"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "crucible_player_user.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update testing
			{
				Config: testAccPlayerUserResourceConfig("550e8400-e29b-41d4-a716-446655440001", "Updated User", ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("crucible_player_user.test", "name", "Updated User"),
				),
			},
		},
	})
}

func TestAccPlayerUserResource_WithRole(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPlayerUserResourceConfig("550e8400-e29b-41d4-a716-446655440002", "User With Role", "Member"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("crucible_player_user.test", "user_id", "550e8400-e29b-41d4-a716-446655440002"),
					resource.TestCheckResourceAttr("crucible_player_user.test", "name", "User With Role"),
					resource.TestCheckResourceAttr("crucible_player_user.test", "role", "Member"),
				),
			},
		},
	})
}

func TestAccPlayerUserResource_InvalidUUID(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccPlayerUserResourceConfig("not-a-uuid", "Test User", ""),
				ExpectError: regexp.MustCompile("must be a valid UUID"),
			},
		},
	})
}

func testAccPlayerUserResourceConfig(userID, name, role string) string {
	if role == "" {
		return fmt.Sprintf(`
provider "crucible" {}

resource "crucible_player_user" "test" {
  user_id = %[1]q
  name    = %[2]q
}
`, userID, name)
	}

	return fmt.Sprintf(`
provider "crucible" {}

resource "crucible_player_user" "test" {
  user_id = %[1]q
  name    = %[2]q
  role    = %[3]q
}
`, userID, name, role)
}
