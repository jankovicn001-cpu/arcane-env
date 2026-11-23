package role_test

import (
	"encoding/hex"
	"errors"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/libtnb/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"kit.getarcane.app"

	"getarcane.dev/arcane/v3/internal/common"
	"getarcane.dev/arcane/v3/internal/database"
	"getarcane.dev/arcane/v3/internal/role"
	"getarcane.dev/arcane/v3/internal/role/children/usergroup"
	"getarcane.dev/arcane/v3/internal/session"
	"getarcane.dev/arcane/v3/internal/settings"
	"getarcane.dev/arcane/v3/internal/user"
	"getarcane.dev/arcane/v3/pkg/authz"
	"getarcane.dev/types/v3"
)

type (
	testEnvironmentRow struct {
		database.BaseModel
		AccessToken *string `gorm:"column:access_token"`
		GroupID     *string `gorm:"column:group_id"`
		Name        string
		ApiUrl      string `gorm:"column:api_url"`
		Status      string
		Enabled     bool
	}

	testEnvironmentGroupRow struct {
		ID   string `gorm:"primaryKey"`
		Name string
	}

	testApiKeyRow struct {
		database.BaseModel
		UserID        *string `gorm:"column:user_id"`
		EnvironmentID *string `gorm:"column:environment_id"`
		ManagedBy     *string `gorm:"column:managed_by"`
		ExpiresAt     *time.Time
		Name          string
		KeyHash       string `gorm:"column:key_hash"`
		KeyPrefix     string `gorm:"column:key_prefix"`
		Kind          string
	}
)

func TestValidatePermissionsAgainstCallerRejectsEscalation(t *testing.T) {
	_, roleSvc := setupUserAndRoleServices(t)

	caller := authz.NewPermissionSet()
	caller.AddGlobal(authz.PermRolesRead, authz.PermRolesList)

	err := roleSvc.ValidatePermissionsAgainstCaller(caller, []string{
		authz.PermRolesRead,
		authz.PermUsersDelete,
	})
	require.Error(t, err)
	require.ErrorIs(t, err, common.ErrRolePermissionEscalation)

	require.NoError(t, roleSvc.ValidatePermissionsAgainstCaller(caller, []string{authz.PermRolesRead}))
	require.NoError(t, roleSvc.ValidatePermissionsAgainstCaller(authz.SudoPermissionSet(), []string{authz.PermUsersDelete}))
}

func TestValidatePermissionsAgainstCallerRejectsEnvOnlyGrantForGlobalRole(t *testing.T) {
	_, roleSvc := setupUserAndRoleServices(t)

	caller := authz.NewPermissionSet()
	caller.AddEnv("env-1", authz.PermContainersStart)

	err := roleSvc.ValidatePermissionsAgainstCaller(caller, []string{authz.PermContainersStart})
	require.Error(t, err)
	require.ErrorIs(t, err, common.ErrRolePermissionEscalation)
}

func TestValidatePermissionsAgainstCallerRejectsUnknownPermissionBeforeEscalation(t *testing.T) {
	_, roleSvc := setupUserAndRoleServices(t)

	err := roleSvc.ValidatePermissionsAgainstCaller(authz.SudoPermissionSet(), []string{"containrs:start"})
	require.Error(t, err)
	require.ErrorIs(t, err, common.ErrUnknownPermission)
	require.NotErrorIs(t, err, common.ErrRolePermissionEscalation)
}

func TestBackfillLegacyRoleAssignments(t *testing.T) {
	ctx := t.Context()

	t.Run("no-op without legacy column", func(t *testing.T) {
		db, roleSvc := setupUserAndRoleServices(t)
		require.False(t, db.Migrator().HasColumn("users", "roles"))
		require.NoError(t, roleSvc.BackfillLegacyRoleAssignments(ctx))
		require.NoError(t, roleSvc.BackfillLegacyRoleAssignments(ctx))
	})

	t.Run("converts legacy roles once", func(t *testing.T) {
		db, err := database.Initialize(ctx, "file:"+filepath.Join(t.TempDir(), "arcane.db"), database.MigrationOptions{})
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, db.Close()) })
		require.True(t, db.Migrator().HasColumn("users", "roles"))
		roleSvc := role.NewRoleService(db)
		require.NoError(t, roleSvc.EnsureBuiltInRoles(ctx))
		require.NoError(t, db.Exec("INSERT INTO environments (id, name, api_url) VALUES (?, ?, ?)", "env-1", "env-1", "http://env-1").Error)

		for id, roles := range map[string]string{
			"seed-admin":    `[]`,
			"legacy-admin":  `["user", " ADMIN "]`,
			"legacy-user":   `["user"]`,
			"legacy-empty":  `[]`,
			"legacy-null":   `null`,
			"legacy-blank":  `[" "]`,
			"legacy-broken": `not json`,
			"scoped-manual": `["admin"]`,
			"global-oidc":   `["admin"]`,
		} {
			require.NoError(t, db.Exec("INSERT INTO users (id, username, password_hash, roles) VALUES (?, ?, ?, ?)", id, id, "unused", roles).Error)
		}
		require.NoError(t, db.Create(&role.UserRoleAssignment{UserID: "seed-admin", RoleID: authz.BuiltInRoleAdmin, Source: role.RoleAssignmentSourceManual}).Error)
		envID := "env-1"
		require.NoError(t, roleSvc.ReplaceUserAssignments(ctx, "scoped-manual", role.RoleAssignmentSourceManual, []role.UserRoleAssignment{{RoleID: authz.BuiltInRoleViewer, EnvironmentID: &envID}}))
		require.NoError(t, roleSvc.ReplaceUserAssignments(ctx, "global-oidc", role.RoleAssignmentSourceOidc, []role.UserRoleAssignment{{RoleID: authz.BuiltInRoleViewer}}))

		require.NoError(t, roleSvc.BackfillLegacyRoleAssignments(ctx))
		require.Equal(t, []string{authz.BuiltInRoleAdmin}, assignedRoleIDs(t, roleSvc, "legacy-admin"))
		for _, id := range []string{"legacy-user", "legacy-empty", "legacy-null", "legacy-blank", "legacy-broken"} {
			require.Equal(t, []string{authz.BuiltInRoleViewer}, assignedRoleIDs(t, roleSvc, id), id)
		}
		scoped, err := roleSvc.ListUserAssignments(ctx, "scoped-manual")
		require.NoError(t, err)
		require.Len(t, scoped, 1)
		require.Equal(t, &envID, scoped[0].EnvironmentID)
		oidc, err := roleSvc.ListUserAssignments(ctx, "global-oidc")
		require.NoError(t, err)
		require.Len(t, oidc, 1)
		require.Equal(t, role.RoleAssignmentSourceOidc, oidc[0].Source)
		ps, err := roleSvc.ResolveUserPermissionsInDB(ctx, db.DB, "scoped-manual")
		require.NoError(t, err)
		require.True(t, ps.Allows(authz.PermContainersList, "env-1"))
		require.False(t, ps.Allows(authz.PermContainersList, ""))

		require.NoError(t, roleSvc.ReplaceUserAssignments(ctx, "legacy-user", role.RoleAssignmentSourceManual, nil))
		require.NoError(t, db.Exec("INSERT INTO users (id, username, password_hash, roles) VALUES (?, ?, ?, ?)", "later-admin", "later-admin", "unused", `["admin"]`).Error)
		require.NoError(t, role.NewRoleService(db).BackfillLegacyRoleAssignments(ctx))
		require.Empty(t, assignedRoleIDs(t, roleSvc, "legacy-user"))
		require.Empty(t, assignedRoleIDs(t, roleSvc, "later-admin"))
	})
}

func assignedRoleIDs(t *testing.T, roleSvc *role.RoleService, userID string) []string {
	t.Helper()
	assignments, err := roleSvc.ListUserAssignments(t.Context(), userID)
	require.NoError(t, err)
	ids := make([]string, 0, len(assignments))
	for _, a := range assignments {
		ids = append(ids, a.RoleID)
	}
	return ids
}

func TestEnsureBuiltInRolesMigratesVariablePermissionsWithoutBackfillingCustomGrants(t *testing.T) {
	ctx := t.Context()
	userSvc, roleSvc := setupUserAndRoleServices(t)

	customRole, err := roleSvc.CreateRole(ctx, "Template Reader", nil, []string{authz.PermTemplatesRead})
	require.NoError(t, err)
	owner := createTestUser(t, userSvc, "variable-migration-owner", "variable-migration-owner")
	scopedKey := testApiKeyRow{
		Name:      "Custom scoped key",
		KeyHash:   "variable-migration-hash",
		KeyPrefix: "arc_vars",
		Kind:      "scoped",
		UserID:    &owner.ID,
	}
	require.NoError(t, userSvc.WithContext(ctx).Create(&scopedKey).Error)
	require.NoError(t, userSvc.WithContext(ctx).Create(&role.ApiKeyPermission{
		ApiKeyID:   scopedKey.ID,
		Permission: authz.PermTemplatesRead,
	}).Error)

	oldEditorPermissions := slices.DeleteFunc(authz.BuiltInEditorPermissions(), func(permission string) bool {
		return slices.Contains([]string{
			authz.PermVariablesList,
			authz.PermVariablesRead,
			authz.PermVariablesCreate,
			authz.PermVariablesUpdate,
			authz.PermVariablesDelete,
			authz.PermVariablesSync,
		}, permission)
	})
	require.NoError(t, userSvc.WithContext(ctx).Model(&role.Role{}).
		Where("id = ?", authz.BuiltInRoleEditor).
		Update("permissions", database.StringSlice(oldEditorPermissions)).Error)

	require.NoError(t, roleSvc.EnsureBuiltInRoles(ctx))

	allVariablePermissions := []string{
		authz.PermVariablesList,
		authz.PermVariablesRead,
		authz.PermVariablesCreate,
		authz.PermVariablesUpdate,
		authz.PermVariablesDelete,
		authz.PermVariablesSync,
	}
	for _, roleID := range []string{authz.BuiltInRoleAdmin, authz.BuiltInRoleEditor, authz.BuiltInRoleNoShellEditor} {
		builtIn, getErr := roleSvc.GetRole(ctx, roleID)
		require.NoError(t, getErr)
		for _, permission := range allVariablePermissions {
			require.Contains(t, []string(builtIn.Permissions), permission, "role %s", roleID)
		}
	}
	for _, roleID := range []string{authz.BuiltInRoleViewer, authz.BuiltInRoleDeployer} {
		builtIn, getErr := roleSvc.GetRole(ctx, roleID)
		require.NoError(t, getErr)
		require.Contains(t, []string(builtIn.Permissions), authz.PermVariablesList)
		require.Contains(t, []string(builtIn.Permissions), authz.PermVariablesRead)
		for _, permission := range allVariablePermissions[2:] {
			require.NotContains(t, []string(builtIn.Permissions), permission, "role %s", roleID)
		}
	}
	monitor, err := roleSvc.GetRole(ctx, authz.BuiltInRoleMonitor)
	require.NoError(t, err)
	for _, permission := range allVariablePermissions {
		require.NotContains(t, []string(monitor.Permissions), permission)
	}

	preservedCustomRole, err := roleSvc.GetRole(ctx, customRole.ID)
	require.NoError(t, err)
	require.Equal(t, []string{authz.PermTemplatesList, authz.PermTemplatesRead}, []string(preservedCustomRole.Permissions))

	var keyPermissions []role.ApiKeyPermission
	require.NoError(t, userSvc.WithContext(ctx).Where("api_key_id = ?", scopedKey.ID).Find(&keyPermissions).Error)
	require.Len(t, keyPermissions, 1)
	require.Equal(t, authz.PermTemplatesRead, keyPermissions[0].Permission)
}

func TestSetUserAssignmentsRejectsUnknownRole(t *testing.T) {
	ctx := t.Context()
	userSvc, roleSvc := setupUserAndRoleServices(t)
	victim := createTestUser(t, userSvc, "victim", "victim")

	err := roleSvc.ReplaceUserAssignments(ctx, victim.ID, role.RoleAssignmentSourceManual, []role.UserRoleAssignment{
		{RoleID: "role_does_not_exist"},
	})
	require.Error(t, err)
	require.ErrorIs(t, err, common.ErrInvalidRoleAssignment)
}

func TestReplaceUserAssignmentsOidcRejectsUnknownRole(t *testing.T) {
	ctx := t.Context()
	userSvc, roleSvc := setupUserAndRoleServices(t)
	oidcUser := createTestUser(t, userSvc, "oidc-user", "oidc-user")

	err := roleSvc.ReplaceUserAssignments(ctx, oidcUser.ID, role.RoleAssignmentSourceOidc, []role.UserRoleAssignment{
		{RoleID: "role_does_not_exist"},
	})
	require.Error(t, err)
	require.ErrorIs(t, err, common.ErrInvalidRoleAssignment)
}

func TestReplaceUserAssignmentsOidcRejectsUnknownEnvironment(t *testing.T) {
	ctx := t.Context()
	userSvc, roleSvc := setupUserAndRoleServices(t)
	oidcUser := createTestUser(t, userSvc, "oidc-user-env", "oidc-user-env")
	missingEnv := "env_does_not_exist"

	// A valid role scoped to a non-existent environment must fail existence
	// validation (same as manual assignments) rather than attempting an insert.
	err := roleSvc.ReplaceUserAssignments(ctx, oidcUser.ID, role.RoleAssignmentSourceOidc, []role.UserRoleAssignment{
		{RoleID: authz.BuiltInRoleViewer, EnvironmentID: &missingEnv},
	})
	require.Error(t, err)
	require.ErrorIs(t, err, common.ErrInvalidRoleAssignment)
}

func TestEffectiveGlobalAdminCountIncludesCustomAllPermissionsRole(t *testing.T) {
	ctx := t.Context()
	userSvc, roleSvc := setupUserAndRoleServices(t)
	customAdmin := createTestUser(t, userSvc, "custom-admin", "custom-admin")
	customRole, err := roleSvc.CreateRole(ctx, "Custom Admin", nil, authz.AllPermissions())
	require.NoError(t, err)

	require.NoError(t, roleSvc.ReplaceUserAssignments(ctx, customAdmin.ID, role.RoleAssignmentSourceManual, []role.UserRoleAssignment{
		{RoleID: customRole.ID, EnvironmentID: nil},
	}))

	count, err := roleSvc.CountGlobalAdmins(ctx, userSvc.DB, "")
	require.NoError(t, err)
	require.Equal(t, 1, count)
	require.NoError(t, roleSvc.AssertGlobalAdminExists(ctx, userSvc.DB))

	err = roleSvc.ReplaceUserAssignments(ctx, customAdmin.ID, role.RoleAssignmentSourceManual, nil)
	require.Error(t, err)
	require.ErrorIs(t, err, common.ErrNoGlobalAdminRemains)
}

func TestEffectiveGlobalAdminCountIgnoresEnvScopedAndServiceAccounts(t *testing.T) {
	ctx := t.Context()
	userSvc, roleSvc := setupUserAndRoleServices(t)
	customRole, err := roleSvc.CreateRole(ctx, "Custom Admin", nil, authz.AllPermissions())
	require.NoError(t, err)
	envID := "env-1"
	createTestEnvironment(t, userSvc, envID, "http://localhost:3552")

	globalAdmin := createTestUser(t, userSvc, "global-admin", "global-admin")
	envScopedAdmin := createTestUser(t, userSvc, "env-scoped-admin", "env-scoped-admin")
	serviceAdmin := &user.User{
		ID:               "service-admin",
		Username:         "service-admin",
		IsServiceAccount: true,
	}
	require.NoError(t, userSvc.WithContext(ctx).Create(serviceAdmin).Error)

	require.NoError(t, roleSvc.ReplaceUserAssignments(ctx, globalAdmin.ID, role.RoleAssignmentSourceManual, []role.UserRoleAssignment{
		{RoleID: customRole.ID, EnvironmentID: nil},
	}))
	require.NoError(t, roleSvc.ReplaceUserAssignments(ctx, envScopedAdmin.ID, role.RoleAssignmentSourceManual, []role.UserRoleAssignment{
		{RoleID: customRole.ID, EnvironmentID: &envID},
	}))
	require.NoError(t, roleSvc.ReplaceUserAssignments(ctx, serviceAdmin.ID, role.RoleAssignmentSourceManual, []role.UserRoleAssignment{
		{RoleID: customRole.ID, EnvironmentID: nil},
	}))

	count, err := roleSvc.CountGlobalAdmins(ctx, userSvc.DB, "")
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func setupAuthServiceTestDB(t *testing.T) *database.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&settings.SettingVariable{},
		&user.User{},
		&session.UserSession{},
		&testEnvironmentRow{},
		&testEnvironmentGroupRow{},
		&role.Role{},
		&role.UserRoleAssignment{},
		&testApiKeyRow{},
		&role.ApiKeyPermission{},
		&role.OidcRoleMapping{},
		&usergroup.Group{},
		&usergroup.Link{},
		&usergroup.Member{},
		&usergroup.Grant{},
	))
	return &database.DB{DB: db}
}

func setupUserAndRoleServices(t *testing.T) (*database.DB, *role.RoleService) {
	t.Helper()
	db := setupAuthServiceTestDB(t)
	roleService := role.NewRoleService(db)
	require.NoError(t, roleService.EnsureBuiltInRoles(t.Context()))
	return db, roleService
}

func createTestUser(t *testing.T, db *database.DB, id, username string) *user.User {
	t.Helper()
	created := &user.User{ID: id, Username: username}
	require.NoError(t, db.WithContext(t.Context()).Create(created).Error)
	return created
}

func grantGlobalAdmin(t *testing.T, roleService *role.RoleService, userID string) {
	t.Helper()
	require.NoError(t, roleService.ReplaceUserAssignments(t.Context(), userID, role.RoleAssignmentSourceManual, []role.UserRoleAssignment{
		{RoleID: authz.BuiltInRoleAdmin},
	}))
}

func createTestEnvironment(t *testing.T, db *database.DB, id, apiURL string) {
	t.Helper()
	now := time.Now()
	require.NoError(t, db.WithContext(t.Context()).Create(&testEnvironmentRow{
		ID: id, CreatedAt: now, UpdatedAt: &now,
		Name:    "env-" + id,
		ApiUrl:  apiURL,
		Status:  "online",
		Enabled: true,
	}).Error)
}

func (testEnvironmentRow) TableName() string { return "environments" }

func (testApiKeyRow) TableName() string { return "api_keys" }

func (testEnvironmentGroupRow) TableName() string { return "environment_groups" }

// createTestGroup creates an environment group holding the given agents.
func createTestGroup(t *testing.T, db *database.DB, groupID string, agentIDs ...string) {
	t.Helper()
	require.NoError(t, db.Create(&testEnvironmentGroupRow{ID: groupID, Name: "group-" + groupID}).Error)
	for _, agentID := range agentIDs {
		createTestEnvironment(t, db, agentID, "agent://"+agentID)
		moveTestAgent(t, db, agentID, &groupID)
	}
}

func moveTestAgent(t *testing.T, db *database.DB, agentID string, groupID *string) {
	t.Helper()
	require.NoError(t, db.Model(&testEnvironmentRow{}).Where("id = ?", agentID).Update("group_id", groupID).Error)
}

func TestResolveExecutionPermissions(t *testing.T) {
	t.Run("personal keys use current user permissions and explicit scope", func(t *testing.T) {
		db, service := setupUserAndRoleServices(t)
		owner := createTestUser(t, db, "owner", "owner")
		grantGlobalAdmin(t, service, owner.ID)
		environmentID := "env-1"
		key := testApiKeyRow{ID: "personal", Kind: "personal", UserID: &owner.ID, EnvironmentID: &environmentID}
		require.NoError(t, db.Create(&key).Error)
		permissions, err := service.ResolveExecutionPermissions(t.Context(), owner.ID, key.ID)
		require.NoError(t, err)
		require.True(t, permissions.Allows(authz.PermJobsRun, environmentID))
		require.False(t, permissions.Allows(authz.PermJobsRun, "env-2"))
		require.False(t, permissions.IsGlobalAdmin())

		require.NoError(t, db.Where("user_id = ?", owner.ID).Delete(&role.UserRoleAssignment{}).Error)
		for _, keyID := range []string{"", key.ID} {
			permissions, err = service.ResolveExecutionPermissions(t.Context(), owner.ID, keyID)
			require.NoError(t, err)
			require.False(t, permissions.Allows(authz.PermJobsRun, environmentID))
		}
	})

	t.Run("scoped keys bypass cached grants", func(t *testing.T) {
		db, service := setupUserAndRoleServices(t)
		owner := createTestUser(t, db, "owner", "owner")
		grantGlobalAdmin(t, service, owner.ID)
		environmentID := "env-1"
		key := testApiKeyRow{ID: "scoped", Kind: "scoped", UserID: &owner.ID, EnvironmentID: &environmentID}
		require.NoError(t, db.Create(&key).Error)
		require.NoError(t, service.SetApiKeyPermissions(t.Context(), key.ID, []role.ApiKeyPermission{{Permission: authz.PermJobsRun}}))
		cached, err := service.ResolveApiKeyPermissions(t.Context(), key.ID)
		require.NoError(t, err)
		require.True(t, cached.Allows(authz.PermJobsRun, "env-2"))
		permissions, err := service.ResolveExecutionPermissions(t.Context(), owner.ID, key.ID)
		require.NoError(t, err)
		require.True(t, permissions.Allows(authz.PermJobsRun, environmentID))
		require.False(t, permissions.Allows(authz.PermJobsRun, "env-2"))

		require.NoError(t, db.Where("api_key_id = ?", key.ID).Delete(&role.ApiKeyPermission{}).Error)
		permissions, err = service.ResolveExecutionPermissions(t.Context(), owner.ID, key.ID)
		require.NoError(t, err)
		require.False(t, permissions.Allows(authz.PermJobsRun, environmentID))
	})

	t.Run("rejects invalid persisted identities", func(t *testing.T) {
		db, service := setupUserAndRoleServices(t)
		owner := createTestUser(t, db, "owner", "owner")
		otherOwner := "other"
		emptyEnvironment := ""
		expired := time.Now().Add(-time.Minute)
		for _, key := range []testApiKeyRow{
			{ID: "expired", Kind: "personal", UserID: &owner.ID, ExpiresAt: &expired},
			{ID: "other-owner", Kind: "personal", UserID: &otherOwner},
			{ID: "ownerless", Kind: "scoped"},
			{ID: "invalid-kind", Kind: "unknown", UserID: &owner.ID},
			{ID: "invalid-scope", Kind: "personal", UserID: &owner.ID, EnvironmentID: &emptyEnvironment},
		} {
			require.NoError(t, db.Create(&key).Error)
			_, err := service.ResolveExecutionPermissions(t.Context(), owner.ID, key.ID)
			require.Error(t, err, key.ID)
		}
		_, err := service.ResolveExecutionPermissions(t.Context(), owner.ID, "deleted-key")
		require.Error(t, err)
		require.NoError(t, db.Delete(owner).Error)
		_, err = service.ResolveExecutionPermissions(t.Context(), owner.ID, "")
		require.Error(t, err)
	})
}

func TestCreateAndUpdateRoleStoreRequiredPermissions(t *testing.T) {
	ctx := t.Context()
	_, roleSvc := setupUserAndRoleServices(t)

	created, err := roleSvc.CreateRole(ctx, "Image patcher", nil, []string{authz.PermImagesPatch})
	require.NoError(t, err)
	require.ElementsMatch(t, []string{
		authz.PermImagesList,
		authz.PermImagesRead,
		authz.PermImagesPatch,
		authz.PermVulnsList,
		authz.PermActivitiesList,
		authz.PermActivitiesRead,
	}, []string(created.Permissions))

	updated, err := roleSvc.UpdateRole(ctx, created.ID, "Image patcher", nil, []string{authz.PermDiagnosticsProfile})
	require.NoError(t, err)
	require.ElementsMatch(t, []string{
		authz.PermDiagnosticsRead,
		authz.PermDiagnosticsProfile,
	}, []string(updated.Permissions))

	stored, err := roleSvc.GetRole(ctx, created.ID)
	require.NoError(t, err)
	require.ElementsMatch(t, []string(updated.Permissions), []string(stored.Permissions))
}

func TestPermissionsManifestExposesTransitiveRequirements(t *testing.T) {
	requires := make(map[string][]string)
	descriptions := make(map[string]string)
	for _, resource := range role.BuildPermissionsManifest().Resources {
		require.NotEmpty(t, resource.Description, "resource %s", resource.Key)
		for _, action := range resource.Actions {
			requires[action.Permission] = action.Requires
			descriptions[action.Permission] = action.Description
		}
	}
	require.ElementsMatch(t, []string{authz.PermImagesList, authz.PermImagesRead, authz.PermVulnsList, authz.PermActivitiesList, authz.PermActivitiesRead}, requires[authz.PermImagesPatch])
	require.Empty(t, requires[authz.PermContainersList])
	require.NotEmpty(t, descriptions[authz.PermContainersList])
}

func TestPermissionsManifestETagIsStableHex(t *testing.T) {
	etag := role.PermissionsManifestETag()
	decoded, err := hex.DecodeString(etag)
	require.NoError(t, err)
	require.Len(t, decoded, 8)
	require.Equal(t, etag, role.PermissionsManifestETag())
}

func TestResolvePermissionsExpandsEnvironmentGroupGrants(t *testing.T) {
	ctx := t.Context()
	db, roleSvc := setupUserAndRoleServices(t)
	grantGlobalAdmin(t, roleSvc, createTestUser(t, db, "root", "root").ID)
	member := createTestUser(t, db, "member", "member")
	createTestGroup(t, db, "prod", "web", "api")
	createTestEnvironment(t, db, "loose", "agent://loose")
	require.NoError(t, roleSvc.ReplaceUserAssignments(ctx, member.ID, role.RoleAssignmentSourceManual, []role.UserRoleAssignment{
		{RoleID: authz.BuiltInRoleViewer, EnvironmentGroupID: new("prod")},
	}))

	ps, err := roleSvc.ResolvePermissions(ctx, member.ID)
	require.NoError(t, err)
	require.True(t, ps.Allows(authz.PermContainersList, "web"))
	require.True(t, ps.Allows(authz.PermContainersList, "api"))
	require.False(t, ps.Allows(authz.PermContainersList, "loose"))
	require.False(t, ps.Allows(authz.PermContainersList, ""), "a group grant never applies globally")
	require.Empty(t, ps.Global)

	moveTestAgent(t, db, "loose", new("prod"))
	moveTestAgent(t, db, "web", nil)
	cached, err := roleSvc.ResolvePermissions(ctx, member.ID)
	require.NoError(t, err)
	require.True(t, cached.Allows(authz.PermContainersList, "web"), "membership changes stay cached until invalidated")

	roleSvc.InvalidateAllUsers()
	ps, err = roleSvc.ResolvePermissions(ctx, member.ID)
	require.NoError(t, err)
	require.True(t, ps.Allows(authz.PermContainersList, "loose"), "an agent joining the group gains the grant")
	require.False(t, ps.Allows(authz.PermContainersList, "web"), "an agent leaving the group loses the grant")
	require.True(t, ps.Allows(authz.PermContainersList, "api"))
}

func TestResolvePermissionsIgnoresEmptyAndDeletedGroups(t *testing.T) {
	ctx := t.Context()
	db, roleSvc := setupUserAndRoleServices(t)
	grantGlobalAdmin(t, roleSvc, createTestUser(t, db, "root", "root").ID)
	member := createTestUser(t, db, "member", "member")
	createTestGroup(t, db, "empty")
	createTestGroup(t, db, "doomed", "web")
	require.NoError(t, roleSvc.ReplaceUserAssignments(ctx, member.ID, role.RoleAssignmentSourceManual, []role.UserRoleAssignment{
		{RoleID: authz.BuiltInRoleAdmin, EnvironmentGroupID: new("empty")},
		{RoleID: authz.BuiltInRoleAdmin, EnvironmentGroupID: new("doomed")},
	}))

	// Simulate a deletion that left the assignment rows behind, as SQLite without foreign keys would.
	moveTestAgent(t, db, "web", nil)
	require.NoError(t, db.Delete(&testEnvironmentGroupRow{}, "id IN ?", []string{"empty", "doomed"}).Error)
	roleSvc.InvalidateAllUsers()

	ps, err := roleSvc.ResolvePermissions(ctx, member.ID)
	require.NoError(t, err)
	require.Empty(t, ps.Global)
	require.Empty(t, ps.PerEnv)
	require.False(t, ps.IsGlobalAdmin())
}

func TestGroupScopedAdminIsNotGlobalAdmin(t *testing.T) {
	ctx := t.Context()
	db, roleSvc := setupUserAndRoleServices(t)
	globalAdmin := createTestUser(t, db, "global-admin", "global-admin")
	groupAdmin := createTestUser(t, db, "group-admin", "group-admin")
	createTestGroup(t, db, "prod", "web")
	grantGlobalAdmin(t, roleSvc, globalAdmin.ID)
	require.NoError(t, roleSvc.ReplaceUserAssignments(ctx, groupAdmin.ID, role.RoleAssignmentSourceManual, []role.UserRoleAssignment{
		{RoleID: authz.BuiltInRoleAdmin, EnvironmentGroupID: new("prod")},
	}))

	count, err := roleSvc.CountGlobalAdmins(ctx, db.DB, "")
	require.NoError(t, err)
	require.Equal(t, 1, count)

	err = roleSvc.ReplaceUserAssignments(ctx, globalAdmin.ID, role.RoleAssignmentSourceManual, []role.UserRoleAssignment{
		{RoleID: authz.BuiltInRoleAdmin, EnvironmentGroupID: new("prod")},
	})
	require.ErrorIs(t, err, common.ErrNoGlobalAdminRemains, "narrowing the last admin to a group is still a demotion")
}

func TestRoleScopesAreMutuallyExclusiveAndValidated(t *testing.T) {
	ctx := t.Context()
	db, roleSvc := setupUserAndRoleServices(t)
	grantGlobalAdmin(t, roleSvc, createTestUser(t, db, "root", "root").ID)
	member := createTestUser(t, db, "member", "member")
	createTestGroup(t, db, "prod", "web")
	requireGroupFieldError := func(err error) {
		t.Helper()
		require.ErrorIs(t, err, common.ErrInvalidRoleAssignment)
		fieldErr, ok := errors.AsType[*types.FieldError](err)
		require.True(t, ok, "expected a field error, got %v", err)
		require.Equal(t, "environmentGroupId", fieldErr.Field)
	}

	requireGroupFieldError(roleSvc.ReplaceUserAssignments(ctx, member.ID, role.RoleAssignmentSourceManual, []role.UserRoleAssignment{
		{RoleID: authz.BuiltInRoleViewer, EnvironmentID: new("web"), EnvironmentGroupID: new("prod")},
	}))
	requireGroupFieldError(roleSvc.ReplaceUserAssignments(ctx, member.ID, role.RoleAssignmentSourceManual, []role.UserRoleAssignment{
		{RoleID: authz.BuiltInRoleViewer, EnvironmentGroupID: new("missing")},
	}))
	_, err := roleSvc.CreateOidcMapping(ctx, types.OidcRoleMappingSpec{
		ClaimValue: "devs", RoleID: authz.BuiltInRoleViewer, EnvironmentID: new("web"), EnvironmentGroupID: new("prod"),
	})
	requireGroupFieldError(err)
	_, err = roleSvc.CreateOidcMapping(ctx, types.OidcRoleMappingSpec{ClaimValue: "devs", RoleID: authz.BuiltInRoleViewer, EnvironmentGroupID: new("missing")})
	requireGroupFieldError(err)

	mapping, err := roleSvc.CreateOidcMapping(ctx, types.OidcRoleMappingSpec{ClaimValue: "devs", RoleID: authz.BuiltInRoleViewer, EnvironmentGroupID: new("prod")})
	require.NoError(t, err)
	require.Equal(t, "prod", *mapping.EnvironmentGroupID)
	_, err = roleSvc.UpdateOidcMapping(ctx, mapping.ID, types.OidcRoleMappingSpec{
		ClaimValue: "devs", RoleID: authz.BuiltInRoleViewer, EnvironmentID: new("web"), EnvironmentGroupID: new("prod"),
	})
	requireGroupFieldError(err)
	updated, err := roleSvc.UpdateOidcMapping(ctx, mapping.ID, types.OidcRoleMappingSpec{ClaimValue: "devs", RoleID: authz.BuiltInRoleViewer, EnvironmentID: new("web")})
	require.NoError(t, err)
	require.Nil(t, updated.EnvironmentGroupID, "switching to an agent scope clears the group scope")

	require.NoError(t, roleSvc.ReplaceUserAssignments(ctx, member.ID, role.RoleAssignmentSourceManual, []role.UserRoleAssignment{
		{RoleID: authz.BuiltInRoleViewer},
		{RoleID: authz.BuiltInRoleViewer, EnvironmentGroupID: new("prod")},
		{RoleID: authz.BuiltInRoleViewer, EnvironmentGroupID: new("prod")},
	}))
	rows, err := roleSvc.ListUserAssignments(ctx, member.ID)
	require.NoError(t, err)
	require.Len(t, rows, 2, "duplicates collapse while a group grant sits beside the same global grant")
}

func TestOidcSyncCopiesGroupScope(t *testing.T) {
	ctx := t.Context()
	db, roleSvc := setupUserAndRoleServices(t)
	grantGlobalAdmin(t, roleSvc, createTestUser(t, db, "root", "root").ID)
	member := createTestUser(t, db, "member", "member")
	createTestGroup(t, db, "prod", "web")
	_, err := roleSvc.CreateOidcMapping(ctx, types.OidcRoleMappingSpec{ClaimValue: "prod-ops", RoleID: authz.BuiltInRoleDeployer, EnvironmentGroupID: new("prod")})
	require.NoError(t, err)
	_, err = roleSvc.CreateOidcMapping(ctx, types.OidcRoleMappingSpec{ClaimValue: "everyone", RoleID: authz.BuiltInRoleViewer})
	require.NoError(t, err)
	mappings, err := roleSvc.ListOidcMappings(ctx)
	require.NoError(t, err)

	require.NoError(t, roleSvc.ReplaceUserAssignments(ctx, member.ID, role.RoleAssignmentSourceOidc, role.MappedAssignments(mappings, []string{"prod-ops"})))
	rows, err := roleSvc.ListUserAssignments(ctx, member.ID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, role.RoleAssignmentSourceOidc, rows[0].Source)
	require.Equal(t, "prod", kit.FromPtr(rows[0].EnvironmentGroupID))
	require.Nil(t, rows[0].EnvironmentID)

	ps, err := roleSvc.ResolvePermissions(ctx, member.ID)
	require.NoError(t, err)
	require.True(t, ps.Allows(authz.PermProjectsDeploy, "web"))
	require.False(t, ps.Allows(authz.PermProjectsDeploy, ""))
}

func TestReconcileEnvOidcMappingsRejectsGroupScope(t *testing.T) {
	_, roleSvc := setupUserAndRoleServices(t)
	err := roleSvc.ReconcileEnvOidcMappings(t.Context(), `[{"claimValue":"ops","roleId":"`+authz.BuiltInRoleViewer+`","environmentGroupId":"prod"}]`)
	require.ErrorContains(t, err, "environmentGroupId is not supported")
}

// createTestUserGroup creates a user group whose members hold the given grants; scopes follow the grant rows.
func createTestUserGroup(t *testing.T, db *database.DB, groupID string, memberIDs []string, grants ...usergroup.Grant) {
	t.Helper()
	require.NoError(t, db.Create(&usergroup.Group{BaseModel: database.BaseModel{ID: groupID}, Name: "team-" + groupID}).Error)
	require.NoError(t, db.Create(&usergroup.Link{UserGroupID: groupID, Kind: role.RoleAssignmentSourceOidc, Value: groupID}).Error)
	for _, memberID := range memberIDs {
		require.NoError(t, db.Create(&usergroup.Member{UserGroupID: groupID, UserID: memberID, Source: role.RoleAssignmentSourceManual}).Error)
	}
	for _, grant := range grants {
		grant.UserGroupID = groupID
		require.NoError(t, db.Create(&grant).Error)
	}
}

func TestResolvePermissionsIncludesUserGroupGrantsForEveryScope(t *testing.T) {
	ctx := t.Context()
	db, roleSvc := setupUserAndRoleServices(t)
	member := createTestUser(t, db, "member", "member")
	outsider := createTestUser(t, db, "outsider", "outsider")
	createTestGroup(t, db, "prod", "web", "api")
	createTestEnvironment(t, db, "box", "agent://box")
	createTestEnvironment(t, db, "loose", "agent://loose")
	createTestUserGroup(t, db, "ops", []string{member.ID},
		usergroup.Grant{RoleID: authz.BuiltInRoleViewer},
		usergroup.Grant{RoleID: authz.BuiltInRoleDeployer, EnvironmentID: new("box")},
		usergroup.Grant{RoleID: authz.BuiltInRoleAdmin, EnvironmentGroupID: new("prod")},
	)

	ps, err := roleSvc.ResolvePermissions(ctx, member.ID)
	require.NoError(t, err)
	require.True(t, ps.Allows(authz.PermContainersList, ""), "a global group grant applies globally")
	require.True(t, ps.Allows(authz.PermProjectsDeploy, "box"), "an agent group grant applies to that agent")
	require.False(t, ps.Allows(authz.PermProjectsDeploy, "loose"))
	require.True(t, ps.Allows(authz.PermUsersDelete, "web"), "an environment group grant expands to its agents")
	require.True(t, ps.Allows(authz.PermUsersDelete, "api"))
	require.False(t, ps.Allows(authz.PermUsersDelete, "box"))
	require.False(t, ps.IsGlobalAdmin(), "admin on an environment never widens to global")

	others, err := roleSvc.ResolvePermissions(ctx, outsider.ID)
	require.NoError(t, err)
	require.Empty(t, others.Global)
	require.Empty(t, others.PerEnv, "grants reach only members")
}

func TestResolvePermissionsNeverWidensMalformedScopes(t *testing.T) {
	ctx := t.Context()
	db, roleSvc := setupUserAndRoleServices(t)
	member := createTestUser(t, db, "member", "member")
	// Rows written around validation must not fall through to a global grant.
	createTestUserGroup(t, db, "ops", []string{member.ID},
		usergroup.Grant{RoleID: authz.BuiltInRoleAdmin, EnvironmentID: new("")},
		usergroup.Grant{RoleID: authz.BuiltInRoleAdmin, EnvironmentGroupID: new("deleted")},
	)
	require.NoError(t, db.Create(&role.UserRoleAssignment{UserID: member.ID, RoleID: authz.BuiltInRoleAdmin, EnvironmentID: new(""), Source: role.RoleAssignmentSourceManual}).Error)

	ps, err := roleSvc.ResolvePermissions(ctx, member.ID)
	require.NoError(t, err)
	require.Empty(t, ps.Global)
	require.Empty(t, ps.PerEnv)
	require.False(t, ps.IsGlobalAdmin())
}

func TestUserGroupAdminNeverSatisfiesLastAdminGuard(t *testing.T) {
	ctx := t.Context()
	db, roleSvc := setupUserAndRoleServices(t)
	direct := createTestUser(t, db, "direct", "direct")
	viaGroup := createTestUser(t, db, "via-group", "via-group")
	grantGlobalAdmin(t, roleSvc, direct.ID)
	createTestUserGroup(t, db, "admins", []string{viaGroup.ID}, usergroup.Grant{RoleID: authz.BuiltInRoleAdmin})

	ps, err := roleSvc.ResolvePermissions(ctx, viaGroup.ID)
	require.NoError(t, err)
	require.True(t, ps.IsGlobalAdmin(), "a global admin grant through a group is effective")
	count, err := roleSvc.CountGlobalAdmins(ctx, db.DB, "")
	require.NoError(t, err)
	require.Equal(t, 1, count, "only direct assignments count toward the guard")

	err = roleSvc.ReplaceUserAssignments(ctx, direct.ID, role.RoleAssignmentSourceManual, nil)
	require.ErrorIs(t, err, common.ErrNoGlobalAdminRemains, "group admins cannot replace the last direct admin")
}

func TestRoleChangesReachUserGroupMembers(t *testing.T) {
	ctx := t.Context()
	db, roleSvc := setupUserAndRoleServices(t)
	grantGlobalAdmin(t, roleSvc, createTestUser(t, db, "root", "root").ID)
	member := createTestUser(t, db, "member", "member")
	custom, err := roleSvc.CreateRole(ctx, "Operators", nil, []string{authz.PermContainersList})
	require.NoError(t, err)
	createTestUserGroup(t, db, "ops", []string{member.ID}, usergroup.Grant{RoleID: custom.ID})

	before, err := roleSvc.ResolvePermissions(ctx, member.ID)
	require.NoError(t, err)
	require.False(t, before.Allows(authz.PermContainersStart, ""))
	_, err = roleSvc.UpdateRole(ctx, custom.ID, "Operators", nil, []string{authz.PermContainersList, authz.PermContainersStart})
	require.NoError(t, err)
	widened, err := roleSvc.ResolvePermissions(ctx, member.ID)
	require.NoError(t, err)
	require.True(t, widened.Allows(authz.PermContainersStart, ""), "updating a role invalidates members holding it through a group")

	require.NoError(t, db.Exec("CREATE TABLE federated_credentials (id text PRIMARY KEY, role_id text)").Error)
	require.NoError(t, roleSvc.DeleteRole(ctx, custom.ID))
	var grants int64
	require.NoError(t, db.Model(&usergroup.Grant{}).Where("role_id = ?", custom.ID).Count(&grants).Error)
	require.Zero(t, grants, "deleting a role deletes its group grants")
	removed, err := roleSvc.ResolvePermissions(ctx, member.ID)
	require.NoError(t, err)
	require.Empty(t, removed.Global, "deleting a role invalidates members holding it through a group")
}

func TestReplaceUserGroupMembershipsSyncsAndInvalidates(t *testing.T) {
	ctx := t.Context()
	db, roleSvc := setupUserAndRoleServices(t)
	member := createTestUser(t, db, "member", "member")
	createTestUserGroup(t, db, "ops", nil, usergroup.Grant{RoleID: authz.BuiltInRoleViewer})

	before, err := roleSvc.ResolvePermissions(ctx, member.ID)
	require.NoError(t, err)
	require.Empty(t, before.Global)

	require.NoError(t, roleSvc.ReplaceUserGroupMemberships(ctx, member.ID, role.RoleAssignmentSourceOidc, []string{"ops", "unlinked"}))
	joined, err := roleSvc.ResolvePermissions(ctx, member.ID)
	require.NoError(t, err)
	require.True(t, joined.Allows(authz.PermContainersList, ""), "a synced membership invalidates the user's cached permissions")
	refs, err := roleSvc.UserGroupRefs(ctx, member.ID)
	require.NoError(t, err)
	require.Equal(t, []types.UserGroupRef{{ID: "ops", Name: "team-ops"}}, refs)

	require.NoError(t, roleSvc.ReplaceUserGroupMemberships(ctx, member.ID, role.RoleAssignmentSourceOidc, nil))
	left, err := roleSvc.ResolvePermissions(ctx, member.ID)
	require.NoError(t, err)
	require.Empty(t, left.Global, "losing the claim removes the membership and its access")
}
