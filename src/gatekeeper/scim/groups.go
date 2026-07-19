package scim

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/asymmetric-effort/convocate/src/gatekeeper/openbao"
)

// ListGroups handles GET /scim/v2/Groups.
func ListGroups(client *openbao.Client, baseURL string, w http.ResponseWriter, r *http.Request) {
	names, err := client.ListGroups()
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("list groups: %v", err))
		return
	}

	groups := make([]Group, 0, len(names))
	for i := 0; i < len(names); i++ {
		group, err := client.GetGroupByName(names[i])
		if err != nil || group == nil {
			continue
		}
		groups = append(groups, openbaoGroupToSCIM(group, baseURL))
	}

	resourcesJSON, _ := json.Marshal(groups)
	resp := ListResponse{
		Schemas:      []string{SchemaListResponse},
		TotalResults: len(groups),
		StartIndex:   1,
		ItemsPerPage: len(groups),
		Resources:    resourcesJSON,
	}

	writeJSON(w, http.StatusOK, resp)
}

// GetGroup handles GET /scim/v2/Groups/{id}.
func GetGroup(client *openbao.Client, baseURL string, w http.ResponseWriter, id string) {
	group, err := client.GetGroupByID(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("get group: %v", err))
		return
	}
	if group == nil {
		writeError(w, http.StatusNotFound, "group not found")
		return
	}

	scimGroup := openbaoGroupToSCIM(group, baseURL)
	writeJSON(w, http.StatusOK, scimGroup)
}

// CreateGroup handles POST /scim/v2/Groups.
func CreateGroup(client *openbao.Client, baseURL string, w http.ResponseWriter, r *http.Request) {
	var input struct {
		Schemas     []string    `json:"schemas"`
		DisplayName string      `json:"displayName"`
		Members     []MemberRef `json:"members"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %v", err))
		return
	}

	if input.DisplayName == "" {
		writeError(w, http.StatusBadRequest, "displayName is required")
		return
	}

	// Extract member entity IDs
	memberIDs := make([]string, 0, len(input.Members))
	for i := 0; i < len(input.Members); i++ {
		if input.Members[i].Value != "" {
			memberIDs = append(memberIDs, input.Members[i].Value)
		}
	}

	group, err := client.CreateGroup(input.DisplayName, memberIDs, nil)
	if err != nil {
		writeError(w, http.StatusConflict, fmt.Sprintf("create group: %v", err))
		return
	}

	scimGroup := openbaoGroupToSCIM(group, baseURL)
	writeJSON(w, http.StatusCreated, scimGroup)
}

// UpdateGroup handles PUT /scim/v2/Groups/{id}.
func UpdateGroup(client *openbao.Client, baseURL string, w http.ResponseWriter, r *http.Request, id string) {
	// Get existing group
	existing, err := client.GetGroupByID(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("get group: %v", err))
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "group not found")
		return
	}

	var input struct {
		Schemas     []string    `json:"schemas"`
		DisplayName string      `json:"displayName"`
		Members     []MemberRef `json:"members"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %v", err))
		return
	}

	displayName := input.DisplayName
	if displayName == "" {
		displayName = existing.Name
	}

	memberIDs := make([]string, 0, len(input.Members))
	for i := 0; i < len(input.Members); i++ {
		if input.Members[i].Value != "" {
			memberIDs = append(memberIDs, input.Members[i].Value)
		}
	}

	if err := client.UpdateGroup(displayName, memberIDs, existing.Metadata); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("update group: %v", err))
		return
	}

	// Re-fetch
	updated, err := client.GetGroupByID(id)
	if err != nil || updated == nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch updated group")
		return
	}

	scimGroup := openbaoGroupToSCIM(updated, baseURL)
	writeJSON(w, http.StatusOK, scimGroup)
}

// DeleteGroup handles DELETE /scim/v2/Groups/{id}.
func DeleteGroup(client *openbao.Client, w http.ResponseWriter, id string) {
	group, err := client.GetGroupByID(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("get group: %v", err))
		return
	}
	if group == nil {
		writeError(w, http.StatusNotFound, "group not found")
		return
	}

	if err := client.DeleteGroup(id); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("delete group: %v", err))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func openbaoGroupToSCIM(group *openbao.Group, baseURL string) Group {
	members := make([]MemberRef, 0, len(group.MemberEntityIDs))
	for i := 0; i < len(group.MemberEntityIDs); i++ {
		members = append(members, MemberRef{
			Value: group.MemberEntityIDs[i],
			Ref:   baseURL + "/scim/v2/Users/" + group.MemberEntityIDs[i],
		})
	}

	return Group{
		Schemas:     []string{SchemaGroup},
		ID:          group.ID,
		DisplayName: group.Name,
		Members:     members,
		Meta: Meta{
			ResourceType: "Group",
			Location:     baseURL + "/scim/v2/Groups/" + group.ID,
		},
	}
}
