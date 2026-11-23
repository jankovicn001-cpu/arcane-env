package usergroup

import (
	"errors"
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
	testUserRow struct {
		DisplayName *string
		ID          string `gorm:"primaryKey"`
		Username    string
	}

	testRoleRow struct {
		ID          string `gorm:"primaryKey"`
		Name        string
		Permissions string
	}

	testIDRow struct {
		ID string `gorm:"primaryKey"`
	}

	testEnvironmentGroupRow testIDRow

	hookCalls struct {
		users []string
		all   int
	}
)

func (testUserRow) TableName() string { return "users" }

func (testRoleRow) TableName() string { return "roles" }

func (testIDRow) TableName() string { return "environments" }

func (testEnvironmentGroupRow) TableName() string { return "environment_groups" }

func newTestService(t *testing.T) (*Service, *database.DB, *hookCalls) {
	t.Helper()
	gormDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := gormDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, gormDB.AutoMigrate(&testUserRow{}, &testRoleRow{}, &testIDRow{}, &testEnvironmentGroupRow{}, &Group{}, &Link{}, &Member{}, &Grant{}))
	require.NoError(t, gormDB.Exec("CREATE UNIQUE INDEX idx_test_members_uniq ON user_group_members(user_group_id, user_id, source)").Error)
	require.NoError(t, gormDB.Create([]testUserRow{{ID: "u-amy", Username: "amy", DisplayName: new("Amy")}, {ID: "u-bob", Username: "Bob"}}).Error)
	require.NoError(t, gormDB.Create([]testRoleRow{{ID: "viewer", Name: "Viewer", Permissions: `["containers:list"]`}, {ID: "admin", Name: "Admin", Permissions: `["*"]`}}).Error)
	require.NoError(t, gormDB.Create(&testIDRow{ID: "web"}).Error)
	require.NoError(t, gormDB.Create(&testEnvironmentGroupRow{ID: "prod"}).Error)
	calls := &hookCalls{}
	return New(&database.DB{DB: gormDB}, Hooks{
		InvalidateUser:     func(userID string) { calls.users = append(calls.users, userID) },
		InvalidateAllUsers: func() { calls.all++ },
	}), &database.DB{DB: gormDB}, calls
}

func requireFieldError(t *testing.T, err, kind error, field string) {
	t.Helper()
	require.ErrorIs(t, err, kind)
	fieldErr, ok := errors.AsType[*types.FieldError](err)
	require.True(t, ok, "expected a field error, got %v", err)
	assert.Equal(t, field, fieldErr.Field)
}

func TestService_CreateUpdateAndList(t *testing.T) {
	ctx := t.Context()
	svc, _, calls := newTestService(t)

	created, err := svc.Create(ctx, types.CreateUserGroupRequest{
		Name:        "  Café Ops  ",
		Description: "  On call  ",
		Links: []types.UserGroupLink{
			{Kind: "oidc", Value: " ops "},
			{Kind: "oidc", Value: "OPS"},
			{Kind: "ldap", Value: "cn=ops"},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "Café Ops", created.Name, "names are trimmed and NFC-normalized")
	assert.Equal(t, "On call", created.Description)
	assert.Equal(t, []types.UserGroupLink{{Kind: "ldap", Value: "cn=ops"}, {Kind: "oidc", Value: "ops"}}, created.Links,
		"link values are trimmed and repeats that differ only in case are dropped")
	assert.Zero(t, created.MemberCount)

	_, err = svc.Create(ctx, types.CreateUserGroupRequest{Name: "CAFÉ OPS"})
	requireFieldError(t, err, common.ErrConflict, "name")
	_, err = svc.Create(ctx, types.CreateUserGroupRequest{Name: "  "})
	requireFieldError(t, err, common.ErrValidation, "name")
	_, err = svc.Create(ctx, types.CreateUserGroupRequest{Name: "x", Description: string(make([]rune, 257))})
	requireFieldError(t, err, common.ErrValidation, "description")
	_, err = svc.Create(ctx, types.CreateUserGroupRequest{Name: "x", Links: []types.UserGroupLink{{Kind: "saml", Value: "ops"}}})
	requireFieldError(t, err, common.ErrValidation, "links")
	_, err = svc.Create(ctx, types.CreateUserGroupRequest{Name: "x", Links: []types.UserGroupLink{{Kind: "oidc", Value: " "}}})
	requireFieldError(t, err, common.ErrValidation, "links")

	alpha, err := svc.Create(ctx, types.CreateUserGroupRequest{Name: "alpha"})
	require.NoError(t, err)
	assert.Empty(t, alpha.Links)
	_, err = svc.Update(ctx, alpha.ID, types.UpdateUserGroupRequest{Name: new("café ops")})
	requireFieldError(t, err, common.ErrConflict, "name")

	assert.Zero(t, calls.all, "creating and renaming do not change permissions")
	updated, err := svc.Update(ctx, created.ID, types.UpdateUserGroupRequest{Description: new("Paged"), Links: &[]types.UserGroupLink{}})
	require.NoError(t, err)
	assert.Equal(t, "Café Ops", updated.Name)
	assert.Equal(t, "Paged", updated.Description)
	assert.Empty(t, updated.Links, "a submitted empty list removes every link")
	assert.Equal(t, 1, calls.all, "link changes invalidate cached permissions")
	_, err = svc.Update(ctx, "missing", types.UpdateUserGroupRequest{Description: new("x")})
	require.ErrorIs(t, err, ErrGroupNotFound)

	groups, err := svc.List(ctx)
	require.NoError(t, err)
	require.Len(t, groups, 2)
	assert.Equal(t, []string{"alpha", "Café Ops"}, []string{groups[0].Name, groups[1].Name}, "groups sort by name case-insensitively")
	_, err = svc.Get(ctx, "missing")
	require.ErrorIs(t, err, ErrGroupNotFound)
}

func TestService_Members(t *testing.T) {
	ctx := t.Context()
	svc, db, calls := newTestService(t)
	group, err := svc.Create(ctx, types.CreateUserGroupRequest{Name: "ops", Links: []types.UserGroupLink{{Kind: "oidc", Value: "ops"}}})
	require.NoError(t, err)

	amy, err := svc.AddMember(ctx, group.ID, types.AddUserGroupMemberRequest{UserID: "u-amy"})
	require.NoError(t, err)
	assert.Equal(t, types.UserGroupMember{ID: amy.ID, UserID: "u-amy", Username: "amy", DisplayName: new("Amy"), Source: sourceManual}, amy)
	assert.Equal(t, []string{"u-amy"}, calls.users, "adding a member invalidates that user")
	_, err = svc.AddMember(ctx, group.ID, types.AddUserGroupMemberRequest{UserID: "u-amy"})
	require.ErrorIs(t, err, ErrMemberExists)
	_, err = svc.AddMember(ctx, group.ID, types.AddUserGroupMemberRequest{UserID: "nobody"})
	requireFieldError(t, err, common.ErrValidation, "userId")
	_, err = svc.AddMember(ctx, "missing", types.AddUserGroupMemberRequest{UserID: "u-amy"})
	require.ErrorIs(t, err, ErrGroupNotFound)

	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		return svc.ReplaceSyncedMemberships(ctx, tx, "u-amy", sourceOidc, []string{"ops"})
	}))
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		return svc.ReplaceSyncedMemberships(ctx, tx, "u-bob", sourceOidc, []string{"ops"})
	}))
	members, err := svc.ListMembers(ctx, group.ID)
	require.NoError(t, err)
	require.Len(t, members, 3)
	assert.Equal(t, []string{"amy/manual", "amy/oidc", "Bob/oidc"}, []string{
		members[0].Username + "/" + members[0].Source,
		members[1].Username + "/" + members[1].Source,
		members[2].Username + "/" + members[2].Source,
	}, "members sort by username, then source")
	listed, err := svc.Get(ctx, group.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, listed.MemberCount, "member count counts distinct users")

	err = svc.RemoveMember(ctx, group.ID, members[2].ID)
	require.ErrorIs(t, err, common.ErrConflict, "synced members cannot be removed manually")
	require.ErrorContains(t, err, "SSO")
	require.ErrorIs(t, svc.RemoveMember(ctx, group.ID, "missing"), ErrMemberNotFound)
	calls.users = nil
	require.NoError(t, svc.RemoveMember(ctx, group.ID, amy.ID))
	assert.Equal(t, []string{"u-amy"}, calls.users, "removing a member invalidates that user")
	members, err = svc.ListMembers(ctx, group.ID)
	require.NoError(t, err)
	assert.Len(t, members, 2, "the synced membership of the same user is kept")
}

func TestService_ReplaceSyncedMemberships(t *testing.T) {
	ctx := t.Context()
	svc, db, _ := newTestService(t)
	ops, err := svc.Create(ctx, types.CreateUserGroupRequest{Name: "ops", Links: []types.UserGroupLink{{Kind: "oidc", Value: "ops"}, {Kind: "ldap", Value: "ops"}}})
	require.NoError(t, err)
	dev, err := svc.Create(ctx, types.CreateUserGroupRequest{Name: "dev", Links: []types.UserGroupLink{{Kind: "oidc", Value: "dev"}}})
	require.NoError(t, err)
	_, err = svc.AddMember(ctx, dev.ID, types.AddUserGroupMemberRequest{UserID: "u-amy"})
	require.NoError(t, err)
	replace := func(source string, values ...string) {
		t.Helper()
		require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
			return svc.ReplaceSyncedMemberships(ctx, tx, "u-amy", source, values)
		}))
	}
	memberships := func() []string {
		t.Helper()
		var rows []Member
		require.NoError(t, db.Order("user_group_id, source").Find(&rows).Error)
		out := make([]string, len(rows))
		for i, row := range rows {
			out[i] = map[string]string{ops.ID: "ops", dev.ID: "dev"}[row.UserGroupID] + "/" + row.Source
		}
		return out
	}

	replace(sourceOidc, "ops", "dev", "unlinked")
	assert.ElementsMatch(t, []string{"ops/oidc", "dev/oidc", "dev/manual"}, memberships())
	replace(sourceOidc, "OPS")
	assert.ElementsMatch(t, []string{"dev/manual"}, memberships(), "values match exactly, like role mappings, and stale synced memberships are removed")
	replace(sourceLdap, "ops", "dev")
	assert.ElementsMatch(t, []string{"ops/ldap", "dev/manual"}, memberships(), "each source only matches links of its own kind")
	replace(sourceOidc)
	assert.ElementsMatch(t, []string{"ops/ldap", "dev/manual"}, memberships(), "a source only replaces its own memberships")
	replace(sourceLdap)
	assert.ElementsMatch(t, []string{"dev/manual"}, memberships())

	require.Error(t, db.Transaction(func(tx *gorm.DB) error {
		return svc.ReplaceSyncedMemberships(ctx, tx, "u-amy", sourceManual, nil)
	}), "manual memberships are never replaced by a sync")
}

func TestService_Grants(t *testing.T) {
	ctx := t.Context()
	svc, _, calls := newTestService(t)
	group, err := svc.Create(ctx, types.CreateUserGroupRequest{Name: "ops"})
	require.NoError(t, err)
	grant := func(roleID string, environmentID, environmentGroupID *string) (types.UserGroupRoleAssignment, error) {
		return svc.CreateGrant(ctx, group.ID, types.CreateUserGroupRoleAssignmentRequest{RoleID: roleID, EnvironmentID: environmentID, EnvironmentGroupID: environmentGroupID})
	}

	global, err := grant("viewer", nil, nil)
	require.NoError(t, err)
	assert.Equal(t, types.UserGroupRoleAssignment{ID: global.ID, RoleID: "viewer", RoleName: "Viewer"}, global)
	agent, err := grant("viewer", new("web"), nil)
	require.NoError(t, err)
	assert.Equal(t, "web", *agent.EnvironmentID)
	environment, err := grant("admin", nil, new("prod"))
	require.NoError(t, err)
	assert.Equal(t, "prod", *environment.EnvironmentGroupID)
	assert.Equal(t, 3, calls.all, "every grant invalidates cached permissions")

	_, err = grant("viewer", nil, nil)
	require.ErrorIs(t, err, ErrGrantExists)
	_, err = grant("viewer", new("web"), new("prod"))
	requireFieldError(t, err, common.ErrValidation, "environmentGroupId")
	_, err = grant("missing", nil, nil)
	requireFieldError(t, err, common.ErrValidation, "roleId")
	_, err = grant("viewer", new("missing"), nil)
	requireFieldError(t, err, common.ErrValidation, "environmentId")
	_, err = grant("viewer", nil, new("missing"))
	requireFieldError(t, err, common.ErrValidation, "environmentGroupId")
	_, err = grant("admin", new(""), nil)
	requireFieldError(t, err, common.ErrValidation, "environmentId")
	_, err = grant("admin", nil, new(""))
	requireFieldError(t, err, common.ErrValidation, "environmentGroupId")
	_, err = svc.CreateGrant(ctx, "missing", types.CreateUserGroupRoleAssignmentRequest{RoleID: "viewer"})
	require.ErrorIs(t, err, ErrGroupNotFound)
	assert.Equal(t, 3, calls.all, "rejected grants change nothing")

	grants, err := svc.ListGrants(ctx, group.ID)
	require.NoError(t, err)
	assert.Equal(t, []string{environment.ID, global.ID, agent.ID}, []string{grants[0].ID, grants[1].ID, grants[2].ID}, "grants sort by role name, then scope")

	require.NoError(t, svc.DeleteGrant(ctx, group.ID, agent.ID))
	assert.Equal(t, 4, calls.all)
	require.ErrorIs(t, svc.DeleteGrant(ctx, group.ID, agent.ID), ErrGrantNotFound)
	other, err := svc.Create(ctx, types.CreateUserGroupRequest{Name: "other"})
	require.NoError(t, err)
	require.ErrorIs(t, svc.DeleteGrant(ctx, other.ID, global.ID), ErrGrantNotFound, "the grant must belong to the group")
}

func TestService_DeleteRemovesEverything(t *testing.T) {
	ctx := t.Context()
	svc, db, calls := newTestService(t)
	group, err := svc.Create(ctx, types.CreateUserGroupRequest{Name: "ops", Links: []types.UserGroupLink{{Kind: "oidc", Value: "ops"}}})
	require.NoError(t, err)
	kept, err := svc.Create(ctx, types.CreateUserGroupRequest{Name: "kept"})
	require.NoError(t, err)
	for _, id := range []string{group.ID, kept.ID} {
		_, err = svc.AddMember(ctx, id, types.AddUserGroupMemberRequest{UserID: "u-amy"})
		require.NoError(t, err)
		_, err = svc.CreateGrant(ctx, id, types.CreateUserGroupRoleAssignmentRequest{RoleID: "admin"})
		require.NoError(t, err)
	}
	calls.all = 0

	require.NoError(t, svc.Delete(ctx, group.ID))
	assert.Equal(t, 1, calls.all, "deleting a group invalidates cached permissions")
	for _, model := range []any{&Grant{}, &Member{}, &Link{}} {
		var count int64
		require.NoError(t, db.Model(model).Where("user_group_id = ?", group.ID).Count(&count).Error)
		assert.Zero(t, count, "deleting a group removes its %T rows", model)
	}
	grants, err := svc.ScopedGrants(db.DB, "u-amy")
	require.NoError(t, err)
	assert.Len(t, grants, 1, "only the remaining group's grant reaches the member")
	require.ErrorIs(t, svc.Delete(ctx, group.ID), ErrGroupNotFound)
}

func TestService_EnvironmentGroupAccess(t *testing.T) {
	ctx := t.Context()
	svc, db, _ := newTestService(t)
	zeta, err := svc.Create(ctx, types.CreateUserGroupRequest{Name: "zeta"})
	require.NoError(t, err)
	alpha, err := svc.Create(ctx, types.CreateUserGroupRequest{Name: "Alpha"})
	require.NoError(t, err)
	_, err = svc.CreateGrant(ctx, alpha.ID, types.CreateUserGroupRoleAssignmentRequest{RoleID: "viewer"})
	require.NoError(t, err)

	var entry types.EnvironmentAccessEntry
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		var grantErr error
		entry, grantErr = svc.GrantEnvironmentGroupAccess(tx, "prod", zeta.ID, "viewer")
		return grantErr
	}))
	assert.Equal(t, types.EnvironmentAccessEntry{
		ID:            entry.ID,
		PrincipalType: types.EnvironmentAccessPrincipalGroup,
		UserGroupID:   zeta.ID,
		UserGroupName: "zeta",
		RoleID:        "viewer",
		RoleName:      "Viewer",
		Source:        sourceManual,
	}, entry)
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		_, grantErr := svc.GrantEnvironmentGroupAccess(tx, "prod", alpha.ID, "viewer")
		return grantErr
	}))
	err = db.Transaction(func(tx *gorm.DB) error {
		_, grantErr := svc.GrantEnvironmentGroupAccess(tx, "prod", zeta.ID, "viewer")
		return grantErr
	})
	require.ErrorIs(t, err, ErrGrantExists)
	err = db.Transaction(func(tx *gorm.DB) error {
		_, grantErr := svc.GrantEnvironmentGroupAccess(tx, "prod", "missing", "viewer")
		return grantErr
	})
	requireFieldError(t, err, common.ErrValidation, "userGroupId")

	entries, err := svc.ListEnvironmentGroupAccess(db.DB, "prod")
	require.NoError(t, err)
	assert.Equal(t, []string{"Alpha", "zeta"}, []string{entries[0].UserGroupName, entries[1].UserGroupName}, "only grants on the environment, sorted by group name")

	deleted, err := svc.RevokeEnvironmentGroupAccess(db.DB, "other", entry.ID)
	require.NoError(t, err)
	assert.False(t, deleted, "revoking is restricted to the environment")
	deleted, err = svc.RevokeEnvironmentGroupAccess(db.DB, "prod", entry.ID)
	require.NoError(t, err)
	assert.True(t, deleted)
}
