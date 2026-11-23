package migrations

import (
	"github.com/pressly/goose/v3"
)

const (
	v121SQLiteUp = `
UPDATE roles
SET permissions = (
    SELECT json_group_array(value) FROM json_each(roles.permissions)
    WHERE value NOT IN (
        'image-updates:list', 'image-updates:read', 'image-updates:check', 'image-updates:apply',
        'system:upgrade', 'containers:autoupdate', 'podman:autoupdate', 'jobs:restart'
    )
)
WHERE EXISTS (
    SELECT 1 FROM json_each(roles.permissions)
    WHERE value IN (
        'image-updates:list', 'image-updates:read', 'image-updates:check', 'image-updates:apply',
        'system:upgrade', 'containers:autoupdate', 'podman:autoupdate', 'jobs:restart'
    )
);
DELETE FROM api_key_permissions WHERE permission IN (
    'image-updates:list', 'image-updates:read', 'image-updates:check', 'image-updates:apply',
    'system:upgrade', 'containers:autoupdate', 'podman:autoupdate', 'jobs:restart'
);
`
	v121PostgresUp = `
UPDATE roles
SET permissions = permissions - ARRAY[
    'image-updates:list', 'image-updates:read', 'image-updates:check', 'image-updates:apply',
    'system:upgrade', 'containers:autoupdate', 'podman:autoupdate', 'jobs:restart'
]::text[];
DELETE FROM api_key_permissions WHERE permission IN (
    'image-updates:list', 'image-updates:read', 'image-updates:check', 'image-updates:apply',
    'system:upgrade', 'containers:autoupdate', 'podman:autoupdate', 'jobs:restart'
);
`
)

// Removing permissions for retired features is an irreversible data cleanup.
func init() {
	goose.AddMigrationContext(run(v121SQLiteUp, v121PostgresUp), nil)
}
