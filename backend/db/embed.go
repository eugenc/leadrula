// Package db embeds the SQL migration files so they ship inside the binary.
package db

import "embed"

//go:embed migrations/*.sql
var Migrations embed.FS

// Dir is the path within the embedded FS that holds migration files.
const Dir = "migrations"
