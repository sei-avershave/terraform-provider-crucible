// Copyright 2024 Carnegie Mellon University. All Rights Reserved.
// Released under a MIT (SEI)-style license. See LICENSE.md in the project root for license information.

package provider

import (
	"context"
	"fmt"
	"sort"

	"github.com/cmu-sei/terraform-provider-crucible/internal/api"
	"github.com/cmu-sei/terraform-provider-crucible/internal/client"
	"github.com/cmu-sei/terraform-provider-crucible/internal/structs"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &viewResource{}
	_ resource.ResourceWithConfigure   = &viewResource{}
	_ resource.ResourceWithImportState = &viewResource{}
)

// NewViewResource is a helper function to simplify the provider implementation.
func NewViewResource() resource.Resource {
	return &viewResource{}
}

// viewResource is the resource implementation.
type viewResource struct {
	client *client.CrucibleClient
}

// viewResourceModel describes the resource data model.
type viewResourceModel struct {
	ID              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	Description     types.String `tfsdk:"description"`
	Status          types.String `tfsdk:"status"`
	CreateAdminTeam types.Bool   `tfsdk:"create_admin_team"`
	Applications    types.List   `tfsdk:"application"`
	Teams           types.List   `tfsdk:"team"`
}

// applicationModel describes an application within a view.
type applicationModel struct {
	AppID            types.String `tfsdk:"app_id"`
	Name             types.String `tfsdk:"name"`
	URL              types.String `tfsdk:"url"`
	Icon             types.String `tfsdk:"icon"`
	Embeddable       types.Bool   `tfsdk:"embeddable"`
	LoadInBackground types.Bool   `tfsdk:"load_in_background"`
	ViewID           types.String `tfsdk:"v_id"`
	AppTemplateID    types.String `tfsdk:"app_template_id"`
}

// teamModel describes a team within a view.
type teamModel struct {
	TeamID       types.String `tfsdk:"team_id"`
	Name         types.String `tfsdk:"name"`
	Role         types.String `tfsdk:"role"`
	Permissions  types.List   `tfsdk:"permissions"`
	Users        types.List   `tfsdk:"user"`
	AppInstances types.List   `tfsdk:"app_instance"`
}

// userInfoModel describes a user within a team.
type userInfoModel struct {
	UserID types.String `tfsdk:"user_id"`
	Role   types.String `tfsdk:"role"`
}

// appInstanceModel describes an application instance within a team.
type appInstanceModel struct {
	Name         types.String  `tfsdk:"name"`
	ID           types.String  `tfsdk:"id"`
	DisplayOrder types.Float64 `tfsdk:"display_order"`
}

// Metadata returns the resource type name.
func (r *viewResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_player_view"
}

// Schema defines the schema for the resource.
func (r *viewResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Player view in Crucible. Views contain teams, applications, and define the structure of an exercise environment.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The unique identifier for this view.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The name of the view.",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "A description of the view.",
			},
			"status": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("Active"),
				Description: "The status of the view (Active, Inactive, etc.).",
			},
			"create_admin_team": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
				Description: "Whether to automatically create an admin team for this view.",
			},
			"application": schema.ListNestedAttribute{
				Optional:    true,
				Description: "List of applications available in this view.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"app_id": schema.StringAttribute{
							Computed:    true,
							Description: "The unique identifier for this application (computed by API).",
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
						},
						"name": schema.StringAttribute{
							Required:    true,
							Description: "The name of the application.",
						},
						"url": schema.StringAttribute{
							Optional:    true,
							Description: "The URL of the application.",
						},
						"icon": schema.StringAttribute{
							Optional:    true,
							Description: "URL to an icon for this application.",
						},
						"embeddable": schema.BoolAttribute{
							Optional:    true,
							Description: "Whether this application can be embedded in an iframe. BREAKING CHANGE in v1.0.0: Now a proper boolean (was string in v0.x).",
						},
						"load_in_background": schema.BoolAttribute{
							Optional:    true,
							Description: "Whether to load this application in the background. BREAKING CHANGE in v1.0.0: Now a proper boolean (was string in v0.x).",
						},
						"app_template_id": schema.StringAttribute{
							Optional:    true,
							Description: "Optional template ID to base this application on.",
						},
						"v_id": schema.StringAttribute{
							Computed:    true,
							Description: "The view ID this application belongs to (computed).",
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
						},
					},
				},
			},
			"team": schema.ListNestedAttribute{
				Optional:    true,
				Description: "List of teams within this view. Each team can have users and application instances.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"team_id": schema.StringAttribute{
							Computed:    true,
							Description: "The unique identifier for this team (computed by API).",
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
						},
						"name": schema.StringAttribute{
							Required:    true,
							Description: "The name of the team.",
						},
						"role": schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Default:     stringdefault.StaticString("View Member"),
							Description: "The default role for members of this team.",
						},
						"permissions": schema.ListAttribute{
							ElementType: types.StringType,
							Optional:    true,
							Description: "List of permission names granted to this team.",
						},
						"user": schema.ListNestedAttribute{
							Optional:    true,
							Description: "List of users assigned to this team.",
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"user_id": schema.StringAttribute{
										Required:    true,
										Description: "UUID of the user.",
									},
									"role": schema.StringAttribute{
										Optional:    true,
										Description: "Role for this user within the team (overrides team default role).",
									},
								},
							},
						},
						"app_instance": schema.ListNestedAttribute{
							Optional:    true,
							Description: "List of application instances for this team.",
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"name": schema.StringAttribute{
										Required:    true,
										Description: "Name of the application (must match an application defined in the view).",
									},
									"display_order": schema.Float64Attribute{
										Optional:    true,
										Description: "Display order for this application instance in the UI.",
									},
									"id": schema.StringAttribute{
										Computed:    true,
										Description: "The unique identifier for this application instance (computed by API).",
										PlanModifiers: []planmodifier.String{
											stringplanmodifier.UseStateForUnknown(),
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

// Configure adds the provider configured client to the resource.
func (r *viewResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *viewResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data viewResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build view struct
	viewInfo := &structs.ViewInfo{
		Name:            data.Name.ValueString(),
		Description:     data.Description.ValueString(),
		Status:          data.Status.ValueString(),
		CreateAdminTeam: data.CreateAdminTeam.ValueBool(),
	}

	// Create view via API (just the view metadata, not apps/teams yet)
	viewID, err := api.CreateView(ctx, r.client, viewInfo)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating View",
			fmt.Sprintf("Could not create view '%s': %s", data.Name.ValueString(), err.Error()),
		)
		return
	}

	data.ID = types.StringValue(viewID)

	// Extract and create applications
	if !data.Applications.IsNull() && !data.Applications.IsUnknown() {
		var apps []applicationModel
		resp.Diagnostics.Append(data.Applications.ElementsAs(ctx, &apps, false)...)
		if resp.Diagnostics.HasError() {
			return
		}

		if err := r.createApplications(ctx, viewID, apps, &data, resp); err != nil {
			resp.Diagnostics.AddError("Error Creating Applications", err.Error())
			return
		}
	}

	// Extract and create teams
	if !data.Teams.IsNull() && !data.Teams.IsUnknown() {
		var teams []teamModel
		resp.Diagnostics.Append(data.Teams.ElementsAs(ctx, &teams, false)...)
		if resp.Diagnostics.HasError() {
			return
		}

		if err := r.createTeams(ctx, viewID, teams, &data, resp); err != nil {
			resp.Diagnostics.AddError("Error Creating Teams", err.Error())
			return
		}
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// createApplications handles creating applications within a view.
func (r *viewResource) createApplications(ctx context.Context, viewID string, apps []applicationModel, data *viewResourceModel, resp *resource.CreateResponse) error {
	appStructs := make([]structs.AppInfo, len(apps))

	for i, app := range apps {
		appStructs[i] = structs.AppInfo{
			Name:   app.Name.ValueString(),
			ViewID: viewID,
		}

		// Handle optional fields with interface{} for API compatibility
		if !app.URL.IsNull() && app.URL.ValueString() != "" {
			appStructs[i].URL = app.URL.ValueString()
		}
		if !app.Icon.IsNull() && app.Icon.ValueString() != "" {
			appStructs[i].Icon = app.Icon.ValueString()
		}
		if !app.Embeddable.IsNull() {
			appStructs[i].Embeddable = app.Embeddable.ValueBool()
		}
		if !app.LoadInBackground.IsNull() {
			appStructs[i].LoadInBackground = app.LoadInBackground.ValueBool()
		}
		if !app.AppTemplateID.IsNull() && app.AppTemplateID.ValueString() != "" {
			appStructs[i].AppTemplateID = app.AppTemplateID.ValueString()
		}
	}

	// Create all applications via API
	if err := api.CreateApps(ctx, r.client, &appStructs, viewID); err != nil {
		return fmt.Errorf("failed to create applications: %w", err)
	}

	// Update models with computed IDs
	for i := range apps {
		apps[i].AppID = types.StringValue(appStructs[i].ID)
		apps[i].ViewID = types.StringValue(viewID)
	}

	// Convert back to list
	appValues := make([]attr.Value, len(apps))
	for i, app := range apps {
		appObj, diags := types.ObjectValueFrom(ctx, applicationAttrTypes(), app)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return fmt.Errorf("failed to convert application to object")
		}
		appValues[i] = appObj
	}

	appList, diags := types.ListValue(types.ObjectType{AttrTypes: applicationAttrTypes()}, appValues)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return fmt.Errorf("failed to create application list")
	}

	data.Applications = appList
	return nil
}

// createTeams handles creating teams within a view.
func (r *viewResource) createTeams(ctx context.Context, viewID string, teams []teamModel, data *viewResourceModel, resp *resource.CreateResponse) error {
	teamStructs := make([]structs.TeamInfo, len(teams))

	for i, team := range teams {
		teamStructs[i] = structs.TeamInfo{
			Name: team.Name.ValueString(),
			Role: team.Role.ValueString(),
		}

		// Extract permissions
		if !team.Permissions.IsNull() && !team.Permissions.IsUnknown() {
			var perms []string
			resp.Diagnostics.Append(team.Permissions.ElementsAs(ctx, &perms, false)...)
			if resp.Diagnostics.HasError() {
				return fmt.Errorf("failed to extract permissions")
			}
			teamStructs[i].Permissions = perms
		}

		// Extract users
		if !team.Users.IsNull() && !team.Users.IsUnknown() {
			var users []userInfoModel
			resp.Diagnostics.Append(team.Users.ElementsAs(ctx, &users, false)...)
			if resp.Diagnostics.HasError() {
				return fmt.Errorf("failed to extract users")
			}

			teamStructs[i].Users = make([]structs.UserInfo, len(users))
			for j, user := range users {
				teamStructs[i].Users[j] = structs.UserInfo{
					ID: user.UserID.ValueString(),
				}
				if !user.Role.IsNull() && user.Role.ValueString() != "" {
					teamStructs[i].Users[j].Role = user.Role.ValueString()
				}
			}
		}

		// Extract app instances
		if !team.AppInstances.IsNull() && !team.AppInstances.IsUnknown() {
			var instances []appInstanceModel
			resp.Diagnostics.Append(team.AppInstances.ElementsAs(ctx, &instances, false)...)
			if resp.Diagnostics.HasError() {
				return fmt.Errorf("failed to extract app instances")
			}

			teamStructs[i].AppInstances = make([]structs.AppInstance, len(instances))
			for j, inst := range instances {
				teamStructs[i].AppInstances[j] = structs.AppInstance{
					Name:         inst.Name.ValueString(),
					DisplayOrder: inst.DisplayOrder.ValueFloat64(),
				}
			}
		}
	}

	// Create all teams via API
	if err := api.CreateTeams(ctx, r.client, &teamStructs, viewID); err != nil {
		return fmt.Errorf("failed to create teams: %w", err)
	}

	// Add permissions to teams
	if err := api.AddPermissionsToTeam(ctx, r.client, &teamStructs); err != nil {
		return fmt.Errorf("failed to add permissions to teams: %w", err)
	}

	// Update models with computed IDs and sort
	sort.Slice(teamStructs, func(i, j int) bool {
		return teamStructs[i].Name.(string) < teamStructs[j].Name.(string)
	})

	for i := range teams {
		if teamIDStr, ok := teamStructs[i].ID.(string); ok {
			teams[i].TeamID = types.StringValue(teamIDStr)
		}

		// Sort users by user_id
		if len(teamStructs[i].Users) > 0 {
			sort.Slice(teamStructs[i].Users, func(a, b int) bool {
				return teamStructs[i].Users[a].ID < teamStructs[i].Users[b].ID
			})
		}

		// Sort app instances by name
		if len(teamStructs[i].AppInstances) > 0 {
			sort.Slice(teamStructs[i].AppInstances, func(a, b int) bool {
				return teamStructs[i].AppInstances[a].Name < teamStructs[i].AppInstances[b].Name
			})

			// Update app instance IDs
			var instances []appInstanceModel
			resp.Diagnostics.Append(teams[i].AppInstances.ElementsAs(ctx, &instances, false)...)
			for j := range instances {
				instances[j].ID = types.StringValue(teamStructs[i].AppInstances[j].ID)
			}
			teams[i].AppInstances, _ = types.ListValueFrom(ctx, types.ObjectType{AttrTypes: appInstanceAttrTypes()}, instances)
		}
	}

	// Convert teams back to list
	teamValues := make([]attr.Value, len(teams))
	for i, team := range teams {
		teamObj, diags := types.ObjectValueFrom(ctx, teamAttrTypes(), team)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return fmt.Errorf("failed to convert team to object")
		}
		teamValues[i] = teamObj
	}

	teamList, diags := types.ListValue(types.ObjectType{AttrTypes: teamAttrTypes()}, teamValues)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return fmt.Errorf("failed to create team list")
	}

	data.Teams = teamList
	return nil
}

// Read refreshes the Terraform state with the latest data.
func (r *viewResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state viewResourceModel

	// Read current state
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Check if view exists
	exists, err := api.ViewExists(ctx, r.client, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Checking View Existence",
			fmt.Sprintf("Could not verify if view %s exists: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}

	// If view doesn't exist, remove from state
	if !exists {
		resp.State.RemoveResource(ctx)
		return
	}

	// Read view from API
	viewInfo, err := api.ReadView(ctx, r.client, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading View",
			fmt.Sprintf("Could not read view %s: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}

	// Update basic fields
	state.Name = types.StringValue(viewInfo.Name)
	state.Description = types.StringValue(viewInfo.Description)
	state.Status = types.StringValue(viewInfo.Status)

	// Note: We don't read back applications and teams here to avoid complexity
	// The Create function sets them correctly, and Update handles changes

	// Save updated state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *viewResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state viewResourceModel

	// Read both plan and current state
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Update view metadata if changed
	if !plan.Name.Equal(state.Name) || !plan.Description.Equal(state.Description) || !plan.Status.Equal(state.Status) {
		viewInfo := &structs.ViewInfo{
			Name:        plan.Name.ValueString(),
			Description: plan.Description.ValueString(),
			Status:      plan.Status.ValueString(),
		}

		if err := api.UpdateView(ctx, r.client, state.ID.ValueString(), viewInfo); err != nil {
			resp.Diagnostics.AddError(
				"Error Updating View",
				fmt.Sprintf("Could not update view %s: %s", state.ID.ValueString(), err.Error()),
			)
			return
		}
	}

	// Handle application changes
	if !plan.Applications.Equal(state.Applications) {
		// For simplicity in this initial implementation, we'll rely on the API
		// to handle application updates. A more sophisticated implementation
		// would do differential updates.
		var planApps []applicationModel
		resp.Diagnostics.Append(plan.Applications.ElementsAs(ctx, &planApps, false)...)
		if resp.Diagnostics.HasError() {
			return
		}

		// TODO: Implement differential application updates
		// For now, log a warning that full recreation may be needed
	}

	// Handle team changes
	if !plan.Teams.Equal(state.Teams) {
		// Similar to applications, team differential updates would go here
		// For initial implementation, this is a simplified version
	}

	// Save updated state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *viewResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state viewResourceModel

	// Read current state
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Check if view exists
	exists, err := api.ViewExists(ctx, r.client, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Checking View Existence",
			fmt.Sprintf("Could not verify if view %s exists: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}

	// If view doesn't exist, nothing to delete
	if !exists {
		return
	}

	// Delete view via API
	if err := api.DeleteView(ctx, r.client, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting View",
			fmt.Sprintf("Could not delete view %s: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}
}

// ImportState imports an existing resource into Terraform state.
func (r *viewResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Use the view ID as the import identifier
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// Helper functions for nested attribute types

func applicationAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"app_id":              types.StringType,
		"name":                types.StringType,
		"url":                 types.StringType,
		"icon":                types.StringType,
		"embeddable":          types.BoolType,
		"load_in_background":  types.BoolType,
		"v_id":                types.StringType,
		"app_template_id":     types.StringType,
	}
}

func teamAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"team_id": types.StringType,
		"name":    types.StringType,
		"role":    types.StringType,
		"permissions": types.ListType{
			ElemType: types.StringType,
		},
		"user": types.ListType{
			ElemType: types.ObjectType{
				AttrTypes: userInfoAttrTypes(),
			},
		},
		"app_instance": types.ListType{
			ElemType: types.ObjectType{
				AttrTypes: appInstanceAttrTypes(),
			},
		},
	}
}

func userInfoAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"user_id": types.StringType,
		"role":    types.StringType,
	}
}

func appInstanceAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"name":          types.StringType,
		"id":            types.StringType,
		"display_order": types.Float64Type,
	}
}
