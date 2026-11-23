package version

import (
	"github.com/danielgtaylor/huma/v2"
)

func RegisterVersion(api huma.API, versionService *VersionService) {
	h := &VersionHandler{versionService: versionService}

	huma.Register(api, huma.Operation{
		OperationID: "getAppVersion",
		Method:      "GET",
		Path:        "/app/version",
		Summary:     "Get app version",
		Description: "Get the current application version",
		Tags:        []string{"Version"},
		Security:    []map[string][]string{},
	}, h.GetAppVersion)
}
