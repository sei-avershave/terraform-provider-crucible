// Copyright 2024 Carnegie Mellon University. All Rights Reserved.
// Released under a MIT (SEI)-style license. See LICENSE.md in the project root for license information.

package provider

import (
	"context"
	"encoding/json"
	"os"

	"github.com/cmu-sei/terraform-provider-crucible/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure the implementation satisfies the expected interfaces.
var _ provider.Provider = &crucibleProvider{}

// crucibleProvider is the provider implementation.
type crucibleProvider struct {
	version string
}

// crucibleProviderModel describes the provider configuration data model.
type crucibleProviderModel struct {
	Username      types.String `tfsdk:"username"`
	Password      types.String `tfsdk:"password"`
	AuthURL       types.String `tfsdk:"auth_url"`
	TokenURL      types.String `tfsdk:"token_url"`
	VMApiURL      types.String `tfsdk:"vm_api_url"`
	PlayerApiURL  types.String `tfsdk:"player_api_url"`
	CasterApiURL  types.String `tfsdk:"caster_api_url"`
	ClientID      types.String `tfsdk:"client_id"`
	ClientSecret  types.String `tfsdk:"client_secret"`
	ClientScopes  types.String `tfsdk:"client_scopes"`
}

// New returns a configured provider instance.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &crucibleProvider{
			version: version,
		}
	}
}

// Metadata returns the provider type name.
func (p *crucibleProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "crucible"
	resp.Version = p.version
}

// Schema defines the provider-level schema for configuration data.
func (p *crucibleProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Terraform provider for managing Crucible Framework resources (Player, VM, and Caster APIs).",
		Attributes: map[string]schema.Attribute{
			"username": schema.StringAttribute{
				Required:    true,
				Description: "Username for OAuth2 authentication. Can be set via SEI_CRUCIBLE_USERNAME environment variable.",
			},
			"password": schema.StringAttribute{
				Required:    true,
				Sensitive:   true,
				Description: "Password for OAuth2 authentication. Can be set via SEI_CRUCIBLE_PASSWORD environment variable.",
			},
			"auth_url": schema.StringAttribute{
				Required:    true,
				Description: "OAuth2 authorization endpoint URL. Can be set via SEI_CRUCIBLE_AUTH_URL environment variable.",
			},
			"token_url": schema.StringAttribute{
				Required:    true,
				Description: "OAuth2 token endpoint URL. Can be set via SEI_CRUCIBLE_TOKEN_URL environment variable (or legacy SEI_CRUCIBLE_TOK_URL).",
			},
			"vm_api_url": schema.StringAttribute{
				Required:    true,
				Description: "Base URL for the VM API. Can be set via SEI_CRUCIBLE_VM_API_URL environment variable.",
			},
			"player_api_url": schema.StringAttribute{
				Required:    true,
				Description: "Base URL for the Player API. Can be set via SEI_CRUCIBLE_PLAYER_API_URL environment variable.",
			},
			"caster_api_url": schema.StringAttribute{
				Required:    true,
				Description: "Base URL for the Caster API. Can be set via SEI_CRUCIBLE_CASTER_API_URL environment variable.",
			},
			"client_id": schema.StringAttribute{
				Required:    true,
				Description: "OAuth2 client ID. Can be set via SEI_CRUCIBLE_CLIENT_ID environment variable.",
			},
			"client_secret": schema.StringAttribute{
				Required:    true,
				Sensitive:   true,
				Description: "OAuth2 client secret. Can be set via SEI_CRUCIBLE_CLIENT_SECRET environment variable.",
			},
			"client_scopes": schema.StringAttribute{
				Required:    true,
				Description: "OAuth2 client scopes as JSON array (e.g., '[\"player-api\",\"vm-api\"]'). Can be set via SEI_CRUCIBLE_CLIENT_SCOPES environment variable.",
			},
		},
	}
}

// Configure prepares the provider for resource operations.
func (p *crucibleProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config crucibleProviderModel

	// Read configuration from Terraform
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Allow environment variables to override config
	if val := os.Getenv("SEI_CRUCIBLE_USERNAME"); val != "" {
		config.Username = types.StringValue(val)
	}
	if val := os.Getenv("SEI_CRUCIBLE_PASSWORD"); val != "" {
		config.Password = types.StringValue(val)
	}
	if val := os.Getenv("SEI_CRUCIBLE_AUTH_URL"); val != "" {
		config.AuthURL = types.StringValue(val)
	}
	// Support both new and legacy environment variable names
	if val := os.Getenv("SEI_CRUCIBLE_TOKEN_URL"); val != "" {
		config.TokenURL = types.StringValue(val)
	} else if val := os.Getenv("SEI_CRUCIBLE_TOK_URL"); val != "" {
		config.TokenURL = types.StringValue(val)
	}
	if val := os.Getenv("SEI_CRUCIBLE_VM_API_URL"); val != "" {
		config.VMApiURL = types.StringValue(val)
	}
	if val := os.Getenv("SEI_CRUCIBLE_PLAYER_API_URL"); val != "" {
		config.PlayerApiURL = types.StringValue(val)
	}
	if val := os.Getenv("SEI_CRUCIBLE_CASTER_API_URL"); val != "" {
		config.CasterApiURL = types.StringValue(val)
	}
	if val := os.Getenv("SEI_CRUCIBLE_CLIENT_ID"); val != "" {
		config.ClientID = types.StringValue(val)
	}
	if val := os.Getenv("SEI_CRUCIBLE_CLIENT_SECRET"); val != "" {
		config.ClientSecret = types.StringValue(val)
	}
	if val := os.Getenv("SEI_CRUCIBLE_CLIENT_SCOPES"); val != "" {
		config.ClientScopes = types.StringValue(val)
	}

	// Validate required configuration
	if config.Username.IsNull() || config.Username.ValueString() == "" {
		resp.Diagnostics.AddError(
			"Missing Username Configuration",
			"The provider requires a username. Set the 'username' attribute or SEI_CRUCIBLE_USERNAME environment variable.",
		)
	}
	if config.Password.IsNull() || config.Password.ValueString() == "" {
		resp.Diagnostics.AddError(
			"Missing Password Configuration",
			"The provider requires a password. Set the 'password' attribute or SEI_CRUCIBLE_PASSWORD environment variable.",
		)
	}
	if config.TokenURL.IsNull() || config.TokenURL.ValueString() == "" {
		resp.Diagnostics.AddError(
			"Missing Token URL Configuration",
			"The provider requires a token URL. Set the 'token_url' attribute or SEI_CRUCIBLE_TOKEN_URL environment variable.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	// Parse client scopes from JSON array string
	var scopes []string
	scopesStr := config.ClientScopes.ValueString()
	if scopesStr != "" {
		if err := json.Unmarshal([]byte(scopesStr), &scopes); err != nil {
			// Try comma-separated format as fallback
			scopes = []string{scopesStr}
		}
	}

	// Build provider configuration
	providerConfig := &client.ProviderConfig{
		Username:      config.Username.ValueString(),
		Password:      config.Password.ValueString(),
		AuthURL:       config.AuthURL.ValueString(),
		TokenURL:      config.TokenURL.ValueString(),
		VMApiURL:      config.VMApiURL.ValueString(),
		PlayerApiURL:  config.PlayerApiURL.ValueString(),
		CasterApiURL:  config.CasterApiURL.ValueString(),
		ClientID:      config.ClientID.ValueString(),
		ClientSecret:  config.ClientSecret.ValueString(),
		ClientScopes:  scopes,
	}

	// Create client
	crucibleClient := client.NewClient(providerConfig)

	// Test authentication by fetching a token
	if _, err := crucibleClient.GetToken(ctx); err != nil {
		resp.Diagnostics.AddError(
			"Authentication Failed",
			fmt.Sprintf("Could not authenticate with Crucible APIs: %s\n\nVerify your credentials and token URL are correct.", err.Error()),
		)
		return
	}

	// Make client available to resources
	resp.ResourceData = crucibleClient
}

// Resources returns the list of resources supported by this provider.
func (p *crucibleProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewPlayerUserResource,
		NewAppTemplateResource,
		NewVlanResource,
		NewVMResource,
		NewViewResource,
	}
}

// DataSources returns the list of data sources supported by this provider.
func (p *crucibleProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		// No data sources currently implemented
	}
}
