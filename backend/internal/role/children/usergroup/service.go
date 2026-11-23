package usergroup

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"uuid"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"kit.getarcane.app"

	"getarcane.dev/arcane/v3/internal/common"
	"getarcane.dev/arcane/v3/internal/database"
	"getarcane.dev/types/v3"
)

type (
	Service struct {
		db    *database.DB
		hooks Hooks
	}

	Hooks struct {
		// InvalidateUser drops one user's cached permissions after their memberships change.
		InvalidateUser func(userID string)
		// InvalidateAllUsers drops every cached permission set after a group's grants, links or existence change.
		InvalidateAllUsers func()
	}

	// ScopedGrant is a role grant a user holds through a group, read for permission resolution.
	ScopedGrant struct {
		EnvironmentID      *string `gorm:"column:environment_id"`
		EnvironmentGroupID *string `gorm:"column:environment_group_id"`
		Permissions        string  `gorm:"column:permissions"`
	}

	countRow struct {
		UserGroupID string `gorm:"column:user_group_id"`
		Count       int    `gorm:"column:count"`
	}

	memberRow struct {
		DisplayName *string `gorm:"column:display_name"`
		ID          string  `gorm:"column:id"`
		UserID      string  `gorm:"column:user_id"`
		Username    string  `gorm:"column:username"`
		Source      string  `gorm:"column:source"`
	}

	grantRow struct {
		EnvironmentID      *string `gorm:"column:environment_id"`
		EnvironmentGroupID *string `gorm:"column:environment_group_id"`
		ID                 string  `gorm:"column:id"`
		UserGroupID        string  `gorm:"column:user_group_id"`
		UserGroupName      string  `gorm:"column:user_group_name"`
		RoleID             string  `gorm:"column:role_id"`
		RoleName           string  `gorm:"column:role_name"`
	}
)

const (
	sourceManual = "manual"
	sourceOidc   = "oidc"
	sourceLdap   = "ldap"

	maxNameLength        = 64
	maxDescriptionLength = 256
	maxLinkValueLength   = 255
)

var (
	ErrGroupNotFound  = common.Classify(common.ErrNotFound, errors.New("user group not found"))
	ErrMemberNotFound = common.Classify(common.ErrNotFound, errors.New("user group member not found"))
	ErrMemberExists   = common.Classify(common.ErrConflict, errors.New("the user is already a manual member of this group"))
	ErrGrantNotFound  = common.Classify(common.ErrNotFound, errors.New("user group role assignment not found"))
	ErrGrantExists    = common.Classify(common.ErrConflict, errors.New("the user group already holds this role on this scope"))
)

func New(db *database.DB, hooks Hooks) *Service {
	return &Service{db: db, hooks: hooks}
}

func (s *Service) List(ctx context.Context) ([]types.UserGroup, error) {
	db := s.db.WithContext(ctx)
	var groups []Group
	if err := db.Find(&groups).Error; err != nil {
		return nil, fmt.Errorf("failed to list user groups: %w", err)
	}
	slices.SortFunc(groups, func(a, b Group) int {
		return cmp.Or(strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)), strings.Compare(a.ID, b.ID))
	})
	return toDTOs(db, groups)
}

func (s *Service) Get(ctx context.Context, id string) (types.UserGroup, error) {
	db := s.db.WithContext(ctx)
	var group Group
	if err := db.Where("id = ?", id).Take(&group).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return types.UserGroup{}, ErrGroupNotFound
		}
		return types.UserGroup{}, fmt.Errorf("failed to get user group: %w", err)
	}
	dtos, err := toDTOs(db, []Group{group})
	if err != nil {
		return types.UserGroup{}, err
	}
	return dtos[0], nil
}

func (s *Service) Create(ctx context.Context, req types.CreateUserGroupRequest) (types.UserGroup, error) {
	name, err := normalizeText("name", req.Name, 1, maxNameLength)
	if err != nil {
		return types.UserGroup{}, err
	}
	description, err := normalizeText("description", req.Description, 0, maxDescriptionLength)
	if err != nil {
		return types.UserGroup{}, err
	}
	links, err := normalizeLinks(req.Links)
	if err != nil {
		return types.UserGroup{}, err
	}
	now := time.Now()
	group := Group{
		BaseModel: database.BaseModel{
			ID:        uuid.New().String(),
			CreatedAt: now,
			UpdatedAt: new(now),
		},
		Name:        name,
		Description: description,
	}
	if createErr := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if takenErr := ensureNameAvailable(tx, name, ""); takenErr != nil {
			return takenErr
		}
		if insertErr := tx.Create(&group).Error; insertErr != nil {
			return insertErr
		}
		return replaceLinks(tx, group.ID, links)
	}); createErr != nil {
		return types.UserGroup{}, s.writeError("create", name, createErr)
	}
	return s.Get(ctx, group.ID)
}

func (s *Service) Update(ctx context.Context, id string, req types.UpdateUserGroupRequest) (types.UserGroup, error) {
	updates := map[string]any{"updated_at": time.Now()}
	var name string
	if req.Name != nil {
		normalized, err := normalizeText("name", *req.Name, 1, maxNameLength)
		if err != nil {
			return types.UserGroup{}, err
		}
		name = normalized
		updates["name"] = normalized
	}
	if req.Description != nil {
		description, err := normalizeText("description", *req.Description, 0, maxDescriptionLength)
		if err != nil {
			return types.UserGroup{}, err
		}
		updates["description"] = description
	}
	var links []types.UserGroupLink
	if req.Links != nil {
		normalized, err := normalizeLinks(*req.Links)
		if err != nil {
			return types.UserGroup{}, err
		}
		links = normalized
	}
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ?", id).Take(&Group{}).Error; err != nil {
			return err
		}
		if name != "" {
			if takenErr := ensureNameAvailable(tx, name, id); takenErr != nil {
				return takenErr
			}
		}
		if err := tx.Model(&Group{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			return err
		}
		if req.Links == nil {
			return nil
		}
		return replaceLinks(tx, id, links)
	}); err != nil {
		return types.UserGroup{}, s.writeError("update", name, err)
	}
	if req.Links != nil {
		s.hooks.InvalidateAllUsers()
	}
	return s.Get(ctx, id)
}

// Delete removes the group's grants, members and links explicitly so no grant outlives it, even without foreign key enforcement.
func (s *Service) Delete(ctx context.Context, id string) error {
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, model := range []any{&Grant{}, &Member{}, &Link{}} {
			if err := tx.Where("user_group_id = ?", id).Delete(model).Error; err != nil {
				return fmt.Errorf("failed to delete user group data: %w", err)
			}
		}
		result := tx.Delete(&Group{}, "id = ?", id)
		if result.Error != nil {
			return fmt.Errorf("failed to delete user group: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return ErrGroupNotFound
		}
		return nil
	}); err != nil {
		return err
	}
	s.hooks.InvalidateAllUsers()
	return nil
}

func (s *Service) ListMembers(ctx context.Context, groupID string) ([]types.UserGroupMember, error) {
	db := s.db.WithContext(ctx)
	if err := ensureGroupExists(db, groupID); err != nil {
		return nil, err
	}
	return queryMembers(db, "m.user_group_id = ?", groupID)
}

// AddMember adds a manual membership; synced memberships of the same user are kept beside it.
func (s *Service) AddMember(ctx context.Context, groupID string, req types.AddUserGroupMemberRequest) (types.UserGroupMember, error) {
	userID := strings.TrimSpace(req.UserID)
	var member types.UserGroupMember
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureGroupExists(tx, groupID); err != nil {
			return err
		}
		if err := lockUser(tx, userID); err != nil {
			return err
		}
		var duplicates int64
		if err := tx.Model(&Member{}).
			Where("user_group_id = ? AND user_id = ? AND source = ?", groupID, userID, sourceManual).
			Count(&duplicates).Error; err != nil {
			return fmt.Errorf("failed to check existing membership: %w", err)
		}
		if duplicates > 0 {
			return ErrMemberExists
		}
		created := Member{UserGroupID: groupID, UserID: userID, Source: sourceManual}
		if err := tx.Create(&created).Error; err != nil {
			if database.IsDuplicateKeyError(tx, err) {
				return ErrMemberExists
			}
			return fmt.Errorf("failed to add user group member: %w", err)
		}
		rows, err := queryMembers(tx, "m.id = ?", created.ID)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return fmt.Errorf("failed to read back member %s", created.ID)
		}
		member = rows[0]
		return nil
	})
	if err != nil {
		return types.UserGroupMember{}, err
	}
	s.hooks.InvalidateUser(userID)
	return member, nil
}

// RemoveMember deletes a manual membership; synced ones follow the user's directory groups.
func (s *Service) RemoveMember(ctx context.Context, groupID, memberID string) error {
	var userID string
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureGroupExists(tx, groupID); err != nil {
			return err
		}
		var existing Member
		if err := tx.Where("id = ? AND user_group_id = ?", memberID, groupID).Take(&existing).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrMemberNotFound
			}
			return fmt.Errorf("failed to load user group member: %w", err)
		}
		if existing.Source != sourceManual {
			return common.Classify(common.ErrConflict, fmt.Errorf(
				"this membership is synced from %s and cannot be removed here; change the group's linked values or the user's directory groups instead",
				syncedSourceLabel(existing.Source),
			))
		}
		if err := tx.Delete(&Member{}, "id = ?", existing.ID).Error; err != nil {
			return fmt.Errorf("failed to remove user group member: %w", err)
		}
		userID = existing.UserID
		return nil
	})
	if err != nil {
		return err
	}
	s.hooks.InvalidateUser(userID)
	return nil
}

func (s *Service) ListGrants(ctx context.Context, groupID string) ([]types.UserGroupRoleAssignment, error) {
	db := s.db.WithContext(ctx)
	if err := ensureGroupExists(db, groupID); err != nil {
		return nil, err
	}
	rows, err := queryGrants(db, "g.user_group_id = ?", groupID)
	if err != nil {
		return nil, err
	}
	out := make([]types.UserGroupRoleAssignment, len(rows))
	for i, r := range rows {
		out[i] = toGrantDTO(r)
	}
	return out, nil
}

func (s *Service) CreateGrant(ctx context.Context, groupID string, req types.CreateUserGroupRoleAssignmentRequest) (types.UserGroupRoleAssignment, error) {
	var out types.UserGroupRoleAssignment
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureGroupExists(tx, groupID); err != nil {
			return err
		}
		row, err := insertGrant(tx, Grant{
			EnvironmentID:      req.EnvironmentID,
			EnvironmentGroupID: req.EnvironmentGroupID,
			UserGroupID:        groupID,
			RoleID:             strings.TrimSpace(req.RoleID),
		})
		if err != nil {
			return err
		}
		out = toGrantDTO(row)
		return nil
	})
	if err != nil {
		return types.UserGroupRoleAssignment{}, err
	}
	s.hooks.InvalidateAllUsers()
	return out, nil
}

func (s *Service) DeleteGrant(ctx context.Context, groupID, grantID string) error {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureGroupExists(tx, groupID); err != nil {
			return err
		}
		result := tx.Delete(&Grant{}, "id = ? AND user_group_id = ?", grantID, groupID)
		if result.Error != nil {
			return fmt.Errorf("failed to delete user group role assignment: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return ErrGrantNotFound
		}
		return nil
	})
	if err != nil {
		return err
	}
	s.hooks.InvalidateAllUsers()
	return nil
}

// ScopedGrants returns the grants a user holds through every membership, whatever its source.
func (s *Service) ScopedGrants(tx *gorm.DB, userID string) ([]ScopedGrant, error) {
	var grants []ScopedGrant
	if err := tx.Table("user_group_role_assignments AS g").
		Select("r.permissions AS permissions, g.environment_id AS environment_id, g.environment_group_id AS environment_group_id").
		Joins("INNER JOIN roles r ON r.id = g.role_id").
		Where("g.user_group_id IN (SELECT m.user_group_id FROM user_group_members m WHERE m.user_id = ?)", userID).
		Scan(&grants).Error; err != nil {
		return nil, fmt.Errorf("failed to resolve user group permissions: %w", err)
	}
	return grants, nil
}

// MemberIDsWithRole lists the users holding the role through a group, so a role change can invalidate them.
func (s *Service) MemberIDsWithRole(tx *gorm.DB, roleID string) ([]string, error) {
	var ids []string
	if err := tx.Table("user_group_members AS m").
		Joins("INNER JOIN user_group_role_assignments g ON g.user_group_id = m.user_group_id").
		Where("g.role_id = ?", roleID).
		Distinct("m.user_id").
		Pluck("m.user_id", &ids).Error; err != nil {
		return nil, fmt.Errorf("failed to list user group members holding role: %w", err)
	}
	return ids, nil
}

// DeleteRoleGrants removes every group grant of the role, so deleting a role never leaves a grant without one.
func (s *Service) DeleteRoleGrants(tx *gorm.DB, roleID string) error {
	if err := tx.Where("role_id = ?", roleID).Delete(&Grant{}).Error; err != nil {
		return fmt.Errorf("failed to delete user group role assignments: %w", err)
	}
	return nil
}

// Refs lists the groups the user belongs to through any source.
func (s *Service) Refs(ctx context.Context, userID string) ([]types.UserGroupRef, error) {
	var refs []types.UserGroupRef
	if err := s.db.WithContext(ctx).Table("user_groups AS ug").
		Select("ug.id, ug.name").
		Where("ug.id IN (SELECT m.user_group_id FROM user_group_members m WHERE m.user_id = ?)", userID).
		Scan(&refs).Error; err != nil {
		return nil, fmt.Errorf("failed to list user groups of user: %w", err)
	}
	slices.SortFunc(refs, func(a, b types.UserGroupRef) int {
		return cmp.Or(strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)), strings.Compare(a.ID, b.ID))
	})
	return refs, nil
}

// ReplaceSyncedMemberships makes the user's memberships from source match the groups linked to values.
// Values match exactly, like role mappings.
func (s *Service) ReplaceSyncedMemberships(ctx context.Context, tx *gorm.DB, userID, source string, values []string) error {
	if source != sourceOidc && source != sourceLdap {
		return fmt.Errorf("unsupported user group membership source %q", source)
	}
	tx = tx.WithContext(ctx)
	if err := lockUser(tx, userID); err != nil {
		return err
	}
	var groupIDs []string
	if len(values) > 0 {
		if err := tx.Model(&Link{}).
			Where("kind = ? AND value IN ?", source, values).
			Distinct("user_group_id").
			Pluck("user_group_id", &groupIDs).Error; err != nil {
			return fmt.Errorf("failed to match user group links: %w", err)
		}
	}
	if err := tx.Where("user_id = ? AND source = ?", userID, source).Delete(&Member{}).Error; err != nil {
		return fmt.Errorf("failed to clear %s user group memberships: %w", source, err)
	}
	if len(groupIDs) == 0 {
		return nil
	}
	members := make([]Member, len(groupIDs))
	for i, groupID := range groupIDs {
		members[i] = Member{UserGroupID: groupID, UserID: userID, Source: source}
	}
	if err := tx.Create(&members).Error; err != nil {
		return fmt.Errorf("failed to insert %s user group memberships: %w", source, err)
	}
	return nil
}

// ListEnvironmentGroupAccess returns the group grants scoped to the environment group, sorted by group name.
func (s *Service) ListEnvironmentGroupAccess(tx *gorm.DB, environmentGroupID string) ([]types.EnvironmentAccessEntry, error) {
	rows, err := queryGrants(tx, "g.environment_group_id = ?", environmentGroupID)
	if err != nil {
		return nil, err
	}
	slices.SortFunc(rows, func(a, b grantRow) int {
		return cmp.Or(
			strings.Compare(strings.ToLower(a.UserGroupName), strings.ToLower(b.UserGroupName)),
			strings.Compare(a.RoleName, b.RoleName),
			strings.Compare(a.ID, b.ID),
		)
	})
	entries := make([]types.EnvironmentAccessEntry, len(rows))
	for i, r := range rows {
		entries[i] = toAccessEntry(r)
	}
	return entries, nil
}

// GrantEnvironmentGroupAccess grants the user group a role on every agent of the environment group.
func (s *Service) GrantEnvironmentGroupAccess(tx *gorm.DB, environmentGroupID, userGroupID, roleID string) (types.EnvironmentAccessEntry, error) {
	if err := ensureGroupExists(tx, userGroupID); err != nil {
		if errors.Is(err, ErrGroupNotFound) {
			return types.EnvironmentAccessEntry{}, fieldError(common.ErrValidation, "userGroupId", fmt.Errorf("user group %q does not exist", userGroupID))
		}
		return types.EnvironmentAccessEntry{}, err
	}
	row, err := insertGrant(tx, Grant{EnvironmentGroupID: &environmentGroupID, UserGroupID: userGroupID, RoleID: roleID})
	if err != nil {
		return types.EnvironmentAccessEntry{}, err
	}
	return toAccessEntry(row), nil
}

// RevokeEnvironmentGroupAccess deletes a group grant scoped to the environment group and reports whether one existed.
func (s *Service) RevokeEnvironmentGroupAccess(tx *gorm.DB, environmentGroupID, grantID string) (bool, error) {
	result := tx.Delete(&Grant{}, "id = ? AND environment_group_id = ?", grantID, environmentGroupID)
	if result.Error != nil {
		return false, fmt.Errorf("failed to delete user group role assignment: %w", result.Error)
	}
	return result.RowsAffected > 0, nil
}

func (s *Service) writeError(action, name string, err error) error {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return ErrGroupNotFound
	case database.IsDuplicateKeyError(s.db.DB, err):
		return nameTakenError(name)
	case errors.Is(err, common.ErrConflict), errors.Is(err, common.ErrValidation):
		return err
	default:
		return fmt.Errorf("failed to %s user group: %w", action, err)
	}
}

func toDTOs(tx *gorm.DB, groups []Group) ([]types.UserGroup, error) {
	out := make([]types.UserGroup, len(groups))
	if len(groups) == 0 {
		return out, nil
	}
	ids := make([]string, len(groups))
	for i, group := range groups {
		ids[i] = group.ID
	}
	var counts []countRow
	if err := tx.Model(&Member{}).
		Select("user_group_id, COUNT(DISTINCT user_id) AS count").
		Where("user_group_id IN ?", ids).
		Group("user_group_id").
		Scan(&counts).Error; err != nil {
		return nil, fmt.Errorf("failed to count user group members: %w", err)
	}
	var links []Link
	if err := tx.Where("user_group_id IN ?", ids).Find(&links).Error; err != nil {
		return nil, fmt.Errorf("failed to list user group links: %w", err)
	}
	slices.SortFunc(links, func(a, b Link) int {
		return cmp.Or(strings.Compare(a.Kind, b.Kind), strings.Compare(strings.ToLower(a.Value), strings.ToLower(b.Value)))
	})
	for i, group := range groups {
		groupLinks := make([]types.UserGroupLink, 0)
		for _, link := range links {
			if link.UserGroupID == group.ID {
				groupLinks = append(groupLinks, types.UserGroupLink{Kind: link.Kind, Value: link.Value})
			}
		}
		memberCount := 0
		if idx := slices.IndexFunc(counts, func(c countRow) bool { return c.UserGroupID == group.ID }); idx >= 0 {
			memberCount = counts[idx].Count
		}
		out[i] = types.UserGroup{
			CreatedAt:   group.CreatedAt,
			UpdatedAt:   *cmp.Or(group.UpdatedAt, &group.CreatedAt),
			ID:          group.ID,
			Name:        group.Name,
			Description: group.Description,
			Links:       groupLinks,
			MemberCount: memberCount,
		}
	}
	return out, nil
}

// insertGrant validates the role and scope, then creates the grant; a scope that cannot be verified is rejected, never widened to global.
func insertGrant(tx *gorm.DB, grant Grant) (grantRow, error) {
	if err := validateGrant(tx, grant); err != nil {
		return grantRow{}, err
	}
	var duplicates int64
	if err := tx.Model(&Grant{}).
		Where("user_group_id = ? AND role_id = ? AND COALESCE(environment_id, '') = ? AND COALESCE(environment_group_id, '') = ?",
			grant.UserGroupID, grant.RoleID, kit.FromPtr(grant.EnvironmentID), kit.FromPtr(grant.EnvironmentGroupID)).
		Count(&duplicates).Error; err != nil {
		return grantRow{}, fmt.Errorf("failed to check existing user group role assignments: %w", err)
	}
	if duplicates > 0 {
		return grantRow{}, ErrGrantExists
	}
	if err := tx.Create(&grant).Error; err != nil {
		if database.IsDuplicateKeyError(tx, err) {
			return grantRow{}, ErrGrantExists
		}
		return grantRow{}, fmt.Errorf("failed to create user group role assignment: %w", err)
	}
	rows, err := queryGrants(tx, "g.id = ?", grant.ID)
	if err != nil {
		return grantRow{}, err
	}
	if len(rows) == 0 {
		return grantRow{}, fmt.Errorf("failed to read back user group role assignment %s", grant.ID)
	}
	return rows[0], nil
}

func validateGrant(tx *gorm.DB, grant Grant) error {
	if grant.EnvironmentID != nil && grant.EnvironmentGroupID != nil {
		return fieldError(common.ErrValidation, "environmentGroupId", errors.New("environmentId and environmentGroupId are mutually exclusive"))
	}
	checks := []struct {
		id    *string
		table string
		field string
		label string
	}{
		{id: &grant.RoleID, table: "roles", field: "roleId", label: "role"},
		{id: grant.EnvironmentID, table: "environments", field: "environmentId", label: "agent"},
		{id: grant.EnvironmentGroupID, table: "environment_groups", field: "environmentGroupId", label: "environment"},
	}
	for _, check := range checks {
		if check.id == nil {
			continue
		}
		// An empty scope must not fall through to a global grant.
		if *check.id == "" {
			return fieldError(common.ErrValidation, check.field, fmt.Errorf("%s must not be empty", check.field))
		}
		var count int64
		if err := tx.Table(check.table).Where("id = ?", *check.id).Count(&count).Error; err != nil {
			return fmt.Errorf("failed to verify %s: %w", check.label, err)
		}
		if count == 0 {
			return fieldError(common.ErrValidation, check.field, fmt.Errorf("%s %q does not exist", check.label, *check.id))
		}
	}
	return nil
}

func queryGrants(tx *gorm.DB, where string, args ...any) ([]grantRow, error) {
	var rows []grantRow
	if err := tx.Table("user_group_role_assignments AS g").
		Select("g.id, g.user_group_id, ug.name AS user_group_name, g.role_id, r.name AS role_name, g.environment_id, g.environment_group_id").
		Joins("INNER JOIN user_groups ug ON ug.id = g.user_group_id").
		Joins("INNER JOIN roles r ON r.id = g.role_id").
		Where(where, args...).
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to list user group role assignments: %w", err)
	}
	slices.SortFunc(rows, func(a, b grantRow) int {
		return cmp.Or(
			strings.Compare(a.RoleName, b.RoleName),
			strings.Compare(kit.FromPtr(a.EnvironmentGroupID), kit.FromPtr(b.EnvironmentGroupID)),
			strings.Compare(kit.FromPtr(a.EnvironmentID), kit.FromPtr(b.EnvironmentID)),
			strings.Compare(a.ID, b.ID),
		)
	})
	return rows, nil
}

func queryMembers(tx *gorm.DB, where string, args ...any) ([]types.UserGroupMember, error) {
	var rows []memberRow
	if err := tx.Table("user_group_members AS m").
		Select("m.id, m.user_id, u.username, u.display_name, m.source").
		Joins("INNER JOIN users u ON u.id = m.user_id").
		Where(where, args...).
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to list user group members: %w", err)
	}
	slices.SortFunc(rows, func(a, b memberRow) int {
		return cmp.Or(
			strings.Compare(strings.ToLower(a.Username), strings.ToLower(b.Username)),
			strings.Compare(a.Source, b.Source),
			strings.Compare(a.ID, b.ID),
		)
	})
	members := make([]types.UserGroupMember, len(rows))
	for i, r := range rows {
		members[i] = types.UserGroupMember{
			DisplayName: r.DisplayName,
			ID:          r.ID,
			UserID:      r.UserID,
			Username:    r.Username,
			Source:      r.Source,
		}
	}
	return members, nil
}

func replaceLinks(tx *gorm.DB, groupID string, links []types.UserGroupLink) error {
	if err := tx.Where("user_group_id = ?", groupID).Delete(&Link{}).Error; err != nil {
		return fmt.Errorf("failed to clear user group links: %w", err)
	}
	if len(links) == 0 {
		return nil
	}
	rows := make([]Link, len(links))
	for i, link := range links {
		rows[i] = Link{UserGroupID: groupID, Kind: link.Kind, Value: link.Value}
	}
	if err := tx.Create(&rows).Error; err != nil {
		return fmt.Errorf("failed to save user group links: %w", err)
	}
	return nil
}

func ensureGroupExists(tx *gorm.DB, groupID string) error {
	var count int64
	if err := tx.Model(&Group{}).Where("id = ?", groupID).Count(&count).Error; err != nil {
		return fmt.Errorf("failed to verify user group: %w", err)
	}
	if count == 0 {
		return ErrGroupNotFound
	}
	return nil
}

// lockUser locks the user row so membership changes serialize with user-domain privilege checks.
func lockUser(tx *gorm.DB, userID string) error {
	var locked struct{ ID string }
	if err := tx.Table("users").
		Select("id").
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", userID).
		Take(&locked).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fieldError(common.ErrValidation, "userId", fmt.Errorf("user %q does not exist", userID))
		}
		return fmt.Errorf("failed to lock user for membership update: %w", err)
	}
	return nil
}
