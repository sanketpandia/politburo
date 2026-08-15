package apikeys

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Repository reads API key rows from the shared api_keys table.
type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Active reports whether the key exists and is enabled.
// found is false when no row matches the key id.
func (r *Repository) Active(ctx context.Context, key string) (active bool, found bool, err error) {
	err = r.db.QueryRowContext(ctx, `SELECT status FROM api_keys WHERE id = $1`, key).Scan(&active)
	if errors.Is(err, sql.ErrNoRows) {
		return false, false, nil
	}
	if err != nil {
		return false, false, fmt.Errorf("lookup api key: %w", err)
	}
	return active, true, nil
}
