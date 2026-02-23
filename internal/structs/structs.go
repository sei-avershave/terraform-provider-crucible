// Copyright 2022 Carnegie Mellon University. All Rights Reserved.
// Released under a MIT (SEI)-style license. See LICENSE.md in the project root for license information.

package structs

import (
	"database/sql"
)

// Structs used throughout provider

// VMInfo used as the payload for VM creation and as the return value for VM retrieval
type VMInfo struct {
	ID         string              `json:"id"`
	URL        string              `json:"url,omitempty"`
	DefaultURL bool                `json:"defaultUrl,omitempty"`
	Name       string              `json:"name"`
	TeamIDs    []string            `json:"teamIds,omitempty"`
	UserID     *string             `json:"userId,omitempty"`
	Embeddable bool                `json:"embeddable,omitempty"`
	Connection *ConsoleConnection  `json:"consoleConnectionInfo,omitempty"`
	Proxmox    *ProxmoxInfo        `json:"proxmoxVmInfo,omitempty"`
}

// ConsoleConnection represents a console connection info block
type ConsoleConnection struct {
	Hostname string `json:"hostname"`
	Port     string `json:"port"`
	Protocol string `json:"protocol"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// ProxmoxInfo represents a proxmox vm info block
type ProxmoxInfo struct {
	Id   int    `json:"id"`
	Node string `json:"node"`
	Type string `json:"type,omitempty"`
}

// ViewInfo used as payload for view creation and return value for view retrieval
type ViewInfo struct {
	Name            string     `json:"name"`
	Description     string     `json:"description,omitempty"`
	Status          string     `json:"status,omitempty"`
	CreateAdminTeam bool       `json:"createAdminTeam,omitempty"`
	Applications    []AppInfo  `json:"-"`
	Teams           []TeamInfo `json:"-"`
}

// AppInfo used as payload for application creation and return value for application retrieval
type AppInfo struct {
	ID               string  `json:"id"`
	Name             *string `json:"name,omitempty"`
	URL              *string `json:"url,omitempty"`
	Icon             *string `json:"icon,omitempty"`
	Embeddable       *bool   `json:"embeddable,omitempty"`
	LoadInBackground *bool   `json:"loadInBackground,omitempty"`
	ViewID           string  `json:"viewId"`
	AppTemplateID    *string `json:"applicationTemplateId,omitempty"`
}

// TeamInfo holds information about a team within a view. Used to create, read, and update teams within views
type TeamInfo struct {
	ID           interface{}   `json:"id,omitempty"`
	Name         interface{}   `json:"name"`
	Role         interface{}   `json:"role,omitempty"`
	Permissions  []string      `json:"permissions,omitempty"`
	Users        []UserInfo    `json:"users,omitempty"`
	AppInstances []AppInstance `json:"appInstances,omitempty"`
}

// UserInfo holds information about a user within a team. See PlayerUser for the representation of a
// user in general.
type UserInfo struct {
	ID   string      `json:"id"`
	Role interface{} `json:"role,omitempty"`
}

// UserHasID takes a slice of userInfo structs and returns true if any of them
// have the specified ID. We can't just make a array contains function
// because Go doesn't have generics
func UserHasID(arr []UserInfo, id string) bool {
	for _, user := range arr {
		if user.ID == id {
			return true
		}
	}
	return false
}

// AppTemplate holds the information needed for CRUD operations on an ApplicationTemplate resource
type AppTemplate struct {
	Name             string `json:"name"`
	URL              string `json:"url,omitempty"`
	Icon             string `json:"icon,omitempty"`
	Embeddable       bool   `json:"embeddable"`
	LoadInBackground bool   `json:"loadInBackground"`
}

// AppInstance holds the info needed to manage application instances
type AppInstance struct {
	Name         string  `json:"name"`
	ID           string  `json:"id"`
	DisplayOrder float64 `json:"displayOrder"`
	Parent       string  `json:"applicationId,omitempty"`
}

// InstanceHasID takes an array of AppInstances and returns whether one of them has the given ID.
func InstanceHasID(arr *[]AppInstance, id string) bool {
	for _, inst := range *arr {
		if inst.ID == id {
			return true
		}
	}
	return false
}

// PlayerUser represents a user outside of a team. IE one that simply exists within player. Used for the
// user resource type.
type PlayerUser struct {
	ID   string      `json:"id"`
	Name string      `json:"name"`
	Role interface{} `json:"roleId,omitempty"`
}

type Vlan struct {
	Id          string `json:"id"`
	PoolId      string `json:"poolId,omitempty"`
	PartitionId string `json:"partitionId,omitempty"`
	VlanId      int    `json:"vlanId"`
	InUse       bool   `json:"inUse"`
	Reserved    bool   `json:"reserved"`
	Tag         string `json:"tag,omitempty"`
}

type VlanCreateCommand struct {
	ProjectId   string        `json:"projectId,omitempty"`
	PartitionId string        `json:"partitionId,omitempty"`
	Tag         string        `json:"tag,omitempty"`
	VlanId      sql.NullInt32 `json:"vlanId,omitempty"`
}
