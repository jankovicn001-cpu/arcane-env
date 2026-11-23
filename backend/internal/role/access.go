package role

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"gorm.io/gorm"

	"getarcane.dev/arcane/v3/internal/common"
	"getarcane.dev/arcane/v3/internal/database"
	"getarcane.dev/types/v3"
)

type (
	environmentAccessRow struct {
		DisplayName *string `gorm:"column:display_name"`
		ID          string  `gorm:"column:id"`
		UserID      string  `gorm:"column:user_id"`
		Username    string  `gorm:"column:username"`
		RoleID      string  `gorm:"column:role_id"`
		RoleName    string  `gorm:"column:role_name"`
		Source      string  `gorm:"column:source"`
	}
)

var (
	ErrEnvironmentGroupNotFound = common.Classify(common.ErrNotFound, errors.New("environment group not found"))
	ErrRoleAssignmentNotFound   = common.Classify(common.ErrNotFound, errors.New("role assignment not found"))
	ErrRoleAssignmentExists     = common.Classify(common.ErrConflict, errors.New("the user already holds this role on the environment"))
)

// ListEnvironmentGroupAccess returns every assignment scoped to the group, whatever its source:
// users sorted by username, then user groups sorted by name.
func (s *RoleService) ListEnvironmentGroupAccess(ctx context.Context, groupID string) ([]types.EnvironmentAccessEntry, error) {
	db := s.db.WithContext(ctx)
	if err := ensureEnvironmentGroupExists(db, groupID); err != nil {
		return nil, err
	}
	users, err := queryEnvironmentAccess(db, "ura.environment_group_id = ?", groupID)
	if err != nil {
		return nil, err
	}
	groups, err := s.userGroups.ListEnvironmentGroupAccess(db, groupID)
	if err != nil {
		return nil, err
	}
	return slices.Concat(users, groups), nil
}

// GrantEnvironmentGroupAccess grants the role, scoped to the group, to exactly one user (as a manual assignment) or user group.
func (s *RoleService) GrantEnvironmentGroupAccess(ctx context.Context, groupID string, req types.CreateEnvironmentAccessRequest) (types.EnvironmentAccessEntry, error) {
	userID := strings.TrimSpace(req.UserID)
	userGroupID := strings.TrimSpace(req.UserGroupID)
	roleID := strings.TrimSpace(req.RoleID)
	if (userID == "") == (userGroupID == "") {
		return types.EnvironmentAccessEntry{}, common.Classify(common.ErrValidation, &types.FieldError{Field: "userId", Err: errors.New("set exactly one of userId and userGroupId")})
	}
	if userGroupID != "" {
		return s.grantUserGroupEnvironmentGroupAccess(ctx, groupID, userGroupID, roleID)
	}
	var entry types.EnvironmentAccessEntry
	err := s.db.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.LockGlobalAdminGuard(tx); err != nil {
			return err
		}
		if err := ensureEnvironmentGroupExists(tx, groupID); err != nil {
			return err
		}
		if err := lockUser(tx, userID); err != nil {
			if errors.Is(err, common.ErrUserNotFound) {
				return common.Classify(common.ErrValidation, &types.FieldError{Field: "userId", Err: fmt.Errorf("user %q does not exist", userID)})
			}
			return err
		}
		var roles int64
		if err := tx.Model(&Role{}).Where("id = ?", roleID).Count(&roles).Error; err != nil {
			return fmt.Errorf("failed to verify role: %w", err)
		}
		if roles == 0 {
			return common.Classify(common.ErrValidation, &types.FieldError{Field: "roleId", Err: fmt.Errorf("role %q does not exist", roleID)})
		}
		var duplicates int64
		if err := tx.Model(&UserRoleAssignment{}).
			Where("user_id = ? AND role_id = ? AND environment_group_id = ? AND source = ?", userID, roleID, groupID, RoleAssignmentSourceManual).
			Count(&duplicates).Error; err != nil {
			return fmt.Errorf("failed to check existing assignments: %w", err)
		}
		if duplicates > 0 {
			return ErrRoleAssignmentExists
		}
		assignment := UserRoleAssignment{
			UserID:             userID,
			RoleID:             roleID,
			EnvironmentGroupID: &groupID,
			Source:             RoleAssignmentSourceManual,
		}
		if err := tx.Create(&assignment).Error; err != nil {
			if database.IsDuplicateKeyError(tx, err) {
				return ErrRoleAssignmentExists
			}
			return fmt.Errorf("failed to create assignment: %w", err)
		}
		created, err := queryEnvironmentAccess(tx, "ura.id = ?", assignment.ID)
		if err != nil {
			return err
		}
		if len(created) == 0 {
			return fmt.Errorf("failed to read back assignment %s", assignment.ID)
		}
		entry = created[0]
		return nil
	})
	if err != nil {
		return types.EnvironmentAccessEntry{}, err
	}
	s.InvalidateUser(userID)
	return entry, nil
}

// RevokeEnvironmentGroupAccess deletes a manual user assignment or a user group grant scoped to the group;
// synced user assignments follow their role mappings.
func (s *RoleService) RevokeEnvironmentGroupAccess(ctx context.Context, groupID, assignmentID string) error {
	var userID string
	var userGroupGrant bool
	err := s.db.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.LockGlobalAdminGuard(tx); err != nil {
			return err
		}
		if err := ensureEnvironmentGroupExists(tx, groupID); err != nil {
			return err
		}
		var existing UserRoleAssignment
		if err := tx.Where("id = ? AND environment_group_id = ?", assignmentID, groupID).Take(&existing).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("failed to load assignment: %w", err)
			}
			deleted, revokeErr := s.userGroups.RevokeEnvironmentGroupAccess(tx, groupID, assignmentID)
			if revokeErr != nil {
				return revokeErr
			}
			if !deleted {
				return ErrRoleAssignmentNotFound
			}
			userGroupGrant = true
			return nil
		}
		if existing.Source != RoleAssignmentSourceManual {
			return common.Classify(common.ErrConflict, fmt.Errorf("this access is synced from %s and cannot be removed here; change the matching role mapping instead", syncedSourceLabel(existing.Source)))
		}
		if err := lockUser(tx, existing.UserID); err != nil {
			return err
		}
		if err := tx.Delete(&UserRoleAssignment{}, "id = ?", existing.ID).Error; err != nil {
			return fmt.Errorf("failed to delete assignment: %w", err)
		}
		userID = existing.UserID
		return nil
	})
	if err != nil {
		return err
	}
	if userGroupGrant {
		s.InvalidateAllUsers()
		return nil
	}
	s.InvalidateUser(userID)
	return nil
}

func (s *RoleService) grantUserGroupEnvironmentGroupAccess(ctx context.Context, groupID, userGroupID, roleID string) (types.EnvironmentAccessEntry, error) {
	var entry types.EnvironmentAccessEntry
	err := s.db.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureEnvironmentGroupExists(tx, groupID); err != nil {
			return err
		}
		created, err := s.userGroups.GrantEnvironmentGroupAccess(tx, groupID, userGroupID, roleID)
		entry = created
		return err
	})
	if err != nil {
		return types.EnvironmentAccessEntry{}, err
	}
	s.InvalidateAllUsers()
	return entry, nil
}

func ensureEnvironmentGroupExists(tx *gorm.DB, groupID string) error {
	var count int64
	if err := tx.Table("environment_groups").Where("id = ?", groupID).Count(&count).Error; err != nil {
		return fmt.Errorf("failed to verify environment group: %w", err)
	}
	if count == 0 {
		return ErrEnvironmentGroupNotFound
	}
	return nil
}

func queryEnvironmentAccess(tx *gorm.DB, where string, args ...any) ([]types.EnvironmentAccessEntry, error) {
	var rows []environmentAccessRow
	if err := tx.Table("user_role_assignments AS ura").
		Select("ura.id, ura.user_id, u.username, u.display_name, ura.role_id, r.name AS role_name, ura.source").
		Joins("INNER JOIN users u ON u.id = ura.user_id").
		Joins("INNER JOIN roles r ON r.id = ura.role_id").
		Where(where, args...).
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to list environment access: %w", err)
	}
	slices.SortFunc(rows, func(a, b environmentAccessRow) int {
		return cmp.Or(
			strings.Compare(strings.ToLower(a.Username), strings.ToLower(b.Username)),
			strings.Compare(a.RoleName, b.RoleName),
			strings.Compare(a.Source, b.Source),
			strings.Compare(a.ID, b.ID),
		)
	})
	entries := make([]types.EnvironmentAccessEntry, len(rows))
	for i, r := range rows {
		entries[i] = types.EnvironmentAccessEntry{
			DisplayName:   r.DisplayName,
			ID:            r.ID,
			PrincipalType: types.EnvironmentAccessPrincipalUser,
			UserID:        r.UserID,
			Username:      r.Username,
			RoleID:        r.RoleID,
			RoleName:      r.RoleName,
			Source:        r.Source,
		}
	}
	return entries, nil
}

func syncedSourceLabel(source string) string {
	switch source {
	case RoleAssignmentSourceOidc:
		return "SSO"
	case RoleAssignmentSourceLdap:
		return "LDAP"
	default:
		return source
	}
}
