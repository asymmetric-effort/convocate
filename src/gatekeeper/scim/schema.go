package scim

import "encoding/json"

const (
	// Schema URIs
	SchemaUser            = "urn:ietf:params:scim:schemas:core:2.0:User"
	SchemaGroup           = "urn:ietf:params:scim:schemas:core:2.0:Group"
	SchemaServiceProvider = "urn:ietf:params:scim:schemas:core:2.0:ServiceProviderConfig"
	SchemaListResponse    = "urn:ietf:params:scim:api:messages:2.0:ListResponse"
	SchemaError           = "urn:ietf:params:scim:api:messages:2.0:Error"
	SchemaResourceType    = "urn:ietf:params:scim:schemas:core:2.0:ResourceType"
	SchemaSchemaDef       = "urn:ietf:params:scim:schemas:core:2.0:Schema"
)

// User represents a SCIM User resource.
type User struct {
	Schemas     []string   `json:"schemas"`
	ID          string     `json:"id"`
	ExternalID  string     `json:"externalId,omitempty"`
	UserName    string     `json:"userName"`
	Name        *Name      `json:"name,omitempty"`
	DisplayName string     `json:"displayName,omitempty"`
	Emails      []Email    `json:"emails,omitempty"`
	Active      bool       `json:"active"`
	Groups      []GroupRef `json:"groups,omitempty"`
	Meta        Meta       `json:"meta"`
}

// Name represents a SCIM User's name.
type Name struct {
	GivenName  string `json:"givenName,omitempty"`
	FamilyName string `json:"familyName,omitempty"`
	Formatted  string `json:"formatted,omitempty"`
}

// Email represents a SCIM email.
type Email struct {
	Value   string `json:"value"`
	Type    string `json:"type,omitempty"`
	Primary bool   `json:"primary,omitempty"`
}

// GroupRef is a reference to a group in a user record.
type GroupRef struct {
	Value   string `json:"value"`
	Display string `json:"display,omitempty"`
	Ref     string `json:"$ref,omitempty"`
}

// Group represents a SCIM Group resource.
type Group struct {
	Schemas     []string    `json:"schemas"`
	ID          string      `json:"id"`
	DisplayName string      `json:"displayName"`
	Members     []MemberRef `json:"members,omitempty"`
	Meta        Meta        `json:"meta"`
}

// MemberRef is a reference to a member in a group.
type MemberRef struct {
	Value   string `json:"value"`
	Display string `json:"display,omitempty"`
	Ref     string `json:"$ref,omitempty"`
}

// Meta holds resource metadata.
type Meta struct {
	ResourceType string `json:"resourceType"`
	Location     string `json:"location,omitempty"`
	Created      string `json:"created,omitempty"`
	LastModified string `json:"lastModified,omitempty"`
}

// ListResponse is a SCIM list response.
type ListResponse struct {
	Schemas      []string        `json:"schemas"`
	TotalResults int             `json:"totalResults"`
	StartIndex   int             `json:"startIndex"`
	ItemsPerPage int             `json:"itemsPerPage"`
	Resources    json.RawMessage `json:"Resources"`
}

// ErrorResponse is a SCIM error.
type ErrorResponse struct {
	Schemas []string `json:"schemas"`
	Detail  string   `json:"detail"`
	Status  string   `json:"status"`
}

// ServiceProviderConfig describes the SCIM service capabilities.
type ServiceProviderConfig struct {
	Schemas               []string        `json:"schemas"`
	DocumentationURI      string          `json:"documentationUri,omitempty"`
	Patch                 SupportedConfig `json:"patch"`
	Bulk                  BulkConfig      `json:"bulk"`
	Filter                FilterConfig    `json:"filter"`
	ChangePassword        SupportedConfig `json:"changePassword"`
	Sort                  SupportedConfig `json:"sort"`
	Etag                  SupportedConfig `json:"etag"`
	AuthenticationSchemes []AuthScheme    `json:"authenticationSchemes"`
}

// SupportedConfig indicates if a feature is supported.
type SupportedConfig struct {
	Supported bool `json:"supported"`
}

// BulkConfig holds bulk operation configuration.
type BulkConfig struct {
	Supported      bool `json:"supported"`
	MaxOperations  int  `json:"maxOperations"`
	MaxPayloadSize int  `json:"maxPayloadSize"`
}

// FilterConfig holds filter configuration.
type FilterConfig struct {
	Supported  bool `json:"supported"`
	MaxResults int  `json:"maxResults"`
}

// AuthScheme describes an authentication scheme.
type AuthScheme struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ResourceType describes a SCIM resource type.
type ResourceType struct {
	Schemas     []string `json:"schemas"`
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Endpoint    string   `json:"endpoint"`
	Schema      string   `json:"schema"`
}

// GetServiceProviderConfig returns the service provider configuration.
func GetServiceProviderConfig() ServiceProviderConfig {
	return ServiceProviderConfig{
		Schemas: []string{SchemaServiceProvider},
		Patch:   SupportedConfig{Supported: false},
		Bulk: BulkConfig{
			Supported:      false,
			MaxOperations:  0,
			MaxPayloadSize: 0,
		},
		Filter: FilterConfig{
			Supported:  true,
			MaxResults: 200,
		},
		ChangePassword: SupportedConfig{Supported: false},
		Sort:           SupportedConfig{Supported: false},
		Etag:           SupportedConfig{Supported: false},
		AuthenticationSchemes: []AuthScheme{
			{
				Type:        "oauthbearertoken",
				Name:        "OAuth Bearer Token",
				Description: "Authentication using an OpenBao token as a Bearer token",
			},
		},
	}
}

// GetResourceTypes returns the supported resource types.
func GetResourceTypes() []ResourceType {
	return []ResourceType{
		{
			Schemas:     []string{SchemaResourceType},
			ID:          "User",
			Name:        "User",
			Description: "User Account",
			Endpoint:    "/scim/v2/Users",
			Schema:      SchemaUser,
		},
		{
			Schemas:     []string{SchemaResourceType},
			ID:          "Group",
			Name:        "Group",
			Description: "Group",
			Endpoint:    "/scim/v2/Groups",
			Schema:      SchemaGroup,
		},
	}
}

// GetSchemas returns the SCIM schema definitions.
func GetSchemas() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"schemas":     []string{SchemaSchemaDef},
			"id":          SchemaUser,
			"name":        "User",
			"description": "User Account",
			"attributes": []map[string]interface{}{
				{"name": "userName", "type": "string", "multiValued": false, "required": true, "mutability": "readWrite"},
				{"name": "name", "type": "complex", "multiValued": false, "required": false, "mutability": "readWrite"},
				{"name": "displayName", "type": "string", "multiValued": false, "required": false, "mutability": "readWrite"},
				{"name": "emails", "type": "complex", "multiValued": true, "required": false, "mutability": "readWrite"},
				{"name": "active", "type": "boolean", "multiValued": false, "required": false, "mutability": "readWrite"},
				{"name": "groups", "type": "complex", "multiValued": true, "required": false, "mutability": "readOnly"},
			},
		},
		{
			"schemas":     []string{SchemaSchemaDef},
			"id":          SchemaGroup,
			"name":        "Group",
			"description": "Group",
			"attributes": []map[string]interface{}{
				{"name": "displayName", "type": "string", "multiValued": false, "required": true, "mutability": "readWrite"},
				{"name": "members", "type": "complex", "multiValued": true, "required": false, "mutability": "readWrite"},
			},
		},
	}
}
