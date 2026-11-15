package sql

import "errors"

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
