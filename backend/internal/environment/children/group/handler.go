package group

import (
	"context"

	"getarcane.dev/arcane/v3/pkg/handlerutil"
	"getarcane.dev/types/v3"
)

type (
	Handler struct {
		service *Service
	}

	ListInput struct{}

	GetInput struct {
		ID string `path:"id" doc:"Environment group ID"`
	}

	CreateInput struct {
		Body types.CreateEnvironmentGroupRequest
	}

	UpdateInput struct {
		Body types.UpdateEnvironmentGroupRequest
		ID   string `path:"id" doc:"Environment group ID"`
	}

	DeleteInput struct {
		ID string `path:"id" doc:"Environment group ID"`
	}
)

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) List(ctx context.Context, _ *ListInput) (*handlerutil.Out[[]types.EnvironmentGroup], error) {
	groups, err := h.service.List(ctx)
	if err != nil {
		return nil, handlerutil.APIError(err)
	}
	return &handlerutil.Out[[]types.EnvironmentGroup]{Body: types.ApiResponse[[]types.EnvironmentGroup]{Success: true, Data: groups}}, nil
}

func (h *Handler) Get(ctx context.Context, input *GetInput) (*handlerutil.Out[types.EnvironmentGroup], error) {
	group, err := h.service.Get(ctx, input.ID)
	if err != nil {
		return nil, handlerutil.APIError(err)
	}
	return &handlerutil.Out[types.EnvironmentGroup]{Body: types.ApiResponse[types.EnvironmentGroup]{Success: true, Data: group}}, nil
}

func (h *Handler) Create(ctx context.Context, input *CreateInput) (*handlerutil.Out[types.EnvironmentGroup], error) {
	group, err := h.service.Create(ctx, input.Body)
	if err != nil {
		return nil, handlerutil.APIError(err)
	}
	return &handlerutil.Out[types.EnvironmentGroup]{Body: types.ApiResponse[types.EnvironmentGroup]{Success: true, Data: group}}, nil
}

func (h *Handler) Update(ctx context.Context, input *UpdateInput) (*handlerutil.Out[types.EnvironmentGroup], error) {
	group, err := h.service.Update(ctx, input.ID, input.Body)
	if err != nil {
		return nil, handlerutil.APIError(err)
	}
	return &handlerutil.Out[types.EnvironmentGroup]{Body: types.ApiResponse[types.EnvironmentGroup]{Success: true, Data: group}}, nil
}

func (h *Handler) Delete(ctx context.Context, input *DeleteInput) (*handlerutil.Out[types.MessageResponse], error) {
	if err := h.service.Delete(ctx, input.ID); err != nil {
		return nil, handlerutil.APIError(err)
	}
	return handlerutil.MessageOutput("Environment deleted successfully", ""), nil
}
