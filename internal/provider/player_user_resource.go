// Copyright 2024 Carnegie Mellon University. All Rights Reserved.
// Released under a MIT (SEI)-style license. See LICENSE.md in the project root for license information.

package provider

import (
	"context"
	"fmt"
	"regexp"

	"github.com/cmu-sei/terraform-provider-crucible/internal/api"
	"github.com/cmu-sei/terraform-provider-crucible/internal/client"
	"github.com/cmu-sei/terraform-provider-crucible/internal/structs"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &playerUserResource{}
	_ resource.ResourceWithConfigure   = &playerUserResource{}
	_ resource.ResourceWithImportState = &playerUserResource{}
)

// NewPlayerUserResource is a helper function to simplify the provider implementation.
func NewPlayerUserResource() resource.Resource {
	return &playerUserResource{}
}

// playerUserResource is the resource implementation.
type playerUserResource struct {
	client *client.CrucibleClient
}

// playerUserResourceModel describes the resource data model.
type playerUserResourceModel struct {
	ID     types.String `tfsdk:"id"`
	UserID types.String `tfsdk:"user_id"`
	Name   types.String `tfsdk:"name"`
	Role   types.String `tfsdk:"role"`
}

// Metadata returns the resource type name.
func (r *playerUserResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_player_user"
}

// Schema defines the schema for the resource.
func (r *playerUserResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Player user resource in Crucible. Users are identity accounts that can be assigned to teams within views.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Internal identifier for this resource. Matches user_id.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"user_id": schema.StringAttribute{
				Required:    true,
				Description: "UUID of the user in the identity provider. Must be a valid UUID.",
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`),
						"must be a valid UUID (lowercase with hyphens)",
					),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Display name for the user.",
			},
			"role": schema.StringAttribute{
				Optional:    true,
				Description: "Role name for this user (e.g., 'Member', 'Admin'). Leave unset if no default role is needed.",
			},
		},
	}
}

// Configure adds the provider configured client to the resource.
func (r *playerUserResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *playerUserResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data playerUserResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build user struct
	user := &structs.PlayerUser{
		ID:   data.UserID.ValueString(),
		Name: data.Name.ValueString(),
	}

	// Handle role - convert empty string to nil, otherwise pass the value
	if !data.Role.IsNull() && !data.Role.IsUnknown() && data.Role.ValueString() != "" {
		user.Role = data.Role.ValueString()
	} else {
		user.Role = ""
	}

	// Create user via API
	if err := api.CreateUser(ctx, r.client, user); err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Player User",
			fmt.Sprintf("Could not create user %s: %s", data.UserID.ValueString(), err.Error()),
		)
		return
	}

	// Set ID in state
	data.ID = types.StringValue(user.ID)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Read refreshes the Terraform state with the latest data.
func (r *playerUserResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state playerUserResourceModel

	// Read current state
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Read user from API
	user, err := api.ReadUser(ctx, r.client, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Player User",
			fmt.Sprintf("Could not read user %s: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}

	// Update state with values from API
	state.UserID = types.StringValue(user.ID)
	state.Name = types.StringValue(user.Name)

	// Handle role - if role is returned, look up the role name by ID
	if user.Role != nil && user.Role != "" {
		roleStr, ok := user.Role.(string)
		if ok && roleStr != "" {
			roleName, err := api.GetRoleByID(ctx, r.client, roleStr)
			if err != nil {
				resp.Diagnostics.AddError(
					"Error Reading User Role",
					fmt.Sprintf("Could not read role for user %s: %s", state.ID.ValueString(), err.Error()),
				)
				return
			}
			state.Role = types.StringValue(roleName)
		} else {
			state.Role = types.StringNull()
		}
	} else {
		state.Role = types.StringNull()
	}

	// Save updated state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *playerUserResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data playerUserResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build user struct
	user := &structs.PlayerUser{
		ID:   data.UserID.ValueString(),
		Name: data.Name.ValueString(),
	}

	// Handle role
	if !data.Role.IsNull() && !data.Role.IsUnknown() && data.Role.ValueString() != "" {
		user.Role = data.Role.ValueString()
	} else {
		user.Role = ""
	}

	// Update user via API
	if err := api.UpdateUser(ctx, r.client, user); err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Player User",
			fmt.Sprintf("Could not update user %s: %s", data.UserID.ValueString(), err.Error()),
		)
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *playerUserResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state playerUserResourceModel

	// Read current state
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Check if user exists
	exists, err := api.UserExists(ctx, r.client, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Checking Player User Existence",
			fmt.Sprintf("Could not verify if user %s exists: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}

	// If user doesn't exist, nothing to delete
	if !exists {
		return
	}

	// Delete user via API
	if err := api.DeleteUser(ctx, r.client, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Player User",
			fmt.Sprintf("Could not delete user %s: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}
}

// ImportState imports an existing resource into Terraform state.
func (r *playerUserResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Use the user ID as the import identifier
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
