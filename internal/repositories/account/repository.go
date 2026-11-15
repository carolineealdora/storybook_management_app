package account

import (
	"time"

	"codebase/models"
	sql "codebase/pkg/orm"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type (
	Repository interface {
		GetCredentialByEmail(email string) (*models.Credential, error)
		GetCredentialByID(id string) (*models.Credential, error)
		TouchLastLogin(credentialID string, at time.Time) error
		CreateRefreshToken(t *models.RefreshToken) error
		GetRefreshToken(tokenHash string) (*models.RefreshToken, error)
		RevokeRefreshToken(tokenHash, credentialID string, at time.Time) error

		GetUser(id string) (*models.User, error)
		GetCredentialByUserID(userID string) (*models.Credential, error)
		UpdateUserName(id, name string, at time.Time) error
		SetPin(userID, pinHash string, at time.Time) error
		IncrementPinFailures(userID string) (int, error)
		LockPin(userID string, until time.Time) error
		ResetPinFailures(userID string) error
	}

	Repo struct {
		db sql.ORM
	}
)

func NewRepository(holder *models.SharedHolder) Repository {
	return &Repo{db: holder.Sql}
}

func (r *Repo) GetCredentialByEmail(email string) (*models.Credential, error) {
	var c models.Credential
	if err := r.db.Where("email = ?", email).First(&c); err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *Repo) GetCredentialByID(id string) (*models.Credential, error) {
	var c models.Credential
	if err := r.db.Where("id = ?", id).First(&c); err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *Repo) TouchLastLogin(credentialID string, at time.Time) error {
	return r.db.Exec(`UPDATE credentials SET last_login_at = ? WHERE id = ?`, at, credentialID)
}

func (r *Repo) CreateRefreshToken(t *models.RefreshToken) error {
	if t.ID == "" {
		t.ID = uuid.NewString()
	}
	return r.db.Create(t)
}

func (r *Repo) GetRefreshToken(tokenHash string) (*models.RefreshToken, error) {
	var t models.RefreshToken
	if err := r.db.Where("token_hash = ?", tokenHash).First(&t); err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *Repo) RevokeRefreshToken(tokenHash, credentialID string, at time.Time) error {
	return r.db.Exec(`UPDATE refresh_tokens SET revoked_at = ? WHERE token_hash = ? AND credential_id = ? AND revoked_at IS NULL`,
		at, tokenHash, credentialID)
}

func (r *Repo) GetUser(id string) (*models.User, error) {
	var u models.User
	if err := r.db.Where("id = ?", id).First(&u); err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *Repo) GetCredentialByUserID(userID string) (*models.Credential, error) {
	var c models.Credential
	if err := r.db.Where("user_id = ?", userID).First(&c); err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *Repo) UpdateUserName(id, name string, at time.Time) error {
	return r.db.Exec(`UPDATE users SET name = ?, updated_at = ? WHERE id = ?`, name, at, id)
}

// SetPin stores a new PIN hash and clears any failed attempts or lock.
func (r *Repo) SetPin(userID, pinHash string, at time.Time) error {
	return r.db.Exec(`UPDATE users SET pin_hash = ?, pin_failed_attempts = 0, pin_locked_until = NULL, updated_at = ? WHERE id = ?`,
		pinHash, at, userID)
}

// IncrementPinFailures bumps the counter atomically (safe under concurrent wrong attempts) and returns the new value.
func (r *Repo) IncrementPinFailures(userID string) (int, error) {
	var counts []int
	if err := r.db.RawSql(`UPDATE users SET pin_failed_attempts = pin_failed_attempts + 1 WHERE id = ? RETURNING pin_failed_attempts`,
		&counts, userID); err != nil {
		return 0, err
	}
	if len(counts) == 0 {
		return 0, gorm.ErrRecordNotFound
	}
	return counts[0], nil
}

func (r *Repo) LockPin(userID string, until time.Time) error {
	return r.db.Exec(`UPDATE users SET pin_failed_attempts = 0, pin_locked_until = ? WHERE id = ?`, until, userID)
}

func (r *Repo) ResetPinFailures(userID string) error {
	return r.db.Exec(`UPDATE users SET pin_failed_attempts = 0, pin_locked_until = NULL WHERE id = ?`, userID)
}
