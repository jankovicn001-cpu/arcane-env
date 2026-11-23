package role

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"getarcane.dev/arcane/v3/internal/middleware"
	"getarcane.dev/arcane/v3/internal/role/children/usergroup"
	"getarcane.dev/arcane/v3/pkg/authz"
	"getarcane.dev/arcane/v3/pkg/handlerutil"
)

type (
	Module struct {
		service *RoleService
	}
)

func New(service *RoleService) *Module {
	return &Module{service: service}
}

func (m *Module) Service() *RoleService {
	if m == nil {
		return nil
	}
	return m.service
}

func (m *Module) RegisterRoutes(api huma.API) {
	if m == nil {
		RegisterRoles(api, nil)
		usergroup.RegisterRoutes(api, usergroup.NewHandler(nil))
		return
	}
	RegisterRoles(api, m.service)
	usergroup.RegisterRoutes(api, usergroup.NewHandler(m.service.userGroups))
}

func RegisterRoles(api huma.API, roleService *RoleService) {
	h := &RoleHandler{roleService: roleService}

	middleware.RegisterWithPermission(api, huma.Operation{
		OperationID: "list-roles",
		Method:      http.MethodGet,
		Path:        "/roles",
		Summary:     "List roles",
		Description: "Get a paginated list of roles (built-in + custom)",
		Tags:        []string{"Roles"},
		Security:    handlerutil.DefaultOperationSecurity(),
	}, authz.PermRolesList, h.ListRoles)

	middleware.RegisterWithPermission(api, huma.Operation{
		OperationID: "get-role",
		Method:      http.MethodGet,
		Path:        "/roles/{id}",
		Summary:     "Get a role",
		Tags:        []string{"Roles"},
		Security:    handlerutil.DefaultOperationSecurity(),
	}, authz.PermRolesRead, h.GetRole)

	huma.Register(api, middleware.RequireGlobalAdmin(api, huma.Operation{
		OperationID: "create-role",
		Method:      http.MethodPost,
		Path:        "/roles",
		Summary:     "Create a custom role",
		Description: "Built-in roles cannot be created via this endpoint; only custom roles are accepted. Reserved for global admins.",
		Tags:        []string{"Roles"},
		Security:    handlerutil.DefaultOperationSecurity(),
	}), h.CreateRole)

	huma.Register(api, middleware.RequireGlobalAdmin(api, huma.Operation{
		OperationID: "update-role",
		Method:      http.MethodPatch,
		Path:        "/roles/{id}",
		Summary:     "Update a custom role",
		Description: "Built-in roles are read-only and return 403 on update. Reserved for global admins.",
		Tags:        []string{"Roles"},
		Security:    handlerutil.DefaultOperationSecurity(),
	}), h.UpdateRole)

	huma.Register(api, middleware.RequireGlobalAdmin(api, huma.Operation{
		OperationID: "delete-role",
		Method:      http.MethodDelete,
		Path:        "/roles/{id}",
		Summary:     "Delete a custom role",
		Description: "Built-in roles are protected; deleting cascades all user assignments. Reserved for global admins.",
		Tags:        []string{"Roles"},
		Security:    handlerutil.DefaultOperationSecurity(),
	}), h.DeleteRole)

	huma.Register(api, huma.Operation{
		OperationID: "get-permissions-manifest",
		Method:      http.MethodGet,
		Path:        "/roles/permissions",
		Summary:     "Get the permission manifest",
		Description: "Returns every permission the server recognizes, grouped by resource. Used by permission-picking UIs.",
		Tags:        []string{"Roles"},
		Security:    handlerutil.DefaultOperationSecurity(),
	}, h.GetPermissionsManifest)

	huma.Register(api, middleware.RequireGlobalAdmin(api, huma.Operation{
		OperationID: "list-user-role-assignments",
		Method:      http.MethodGet,
		Path:        "/users/{userId}/roles",
		Summary:     "List a user's role assignments",
		Description: "Reserved for global admins.",
		Tags:        []string{"Roles"},
		Security:    handlerutil.DefaultOperationSecurity(),
	}), h.ListUserRoleAssignments)

	huma.Register(api, middleware.RequireGlobalAdmin(api, huma.Operation{
		OperationID: "set-user-role-assignments",
		Method:      http.MethodPut,
		Path:        "/users/{userId}/roles",
		Summary:     "Replace a user's manual role assignments",
		Description: "Replaces every source='manual' assignment for the user. source='oidc' assignments are not touched. Reserved for global admins; enforces the last-admin guard.",
		Tags:        []string{"Roles"},
		Security:    handlerutil.DefaultOperationSecurity(),
	}), h.SetUserRoleAssignments)

	huma.Register(api, middleware.RequireGlobalAdmin(api, huma.Operation{
		OperationID: "list-environment-group-role-assignments",
		Method:      http.MethodGet,
		Path:        "/environment-groups/{id}/role-assignments",
		Summary:     "List access to an environment group",
		Description: "Lists every role assignment (manual, OIDC and LDAP) and user group grant scoped to the environment group; users first by username, then groups by name. Reserved for global admins.",
		Tags:        []string{"Roles"},
		Security:    handlerutil.DefaultOperationSecurity(),
	}), h.ListEnvironmentGroupAccess)

	huma.Register(api, middleware.RequireGlobalAdmin(api, huma.Operation{
		OperationID: "create-environment-group-role-assignment",
		Method:      http.MethodPost,
		Path:        "/environment-groups/{id}/role-assignments",
		Summary:     "Grant access to an environment group",
		Description: "Grants a user or a user group a role on every agent in the environment group; set exactly one of userId and userGroupId. Reserved for global admins.",
		Tags:        []string{"Roles"},
		Security:    handlerutil.DefaultOperationSecurity(),
	}), h.GrantEnvironmentGroupAccess)

	huma.Register(api, middleware.RequireGlobalAdmin(api, huma.Operation{
		OperationID: "delete-environment-group-role-assignment",
		Method:      http.MethodDelete,
		Path:        "/environment-groups/{id}/role-assignments/{assignmentId}",
		Summary:     "Revoke access to an environment group",
		Description: "Removes a manual user assignment or a user group grant scoped to the environment group. Synced user assignments are rejected. Reserved for global admins.",
		Tags:        []string{"Roles"},
		Security:    handlerutil.DefaultOperationSecurity(),
	}), h.RevokeEnvironmentGroupAccess)
}
