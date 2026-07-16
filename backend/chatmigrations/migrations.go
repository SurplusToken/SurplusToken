// Package chatmigrations embeds migrations for the independent chat database.
package chatmigrations

import "embed"

// FS contains all chat database migrations.
//
//go:embed *.sql
var FS embed.FS
