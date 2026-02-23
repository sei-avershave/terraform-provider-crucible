// Copyright 2024 Carnegie Mellon University. All Rights Reserved.
// Released under a MIT (SEI)-style license. See LICENSE.md in the project root for license information.

package provider

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/cmu-sei/terraform-provider-crucible/internal/api"
	"github.com/cmu-sei/terraform-provider-crucible/internal/client"
	"github.com/cmu-sei/terraform-provider-crucible/internal/structs"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &vmResource{}
	_ resource.ResourceWithConfigure   = &vmResource{}
	_ resource.ResourceWithImportState = &vmResource{}
)

// NewVMResource is a helper function to simplify the provider implementation.
func NewVMResource() resource.Resource {
	return &vmResource{}
}

// vmResource is the resource implementation.
type vmResource struct {
	client *client.CrucibleClient
}

// vmResourceModel describes the resource data model.
type vmResourceModel struct {
	ID                types.String `tfsdk:"id"`
	VMID              types.String `tfsdk:"vm_id"`
	URL               types.String `tfsdk:"url"`
	DefaultURL        types.Bool   `tfsdk:"default_url"`
	Name              types.String `tfsdk:"name"`
	TeamIDs           types.List   `tfsdk:"team_ids"`
	UserID            types.String `tfsdk:"user_id"`
	Embeddable        types.Bool   `tfsdk:"embeddable"`
	ConsoleConnection types.Object `tfsdk:"console_connection_info"`
	ProxmoxInfo       types.Object `tfsdk:"proxmox_vm_info"`
}

// consoleConnectionModel describes console connection nested attribute.
type consoleConnectionModel struct {
	Hostname types.String `tfsdk:"hostname"`
	Port     types.String `tfsdk:"port"`
	Protocol types.String `tfsdk:"protocol"`
	Username types.String `tfsdk:"username"`
	Password types.String `tfsdk:"password"`
}

// proxmoxInfoModel describes proxmox VM info nested attribute.
type proxmoxInfoModel struct {
	ID   types.String `tfsdk:"id"`
	Node types.String `tfsdk:"node"`
	Type types.String `tfsdk:"type"`
}

// Metadata returns the resource type name.
func (r *vmResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_player_virtual_machine"
}

// Schema defines the schema for the resource.
func (r *vmResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Player virtual machine resource in Crucible. VMs can be assigned to teams and configured with console connection details for VSphere, Guacamole, or Proxmox.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Internal identifier for this resource. Matches vm_id.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"vm_id": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "UUID for this virtual machine. If not provided, a UUID will be generated automatically.",
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`),
						"must be a valid UUID (lowercase with hyphens)",
					),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"url": schema.StringAttribute{
				Optional:    true,
				Description: "URL for accessing this VM. If not specified, a default URL will be computed by the API based on the VM type.",
			},
			"default_url": schema.BoolAttribute{
				Computed:    true,
				Description: "Indicates whether the URL was computed by the API (true) or explicitly provided (false).",
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Display name for this virtual machine.",
			},
			"team_ids": schema.ListAttribute{
				ElementType: types.StringType,
				Required:    true,
				Description: "List of team UUIDs that can access this VM. Must contain at least one team.",
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
			},
			"user_id": schema.StringAttribute{
				Optional:    true,
				Description: "Optional user UUID to associate with this VM.",
			},
			"embeddable": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
				Description: "Whether this VM can be embedded in an iframe.",
			},
			"console_connection_info": schema.SingleNestedAttribute{
				Optional:    true,
				Description: "Console connection information for accessing the VM via Guacamole or direct connection.",
				Attributes: map[string]schema.Attribute{
					"hostname": schema.StringAttribute{
						Optional:    true,
						Description: "Hostname or IP address of the console server.",
					},
					"port": schema.StringAttribute{
						Optional:    true,
						Description: "Port number for the console connection.",
					},
					"protocol": schema.StringAttribute{
						Optional:    true,
						Description: "Protocol for console connection (ssh, vnc, rdp).",
						Validators: []validator.String{
							stringvalidator.OneOf("ssh", "vnc", "rdp"),
						},
					},
					"username": schema.StringAttribute{
						Optional:    true,
						Description: "Username for console authentication.",
					},
					"password": schema.StringAttribute{
						Optional:    true,
						Sensitive:   true,
						Description: "Password for console authentication.",
					},
				},
			},
			"proxmox_vm_info": schema.SingleNestedAttribute{
				Optional:    true,
				Description: "Proxmox-specific VM information. Used when VMs are managed by Proxmox.",
				Attributes: map[string]schema.Attribute{
					"id": schema.StringAttribute{
						Optional:    true,
						Description: "Proxmox VM ID. Can be numeric (e.g., '100') or in Proxmox provider format (e.g., 'node/qemu/100').",
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.RequiresReplace(),
						},
					},
					"node": schema.StringAttribute{
						Optional:    true,
						Description: "Proxmox node name where this VM resides.",
					},
					"type": schema.StringAttribute{
						Optional:    true,
						Computed:    true,
						Default:     stringdefault.StaticString("QEMU"),
						Description: "Proxmox VM type (QEMU or LXC).",
						Validators: []validator.String{
							stringvalidator.OneOf("QEMU", "LXC"),
						},
					},
				},
			},
		},
	}
}

// Configure adds the provider configured client to the resource.
func (r *vmResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *vmResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data vmResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Generate VM ID if not provided
	vmID := data.VMID.ValueString()
	if vmID == "" {
		vmID = uuid.NewString()
		data.VMID = types.StringValue(vmID)
	}

	// Extract team IDs from list
	var teamIDs []string
	resp.Diagnostics.Append(data.TeamIDs.ElementsAs(ctx, &teamIDs, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build VM struct
	vmInfo := &structs.VMInfo{
		ID:         vmID,
		URL:        data.URL.ValueString(),
		Name:       data.Name.ValueString(),
		TeamIDs:    teamIDs,
		Embeddable: data.Embeddable.ValueBool(),
	}

	// Handle optional user_id
	if !data.UserID.IsNull() && !data.UserID.IsUnknown() && data.UserID.ValueString() != "" {
		vmInfo.UserID = data.UserID.ValueString()
	} else {
		vmInfo.UserID = nil
	}

	// Handle console_connection_info nested block
	if !data.ConsoleConnection.IsNull() && !data.ConsoleConnection.IsUnknown() {
		var connModel consoleConnectionModel
		resp.Diagnostics.Append(data.ConsoleConnection.As(ctx, &connModel, types.ObjectAsOptions{})...)
		if resp.Diagnostics.HasError() {
			return
		}

		vmInfo.Connection = &structs.ConsoleConnection{
			Hostname: connModel.Hostname.ValueString(),
			Port:     connModel.Port.ValueString(),
			Protocol: connModel.Protocol.ValueString(),
			Username: connModel.Username.ValueString(),
			Password: connModel.Password.ValueString(),
		}
	}

	// Handle proxmox_vm_info nested block
	if !data.ProxmoxInfo.IsNull() && !data.ProxmoxInfo.IsUnknown() {
		var proxmoxModel proxmoxInfoModel
		resp.Diagnostics.Append(data.ProxmoxInfo.As(ctx, &proxmoxModel, types.ObjectAsOptions{})...)
		if resp.Diagnostics.HasError() {
			return
		}

		// Parse proxmox ID - can be "100" or "node/qemu/100"
		proxmoxID := proxmoxModel.ID.ValueString()
		var intID int
		if strings.Contains(proxmoxID, "/") {
			parts := strings.Split(proxmoxID, "/")
			if len(parts) == 3 {
				fmt.Sscanf(parts[2], "%d", &intID)
			}
		} else {
			fmt.Sscanf(proxmoxID, "%d", &intID)
		}

		vmInfo.Proxmox = &structs.ProxmoxInfo{
			Id:   intID,
			Node: proxmoxModel.Node.ValueString(),
			Type: proxmoxModel.Type.ValueString(),
		}
	}

	// Create VM via API
	if err := api.CreateVM(ctx, r.client, vmInfo); err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Virtual Machine",
			fmt.Sprintf("Could not create VM '%s': %s", data.Name.ValueString(), err.Error()),
		)
		return
	}

	// Set computed values
	data.ID = types.StringValue(vmID)
	data.DefaultURL = types.BoolValue(vmInfo.DefaultURL)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Read refreshes the Terraform state with the latest data.
func (r *vmResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state vmResourceModel

	// Read current state
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Check if VM exists
	exists, err := api.VMExists(ctx, r.client, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Checking VM Existence",
			fmt.Sprintf("Could not verify if VM %s exists: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}

	// If VM doesn't exist, remove from state
	if !exists {
		resp.State.RemoveResource(ctx)
		return
	}

	// Read VM from API
	vmInfo, err := api.GetVMInfo(ctx, r.client, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Virtual Machine",
			fmt.Sprintf("Could not read VM %s: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}

	// Sort team IDs to prevent spurious diffs
	sort.Strings(vmInfo.TeamIDs)

	// Update state with values from API
	state.VMID = types.StringValue(vmInfo.ID)
	state.URL = types.StringValue(vmInfo.URL)
	state.DefaultURL = types.BoolValue(vmInfo.DefaultURL)
	state.Name = types.StringValue(vmInfo.Name)
	state.Embeddable = types.BoolValue(vmInfo.Embeddable)

	// Convert team IDs to list
	teamIDValues := make([]attr.Value, len(vmInfo.TeamIDs))
	for i, id := range vmInfo.TeamIDs {
		teamIDValues[i] = types.StringValue(id)
	}
	teamIDList, diags := types.ListValue(types.StringType, teamIDValues)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.TeamIDs = teamIDList

	// Handle optional user_id
	if vmInfo.UserID != nil {
		if userIDStr, ok := vmInfo.UserID.(string); ok && userIDStr != "" {
			state.UserID = types.StringValue(userIDStr)
		} else {
			state.UserID = types.StringNull()
		}
	} else {
		state.UserID = types.StringNull()
	}

	// Handle console_connection_info nested object
	if vmInfo.Connection != nil {
		connAttrs := map[string]attr.Value{
			"hostname": types.StringValue(vmInfo.Connection.Hostname),
			"port":     types.StringValue(vmInfo.Connection.Port),
			"protocol": types.StringValue(vmInfo.Connection.Protocol),
			"username": types.StringValue(vmInfo.Connection.Username),
			"password": types.StringValue(vmInfo.Connection.Password),
		}
		connObj, diags := types.ObjectValue(consoleConnectionAttrTypes(), connAttrs)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		state.ConsoleConnection = connObj
	} else {
		state.ConsoleConnection = types.ObjectNull(consoleConnectionAttrTypes())
	}

	// Handle proxmox_vm_info nested object
	if vmInfo.Proxmox != nil {
		proxmoxAttrs := map[string]attr.Value{
			"id":   types.StringValue(fmt.Sprintf("%d", vmInfo.Proxmox.Id)),
			"node": types.StringValue(vmInfo.Proxmox.Node),
			"type": types.StringValue(vmInfo.Proxmox.Type),
		}
		proxmoxObj, diags := types.ObjectValue(proxmoxInfoAttrTypes(), proxmoxAttrs)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		state.ProxmoxInfo = proxmoxObj
	} else {
		state.ProxmoxInfo = types.ObjectNull(proxmoxInfoAttrTypes())
	}

	// Save updated state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *vmResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state vmResourceModel

	// Read both plan and current state
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Handle team membership changes
	if !plan.TeamIDs.Equal(state.TeamIDs) {
		var oldTeams, newTeams []string
		resp.Diagnostics.Append(state.TeamIDs.ElementsAs(ctx, &oldTeams, false)...)
		resp.Diagnostics.Append(plan.TeamIDs.ElementsAs(ctx, &newTeams, false)...)
		if resp.Diagnostics.HasError() {
			return
		}

		// Find teams to remove (in old but not in new)
		var toRemove []string
		for _, teamID := range oldTeams {
			found := false
			for _, newTeam := range newTeams {
				if teamID == newTeam {
					found = true
					break
				}
			}
			if !found {
				toRemove = append(toRemove, teamID)
			}
		}

		// Find teams to add (in new but not in old)
		var toAdd []string
		for _, teamID := range newTeams {
			found := false
			for _, oldTeam := range oldTeams {
				if teamID == oldTeam {
					found = true
					break
				}
			}
			if !found {
				toAdd = append(toAdd, teamID)
			}
		}

		// Remove from teams
		if len(toRemove) > 0 {
			if err := api.RemoveVMFromTeams(ctx, r.client, state.ID.ValueString(), toRemove); err != nil {
				resp.Diagnostics.AddError(
					"Error Removing VM from Teams",
					fmt.Sprintf("Could not remove VM %s from teams: %s", state.ID.ValueString(), err.Error()),
				)
				return
			}
		}

		// Add to teams
		if len(toAdd) > 0 {
			if err := api.AddVMToTeams(ctx, r.client, state.ID.ValueString(), toAdd); err != nil {
				resp.Diagnostics.AddError(
					"Error Adding VM to Teams",
					fmt.Sprintf("Could not add VM %s to teams: %s", state.ID.ValueString(), err.Error()),
				)
				return
			}
		}
	}

	// Extract team IDs for update call
	var teamIDs []string
	resp.Diagnostics.Append(plan.TeamIDs.ElementsAs(ctx, &teamIDs, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build VM struct with updated values
	vmInfo := &structs.VMInfo{
		ID:         state.ID.ValueString(),
		URL:        plan.URL.ValueString(),
		Name:       plan.Name.ValueString(),
		TeamIDs:    teamIDs,
		Embeddable: plan.Embeddable.ValueBool(),
	}

	// Handle optional user_id
	if !plan.UserID.IsNull() && !plan.UserID.IsUnknown() && plan.UserID.ValueString() != "" {
		vmInfo.UserID = plan.UserID.ValueString()
	} else {
		vmInfo.UserID = nil
	}

	// Handle console_connection_info
	if !plan.ConsoleConnection.IsNull() && !plan.ConsoleConnection.IsUnknown() {
		var connModel consoleConnectionModel
		resp.Diagnostics.Append(plan.ConsoleConnection.As(ctx, &connModel, types.ObjectAsOptions{})...)
		if resp.Diagnostics.HasError() {
			return
		}

		vmInfo.Connection = &structs.ConsoleConnection{
			Hostname: connModel.Hostname.ValueString(),
			Port:     connModel.Port.ValueString(),
			Protocol: connModel.Protocol.ValueString(),
			Username: connModel.Username.ValueString(),
			Password: connModel.Password.ValueString(),
		}
	}

	// Handle proxmox_vm_info
	if !plan.ProxmoxInfo.IsNull() && !plan.ProxmoxInfo.IsUnknown() {
		var proxmoxModel proxmoxInfoModel
		resp.Diagnostics.Append(plan.ProxmoxInfo.As(ctx, &proxmoxModel, types.ObjectAsOptions{})...)
		if resp.Diagnostics.HasError() {
			return
		}

		proxmoxID := proxmoxModel.ID.ValueString()
		var intID int
		if strings.Contains(proxmoxID, "/") {
			parts := strings.Split(proxmoxID, "/")
			if len(parts) == 3 {
				fmt.Sscanf(parts[2], "%d", &intID)
			}
		} else {
			fmt.Sscanf(proxmoxID, "%d", &intID)
		}

		vmInfo.Proxmox = &structs.ProxmoxInfo{
			Id:   intID,
			Node: proxmoxModel.Node.ValueString(),
			Type: proxmoxModel.Type.ValueString(),
		}
	}

	// Update VM via API
	if err := api.UpdateVM(ctx, r.client, vmInfo); err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Virtual Machine",
			fmt.Sprintf("Could not update VM %s: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}

	// Read back to get computed values
	plan.DefaultURL = types.BoolValue(vmInfo.DefaultURL)

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *vmResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state vmResourceModel

	// Read current state
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Check if VM exists
	exists, err := api.VMExists(ctx, r.client, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Checking VM Existence",
			fmt.Sprintf("Could not verify if VM %s exists: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}

	// If VM doesn't exist, nothing to delete
	if !exists {
		return
	}

	// Delete VM via API
	if err := api.DeleteVM(ctx, r.client, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Virtual Machine",
			fmt.Sprintf("Could not delete VM %s: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}
}

// ImportState imports an existing resource into Terraform state.
func (r *vmResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Use the VM ID as the import identifier
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// Helper functions for nested attribute types

func consoleConnectionAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"hostname": types.StringType,
		"port":     types.StringType,
		"protocol": types.StringType,
		"username": types.StringType,
		"password": types.StringType,
	}
}

func proxmoxInfoAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"id":   types.StringType,
		"node": types.StringType,
		"type": types.StringType,
	}
}
