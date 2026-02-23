// Copyright 2022 Carnegie Mellon University. All Rights Reserved.
// Released under a MIT (SEI)-style license. See LICENSE.md in the project root for license information.

package provider

import (
	"crucible_provider/internal/api"
	"crucible_provider/internal/structs"
	"crucible_provider/internal/util"
	"fmt"
	"log"
	"reflect"
	"sort"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

func playerView() *schema.Resource {
	return &schema.Resource{
		Create: playerViewCreate,
		Read:   playerViewRead,
		Update: playerViewUpdate,
		Delete: playerViewDelete,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"description": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"status": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "Active",
			},
			"create_admin_team": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
			},
			"application": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"app_id": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"name": {
							Type:     schema.TypeString,
							Required: true,
						},
						"url": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"icon": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"embeddable": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"load_in_background": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"app_template_id": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"v_id": {
							Type:     schema.TypeString,
							Computed: true,
						},
					},
				},
			},
			"team": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"team_id": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"name": {
							Type:     schema.TypeString,
							Required: true,
						},
						"role": {
							Type:     schema.TypeString,
							Optional: true,
							Default:  "View Member",
						},
						"permissions": {
							Type:     schema.TypeList,
							Optional: true,
							Elem: &schema.Schema{
								Type: schema.TypeString,
							},
						},
						"app_instance": {
							Type:     schema.TypeList,
							Optional: true,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"name": {
										Type:     schema.TypeString,
										Required: true,
									},
									"display_order": {
										Type:     schema.TypeFloat,
										Optional: true,
									},
									"id": {
										Type:     schema.TypeString,
										Computed: true,
									},
								},
							},
						},
						"user": {
							Type:     schema.TypeList,
							Optional: true,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"user_id": {
										Type:     schema.TypeString,
										Required: true,
									},
									"role": {
										Type:     schema.TypeString,
										Optional: true,
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

// Get view properties from d
// Call API to create view
// If no error, set local state
// Call read to make sure everything worked
func playerViewCreate(d *schema.ResourceData, m interface{}) error {
	if m == nil {
		return fmt.Errorf("error configuring provider")
	}

	// Set up the view itself
	view := &structs.ViewInfo{
		Name:            d.Get("name").(string),
		Description:     d.Get("description").(string),
		Status:          d.Get("status").(string),
		CreateAdminTeam: d.Get("create_admin_team").(bool),
	}

	casted := m.(map[string]string)
	id, err := api.CreateView(view, casted)
	if err != nil {
		return err
	}

	d.SetId(id)

	err = d.Set("name", view.Name)
	if err != nil {
		return err
	}

	err = d.Set("description", view.Description)
	if err != nil {
		return err
	}

	err = d.Set("status", view.Status)
	if err != nil {
		return err
	}

	err = d.Set("create_admin_team", view.CreateAdminTeam)
	if err != nil {
		return err
	}

	// If any applications are in the config, create those
	apps := d.Get("application").([]interface{})
	if len(apps) > 0 {
		err := createApps(d, casted, &apps)
		if err != nil {
			return err
		}
	}

	// Create any teams specified in the config
	teams := d.Get("team").([]interface{})
	if len(teams) > 0 {
		err := createTeams(d, casted, &teams)
		if err != nil {
			return err
		}
	}

	log.Printf("! View created with ID %s", d.Id())
	log.Printf("! At end of main create, applications local state: %+v", d.Get("application"))
	log.Printf("! At end of main crate, team local state: %+v", d.Get("team"))
	return playerViewRead(d, m)
}

// Check if view exists. If not, set id to "" and return nil
// Read view info from API
// Use it to update local state
// I never change the id of the view. May need to reevaluate if that causes bugs
func playerViewRead(d *schema.ResourceData, m interface{}) error {
	id := d.Id()
	casted := m.(map[string]string)
	exists, err := api.ViewExists(id, casted)
	if err != nil {
		return err
	}
	if !exists {
		d.SetId("")
		return nil
	}

	// Call API to read state of the view
	view, err := api.ReadView(id, casted)
	if err != nil {
		return err
	}

	// Write state for applications and teams into map form so local state can be set
	localMapsTeams := new([]map[string]interface{})

	// We sort the slice of teams to make sure it's always in a consistent order in local state
	// Otherwise, terraform may try to update a team when it doesn't need to
	teams := view.Teams
	sort.Slice(teams, func(i, j int) bool {
		return teams[i].Name.(string) < teams[j].Name.(string)
	})
	view.Teams = teams
	// We also must sort the users and app instances within each team
	for _, team := range view.Teams {
		sort.Slice(team.Users, func(i, j int) bool {
			return team.Users[i].ID < team.Users[j].ID
		})
		sort.Slice(team.AppInstances, func(i, j int) bool {
			return team.AppInstances[i].Name < team.AppInstances[j].Name
		})
	}

	// Sort applications
	sort.Slice(view.Applications, func(i, j int) bool {
		return view.Applications[i].Name.(string) < view.Applications[j].Name.(string)
	})

	localMaps := new([]map[string]interface{})
	for _, app := range view.Applications {
		*localMaps = append(*localMaps, app.ToMap())
	}

	createAdmin := d.Get("create_admin_team").(bool)
	for _, team := range view.Teams {
		if team.Name == "Admin" && createAdmin {
			continue
		}

		asMap := team.ToMap()
		*localMapsTeams = append(*localMapsTeams, asMap)
	}

	// Set local state
	err = d.Set("name", view.Name)
	if err != nil {
		return err
	}

	err = d.Set("description", view.Description)
	if err != nil {
		return err
	}

	err = d.Set("status", view.Status)
	if err != nil {
		return err
	}

	log.Printf("! In read, setting local app state to: %+v", localMaps)
	err = d.Set("application", localMaps)
	if err != nil {
		return err
	}

	log.Printf("! In read, setting local team state to %+v", localMapsTeams)
	err = d.Set("team", localMapsTeams)
	if err != nil {
		return err
	}

	log.Printf("! At very bottom of read, d.Get(\"team\") = %+v", d.Get("team"))
	return nil
}

func playerViewUpdate(d *schema.ResourceData, m interface{}) error {
	if m == nil {
		return fmt.Errorf("error configuring provider")
	}

	log.Printf("! In main update, before any updates, teams are %+v", d.Get("team"))

	// Update the view itself
	view := &structs.ViewInfo{
		Name:        d.Get("name").(string),
		Description: d.Get("description").(string),
		Status:      d.Get("status").(string),
	}

	casted := m.(map[string]string)
	err := api.UpdateView(view, casted, d.Id())
	if err != nil {
		return err
	}

	log.Printf("! In main update, after updating view, teams are %+v", d.Get("team"))

	// Update any applications that have changed. This may include deleting applications as well as creating new ones
	if d.HasChange("application") {
		err := updateApps(d, casted)
		if err != nil {
			return err
		}
	}

	log.Printf("! In main update, after updating apps, teams are %+v", d.Get("team"))

	// Handle any updates to the teams within this view
	if d.HasChange("team") {
		err := updateTeams(d, casted, d.Id())
		if err != nil {
			return err
		}
	}

	// Update local state of view
	err = d.Set("name", view.Name)
	if err != nil {
		return err
	}

	err = d.Set("description", view.Description)
	if err != nil {
		return err
	}

	err = d.Set("status", view.Status)
	if err != nil {
		return err
	}

	return playerViewRead(d, m)
}

// Check if view exists. If not, return nil
// Call API delete function. Return nil on success or some error on failure
// This will also delete any apps or teams inside this view
func playerViewDelete(d *schema.ResourceData, m interface{}) error {
	if m == nil {
		return fmt.Errorf("error configuring provider")
	}

	// Delete the view itself. This will also destroy anything inside the view, ie teams or applications
	id := d.Id()
	casted := m.(map[string]string)
	exists, err := api.ViewExists(id, casted)

	if err != nil {
		return err
	}

	if !exists {
		return nil
	}

	return api.DeleteView(id, casted)

}

// ------------ Private functions ------------

// ------------ Create functions for nested resources ------------

// Create the apps specified in the configuration
func createApps(d *schema.ResourceData, m map[string]string, apps *[]interface{}) error {
	appStructs := new([]*structs.AppInfo)
	for _, app := range *apps {
		asMap := app.(map[string]interface{})

		// Generate a uuid if one is not provided
		if asMap["app_id"] == "" {
			asMap["app_id"] = uuid.New().String()
			log.Printf("! app_id = %v", asMap["app_id"])
		}
		curr := structs.AppInfoFromMap(asMap)
		curr.ViewID = d.Id()
		*appStructs = append(*appStructs, curr)
	}

	// Call API to create the applications
	err := api.CreateApps(appStructs, m, d.Id())
	if err != nil {
		return err
	}

	// Set local state
	sort.Slice((*appStructs), func(i, j int) bool {
		return (*(*appStructs)[i]).Name.(string) < (*(*appStructs)[j]).Name.(string)
	})

	localMaps := new([]map[string]interface{})
	for _, app := range *appStructs {
		*localMaps = append(*localMaps, app.ToMap())
	}

	return d.Set("application", localMaps)
}

// Creates the teams specified in the configuration
func createTeams(d *schema.ResourceData, m map[string]string, teams *[]interface{}) error {
	log.Printf("! At top of createTeams")

	teamStructs := new([]*structs.TeamInfo)
	for _, team := range *teams {
		asMap := team.(map[string]interface{})
		curr := structs.TeamInfoFromMap(asMap)
		*teamStructs = append(*teamStructs, curr)
	}

	applications := d.Get("application").([]interface{})
	appStructs := new([]*structs.AppInfo)
	for _, app := range applications {
		asMap := app.(map[string]interface{})
		*appStructs = append(*appStructs, structs.AppInfoFromMap(asMap))
	}

	// Find the GUID mapping to the name app name provided in each app instance
	for i, team := range *teamStructs {
		for j, app := range team.AppInstances {
			for _, parent := range *appStructs {
				if parent.Name == app.Name {
					app.Parent = parent.ID
					(*teamStructs)[i].AppInstances[j] = app
				}
			}
		}
	}

	// Call API to create teams
	err := api.CreateTeams(teamStructs, d.Id(), m)
	if err != nil {
		return err
	}

	// Add permissions to the teams
	err = api.AddPermissionsToTeam(teamStructs, m)
	if err != nil {
		return err
	}

	// Set local state

	// Sort so we have a consistent ordering of the teams
	sort.Slice(*teamStructs, func(i, j int) bool {
		return (*teamStructs)[i].Name.(string) < (*teamStructs)[j].Name.(string)
	})

	// Sort the users and app instances within each team
	for _, team := range *teamStructs {
		sort.Slice(team.Users, func(i, j int) bool {
			return team.Users[i].ID < team.Users[j].ID
		})
		sort.Slice(team.AppInstances, func(i, j int) bool {
			return team.AppInstances[i].Name < team.AppInstances[j].Name
		})
	}

	localMaps := new([]map[string]interface{})
	for _, team := range *teamStructs {
		log.Printf("! Converting a team to a map in createTeams")
		*localMaps = append(*localMaps, team.ToMap())
	}

	log.Printf("! To set to local state of teams after create: %+v", localMaps)

	return d.Set("team", localMaps)
}

// ------------ Update functions for nested resources ------------

// Updates the state of the applications within a view
func updateApps(d *schema.ResourceData, m map[string]string) error {
	// Get old and new values
	// Consider each value in old. If it does not exist in new, delete that app. If it exists but has had its properties
	// modified, updated that app. It the value exists and is unchanged, do nothing. If there are values that are in new
	// but not old, create those apps.
	oldGeneric, currentGeneric := d.GetChange("application")

	// slice of map[string]interface{}, but go won't let us do the cast like that
	old := oldGeneric.([]interface{})
	current := currentGeneric.([]interface{})

	// Find any applications that need to be deleted or updated
	toDelete := new([]string) // id's of the apps we are deleting
	toUpdate := new([]*structs.AppInfo)
	toCreate := new([]*structs.AppInfo)

	for _, app := range old {
		oldMap := app.(map[string]interface{})
		value := oldMap["app_id"].(string)
		// There is no app with this id, it has been deleted
		if !util.PairInList(current, "app_id", value) {
			*toDelete = append(*toDelete, value)
		} else {
			// An app with this id existed in old and still exists in current, so update it
			// Find the entry in current corresponding to this id
			for _, curr := range current {
				currMap := curr.(map[string]interface{})
				// The current map corresponds to the same app as the old one AND there is some change between the two
				if currMap["app_id"].(string) == value && !reflect.DeepEqual(oldMap, currMap) {
					info := structs.AppInfoFromMap(currMap)
					*toUpdate = append(*toUpdate, info)
				}
			}
		}
	}
	// Find any applications that did not exist before and need to be created. Ones that are in current but not in old
	for _, app := range current {
		currMap := app.(map[string]interface{})
		value := currMap["app_id"].(string)

		if !util.PairInList(old, "app_id", value) {
			info := structs.AppInfoFromMap(currMap)
			info.ID = uuid.New().String()
			info.ViewID = d.Id()
			*toCreate = append(*toCreate, info)
		}
	}
	// Apply updates to applications
	err := api.DeleteApps(toDelete, m)
	if err != nil {
		return err
	}
	err = api.UpdateApps(toUpdate, m)
	if err != nil {
		return err
	}
	err = api.CreateApps(toCreate, m, d.Id())
	if err != nil {
		return err
	}

	// Update local state of apps
	local := new([]map[string]interface{})
	for _, app := range *toUpdate {
		*local = append(*local, app.ToMap())
	}
	for _, app := range *toCreate {
		*local = append(*local, app.ToMap())
	}

	return d.Set("application", local)
}

// Update the teams within a view
func updateTeams(d *schema.ResourceData, m map[string]string, viewID string) error {
	// Logic is the same as for applications. Delete teams that are in old but not current,
	// update teams that are in both, and create teams that are in current but not old
	oldGeneric, currentGeneric := d.GetChange("team")

	old := oldGeneric.([]interface{})
	current := currentGeneric.([]interface{})
	log.Printf("! Old local team state: %+v", old)
	log.Printf("! Current local team state: %+v", current)

	toDelete := new([]string)
	toUpdate := new([]*structs.TeamInfo)
	toCreate := new([]*structs.TeamInfo)

	// The old versions of the teams we updated. Will make looking for change in the users more efficient
	oldUpdated := new([]*structs.TeamInfo)

	// Check for deleted and updated teams
	for _, team := range old {
		oldMap := team.(map[string]interface{})
		value := oldMap["team_id"].(string)

		if !util.PairInList(current, "team_id", value) {
			*toDelete = append(*toDelete, value)
		} else {
			for _, curr := range current {
				currMap := curr.(map[string]interface{})
				// if the current map corresponds to the same team AND there is some difference between old and new maps
				if currMap["team_id"].(string) == value && !reflect.DeepEqual(oldMap, currMap) {
					info := structs.TeamInfoFromMap(currMap)
					*toUpdate = append(*toUpdate, info)
					*oldUpdated = append(*oldUpdated, structs.TeamInfoFromMap(oldMap))
					log.Printf("! Appended to oldUpdated: %+v", structs.TeamInfoFromMap(oldMap))
				}
			}
		}
	}

	// Check for new teams
	for _, team := range current {
		currMap := team.(map[string]interface{})
		value := currMap["team_id"].(string)

		if !util.PairInList(old, "team_id", value) {
			info := structs.TeamInfoFromMap(currMap)
			*toCreate = append(*toCreate, info)
		}
	}

	// Map teamIDs to the permissions to add to/remove from those teams
	permsToRemove := make(map[string][]string)
	permsToAdd := make(map[string][]string)

	toRemoveCurr := new([]string)
	toAddCurr := new([]string)
	// Check for removed and added permissions
	for i, oldTeam := range *oldUpdated {
		// Clear out the lists of permission IDs. This way we only alloc memory once.
		*toRemoveCurr = (*toRemoveCurr)[:0]
		*toAddCurr = (*toAddCurr)[:0]

		// oldUpdated and toUpdated are in the same order so we can index them like this
		oldPerms := oldTeam.Permissions
		currPerms := (*toUpdate)[i].Permissions

		// Find anything that was in old but is not in current
		for _, oldPerm := range oldPerms {
			if !util.StrSliceContains(&currPerms, oldPerm) {
				*toRemoveCurr = append(*toRemoveCurr, oldPerm)
			}
		}
		log.Printf("! permsToRemove[%v] = %v", oldTeam.ID, *toRemoveCurr)
		permsToRemove[oldTeam.ID.(string)] = *toRemoveCurr

		// Find anything that is not in old but is in current
		for _, currPerm := range currPerms {
			if !util.StrSliceContains(&oldPerms, currPerm) {
				*toAddCurr = append(*toAddCurr, currPerm)
			}
		}
		permsToAdd[oldTeam.ID.(string)] = *toAddCurr
	}

	// Call API to add/remove permissions on existing teams
	err := api.UpdateTeamPermissions(permsToAdd, permsToRemove, m)
	if err != nil {
		return err
	}

	applications := d.Get("application").([]interface{})
	appStructs := new([]*structs.AppInfo)
	for _, app := range applications {
		asMap := app.(map[string]interface{})
		*appStructs = append(*appStructs, structs.AppInfoFromMap(asMap))
	}

	// Find the GUID mapping to the name app name provided in each app instance
	for i, team := range *toCreate {
		for j, app := range team.AppInstances {
			for _, parent := range *appStructs {
				if parent.Name == app.Name {
					app.Parent = parent.ID
					(*toCreate)[i].AppInstances[j] = app
				}
			}
		}
	}

	// Update remote state
	err = api.DeleteTeams(toDelete, m)
	if err != nil {
		return err
	}
	err = api.UpdateTeams(toUpdate, m)
	if err != nil {
		return err
	}
	err = api.CreateTeams(toCreate, viewID, m)
	if err != nil {
		return err
	}
	// Add permissions for the created teams
	err = api.AddPermissionsToTeam(toCreate, m)
	if err != nil {
		return err
	}

	err = updateUsers(oldUpdated, toUpdate, m, d.Id())
	if err != nil {
		return err
	}

	apps := d.Get("application")
	if apps != nil {
		err = updateInstances(oldUpdated, toUpdate, apps.([]interface{}), m)
		if err != nil {
			return err
		}
	}

	// Update local state of teams
	local := new([]map[string]interface{})
	for _, team := range *toUpdate {
		*local = append(*local, team.ToMap())
	}
	for _, team := range *toCreate {
		*local = append(*local, team.ToMap())
	}

	return d.Set("team", local)
}

// Updates the users within a team
func updateUsers(oldUpdated, toUpdate *[]*structs.TeamInfo, m map[string]string, viewID string) error {
	// Check for users that have been removed - in old but not in current
	removedUsers := make(map[string][]string) // map of teams to the users that have been removed from them

	oldUsers := new([]structs.UserInfo)
	currUsers := new([]structs.UserInfo)

	// The users to add to the team
	toAdd := new([]string)

	// Maps the teams who have users with updated roles to the structs representing those users
	toChangeRole := make(map[string][]structs.UserInfo)
	usersWithChangedRole := new([]structs.UserInfo)

	for i, oldTeam := range *oldUpdated {
		// Get the list of users for the old version of the team
		*oldUsers = (*oldUsers)[:0]
		*oldUsers = oldTeam.Users

		// The slices line up so we can just index into the other one
		currTeam := (*toUpdate)[i]
		// Get the list of users for the current version of the team
		*currUsers = (*currUsers)[:0]
		*currUsers = currTeam.Users

		log.Printf("! Old usuers %+v", *oldUsers)
		log.Printf("! Current usuers %+v", *currUsers)

		// Figure out what users, if any, should be removed from the team - those in old but not current

		*usersWithChangedRole = (*usersWithChangedRole)[:0]
		// Special case if there are no users left on the team
		if len(*currUsers) == 0 {
			for _, user := range *oldUsers {
				removedUsers[currTeam.ID.(string)] = append(removedUsers[currTeam.ID.(string)], user.ID)
			}

		} else {
			// Now we have lists of users to compare
			// For each old user, see if it is gone form current state. If yes, remove user from team
			for _, oldUser := range *oldUsers {
				if !structs.UserHasID(*currUsers, oldUser.ID) {
					removedUsers[currTeam.ID.(string)] = append(removedUsers[currTeam.ID.(string)], oldUser.ID)
				}
			}
			toChangeRole[oldTeam.ID.(string)] = *usersWithChangedRole
		}

		*toAdd = (*toAdd)[:0]
		// Find any users that should be added to the team - those in current but not in old
		for _, currUser := range *currUsers {
			// If the current user's ID is not found in the list of old users - add this user
			if !structs.UserHasID(*oldUsers, currUser.ID) {
				*toAdd = append(*toAdd, currUser.ID)
			} else {
				var old structs.UserInfo
				found := false
				// Get the old version of this user
				for _, oldUser := range *oldUsers {
					if oldUser.ID == currUser.ID {
						old = oldUser
						found = true
					}
				}
				// Check that it has changed
				// If yes, update the user
				if found && old.Role != currUser.Role {
					err := api.SetUserRole(oldTeam.ID.(string), viewID, currUser, m)
					if err != nil {
						return err
					}
				}
			}
		}

		err := api.AddUsersToTeam(toAdd, currTeam.ID.(string), m)
		if err != nil {
			return err
		}
	}

	return api.RemoveUsers(removedUsers, m)
}

// Update the application instances within a team
func updateInstances(old, current *[]*structs.TeamInfo, apps []interface{}, m map[string]string) error {
	deleted := new([]string) // The IDs of the app instances to be deleted

	oldInstances := new([]structs.AppInstance)
	currInstances := new([]structs.AppInstance)

	// Look for instances to delete. Updating existing instances also happens here for greater efficiency
	for i, oldTeam := range *old {
		log.Printf("! Old team: %+v", oldTeam)
		log.Printf("! Curr team: %+v", (*current)[i])

		// Get the instances for the old team being considered
		*oldInstances = (*oldInstances)[:0]
		*oldInstances = oldTeam.AppInstances
		log.Printf("! Old instances: %+v", *oldInstances)

		// The slices line up so we can just index into the other one
		currTeam := (*current)[i]
		// Get the list of instances for the current version of the team
		*currInstances = (*currInstances)[:0]
		*currInstances = currTeam.AppInstances

		// Look for instances to delete. If an instance is not being deleted and it has changed, update it

		// If there are no instances in the current team, they were all deleted
		if len(*currInstances) == 0 {
			log.Printf("! No instances in current state. Delete them all.")
			for _, inst := range *oldInstances {
				*deleted = append(*deleted, inst.ID)
			}
		} else {
			// At least one instance remains, find which should be deleted and which should just be updated
			for _, oldInst := range *oldInstances {
				// This instance is not in current team, we want to remove it
				if !structs.InstanceHasID(currInstances, oldInst.ID) {
					*deleted = append(*deleted, oldInst.ID)
				}
			}
		}

		// Find the instances to add to the team - those in current but not old
		for _, currInst := range *currInstances {
			if !structs.InstanceHasID(oldInstances, currInst.ID) {
				for _, app := range apps {
					asMap := app.(map[string]interface{})
					if asMap["name"] == currInst.Name {
						_, err := api.AddApplication(asMap["app_id"].(string), oldTeam.ID.(string), currInst.DisplayOrder, m)
						if err != nil {
							return err
						}
					}
				}
			} else {
				// Get the old version of this instance
				var old structs.AppInstance
				found := false
				for _, oldInst := range *oldInstances {
					if oldInst.ID == currInst.ID {
						old = oldInst
						found = true
					}
				}

				// If it has changed, update it
				if found && (old.Name != currInst.Name || old.DisplayOrder != currInst.DisplayOrder) {
					err := api.UpdateAppInstance(currInst, oldTeam.ID.(string), m)
					if err != nil {
						return err
					}
				}
			}
		}
	}

	// Delete the appropriate app instances
	log.Printf("! App instances to delete: %+v", deleted)
	err := api.DeleteAppInstances(deleted, m)
	if err != nil {
		return err
	}

	return nil
}
