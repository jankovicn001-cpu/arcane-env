package group

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
	"uuid"

	"gorm.io/gorm"

	"kit.getarcane.app/normalization"

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
		// OnChange tells the parent that agents' embedded group data changed.
		OnChange func()
		// OnAccessChange tells the parent that a deletion changed which agents role grants reach.
		OnAccessChange func()
	}

	countRow struct {
		GroupID string
		Count   int
	}
)

const (
	maxNameLength        = 64
	maxDescriptionLength = 256
)

var ErrGroupNotFound = common.Classify(common.ErrNotFound, errors.New("environment group not found"))

func New(db *database.DB, hooks Hooks) *Service {
	return &Service{db: db, hooks: hooks}
}

func (s *Service) List(ctx context.Context) ([]types.EnvironmentGroup, error) {
	var groups []Group
	if err := s.db.WithContext(ctx).Find(&groups).Error; err != nil {
		return nil, fmt.Errorf("failed to list environment groups: %w", err)
	}
	slices.SortFunc(groups, func(a, b Group) int {
		return cmp.Or(strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)), strings.Compare(a.ID, b.ID))
	})
	counts, err := s.environmentCounts(ctx, nil)
	if err != nil {
		return nil, err
	}
	out := make([]types.EnvironmentGroup, 0, len(groups))
	for _, group := range groups {
		out = append(out, toDTO(group, counts[group.ID]))
	}
	return out, nil
}

func (s *Service) Get(ctx context.Context, id string) (types.EnvironmentGroup, error) {
	group, err := s.Find(ctx, id)
	if err != nil {
		return types.EnvironmentGroup{}, err
	}
	counts, err := s.environmentCounts(ctx, []string{id})
	if err != nil {
		return types.EnvironmentGroup{}, err
	}
	return toDTO(*group, counts[id]), nil
}

// Find loads the persisted group so the parent can validate and embed agent assignments.
func (s *Service) Find(ctx context.Context, id string) (*Group, error) {
	var group Group
	err := s.db.WithContext(ctx).Where("id = ?", id).Take(&group).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrGroupNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get environment group: %w", err)
	}
	return &group, nil
}

func (s *Service) Create(ctx context.Context, req types.CreateEnvironmentGroupRequest) (types.EnvironmentGroup, error) {
	name, err := normalizeText("name", req.Name, 1, maxNameLength)
	if err != nil {
		return types.EnvironmentGroup{}, err
	}
	description, err := normalizeText("description", req.Description, 0, maxDescriptionLength)
	if err != nil {
		return types.EnvironmentGroup{}, err
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
		return tx.Create(&group).Error
	}); createErr != nil {
		return types.EnvironmentGroup{}, s.writeError("create", name, createErr)
	}
	return toDTO(group, 0), nil
}

func (s *Service) Update(ctx context.Context, id string, req types.UpdateEnvironmentGroupRequest) (types.EnvironmentGroup, error) {
	updates := map[string]any{"updated_at": time.Now()}
	var name string
	if req.Name != nil {
		normalized, err := normalizeText("name", *req.Name, 1, maxNameLength)
		if err != nil {
			return types.EnvironmentGroup{}, err
		}
		name = normalized
		updates["name"] = normalized
	}
	if req.Description != nil {
		description, err := normalizeText("description", *req.Description, 0, maxDescriptionLength)
		if err != nil {
			return types.EnvironmentGroup{}, err
		}
		updates["description"] = description
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
		return tx.Model(&Group{}).Where("id = ?", id).Updates(updates).Error
	}); err != nil {
		return types.EnvironmentGroup{}, s.writeError("update", name, err)
	}
	s.hooks.OnChange()
	return s.Get(ctx, id)
}

// Delete removes the group's role grants and unassigns its agents explicitly because SQLite may run without foreign key enforcement.
// Grants are deleted, never unscoped, so they cannot widen to global access.
func (s *Service) Delete(ctx context.Context, id string) error {
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM user_role_assignments WHERE environment_group_id = ?", id).Error; err != nil {
			return fmt.Errorf("failed to delete environment group role assignments: %w", err)
		}
		if err := tx.Exec("DELETE FROM oidc_role_mappings WHERE environment_group_id = ?", id).Error; err != nil {
			return fmt.Errorf("failed to delete environment group role mappings: %w", err)
		}
		if err := tx.Exec("DELETE FROM user_group_role_assignments WHERE environment_group_id = ?", id).Error; err != nil {
			return fmt.Errorf("failed to delete environment group user group grants: %w", err)
		}
		if err := tx.Table("environments").Where("group_id = ?", id).Update("group_id", nil).Error; err != nil {
			return fmt.Errorf("failed to unassign environment group agents: %w", err)
		}
		result := tx.Delete(&Group{}, "id = ?", id)
		if result.Error != nil {
			return fmt.Errorf("failed to delete environment group: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return ErrGroupNotFound
		}
		return nil
	}); err != nil {
		return err
	}
	s.hooks.OnChange()
	s.hooks.OnAccessChange()
	return nil
}

func (s *Service) environmentCounts(ctx context.Context, ids []string) (map[string]int, error) {
	query := s.db.WithContext(ctx).Table("environments").
		Select("group_id, COUNT(*) AS count").
		Where("group_id IS NOT NULL AND hidden = ?", false)
	if ids != nil {
		query = query.Where("group_id IN ?", ids)
	}
	var rows []countRow
	if err := query.Group("group_id").Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to count environment group agents: %w", err)
	}
	counts := make(map[string]int, len(rows))
	for _, row := range rows {
		counts[row.GroupID] = row.Count
	}
	return counts, nil
}

func (s *Service) writeError(action, name string, err error) error {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return ErrGroupNotFound
	case database.IsDuplicateKeyError(s.db.DB, err):
		return nameTakenError(name)
	case errors.Is(err, common.ErrConflict):
		return err
	default:
		return fmt.Errorf("failed to %s environment group: %w", action, err)
	}
}

// ensureNameAvailable folds case in Go because SQLite's lower() only folds ASCII.
func ensureNameAvailable(tx *gorm.DB, name, exceptID string) error {
	var names []string
	if err := tx.Model(&Group{}).Where("id <> ?", exceptID).Pluck("name", &names).Error; err != nil {
		return fmt.Errorf("failed to check environment group name: %w", err)
	}
	if slices.ContainsFunc(names, func(existing string) bool { return strings.EqualFold(existing, name) }) {
		return nameTakenError(name)
	}
	return nil
}

func nameTakenError(name string) error {
	return common.Classify(common.ErrConflict, &types.FieldError{Field: "name", Err: fmt.Errorf("an environment named %q already exists", name)})
}

func normalizeText(field, value string, minLength, maxLength int) (string, error) {
	text := normalization.Text(value, normalization.TextOptions{Trim: true, NFC: true})
	if length := utf8.RuneCountInString(text); length < minLength || length > maxLength {
		return "", common.Classify(common.ErrValidation, &types.FieldError{
			Field: field,
			Err:   fmt.Errorf("%s must be between %d and %d characters", field, minLength, maxLength),
		})
	}
	return text, nil
}

func toDTO(group Group, environmentCount int) types.EnvironmentGroup {
	return types.EnvironmentGroup{
		CreatedAt:        group.CreatedAt,
		UpdatedAt:        *cmp.Or(group.UpdatedAt, &group.CreatedAt),
		ID:               group.ID,
		Name:             group.Name,
		Description:      group.Description,
		EnvironmentCount: environmentCount,
	}
}
