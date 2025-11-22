package sqlite

import (
	_ "embed"
)

// Встроенные SQL-миграции клиента (SQLite).
//
//go:embed migrations/001_init.sql
var initDDL string

func initialDDL() string { return initDDL }
