package version

import (
	"context"

	"getarcane.dev/types/v3"
)

type (
	VersionHandler struct {
		versionService *VersionService
	}

	GetAppVersionInput struct{}

	GetAppVersionOutput struct {
		Body types.VersionInfo
	}
)

func (h *VersionHandler) GetAppVersion(ctx context.Context, _ *GetAppVersionInput) (*GetAppVersionOutput, error) {
	info := h.versionService.GetAppVersionInfo(ctx)

	return &GetAppVersionOutput{
		Body: *info,
	}, nil
}
