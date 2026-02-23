// Copyright 2024 Carnegie Mellon University. All Rights Reserved.
// Released under a MIT (SEI)-style license. See LICENSE.md in the project root for license information.

package provider

import (
	"context"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// testAccProtoV6ProviderFactories are used to instantiate a provider during
// acceptance testing. The factory function will be invoked for every Terraform
// CLI command executed to create a provider server to which the CLI can
// reattach.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"crucible": providerserver.NewProtocol6WithError(New("test")()),
}

// testAccPreCheck validates that required environment variables are set for acceptance tests
func testAccPreCheck(t *testing.T) {
	required := []string{
		"SEI_CRUCIBLE_USERNAME",
		"SEI_CRUCIBLE_PASSWORD",
		"SEI_CRUCIBLE_TOKEN_URL",
		"SEI_CRUCIBLE_CLIENT_ID",
		"SEI_CRUCIBLE_CLIENT_SECRET",
		"SEI_CRUCIBLE_VM_API_URL",
		"SEI_CRUCIBLE_PLAYER_API_URL",
		"SEI_CRUCIBLE_CASTER_API_URL",
	}

	for _, envVar := range required {
		if os.Getenv(envVar) == "" {
			t.Skipf("Skipping acceptance test - %s not set", envVar)
		}
	}
}
