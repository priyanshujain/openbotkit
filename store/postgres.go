package store

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

func openPostgres(dsn string) (*DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return &DB{DB: db, driver: "postgres"}, nil
}
