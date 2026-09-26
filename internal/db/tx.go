package db

import (
	"context"
	"database/sql"
	"fmt"
)

// InTx keeps multi-query domain operations atomic without exposing the generated DB handle.
func (q *Queries) InTx(ctx context.Context, fn func(*Queries) error) error {
	db, ok := q.db.(*sql.DB)
	if !ok {
		return fmt.Errorf("database transaction requires *sql.DB")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(q.WithTx(tx)); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}
