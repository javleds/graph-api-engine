package migrations

import "embed"

// Files contains immutable, checksummed migrations for both databases.
//
//go:embed engine/*.sql cache/*.sql
var Files embed.FS
