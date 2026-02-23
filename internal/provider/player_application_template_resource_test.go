// Copyright 2024 Carnegie Mellon University. All Rights Reserved.
// Released under a MIT (SEI)-style license. See LICENSE.md in the project root for license information.

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccAppTemplateResource_Basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccAppTemplateResourceConfig("Test Template", "https://example.com", "mdi-test", true, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("crucible_player_application_template.test", "name", "Test Template"),
					resource.TestCheckResourceAttr("crucible_player_application_template.test", "url", "https://example.com"),
					resource.TestCheckResourceAttr("crucible_player_application_template.test", "icon", "mdi-test"),
					resource.TestCheckResourceAttr("crucible_player_application_template.test", "embeddable", "true"),
					resource.TestCheckResourceAttr("crucible_player_application_template.test", "load_in_background", "false"),
					resource.TestCheckResourceAttrSet("crucible_player_application_template.test", "id"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "crucible_player_application_template.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update testing
			{
				Config: testAccAppTemplateResourceConfig("Updated Template", "https://updated.com", "mdi-updated", false, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("crucible_player_application_template.test", "name", "Updated Template"),
					resource.TestCheckResourceAttr("crucible_player_application_template.test", "url", "https://updated.com"),
					resource.TestCheckResourceAttr("crucible_player_application_template.test", "embeddable", "false"),
					resource.TestCheckResourceAttr("crucible_player_application_template.test", "load_in_background", "true"),
				),
			},
		},
	})
}

func TestAccAppTemplateResource_MinimalConfig(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAppTemplateResourceConfigMinimal("Minimal Template"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("crucible_player_application_template.test", "name", "Minimal Template"),
					resource.TestCheckResourceAttrSet("crucible_player_application_template.test", "id"),
				),
			},
		},
	})
}

func testAccAppTemplateResourceConfig(name, url, icon string, embeddable, loadInBackground bool) string {
	return fmt.Sprintf(`
provider "crucible" {}

resource "crucible_player_application_template" "test" {
  name               = %[1]q
  url                = %[2]q
  icon               = %[3]q
  embeddable         = %[4]t
  load_in_background = %[5]t
}
`, name, url, icon, embeddable, loadInBackground)
}

func testAccAppTemplateResourceConfigMinimal(name string) string {
	return fmt.Sprintf(`
provider "crucible" {}

resource "crucible_player_application_template" "test" {
  name = %[1]q
}
`, name)
}
