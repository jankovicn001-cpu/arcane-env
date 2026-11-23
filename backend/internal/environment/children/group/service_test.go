package group

import (
	"errors"
	"strings"
	"testing"

	"github.com/libtnb/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"getarcane.dev/arcane/v3/internal/common"
	"getarcane.dev/arcane/v3/internal/database"
	"getarcane.dev/types/v3"
)

type (
	testEnvironmentRow struct {
		GroupID *string
		ID      string `gorm:"primaryKey"`
		Hidden  bool
	}

	testRoleAssignmentRow struct {
		EnvironmentGroupID *string
		ID                 string `gorm:"primaryKey"`
	}

	testRoleMappingRow struct {
		EnvironmentGroupID *string
		ID                 string `gorm:"primaryKey"`
	}

	testUserGroupGrantRow struct {
		EnvironmentGroupID *string
		ID                 string `gorm:"primaryKey"`
	}

	hookCounts struct {
		changes       int
		accessChanges int
	}
)

func (testEnvironmentRow) TableName() string { return "environments" }

func (testRoleAssignmentRow) TableName() string { return "user_role_assignments" }

func (testRoleMappingRow) TableName() string { return "oidc_role_mappings" }

func (testUserGroupGrantRow) TableName() string { return "user_group_role_assignments" }

func newTestService(t *testing.T) (*Service, *database.DB, *hookCounts) {
	t.Helper()
	gormDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, gormDB.AutoMigrate(&testEnvironmentRow{}, &testRoleAssignmentRow{}, &testRoleMappingRow{}, &testUserGroupGrantRow{}, &Group{}))
	sqlDB, err := gormDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	db := &database.DB{DB: gormDB}
	counts := &hookCounts{}
	return New(db, Hooks{
		OnChange:       func() { counts.changes++ },
		OnAccessChange: func() { counts.accessChanges++ },
	}), db, counts
}

func requireFieldError(t *testing.T, err, kind error, field string) {
	t.Helper()
	require.ErrorIs(t, err, kind)
	fieldErr, ok := errors.AsType[*types.FieldError](err)
	require.True(t, ok, "expected a field error, got %v", err)
	assert.Equal(t, field, fieldErr.Field)
}

func TestService_CreateNormalizesAndValidates(t *testing.T) {
	ctx := t.Context()
	svc, _, _ := newTestService(t)

	created, err := svc.Create(ctx, types.CreateEnvironmentGroupRequest{Name: "  Café  ", Description: "  Production hosts  "})
	require.NoError(t, err)
	assert.Equal(t, "Café", created.Name, "names are trimmed and NFC-normalized")
	assert.Equal(t, "Production hosts", created.Description)
	assert.NotEmpty(t, created.ID)
	assert.Zero(t, created.EnvironmentCount)
	assert.False(t, created.UpdatedAt.IsZero())

	_, err = svc.Create(ctx, types.CreateEnvironmentGroupRequest{Name: "CAFÉ"})
	requireFieldError(t, err, common.ErrConflict, "name")

	_, err = svc.Create(ctx, types.CreateEnvironmentGroupRequest{Name: "   "})
	requireFieldError(t, err, common.ErrValidation, "name")

	_, err = svc.Create(ctx, types.CreateEnvironmentGroupRequest{Name: strings.Repeat("界", maxNameLength+1)})
	requireFieldError(t, err, common.ErrValidation, "name")

	atLimit, err := svc.Create(ctx, types.CreateEnvironmentGroupRequest{Name: strings.Repeat("界", maxNameLength)})
	require.NoError(t, err)
	assert.Len(t, []rune(atLimit.Name), maxNameLength)

	_, err = svc.Create(ctx, types.CreateEnvironmentGroupRequest{Name: "Staging", Description: strings.Repeat("d", maxDescriptionLength+1)})
	requireFieldError(t, err, common.ErrValidation, "description")
}

func TestService_ListSortsByNameAndCountsVisibleAgents(t *testing.T) {
	ctx := t.Context()
	svc, db, _ := newTestService(t)

	prod, err := svc.Create(ctx, types.CreateEnvironmentGroupRequest{Name: "prod"})
	require.NoError(t, err)
	dev, err := svc.Create(ctx, types.CreateEnvironmentGroupRequest{Name: "Dev"})
	require.NoError(t, err)
	require.NoError(t, db.Create([]testEnvironmentRow{
		{ID: "web", GroupID: new(prod.ID)},
		{ID: "api", GroupID: new(prod.ID)},
		{ID: "node", GroupID: new(prod.ID), Hidden: true},
		{ID: "loose"},
	}).Error)

	groups, err := svc.List(ctx)
	require.NoError(t, err)
	require.Len(t, groups, 2)
	assert.Equal(t, []string{"Dev", "prod"}, []string{groups[0].Name, groups[1].Name})
	assert.Zero(t, groups[0].EnvironmentCount)
	assert.Equal(t, 2, groups[1].EnvironmentCount, "hidden agents are not counted")

	got, err := svc.Get(ctx, prod.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, got.EnvironmentCount)
	assert.Equal(t, dev.ID, groups[0].ID)

	_, err = svc.Get(ctx, "missing")
	require.ErrorIs(t, err, ErrGroupNotFound)
}

func TestService_Update(t *testing.T) {
	ctx := t.Context()
	svc, _, hooks := newTestService(t)

	prod, err := svc.Create(ctx, types.CreateEnvironmentGroupRequest{Name: "prod", Description: "Production"})
	require.NoError(t, err)
	_, err = svc.Create(ctx, types.CreateEnvironmentGroupRequest{Name: "staging"})
	require.NoError(t, err)

	renamed, err := svc.Update(ctx, prod.ID, types.UpdateEnvironmentGroupRequest{Name: new("  PROD  ")})
	require.NoError(t, err, "a group may change the case of its own name")
	assert.Equal(t, "PROD", renamed.Name)
	assert.Equal(t, "Production", renamed.Description, "omitted fields are untouched")
	assert.Equal(t, 1, hooks.changes)

	described, err := svc.Update(ctx, prod.ID, types.UpdateEnvironmentGroupRequest{Description: new("")})
	require.NoError(t, err)
	assert.Empty(t, described.Description)
	assert.Equal(t, "PROD", described.Name)

	_, err = svc.Update(ctx, prod.ID, types.UpdateEnvironmentGroupRequest{Name: new("Staging")})
	requireFieldError(t, err, common.ErrConflict, "name")

	_, err = svc.Update(ctx, prod.ID, types.UpdateEnvironmentGroupRequest{Name: new(" ")})
	requireFieldError(t, err, common.ErrValidation, "name")

	_, err = svc.Update(ctx, "missing", types.UpdateEnvironmentGroupRequest{Name: new("other")})
	require.ErrorIs(t, err, ErrGroupNotFound)
	assert.Equal(t, 2, hooks.changes, "failed updates do not notify")
	assert.Zero(t, hooks.accessChanges, "renames do not change who a grant reaches")
}

func TestService_DeleteUnassignsAgentsAndRemovesGrants(t *testing.T) {
	ctx := t.Context()
	svc, db, hooks := newTestService(t)

	prod, err := svc.Create(ctx, types.CreateEnvironmentGroupRequest{Name: "prod"})
	require.NoError(t, err)
	dev, err := svc.Create(ctx, types.CreateEnvironmentGroupRequest{Name: "dev"})
	require.NoError(t, err)
	require.NoError(t, db.Create([]testEnvironmentRow{
		{ID: "web", GroupID: new(prod.ID)},
		{ID: "box", GroupID: new(dev.ID)},
	}).Error)
	require.NoError(t, db.Create([]testRoleAssignmentRow{
		{ID: "prod-grant", EnvironmentGroupID: new(prod.ID)},
		{ID: "dev-grant", EnvironmentGroupID: new(dev.ID)},
		{ID: "global-grant"},
	}).Error)
	require.NoError(t, db.Create([]testRoleMappingRow{
		{ID: "prod-mapping", EnvironmentGroupID: new(prod.ID)},
		{ID: "dev-mapping", EnvironmentGroupID: new(dev.ID)},
	}).Error)
	require.NoError(t, db.Create([]testUserGroupGrantRow{
		{ID: "prod-team-grant", EnvironmentGroupID: new(prod.ID)},
		{ID: "dev-team-grant", EnvironmentGroupID: new(dev.ID)},
	}).Error)

	require.NoError(t, svc.Delete(ctx, prod.ID))
	assert.Equal(t, 1, hooks.changes)
	assert.Equal(t, 1, hooks.accessChanges)

	var assignments []testRoleAssignmentRow
	require.NoError(t, db.Order("id").Find(&assignments).Error)
	assert.Equal(t, []testRoleAssignmentRow{
		{ID: "dev-grant", EnvironmentGroupID: new(dev.ID)},
		{ID: "global-grant"},
	}, assignments, "the deleted group's grants are removed, never left unscoped")
	var mappings []testRoleMappingRow
	require.NoError(t, db.Find(&mappings).Error)
	assert.Equal(t, []testRoleMappingRow{{ID: "dev-mapping", EnvironmentGroupID: new(dev.ID)}}, mappings)
	var userGroupGrants []testUserGroupGrantRow
	require.NoError(t, db.Find(&userGroupGrants).Error)
	assert.Equal(t, []testUserGroupGrantRow{{ID: "dev-team-grant", EnvironmentGroupID: new(dev.ID)}}, userGroupGrants,
		"user group grants on the deleted group are removed, never left unscoped")

	var rows []testEnvironmentRow
	require.NoError(t, db.Order("id").Find(&rows).Error)
	require.Len(t, rows, 2)
	assert.Equal(t, dev.ID, *rows[0].GroupID, "other groups keep their agents")
	assert.Nil(t, rows[1].GroupID)

	_, err = svc.Find(ctx, prod.ID)
	require.ErrorIs(t, err, ErrGroupNotFound)
	require.ErrorIs(t, svc.Delete(ctx, prod.ID), ErrGroupNotFound)
	assert.Equal(t, 1, hooks.changes)
	assert.Equal(t, 1, hooks.accessChanges)
}
