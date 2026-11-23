package migrations

import (
	"github.com/pressly/goose/v3"
)

const (
	v118SQLiteUp = `
CREATE TABLE IF NOT EXISTS environment_groups (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_environment_groups_name ON environment_groups(lower(name));
ALTER TABLE environments ADD COLUMN group_id TEXT REFERENCES environment_groups(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_environments_group_id ON environments(group_id);
`
	v118PostgresUp = `
CREATE TABLE IF NOT EXISTS environment_groups (
    id text PRIMARY KEY,
    name text NOT NULL,
    description text DEFAULT '' NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_environment_groups_name ON environment_groups(lower(name));
ALTER TABLE environments ADD COLUMN IF NOT EXISTS group_id text REFERENCES environment_groups(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_environments_group_id ON environments(group_id);
`
	v118Down = `
DROP INDEX IF EXISTS idx_environments_group_id;
ALTER TABLE environments DROP COLUMN group_id;
DROP TABLE IF EXISTS environment_groups;
`
)

func init() {
	goose.AddMigrationContext(run(v118SQLiteUp, v118PostgresUp), run(v118Down, v118Down))
}
