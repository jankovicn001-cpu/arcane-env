package types

import (
	"time"
)

type (
	UserGroup struct {
		CreatedAt   time.Time       `json:"createdAt" doc:"Creation timestamp"`
		UpdatedAt   time.Time       `json:"updatedAt" doc:"Last update timestamp"`
		ID          string          `json:"id" doc:"User group ID"`
		Name        string          `json:"name" doc:"User group name"`
		Description string          `json:"description" doc:"User group description"`
		Links       []UserGroupLink `json:"links" doc:"SSO claim values and LDAP group names whose users join the group automatically"`
		MemberCount int             `json:"memberCount" doc:"Number of distinct users in the group, from every source"`
	}

	UserGroupLink struct {
		Kind  string `json:"kind" enum:"oidc,ldap" doc:"Directory the value comes from"`
		Value string `json:"value" minLength:"1" maxLength:"255" doc:"OIDC claim value or LDAP group name; matched exactly, like role mappings"`
	}

	UserGroupRef struct {
		ID   string `json:"id" doc:"User group ID"`
		Name string `json:"name" doc:"User group name"`
	}

	UserGroupMember struct {
		DisplayName *string `json:"displayName,omitempty" doc:"Display name of the user"`
		ID          string  `json:"id" doc:"Membership ID"`
		UserID      string  `json:"userId" doc:"ID of the member user"`
		Username    string  `json:"username" doc:"Username of the member user"`
		Source      string  `json:"source" enum:"manual,oidc,ldap" doc:"How the user joined; only manual memberships can be removed"`
	}

	UserGroupRoleAssignment struct {
		EnvironmentID      *string `json:"environmentId,omitempty" doc:"Agent ID the assignment is scoped to; omitted with environmentGroupId for a global assignment"`
		EnvironmentGroupID *string `json:"environmentGroupId,omitempty" doc:"Environment group ID the assignment is scoped to; applies to every agent in the group"`
		ID                 string  `json:"id" doc:"Role assignment ID"`
		RoleID             string  `json:"roleId" doc:"ID of the granted role"`
		RoleName           string  `json:"roleName" doc:"Name of the granted role"`
	}

	CreateUserGroupRequest struct {
		Name        string          `json:"name" minLength:"1" maxLength:"64" doc:"User group name, unique case-insensitively" unorm:"nfc" trim:"true"`
		Description string          `json:"description,omitempty" required:"false" maxLength:"256" doc:"User group description" unorm:"nfc" trim:"true"`
		Links       []UserGroupLink `json:"links,omitempty" required:"false" doc:"SSO claim values and LDAP group names whose users join the group automatically"`
	}

	UpdateUserGroupRequest struct {
		Name        *string          `json:"name,omitempty" required:"false" minLength:"1" maxLength:"64" doc:"User group name, unique case-insensitively" unorm:"nfc" trim:"true"`
		Description *string          `json:"description,omitempty" required:"false" maxLength:"256" doc:"User group description" unorm:"nfc" trim:"true"`
		Links       *[]UserGroupLink `json:"links,omitempty" required:"false" doc:"Replaces every linked value when present"`
	}

	AddUserGroupMemberRequest struct {
		UserID string `json:"userId" minLength:"1" doc:"ID of the user to add as a manual member"`
	}

	CreateUserGroupRoleAssignmentRequest struct {
		EnvironmentID      *string `json:"environmentId,omitempty" required:"false" doc:"Agent ID to scope the assignment to; omit both scopes for a global assignment"`
		EnvironmentGroupID *string `json:"environmentGroupId,omitempty" required:"false" doc:"Environment group ID to scope the assignment to. Mutually exclusive with environmentId"`
		RoleID             string  `json:"roleId" minLength:"1" doc:"ID of the role to grant"`
	}
)
