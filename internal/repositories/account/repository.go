package account

import (
	"time"

	"codebase/models"
	sql "codebase/pkg/orm"

	"github.com/google/uuid"
)

type (
	Repository interface {
		GetCredentialByEmail(email string) (*models.Credential, error)
		GetCredentialByID(id string) (*models.Credential, error)
		TouchLastLogin(credentialID string, at time.Time) error
		CreateRefreshToken(t *models.RefreshToken) error
		GetRefreshToken(tokenHash string) (*models.RefreshToken, error)
		RevokeRefreshToken(tokenHash, credentialID string, at time.Time) error
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
