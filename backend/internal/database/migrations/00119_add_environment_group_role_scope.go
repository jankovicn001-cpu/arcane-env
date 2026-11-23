package migrations

import (
	"github.com/pressly/goose/v3"
)

const (
	v119SQLiteUp = `
ALTER TABLE user_role_assignments ADD COLUMN environment_group_id TEXT
    REFERENCES environment_groups(id) ON DELETE CASCADE
    CHECK (environment_group_id IS NULL OR environment_id IS NULL);
ALTER TABLE oidc_role_mappings ADD COLUMN environment_group_id TEXT
    REFERENCES environment_groups(id) ON DELETE CASCADE
    CHECK (environment_group_id IS NULL OR environment_id IS NULL);
CREATE INDEX IF NOT EXISTS idx_ura_env_group ON user_role_assignments(environment_group_id);
CREATE INDEX IF NOT EXISTS idx_orm_env_group ON oidc_role_mappings(environment_group_id);
DROP INDEX IF EXISTS idx_ura_uniq;
CREATE UNIQUE INDEX idx_ura_uniq
    ON user_role_assignments(user_id, role_id, COALESCE(environment_id, ''), COALESCE(environment_group_id, ''), source);
`
	v119PostgresUp = `
ALTER TABLE user_role_assignments ADD COLUMN IF NOT EXISTS environment_group_id text
    REFERENCES environment_groups(id) ON DELETE CASCADE;
ALTER TABLE user_role_assignments ADD CONSTRAINT user_role_assignments_single_scope
    CHECK (environment_group_id IS NULL OR environment_id IS NULL);
ALTER TABLE oidc_role_mappings ADD COLUMN IF NOT EXISTS environment_group_id text
    REFERENCES environment_groups(id) ON DELETE CASCADE;
ALTER TABLE oidc_role_mappings ADD CONSTRAINT oidc_role_mappings_single_scope
    CHECK (environment_group_id IS NULL OR environment_id IS NULL);
CREATE INDEX IF NOT EXISTS idx_ura_env_group ON user_role_assignments USING btree (environment_group_id);
CREATE INDEX IF NOT EXISTS idx_orm_env_group ON oidc_role_mappings USING btree (environment_group_id);
DROP INDEX IF EXISTS idx_ura_uniq;
CREATE UNIQUE INDEX idx_ura_uniq
    ON user_role_assignments USING btree (user_id, role_id, COALESCE(environment_id, ''::text), COALESCE(environment_group_id, ''::text), source);
`
	// Group grants are deleted before the column goes; dropping it alone would turn them into global grants.
	v119SQLiteDown = `
DELETE FROM user_role_assignments WHERE environment_group_id IS NOT NULL;
DELETE FROM oidc_role_mappings WHERE environment_group_id IS NOT NULL;
DROP INDEX IF EXISTS idx_ura_uniq;
DROP INDEX IF EXISTS idx_ura_env_group;
DROP INDEX IF EXISTS idx_orm_env_group;
ALTER TABLE user_role_assignments DROP COLUMN environment_group_id;
ALTER TABLE oidc_role_mappings DROP COLUMN environment_group_id;
CREATE UNIQUE INDEX idx_ura_uniq
    ON user_role_assignments(user_id, role_id, COALESCE(environment_id, ''), source);
`
	v119PostgresDown = `
DELETE FROM user_role_assignments WHERE environment_group_id IS NOT NULL;
DELETE FROM oidc_role_mappings WHERE environment_group_id IS NOT NULL;
DROP INDEX IF EXISTS idx_ura_uniq;
DROP INDEX IF EXISTS idx_ura_env_group;
DROP INDEX IF EXISTS idx_orm_env_group;
ALTER TABLE user_role_assignments DROP CONSTRAINT IF EXISTS user_role_assignments_single_scope;
ALTER TABLE oidc_role_mappings DROP CONSTRAINT IF EXISTS oidc_role_mappings_single_scope;
ALTER TABLE user_role_assignments DROP COLUMN IF EXISTS environment_group_id;
ALTER TABLE oidc_role_mappings DROP COLUMN IF EXISTS environment_group_id;
CREATE UNIQUE INDEX idx_ura_uniq
    ON user_role_assignments USING btree (user_id, role_id, COALESCE(environment_id, ''::text), source);
`
)

func init() {
	goose.AddMigrationContext(run(v119SQLiteUp, v119PostgresUp), run(v119SQLiteDown, v119PostgresDown))
}
