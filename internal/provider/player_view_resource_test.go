// Copyright 2024 Carnegie Mellon University. All Rights Reserved.
// Released under a MIT (SEI)-style license. See LICENSE.md in the project root for license information.

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccViewResource_Basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccViewResourceConfigBasic("Test View", "This is a test view"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("crucible_player_view.test", "name", "Test View"),
					resource.TestCheckResourceAttr("crucible_player_view.test", "description", "This is a test view"),
					resource.TestCheckResourceAttrSet("crucible_player_view.test", "id"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "crucible_player_view.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update testing
			{
				Config: testAccViewResourceConfigBasic("Updated View", "Updated description"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("crucible_player_view.test", "name", "Updated View"),
					resource.TestCheckResourceAttr("crucible_player_view.test", "description", "Updated description"),
				),
			},
		},
	})
}

func TestAccViewResource_WithApplications(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccViewResourceConfigWithApps("View With Apps"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("crucible_player_view.test", "application.#", "2"),
					resource.TestCheckResourceAttr("crucible_player_view.test", "application.0.name", "App One"),
					resource.TestCheckResourceAttr("crucible_player_view.test", "application.0.embeddable", "true"),
					resource.TestCheckResourceAttr("crucible_player_view.test", "application.1.name", "App Two"),
					resource.TestCheckResourceAttr("crucible_player_view.test", "application.1.load_in_background", "true"),
				),
			},
		},
	})
}

func TestAccViewResource_WithTeams(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccViewResourceConfigWithTeams("View With Teams"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("crucible_player_view.test", "team.#", "1"),
					resource.TestCheckResourceAttr("crucible_player_view.test", "team.0.name", "Test Team"),
					resource.TestCheckResourceAttrSet("crucible_player_view.test", "team.0.team_id"),
				),
			},
		},
	})
}

func TestAccViewResource_Complete(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with full nested structure
			{
				Config: testAccViewResourceConfigComplete("Complete View"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("crucible_player_view.test", "name", "Complete View"),
					resource.TestCheckResourceAttr("crucible_player_view.test", "status", "Active"),
					resource.TestCheckResourceAttr("crucible_player_view.test", "create_admin_team", "true"),
					resource.TestCheckResourceAttr("crucible_player_view.test", "application.#", "1"),
					resource.TestCheckResourceAttr("crucible_player_view.test", "team.#", "1"),
					resource.TestCheckResourceAttr("crucible_player_view.test", "team.0.user.#", "1"),
					resource.TestCheckResourceAttr("crucible_player_view.test", "team.0.app_instance.#", "1"),
				),
			},
			// Update nested resources
			{
				Config: testAccViewResourceConfigCompleteUpdated("Complete View"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("crucible_player_view.test", "team.0.user.#", "2"),
					resource.TestCheckResourceAttr("crucible_player_view.test", "team.0.app_instance.#", "1"),
				),
			},
		},
	})
}

func testAccViewResourceConfigBasic(name, description string) string {
	return fmt.Sprintf(`
provider "crucible" {}

resource "crucible_player_view" "test" {
  name        = %[1]q
  description = %[2]q
  status      = "Active"
}
`, name, description)
}

func testAccViewResourceConfigWithApps(name string) string {
	return fmt.Sprintf(`
provider "crucible" {}

resource "crucible_player_view" "test" {
  name   = %[1]q
  status = "Active"

  application {
    name               = "App One"
    url                = "https://app1.example.com"
    embeddable         = true
    load_in_background = false
  }

  application {
    name               = "App Two"
    url                = "https://app2.example.com"
    embeddable         = false
    load_in_background = true
  }
}
`, name)
}

func testAccViewResourceConfigWithTeams(name string) string {
	return fmt.Sprintf(`
provider "crucible" {}

resource "crucible_player_view" "test" {
  name   = %[1]q
  status = "Active"

  team {
    name = "Test Team"
    role = "Member"
  }
}
`, name)
}

func testAccViewResourceConfigComplete(name string) string {
	return fmt.Sprintf(`
provider "crucible" {}

resource "crucible_player_user" "test_user" {
  user_id = "550e8400-e29b-41d4-a716-446655440030"
  name    = "Test User"
}

resource "crucible_player_view" "test" {
  name              = %[1]q
  description       = "Full test with nested resources"
  status            = "Active"
  create_admin_team = true

  application {
    name               = "Test App"
    url                = "https://testapp.example.com"
    icon               = "mdi-test"
    embeddable         = true
    load_in_background = false
  }

  team {
    name = "Test Team"
    role = "Member"

    user {
      user_id = crucible_player_user.test_user.user_id
      role    = "Member"
    }

    app_instance {
      name          = "Test App"
      display_order = 0
    }
  }
}
`, name)
}

func testAccViewResourceConfigCompleteUpdated(name string) string {
	return fmt.Sprintf(`
provider "crucible" {}

resource "crucible_player_user" "test_user" {
  user_id = "550e8400-e29b-41d4-a716-446655440030"
  name    = "Test User"
}

resource "crucible_player_user" "test_user2" {
  user_id = "550e8400-e29b-41d4-a716-446655440031"
  name    = "Test User 2"
}

resource "crucible_player_view" "test" {
  name              = %[1]q
  description       = "Updated with more users"
  status            = "Active"
  create_admin_team = true

  application {
    name               = "Test App"
    url                = "https://testapp.example.com"
    icon               = "mdi-test"
    embeddable         = true
    load_in_background = false
  }

  team {
    name = "Test Team"
    role = "Member"

    user {
      user_id = crucible_player_user.test_user.user_id
      role    = "Member"
    }

    user {
      user_id = crucible_player_user.test_user2.user_id
      role    = "Observer"
    }

    app_instance {
      name          = "Test App"
      display_order = 0
    }
  }
}
`, name)
}
