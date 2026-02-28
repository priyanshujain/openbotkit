package store

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
)

// DB is a thin wrapper around *sql.DB that tracks the driver dialect.
type DB struct {
	*sql.DB
	driver string
}

// Config holds database connection configuration.
type Config struct {
	Driver string // "sqlite" or "postgres"
	DSN    string // data source name / file path
}

// Open opens a database connection based on the config.
func Open(cfg Config) (*DB, error) {
	switch cfg.Driver {
	case "sqlite":
		return openSQLite(cfg.DSN)
	case "postgres":
		return openPostgres(cfg.DSN)
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", cfg.Driver)
	}
}

// SQLiteConfig returns a Config for a SQLite database at the given path.
func SQLiteConfig(path string) Config {
	return Config{Driver: "sqlite", DSN: path}
}

// PostgresConfig returns a Config for a Postgres database with the given DSN.
func PostgresConfig(dsn string) Config {
	return Config{Driver: "postgres", DSN: dsn}
}

// IsSQLite returns true if the underlying database is SQLite.
func (db *DB) IsSQLite() bool {
	return db.driver == "sqlite"
}

// IsPostgres returns true if the underlying database is Postgres.
func (db *DB) IsPostgres() bool {
	return db.driver == "postgres"
}

// Rebind rewrites ? placeholders to $1, $2, ... for Postgres.
// For SQLite, the query is returned unchanged.
func (db *DB) Rebind(query string) string {
	if db.IsSQLite() {
		return query
	}
	// Replace ? with $1, $2, etc. for Postgres.
	var buf strings.Builder
	n := 1
	for i := 0; i < len(query); i++ {
		if query[i] == '?' {
			buf.WriteString("$")
			buf.WriteString(strconv.Itoa(n))
			n++
		} else {
			buf.WriteByte(query[i])
		}
	}
	return buf.String()
}
