package usergroup

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"getarcane.dev/arcane/v3/internal/middleware"
	"getarcane.dev/arcane/v3/pkg/authz"
	"getarcane.dev/arcane/v3/pkg/handlerutil"
)

// RegisterRoutes registers user group reads behind global users:list and every write behind global admin.
func RegisterRoutes(api huma.API, h *Handler) {
	tags := []string{"User Groups"}

	middleware.RegisterWithPermission(api, huma.Operation{
		OperationID: "list-user-groups",
		Method:      http.MethodGet,
		Path:        "/user-groups",
		Summary:     "List user groups",
		Description: "List user groups sorted by name, with member counts and linked SSO and LDAP values",
		Tags:        tags,
		Security:    handlerutil.DefaultOperationSecurity(),
	}, authz.PermUsersList, h.List)

	middleware.RegisterWithPermission(api, huma.Operation{
		OperationID: "get-user-group",
		Method:      http.MethodGet,
		Path:        "/user-groups/{id}",
		Summary:     "Get a user group",
		Tags:        tags,
		Security:    handlerutil.DefaultOperationSecurity(),
	}, authz.PermUsersList, h.Get)

	huma.Register(api, middleware.RequireGlobalAdmin(api, huma.Operation{
		OperationID: "create-user-group",
		Method:      http.MethodPost,
		Path:        "/user-groups",
		Summary:     "Create a user group",
		Description: "Reserved for global admins.",
		Tags:        tags,
		Security:    handlerutil.DefaultOperationSecurity(),
	}), h.Create)

	huma.Register(api, middleware.RequireGlobalAdmin(api, huma.Operation{
		OperationID: "update-user-group",
		Method:      http.MethodPatch,
		Path:        "/user-groups/{id}",
		Summary:     "Update a user group",
		Description: "links, when present, replaces every linked value. Reserved for global admins.",
		Tags:        tags,
		Security:    handlerutil.DefaultOperationSecurity(),
	}), h.Update)

	huma.Register(api, middleware.RequireGlobalAdmin(api, huma.Operation{
		OperationID: "delete-user-group",
		Method:      http.MethodDelete,
		Path:        "/user-groups/{id}",
		Summary:     "Delete a user group",
		Description: "Deletes the group with its links, members and role assignments. Reserved for global admins.",
		Tags:        tags,
		Security:    handlerutil.DefaultOperationSecurity(),
	}), h.Delete)

	middleware.RegisterWithPermission(api, huma.Operation{
		OperationID: "list-user-group-members",
		Method:      http.MethodGet,
		Path:        "/user-groups/{id}/members",
		Summary:     "List user group members",
		Description: "Lists manual, SSO and LDAP memberships sorted by username",
		Tags:        tags,
		Security:    handlerutil.DefaultOperationSecurity(),
	}, authz.PermUsersList, h.ListMembers)

	huma.Register(api, middleware.RequireGlobalAdmin(api, huma.Operation{
		OperationID: "add-user-group-member",
		Method:      http.MethodPost,
		Path:        "/user-groups/{id}/members",
		Summary:     "Add a user group member",
		Description: "Adds the user as a manual member. Reserved for global admins.",
		Tags:        tags,
		Security:    handlerutil.DefaultOperationSecurity(),
	}), h.AddMember)

	huma.Register(api, middleware.RequireGlobalAdmin(api, huma.Operation{
		OperationID: "remove-user-group-member",
		Method:      http.MethodDelete,
		Path:        "/user-groups/{id}/members/{memberId}",
		Summary:     "Remove a user group member",
		Description: "Removes a manual membership. SSO and LDAP memberships follow the user's directory groups and are rejected. Reserved for global admins.",
		Tags:        tags,
		Security:    handlerutil.DefaultOperationSecurity(),
	}), h.RemoveMember)

	middleware.RegisterWithPermission(api, huma.Operation{
		OperationID: "list-user-group-role-assignments",
		Method:      http.MethodGet,
		Path:        "/user-groups/{id}/role-assignments",
		Summary:     "List a user group's role assignments",
		Tags:        tags,
		Security:    handlerutil.DefaultOperationSecurity(),
	}, authz.PermUsersList, h.ListGrants)

	huma.Register(api, middleware.RequireGlobalAdmin(api, huma.Operation{
		OperationID: "create-user-group-role-assignment",
		Method:      http.MethodPost,
		Path:        "/user-groups/{id}/role-assignments",
		Summary:     "Grant a role to a user group",
		Description: "Grants the role globally, on one agent, or on every agent of an environment group to every member. Reserved for global admins.",
		Tags:        tags,
		Security:    handlerutil.DefaultOperationSecurity(),
	}), h.CreateGrant)

	huma.Register(api, middleware.RequireGlobalAdmin(api, huma.Operation{
		OperationID: "delete-user-group-role-assignment",
		Method:      http.MethodDelete,
		Path:        "/user-groups/{id}/role-assignments/{assignmentId}",
		Summary:     "Revoke a role from a user group",
		Description: "Reserved for global admins.",
		Tags:        tags,
		Security:    handlerutil.DefaultOperationSecurity(),
	}), h.DeleteGrant)
}
