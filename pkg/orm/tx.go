package sql

import (
	"errors"

	"github.com/jackc/pgconn"
)

// WithTx runs fn inside a transaction: commit on nil, rollback on error.
func WithTx(db ORM, fn func(tx ORM) error) error {
	tx := db.Begin()
	if err := tx.Error(); err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return errors.Join(err, rbErr)
		}
		return err
	}
	return tx.Commit()
}

var (
	// ErrDuplicate: a unique constraint rejected the write (idempotency key, purchase, active subscription).
	ErrDuplicate = errors.New("duplicate row")
	// ErrStateChanged: a conditional update matched no row because another request changed the state first.
	ErrStateChanged = errors.New("row state changed concurrently")
)

// IsUniqueViolation reports a Postgres unique_violation (SQLSTATE 23505).
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
