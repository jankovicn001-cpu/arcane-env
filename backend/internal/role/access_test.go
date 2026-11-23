package role_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"getarcane.dev/arcane/v3/internal/common"
	"getarcane.dev/arcane/v3/internal/role"
	"getarcane.dev/arcane/v3/internal/role/children/usergroup"
	"getarcane.dev/arcane/v3/pkg/authz"
	"getarcane.dev/types/v3"
)

func TestEnvironmentGroupAccess(t *testing.T) {
	ctx := t.Context()
	db, roleSvc := setupUserAndRoleServices(t)
	grantGlobalAdmin(t, roleSvc, createTestUser(t, db, "root", "root").ID)
	createTestUser(t, db, "u-zed", "zed")
	createTestUser(t, db, "u-amy", "Amy")
	createTestGroup(t, db, "prod", "web")
	createTestGroup(t, db, "dev", "box")
	requireFieldError := func(err error, field string) {
		t.Helper()
		require.ErrorIs(t, err, common.ErrValidation)
		fieldErr, ok := errors.AsType[*types.FieldError](err)
		require.True(t, ok, "expected a field error, got %v", err)
		require.Equal(t, field, fieldErr.Field)
	}

	// Warm the cache so the grant must invalidate it.
	before, err := roleSvc.ResolvePermissions(ctx, "u-zed")
	require.NoError(t, err)
	require.False(t, before.Allows(authz.PermContainersList, "web"))

	granted, err := roleSvc.GrantEnvironmentGroupAccess(ctx, "prod", types.CreateEnvironmentAccessRequest{UserID: "u-zed", RoleID: authz.BuiltInRoleViewer})
	require.NoError(t, err)
	require.Equal(t, types.EnvironmentAccessEntry{
		ID:            granted.ID,
		PrincipalType: types.EnvironmentAccessPrincipalUser,
		UserID:        "u-zed",
		Username:      "zed",
		RoleID:        authz.BuiltInRoleViewer,
		RoleName:      "Viewer",
		Source:        role.RoleAssignmentSourceManual,
	}, granted)
	after, err := roleSvc.ResolvePermissions(ctx, "u-zed")
	require.NoError(t, err)
	require.True(t, after.Allows(authz.PermContainersList, "web"), "granting access invalidates the user's cached permissions")
	require.False(t, after.Allows(authz.PermContainersList, "box"))

	_, err = roleSvc.GrantEnvironmentGroupAccess(ctx, "prod", types.CreateEnvironmentAccessRequest{UserID: "u-zed", RoleID: authz.BuiltInRoleViewer})
	require.ErrorIs(t, err, role.ErrRoleAssignmentExists)
	require.ErrorIs(t, err, common.ErrConflict)
	_, err = roleSvc.GrantEnvironmentGroupAccess(ctx, "missing", types.CreateEnvironmentAccessRequest{UserID: "u-zed", RoleID: authz.BuiltInRoleViewer})
	require.ErrorIs(t, err, common.ErrNotFound)
	_, err = roleSvc.GrantEnvironmentGroupAccess(ctx, "prod", types.CreateEnvironmentAccessRequest{UserID: "nobody", RoleID: authz.BuiltInRoleViewer})
	requireFieldError(err, "userId")
	_, err = roleSvc.GrantEnvironmentGroupAccess(ctx, "prod", types.CreateEnvironmentAccessRequest{UserID: "u-zed", RoleID: "missing"})
	requireFieldError(err, "roleId")

	require.NoError(t, roleSvc.ReplaceUserAssignments(ctx, "u-amy", role.RoleAssignmentSourceOidc, []role.UserRoleAssignment{
		{RoleID: authz.BuiltInRoleDeployer, EnvironmentGroupID: new("prod")},
		{RoleID: authz.BuiltInRoleViewer, EnvironmentGroupID: new("dev")},
		{RoleID: authz.BuiltInRoleViewer},
	}))
	entries, err := roleSvc.ListEnvironmentGroupAccess(ctx, "prod")
	require.NoError(t, err)
	require.Len(t, entries, 2, "only assignments scoped to the group are listed")
	require.Equal(t, []string{"Amy", "zed"}, []string{entries[0].Username, entries[1].Username}, "entries sort by username")
	require.Equal(t, role.RoleAssignmentSourceOidc, entries[0].Source)
	_, err = roleSvc.ListEnvironmentGroupAccess(ctx, "missing")
	require.ErrorIs(t, err, common.ErrNotFound)

	err = roleSvc.RevokeEnvironmentGroupAccess(ctx, "prod", entries[0].ID)
	require.ErrorIs(t, err, common.ErrConflict, "synced assignments follow their role mappings")
	require.ErrorContains(t, err, "SSO")
	require.ErrorIs(t, roleSvc.RevokeEnvironmentGroupAccess(ctx, "dev", granted.ID), role.ErrRoleAssignmentNotFound, "the assignment must belong to the group")
	require.ErrorIs(t, roleSvc.RevokeEnvironmentGroupAccess(ctx, "missing", granted.ID), role.ErrEnvironmentGroupNotFound)

	require.NoError(t, roleSvc.RevokeEnvironmentGroupAccess(ctx, "prod", granted.ID))
	revoked, err := roleSvc.ResolvePermissions(ctx, "u-zed")
	require.NoError(t, err)
	require.False(t, revoked.Allows(authz.PermContainersList, "web"), "revoking access invalidates the user's cached permissions")
	entries, err = roleSvc.ListEnvironmentGroupAccess(ctx, "prod")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.ErrorIs(t, roleSvc.RevokeEnvironmentGroupAccess(ctx, "prod", granted.ID), role.ErrRoleAssignmentNotFound)
}

func TestEnvironmentGroupAccessForUserGroups(t *testing.T) {
	ctx := t.Context()
	db, roleSvc := setupUserAndRoleServices(t)
	grantGlobalAdmin(t, roleSvc, createTestUser(t, db, "root", "root").ID)
	member := createTestUser(t, db, "u-amy", "amy")
	createTestGroup(t, db, "prod", "web")
	createTestGroup(t, db, "dev", "box")
	createTestUserGroup(t, db, "ops", []string{member.ID})
	createTestUserGroup(t, db, "audit", nil)
	requireValidation := func(err error, field string) {
		t.Helper()
		require.ErrorIs(t, err, common.ErrValidation)
		fieldErr, ok := errors.AsType[*types.FieldError](err)
		require.True(t, ok, "expected a field error, got %v", err)
		require.Equal(t, field, fieldErr.Field)
	}

	before, err := roleSvc.ResolvePermissions(ctx, member.ID)
	require.NoError(t, err)
	require.False(t, before.Allows(authz.PermContainersList, "web"))

	granted, err := roleSvc.GrantEnvironmentGroupAccess(ctx, "prod", types.CreateEnvironmentAccessRequest{UserGroupID: "ops", RoleID: authz.BuiltInRoleViewer})
	require.NoError(t, err)
	require.Equal(t, types.EnvironmentAccessEntry{
		ID:            granted.ID,
		PrincipalType: types.EnvironmentAccessPrincipalGroup,
		UserGroupID:   "ops",
		UserGroupName: "team-ops",
		RoleID:        authz.BuiltInRoleViewer,
		RoleName:      "Viewer",
		Source:        role.RoleAssignmentSourceManual,
	}, granted)
	after, err := roleSvc.ResolvePermissions(ctx, member.ID)
	require.NoError(t, err)
	require.True(t, after.Allows(authz.PermContainersList, "web"), "granting a group invalidates its members' cached permissions")
	require.False(t, after.Allows(authz.PermContainersList, "box"))
	require.False(t, after.Allows(authz.PermContainersList, ""), "an environment grant to a group never applies globally")

	_, err = roleSvc.GrantEnvironmentGroupAccess(ctx, "prod", types.CreateEnvironmentAccessRequest{UserGroupID: "ops", RoleID: authz.BuiltInRoleViewer})
	require.ErrorIs(t, err, usergroup.ErrGrantExists)
	_, err = roleSvc.GrantEnvironmentGroupAccess(ctx, "prod", types.CreateEnvironmentAccessRequest{UserID: member.ID, UserGroupID: "ops", RoleID: authz.BuiltInRoleViewer})
	requireValidation(err, "userId")
	_, err = roleSvc.GrantEnvironmentGroupAccess(ctx, "prod", types.CreateEnvironmentAccessRequest{RoleID: authz.BuiltInRoleViewer})
	requireValidation(err, "userId")
	_, err = roleSvc.GrantEnvironmentGroupAccess(ctx, "prod", types.CreateEnvironmentAccessRequest{UserGroupID: "missing", RoleID: authz.BuiltInRoleViewer})
	requireValidation(err, "userGroupId")
	_, err = roleSvc.GrantEnvironmentGroupAccess(ctx, "prod", types.CreateEnvironmentAccessRequest{UserGroupID: "ops", RoleID: "missing"})
	requireValidation(err, "roleId")
	_, err = roleSvc.GrantEnvironmentGroupAccess(ctx, "missing", types.CreateEnvironmentAccessRequest{UserGroupID: "ops", RoleID: authz.BuiltInRoleViewer})
	require.ErrorIs(t, err, common.ErrNotFound)

	_, err = roleSvc.GrantEnvironmentGroupAccess(ctx, "prod", types.CreateEnvironmentAccessRequest{UserGroupID: "audit", RoleID: authz.BuiltInRoleViewer})
	require.NoError(t, err)
	_, err = roleSvc.GrantEnvironmentGroupAccess(ctx, "prod", types.CreateEnvironmentAccessRequest{UserID: createTestUser(t, db, "u-zed", "zed").ID, RoleID: authz.BuiltInRoleDeployer})
	require.NoError(t, err)
	_, err = roleSvc.GrantEnvironmentGroupAccess(ctx, "dev", types.CreateEnvironmentAccessRequest{UserGroupID: "ops", RoleID: authz.BuiltInRoleViewer})
	require.NoError(t, err)
	entries, err := roleSvc.ListEnvironmentGroupAccess(ctx, "prod")
	require.NoError(t, err)
	require.Len(t, entries, 3, "only grants scoped to the environment are listed")
	require.Equal(t, []string{"user:zed", "group:team-audit", "group:team-ops"}, []string{
		entries[0].PrincipalType + ":" + entries[0].Username,
		entries[1].PrincipalType + ":" + entries[1].UserGroupName,
		entries[2].PrincipalType + ":" + entries[2].UserGroupName,
	}, "users come first, then groups by name")

	require.ErrorIs(t, roleSvc.RevokeEnvironmentGroupAccess(ctx, "dev", granted.ID), role.ErrRoleAssignmentNotFound, "the group grant must belong to the environment")
	require.NoError(t, roleSvc.RevokeEnvironmentGroupAccess(ctx, "prod", granted.ID))
	revoked, err := roleSvc.ResolvePermissions(ctx, member.ID)
	require.NoError(t, err)
	require.False(t, revoked.Allows(authz.PermContainersList, "web"), "revoking a group grant invalidates its members' cached permissions")
	require.True(t, revoked.Allows(authz.PermContainersList, "box"), "the group's grant on another environment is untouched")
	require.ErrorIs(t, roleSvc.RevokeEnvironmentGroupAccess(ctx, "prod", granted.ID), role.ErrRoleAssignmentNotFound)
}
