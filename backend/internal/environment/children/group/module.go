package group

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"getarcane.dev/arcane/v3/internal/middleware"
	"getarcane.dev/arcane/v3/pkg/authz"
	"getarcane.dev/arcane/v3/pkg/handlerutil"
)

func RegisterRoutes(api huma.API, h *Handler) {
	middleware.RegisterWithPermission(api, huma.Operation{
		OperationID: "listEnvironmentGroups",
		Method:      http.MethodGet,
		Path:        "/environment-groups",
		Summary:     "List environment groups",
		Description: "List the environment groups agents can be assigned to, sorted by name",
		Tags:        []string{"Environment Groups"},
		Security:    handlerutil.DefaultOperationSecurity(),
	}, authz.PermEnvironmentsList, h.List)

	middleware.RegisterWithPermission(api, huma.Operation{
		OperationID: "getEnvironmentGroup",
		Method:      http.MethodGet,
		Path:        "/environment-groups/{id}",
		Summary:     "Get an environment group",
		Tags:        []string{"Environment Groups"},
		Security:    handlerutil.DefaultOperationSecurity(),
	}, authz.PermEnvironmentsRead, h.Get)

	middleware.RegisterWithPermission(api, huma.Operation{
		OperationID: "createEnvironmentGroup",
		Method:      http.MethodPost,
		Path:        "/environment-groups",
		Summary:     "Create an environment group",
		Tags:        []string{"Environment Groups"},
		Security:    handlerutil.DefaultOperationSecurity(),
	}, authz.PermEnvironmentsCreate, h.Create)

	middleware.RegisterWithPermission(api, huma.Operation{
		OperationID: "updateEnvironmentGroup",
		Method:      http.MethodPatch,
		Path:        "/environment-groups/{id}",
		Summary:     "Update an environment group",
		Tags:        []string{"Environment Groups"},
		Security:    handlerutil.DefaultOperationSecurity(),
	}, authz.PermEnvironmentsUpdate, h.Update)

	middleware.RegisterWithPermission(api, huma.Operation{
		OperationID: "deleteEnvironmentGroup",
		Method:      http.MethodDelete,
		Path:        "/environment-groups/{id}",
		Summary:     "Delete an environment group",
		Description: "Delete an environment group; its agents become unassigned",
		Tags:        []string{"Environment Groups"},
		Security:    handlerutil.DefaultOperationSecurity(),
	}, authz.PermEnvironmentsDelete, h.Delete)
}
