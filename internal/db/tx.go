package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// InTx keeps multi-query domain operations atomic without exposing the generated DB handle.
func (q *Queries) InTx(ctx context.Context, fn func(*Queries) error) (txErr error) {
	db, ok := q.db.(*sql.DB)
	if !ok {
		return fmt.Errorf("database transaction requires *sql.DB")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			if txErr != nil {
				txErr = errors.Join(txErr, err)
			} else {
				txErr = err
			}
		}
	}()
	if err := fn(q.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit()
}
