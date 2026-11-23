// Package migrations embeds the PostgreSQL schema migrations.
package migrations

import "embed"

// Files contains the Goose migration scripts.
//
//go:embed *.sql
var Files embed.FS
