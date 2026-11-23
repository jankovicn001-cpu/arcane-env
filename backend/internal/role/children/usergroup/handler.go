package usergroup

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

	GroupInput struct {
		ID string `path:"id" doc:"User group ID"`
	}

	CreateInput struct {
		Body types.CreateUserGroupRequest
	}

	UpdateInput struct {
		Body types.UpdateUserGroupRequest
		ID   string `path:"id" doc:"User group ID"`
	}

	AddMemberInput struct {
		Body types.AddUserGroupMemberRequest
		ID   string `path:"id" doc:"User group ID"`
	}

	MemberInput struct {
		ID       string `path:"id" doc:"User group ID"`
		MemberID string `path:"memberId" doc:"Membership ID"`
	}

	CreateGrantInput struct {
		Body types.CreateUserGroupRoleAssignmentRequest
		ID   string `path:"id" doc:"User group ID"`
	}

	GrantInput struct {
		ID           string `path:"id" doc:"User group ID"`
		AssignmentID string `path:"assignmentId" doc:"Role assignment ID"`
	}
)

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) List(ctx context.Context, _ *ListInput) (*handlerutil.Out[[]types.UserGroup], error) {
	groups, err := h.service.List(ctx)
	return output(groups, err)
}

func (h *Handler) Get(ctx context.Context, input *GroupInput) (*handlerutil.Out[types.UserGroup], error) {
	group, err := h.service.Get(ctx, input.ID)
	return output(group, err)
}

func (h *Handler) Create(ctx context.Context, input *CreateInput) (*handlerutil.Out[types.UserGroup], error) {
	group, err := h.service.Create(ctx, input.Body)
	return output(group, err)
}

func (h *Handler) Update(ctx context.Context, input *UpdateInput) (*handlerutil.Out[types.UserGroup], error) {
	group, err := h.service.Update(ctx, input.ID, input.Body)
	return output(group, err)
}

func (h *Handler) Delete(ctx context.Context, input *GroupInput) (*handlerutil.Out[types.MessageResponse], error) {
	if err := h.service.Delete(ctx, input.ID); err != nil {
		return nil, handlerutil.APIError(err)
	}
	return handlerutil.MessageOutput("User group deleted successfully", ""), nil
}

func (h *Handler) ListMembers(ctx context.Context, input *GroupInput) (*handlerutil.Out[[]types.UserGroupMember], error) {
	members, err := h.service.ListMembers(ctx, input.ID)
	return output(members, err)
}

func (h *Handler) AddMember(ctx context.Context, input *AddMemberInput) (*handlerutil.Out[types.UserGroupMember], error) {
	member, err := h.service.AddMember(ctx, input.ID, input.Body)
	return output(member, err)
}

func (h *Handler) RemoveMember(ctx context.Context, input *MemberInput) (*handlerutil.Out[types.MessageResponse], error) {
	if err := h.service.RemoveMember(ctx, input.ID, input.MemberID); err != nil {
		return nil, handlerutil.APIError(err)
	}
	return handlerutil.MessageOutput("Member removed", ""), nil
}

func (h *Handler) ListGrants(ctx context.Context, input *GroupInput) (*handlerutil.Out[[]types.UserGroupRoleAssignment], error) {
	grants, err := h.service.ListGrants(ctx, input.ID)
	return output(grants, err)
}

func (h *Handler) CreateGrant(ctx context.Context, input *CreateGrantInput) (*handlerutil.Out[types.UserGroupRoleAssignment], error) {
	grant, err := h.service.CreateGrant(ctx, input.ID, input.Body)
	return output(grant, err)
}

func (h *Handler) DeleteGrant(ctx context.Context, input *GrantInput) (*handlerutil.Out[types.MessageResponse], error) {
	if err := h.service.DeleteGrant(ctx, input.ID, input.AssignmentID); err != nil {
		return nil, handlerutil.APIError(err)
	}
	return handlerutil.MessageOutput("Access removed", ""), nil
}

func output[T any](data T, err error) (*handlerutil.Out[T], error) {
	if err != nil {
		return nil, handlerutil.APIError(err)
	}
	return &handlerutil.Out[T]{Body: types.ApiResponse[T]{Success: true, Data: data}}, nil
}
