// Copyright 2022 Carnegie Mellon University. All Rights Reserved.
// Released under a MIT (SEI)-style license. See LICENSE.md in the project root for license information.

package provider

import (
	"os"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

// Provider returns an instance of the provider
func Provider() *schema.Provider {
	return &schema.Provider{
		ResourcesMap: map[string]*schema.Resource{
			"crucible_player_virtual_machine":      playerVirtualMachine(),
			"crucible_player_view":                 playerView(),
			"crucible_player_application_template": applicationTemplate(),
			"crucible_player_user":                 user(),
			"crucible_vlan":                        casterVlan(),
		},
		Schema: map[string]*schema.Schema{
			"username": {
				Type:     schema.TypeString,
				Required: true,
				DefaultFunc: func() (interface{}, error) {
					return os.Getenv("SEI_CRUCIBLE_USERNAME"), nil
				},
			},
			"password": {
				Type:     schema.TypeString,
				Required: true,
				DefaultFunc: func() (interface{}, error) {
					return os.Getenv("SEI_CRUCIBLE_PASSWORD"), nil
				},
			},
			"auth_url": {
				Type:     schema.TypeString,
				Required: true,
				DefaultFunc: func() (interface{}, error) {
					return os.Getenv("SEI_CRUCIBLE_AUTH_URL"), nil
				},
			},
			"token_url": {
				Type:     schema.TypeString,
				Required: true,
				DefaultFunc: func() (interface{}, error) {
					// Support both old and new environment variable names for backward compatibility
				if val := os.Getenv("SEI_CRUCIBLE_TOKEN_URL"); val != "" {
					return val, nil
				}
				return os.Getenv("SEI_CRUCIBLE_TOK_URL"), nil
				},
			},
			"vm_api_url": {
				Type:     schema.TypeString,
				Required: true,
				DefaultFunc: func() (interface{}, error) {
					return os.Getenv("SEI_CRUCIBLE_VM_API_URL"), nil
				},
			},
			"player_api_url": {
				Type:     schema.TypeString,
				Required: true,
				DefaultFunc: func() (interface{}, error) {
					return os.Getenv("SEI_CRUCIBLE_PLAYER_API_URL"), nil
				},
			},
			"caster_api_url": {
				Type:     schema.TypeString,
				Required: true,
				DefaultFunc: func() (interface{}, error) {
					return os.Getenv("SEI_CRUCIBLE_CASTER_API_URL"), nil
				},
			},
			"client_id": {
				Type:     schema.TypeString,
				Required: true,
				DefaultFunc: func() (interface{}, error) {
					return os.Getenv("SEI_CRUCIBLE_CLIENT_ID"), nil
				},
			},
			"client_secret": {
				Type:     schema.TypeString,
				Required: true,
				DefaultFunc: func() (interface{}, error) {
					return os.Getenv("SEI_CRUCIBLE_CLIENT_SECRET"), nil
				},
			},
			"client_scopes": {
				Type: schema.TypeList,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Optional: true,
				DefaultFunc: func() (interface{}, error) {
					if val := os.Getenv("SEI_CRUCIBLE_CLIENT_SCOPES"); val != "" {
						return val, nil
					}
					return []interface{}{}, nil // Return an empty list if the environment variable is not set
				},
			},
		},
		ConfigureFunc: config,
	}
}

// This will read in the key-value pairs supplied in the provider block of the config file.
// The map that is returned can be accessed in the CRUD functions in a _server.go file via the m parameter.
func config(r *schema.ResourceData) (interface{}, error) {
	user := r.Get("username")
	pass := r.Get("password")
	auth := r.Get("auth_url")
	playerTok := r.Get("token_url")
	vmAPI := r.Get("vm_api_url")
	playerAPI := r.Get("player_api_url")
	casterAPI := r.Get("caster_api_url")
	id := r.Get("client_id")
	sec := r.Get("client_secret")
	scopesInterface := r.Get("client_scopes").([]interface{})
	scopesList := make([]string, len(scopesInterface))
	for i, v := range scopesInterface {
		scopesList[i] = v.(string) // Convert each item to string
	}
	scopes := strings.Join(scopesList, ",")

	if user == nil || pass == nil || auth == nil || playerTok == nil || vmAPI == nil || id == nil || sec == nil ||
		playerAPI == nil || casterAPI == nil || scopesInterface == nil {
		return nil, nil
	}

	m := make(map[string]string)
	m["username"] = user.(string)
	m["password"] = pass.(string)
	m["auth_url"] = auth.(string)
	m["player_token_url"] = playerTok.(string)
	m["vm_api_url"] = vmAPI.(string)
	m["player_api_url"] = playerAPI.(string)
	m["caster_api_url"] = casterAPI.(string)
	m["client_id"] = id.(string)
	m["client_secret"] = sec.(string)
	m["client_scopes"] = scopes
	return m, nil
}
