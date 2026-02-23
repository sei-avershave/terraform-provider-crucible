// Copyright 2024 Carnegie Mellon University. All Rights Reserved.
// Released under a MIT (SEI)-style license. See LICENSE.md in the project root for license information.

package provider

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/cmu-sei/terraform-provider-crucible/internal/api"
	"github.com/cmu-sei/terraform-provider-crucible/internal/client"
	"github.com/cmu-sei/terraform-provider-crucible/internal/structs"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &vlanResource{}
	_ resource.ResourceWithConfigure   = &vlanResource{}
	_ resource.ResourceWithImportState = &vlanResource{}
)

// NewVlanResource is a helper function to simplify the provider implementation.
func NewVlanResource() resource.Resource {
	return &vlanResource{}
}

// vlanResource is the resource implementation.
type vlanResource struct {
	client *client.CrucibleClient
}

// vlanResourceModel describes the resource data model.
type vlanResourceModel struct {
	ID          types.String `tfsdk:"id"`
	PartitionID types.String `tfsdk:"partition_id"`
	PoolID      types.String `tfsdk:"pool_id"`
	ProjectID   types.String `tfsdk:"project_id"`
	Tag         types.String `tfsdk:"tag"`
	VlanID      types.Int64  `tfsdk:"vlan_id"`
}

// Metadata returns the resource type name.
func (r *vlanResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vlan"
}

// Schema defines the schema for the resource.
func (r *vlanResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a VLAN allocation from the Caster API. VLANs can be allocated by partition or by project (mutually exclusive). Once created, VLAN resources are immutable and must be replaced if changes are needed.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The unique identifier for this VLAN allocation.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"partition_id": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "The partition ID to allocate a VLAN from. Conflicts with project_id. If neither is specified, a VLAN is allocated from the default partition.",
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.MatchRoot("project_id")),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"pool_id": schema.StringAttribute{
				Computed:    true,
				Description: "The ID of the pool this VLAN was allocated from (computed).",
			},
			"project_id": schema.StringAttribute{
				Optional:    true,
				Description: "The project ID to allocate a VLAN for. Conflicts with partition_id.",
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.MatchRoot("partition_id")),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"tag": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Optional tag for this VLAN allocation.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"vlan_id": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "The VLAN ID (tag number). If not specified, one will be allocated automatically.",
				Validators: []validator.Int64{
					int64validator.Between(1, 4094),
				},
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
		},
	}
}

// Configure adds the provider configured client to the resource.
func (r *vlanResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *vlanResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data vlanResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build VLAN create command
	cmd := &structs.VlanCreateCommand{
		ProjectId:   data.ProjectID.ValueString(),
		PartitionId: data.PartitionID.ValueString(),
		Tag:         data.Tag.ValueString(),
	}

	// Handle optional vlan_id with sql.NullInt32
	if !data.VlanID.IsNull() && !data.VlanID.IsUnknown() {
		cmd.VlanId = sql.NullInt32{
			Int32: int32(data.VlanID.ValueInt64()),
			Valid: true,
		}
	} else {
		cmd.VlanId = sql.NullInt32{Valid: false}
	}

	// Create VLAN via API
	vlan, err := api.CreateVlan(ctx, r.client, cmd)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating VLAN",
			fmt.Sprintf("Could not allocate VLAN: %s", err.Error()),
		)
		return
	}

	// Set values in state
	data.ID = types.StringValue(vlan.Id)
	data.VlanID = types.Int64Value(int64(vlan.VlanId))
	data.PoolID = types.StringValue(vlan.PoolId)
	data.PartitionID = types.StringValue(vlan.PartitionId)
	data.Tag = types.StringValue(vlan.Tag)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Read refreshes the Terraform state with the latest data.
func (r *vlanResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state vlanResourceModel

	// Read current state
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Read VLAN from API
	vlan, err := api.ReadVlan(ctx, r.client, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading VLAN",
			fmt.Sprintf("Could not read VLAN %s: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}

	// If VLAN is not in use, remove from state
	if !vlan.InUse {
		resp.State.RemoveResource(ctx)
		return
	}

	// Update state with values from API
	state.VlanID = types.Int64Value(int64(vlan.VlanId))
	state.PoolID = types.StringValue(vlan.PoolId)
	state.PartitionID = types.StringValue(vlan.PartitionId)
	state.Tag = types.StringValue(vlan.Tag)

	// Save updated state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update is not implemented - VLANs are immutable and require replacement.
func (r *vlanResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Update Not Supported",
		"VLAN resources are immutable. Any changes require resource replacement (ForceNew).",
	)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *vlanResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state vlanResourceModel

	// Read current state
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Delete (release) VLAN via API
	if err := api.DeleteVlan(ctx, r.client, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError(
			"Error Releasing VLAN",
			fmt.Sprintf("Could not release VLAN %s: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}
}

// ImportState imports an existing resource into Terraform state.
func (r *vlanResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Use the VLAN ID as the import identifier
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
