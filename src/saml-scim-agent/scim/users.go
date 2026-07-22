package scim

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/asymmetric-effort/convocate/src/saml-scim-agent/openbao"
)

// ListUsers handles GET /scim/v2/Users.
func ListUsers(client *openbao.Client, baseURL string, w http.ResponseWriter, r *http.Request) {
	names, err := client.ListEntities()
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("list entities: %v", err))
		return
	}

	users := make([]User, 0, len(names))
	for i := 0; i < len(names); i++ {
		entity, err := client.GetEntityByName(names[i])
		if err != nil || entity == nil {
			continue
		}
		users = append(users, entityToUser(entity, baseURL))
	}

	resourcesJSON, _ := json.Marshal(users)
	resp := ListResponse{
		Schemas:      []string{SchemaListResponse},
		TotalResults: len(users),
		StartIndex:   1,
		ItemsPerPage: len(users),
		Resources:    resourcesJSON,
	}

	writeJSON(w, http.StatusOK, resp)
}

// GetUser handles GET /scim/v2/Users/{id}.
func GetUser(client *openbao.Client, baseURL string, w http.ResponseWriter, id string) {
	entity, err := client.GetEntityByID(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("get entity: %v", err))
		return
	}
	if entity == nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	user := entityToUser(entity, baseURL)
	writeJSON(w, http.StatusOK, user)
}

// CreateUser handles POST /scim/v2/Users.
func CreateUser(client *openbao.Client, baseURL string, w http.ResponseWriter, r *http.Request) {
	var input struct {
		Schemas     []string `json:"schemas"`
		UserName    string   `json:"userName"`
		Name        *Name    `json:"name"`
		DisplayName string   `json:"displayName"`
		Emails      []Email  `json:"emails"`
		Active      bool     `json:"active"`
		Password    string   `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %v", err))
		return
	}

	if input.UserName == "" {
		writeError(w, http.StatusBadRequest, "userName is required")
		return
	}

	// Build metadata
	metadata := map[string]string{}
	if input.DisplayName != "" {
		metadata["display_name"] = input.DisplayName
	}
	if input.Name != nil {
		if input.Name.GivenName != "" {
			metadata["given_name"] = input.Name.GivenName
		}
		if input.Name.FamilyName != "" {
			metadata["family_name"] = input.Name.FamilyName
		}
	}
	if len(input.Emails) > 0 {
		metadata["email"] = input.Emails[0].Value
	}

	// Create entity in OpenBao
	entity, err := client.CreateEntity(input.UserName, metadata)
	if err != nil {
		writeError(w, http.StatusConflict, fmt.Sprintf("create entity: %v", err))
		return
	}

	// Create userpass credentials if password provided
	if input.Password != "" {
		if err := client.CreateUserpass(input.UserName, input.Password); err != nil {
			// Best effort - entity is created but userpass might fail
			_ = err
		}
	}

	user := entityToUser(entity, baseURL)
	writeJSON(w, http.StatusCreated, user)
}

// UpdateUser handles PUT /scim/v2/Users/{id}.
func UpdateUser(client *openbao.Client, baseURL string, w http.ResponseWriter, r *http.Request, id string) {
	// First get the existing entity to find its name
	entity, err := client.GetEntityByID(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("get entity: %v", err))
		return
	}
	if entity == nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	var input struct {
		Schemas     []string `json:"schemas"`
		UserName    string   `json:"userName"`
		Name        *Name    `json:"name"`
		DisplayName string   `json:"displayName"`
		Emails      []Email  `json:"emails"`
		Active      bool     `json:"active"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %v", err))
		return
	}

	// Build updated metadata
	metadata := map[string]string{}
	if input.DisplayName != "" {
		metadata["display_name"] = input.DisplayName
	}
	if input.Name != nil {
		if input.Name.GivenName != "" {
			metadata["given_name"] = input.Name.GivenName
		}
		if input.Name.FamilyName != "" {
			metadata["family_name"] = input.Name.FamilyName
		}
	}
	if len(input.Emails) > 0 {
		metadata["email"] = input.Emails[0].Value
	}

	if err := client.UpdateEntity(entity.Name, metadata, !input.Active); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("update entity: %v", err))
		return
	}

	// Re-fetch to get updated entity
	updated, err := client.GetEntityByID(id)
	if err != nil || updated == nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch updated entity")
		return
	}

	user := entityToUser(updated, baseURL)
	writeJSON(w, http.StatusOK, user)
}

// DeleteUser handles DELETE /scim/v2/Users/{id}.
func DeleteUser(client *openbao.Client, w http.ResponseWriter, id string) {
	entity, err := client.GetEntityByID(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("get entity: %v", err))
		return
	}
	if entity == nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	// Delete userpass
	_ = client.DeleteUserpass(entity.Name)

	// Delete entity
	if err := client.DeleteEntity(entity.Name); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("delete entity: %v", err))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func entityToUser(entity *openbao.Entity, baseURL string) User {
	user := User{
		Schemas:  []string{SchemaUser},
		ID:       entity.ID,
		UserName: entity.Name,
		Active:   !entity.Disabled,
		Meta: Meta{
			ResourceType: "User",
			Location:     baseURL + "/scim/v2/Users/" + entity.ID,
		},
	}

	if entity.Metadata != nil {
		if email, ok := entity.Metadata["email"]; ok && email != "" {
			user.Emails = []Email{{Value: email, Type: "work", Primary: true}}
		}
		if dn, ok := entity.Metadata["display_name"]; ok {
			user.DisplayName = dn
		}
		givenName := entity.Metadata["given_name"]
		familyName := entity.Metadata["family_name"]
		if givenName != "" || familyName != "" {
			user.Name = &Name{
				GivenName:  givenName,
				FamilyName: familyName,
				Formatted:  strings.TrimSpace(givenName + " " + familyName),
			}
		}
	}

	// Map group IDs to refs
	groups := make([]GroupRef, 0, len(entity.GroupIDs))
	for i := 0; i < len(entity.GroupIDs); i++ {
		groups = append(groups, GroupRef{
			Value: entity.GroupIDs[i],
			Ref:   baseURL + "/scim/v2/Groups/" + entity.GroupIDs[i],
		})
	}
	user.Groups = groups

	return user
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/scim+json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, detail string) {
	resp := ErrorResponse{
		Schemas: []string{SchemaError},
		Detail:  detail,
		Status:  fmt.Sprintf("%d", status),
	}
	w.Header().Set("Content-Type", "application/scim+json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}

// nowRFC3339 returns the current time in RFC3339 format.
func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}
