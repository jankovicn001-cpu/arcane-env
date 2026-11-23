package migrations

import (
	"github.com/pressly/goose/v3"
)

const (
	v120SQLiteUp = `
CREATE TABLE IF NOT EXISTS user_groups (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_groups_name ON user_groups(lower(name));
CREATE TABLE IF NOT EXISTS user_group_links (
    id            TEXT PRIMARY KEY,
    user_group_id TEXT NOT NULL REFERENCES user_groups(id) ON DELETE CASCADE,
    kind          TEXT NOT NULL CHECK (kind IN ('oidc', 'ldap')),
    value         TEXT NOT NULL,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_group_links_uniq ON user_group_links(user_group_id, kind, lower(value));
CREATE INDEX IF NOT EXISTS idx_user_group_links_kind_value ON user_group_links(kind, value);
CREATE TABLE IF NOT EXISTS user_group_members (
    id            TEXT PRIMARY KEY,
    user_group_id TEXT NOT NULL REFERENCES user_groups(id) ON DELETE CASCADE,
    user_id       TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    source        TEXT NOT NULL DEFAULT 'manual' CHECK (source IN ('manual', 'oidc', 'ldap')),
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_group_members_uniq ON user_group_members(user_group_id, user_id, source);
CREATE INDEX IF NOT EXISTS idx_user_group_members_user ON user_group_members(user_id);
CREATE TABLE IF NOT EXISTS user_group_role_assignments (
    id                   TEXT PRIMARY KEY,
    user_group_id        TEXT NOT NULL REFERENCES user_groups(id) ON DELETE CASCADE,
    role_id              TEXT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    environment_id       TEXT REFERENCES environments(id) ON DELETE CASCADE,
    environment_group_id TEXT REFERENCES environment_groups(id) ON DELETE CASCADE,
    created_at           DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at           DATETIME,
    CHECK (environment_id IS NULL OR environment_group_id IS NULL)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_ugra_uniq
    ON user_group_role_assignments(user_group_id, role_id, COALESCE(environment_id, ''), COALESCE(environment_group_id, ''));
CREATE INDEX IF NOT EXISTS idx_ugra_role ON user_group_role_assignments(role_id);
CREATE INDEX IF NOT EXISTS idx_ugra_env ON user_group_role_assignments(environment_id);
CREATE INDEX IF NOT EXISTS idx_ugra_env_group ON user_group_role_assignments(environment_group_id);
`
	v120PostgresUp = `
CREATE TABLE IF NOT EXISTS user_groups (
    id text PRIMARY KEY,
    name text NOT NULL,
    description text DEFAULT '' NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_groups_name ON user_groups(lower(name));
CREATE TABLE IF NOT EXISTS user_group_links (
    id text PRIMARY KEY,
    user_group_id text NOT NULL REFERENCES user_groups(id) ON DELETE CASCADE,
    kind text NOT NULL CHECK (kind IN ('oidc', 'ldap')),
    value text NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_group_links_uniq ON user_group_links USING btree (user_group_id, kind, lower(value));
CREATE INDEX IF NOT EXISTS idx_user_group_links_kind_value ON user_group_links USING btree (kind, value);
CREATE TABLE IF NOT EXISTS user_group_members (
    id text PRIMARY KEY,
    user_group_id text NOT NULL REFERENCES user_groups(id) ON DELETE CASCADE,
    user_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    source text DEFAULT 'manual' NOT NULL CHECK (source IN ('manual', 'oidc', 'ldap')),
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_group_members_uniq ON user_group_members USING btree (user_group_id, user_id, source);
CREATE INDEX IF NOT EXISTS idx_user_group_members_user ON user_group_members USING btree (user_id);
CREATE TABLE IF NOT EXISTS user_group_role_assignments (
    id text PRIMARY KEY,
    user_group_id text NOT NULL REFERENCES user_groups(id) ON DELETE CASCADE,
    role_id text NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    environment_id text REFERENCES environments(id) ON DELETE CASCADE,
    environment_group_id text REFERENCES environment_groups(id) ON DELETE CASCADE,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone,
    CONSTRAINT user_group_role_assignments_single_scope CHECK (environment_id IS NULL OR environment_group_id IS NULL)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_ugra_uniq
    ON user_group_role_assignments USING btree (user_group_id, role_id, COALESCE(environment_id, ''::text), COALESCE(environment_group_id, ''::text));
CREATE INDEX IF NOT EXISTS idx_ugra_role ON user_group_role_assignments USING btree (role_id);
CREATE INDEX IF NOT EXISTS idx_ugra_env ON user_group_role_assignments USING btree (environment_id);
CREATE INDEX IF NOT EXISTS idx_ugra_env_group ON user_group_role_assignments USING btree (environment_group_id);
`
	// Dropping the tables drops every group grant with them, so nothing widens to a user's direct access.
	v120Down = `
DROP TABLE IF EXISTS user_group_role_assignments;
DROP TABLE IF EXISTS user_group_members;
DROP TABLE IF EXISTS user_group_links;
DROP TABLE IF EXISTS user_groups;
`
)

func init() {
	goose.AddMigrationContext(run(v120SQLiteUp, v120PostgresUp), run(v120Down, v120Down))
}
