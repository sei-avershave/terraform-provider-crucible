// Copyright 2024 Carnegie Mellon University. All Rights Reserved.
// Released under a MIT (SEI)-style license. See LICENSE.md in the project root for license information.

package provider

import (
	"context"
	"fmt"

	"github.com/cmu-sei/terraform-provider-crucible/internal/api"
	"github.com/cmu-sei/terraform-provider-crucible/internal/client"
	"github.com/cmu-sei/terraform-provider-crucible/internal/structs"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &appTemplateResource{}
	_ resource.ResourceWithConfigure   = &appTemplateResource{}
	_ resource.ResourceWithImportState = &appTemplateResource{}
)

// NewAppTemplateResource is a helper function to simplify the provider implementation.
func NewAppTemplateResource() resource.Resource {
	return &appTemplateResource{}
}

// appTemplateResource is the resource implementation.
type appTemplateResource struct {
	client *client.CrucibleClient
}

// appTemplateResourceModel describes the resource data model.
type appTemplateResourceModel struct {
	ID               types.String `tfsdk:"id"`
	Name             types.String `tfsdk:"name"`
	URL              types.String `tfsdk:"url"`
	Icon             types.String `tfsdk:"icon"`
	Embeddable       types.Bool   `tfsdk:"embeddable"`
	LoadInBackground types.Bool   `tfsdk:"load_in_background"`
}

// Metadata returns the resource type name.
func (r *appTemplateResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_player_application_template"
}

// Schema defines the schema for the resource.
func (r *appTemplateResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Player application template in Crucible. Application templates define reusable application configurations that can be instantiated within views.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The unique identifier for this application template.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The name of the application template.",
			},
			"url": schema.StringAttribute{
				Optional:    true,
				Description: "The URL of the application.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"icon": schema.StringAttribute{
				Optional:    true,
				Description: "URL to an icon image for this application.",
			},
			"embeddable": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Whether this application can be embedded in an iframe.",
			},
			"load_in_background": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Whether this application should be loaded in the background when the view is opened.",
			},
		},
	}
}

// Configure adds the provider configured client to the resource.
func (r *appTemplateResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.CrucibleClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.CrucibleClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

// Create creates the resource and sets the initial Terraform state.
func (r *appTemplateResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data appTemplateResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build template struct
	template := &structs.AppTemplate{
		Name:             data.Name.ValueString(),
		URL:              data.URL.ValueString(),
		Icon:             data.Icon.ValueString(),
		Embeddable:       data.Embeddable.ValueBool(),
		LoadInBackground: data.LoadInBackground.ValueBool(),
	}

	// Create template via API
	id, err := api.CreateAppTemplate(ctx, r.client, template)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Application Template",
			fmt.Sprintf("Could not create application template '%s': %s", data.Name.ValueString(), err.Error()),
		)
		return
	}

	// Set ID in state
	data.ID = types.StringValue(id)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Read refreshes the Terraform state with the latest data.
func (r *appTemplateResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state appTemplateResourceModel

	// Read current state
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Check if template exists
	exists, err := api.AppTemplateExists(ctx, r.client, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Checking Application Template Existence",
			fmt.Sprintf("Could not verify if application template %s exists: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}

	// If template doesn't exist, remove from state
	if !exists {
		resp.State.RemoveResource(ctx)
		return
	}

	// Read template from API
	template, err := api.AppTemplateRead(ctx, r.client, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Application Template",
			fmt.Sprintf("Could not read application template %s: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}

	// Update state with values from API
	state.Name = types.StringValue(template.Name)
	state.URL = types.StringValue(template.URL)
	state.Icon = types.StringValue(template.Icon)
	state.Embeddable = types.BoolValue(template.Embeddable)
	state.LoadInBackground = types.BoolValue(template.LoadInBackground)

	// Save updated state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *appTemplateResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data appTemplateResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build template struct
	template := &structs.AppTemplate{
		Name:             data.Name.ValueString(),
		URL:              data.URL.ValueString(),
		Icon:             data.Icon.ValueString(),
		Embeddable:       data.Embeddable.ValueBool(),
		LoadInBackground: data.LoadInBackground.ValueBool(),
	}

	// Update template via API
	if err := api.AppTemplateUpdate(ctx, r.client, data.ID.ValueString(), template); err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Application Template",
			fmt.Sprintf("Could not update application template %s: %s", data.ID.ValueString(), err.Error()),
		)
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *appTemplateResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state appTemplateResourceModel

	// Read current state
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Check if template exists
	exists, err := api.AppTemplateExists(ctx, r.client, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Checking Application Template Existence",
			fmt.Sprintf("Could not verify if application template %s exists: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}

	// If template doesn't exist, nothing to delete
	if !exists {
		return
	}

	// Delete template via API
	if err := api.DeleteAppTemplate(ctx, r.client, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Application Template",
			fmt.Sprintf("Could not delete application template %s: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}
}

// ImportState imports an existing resource into Terraform state.
func (r *appTemplateResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Use the template ID as the import identifier
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
