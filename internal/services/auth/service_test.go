package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	respConst "codebase/internal/constant/response"
	helpers "codebase/internal/helper"
	"codebase/models"
	repository_mock "codebase/test/mock/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	secret = []byte("test-secret-test-secret-test-secret!!")
	now    = time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)
)

func newSvc(repo *repository_mock.AccountRepository) *ServiceImpl {
	return &ServiceImpl{
		repo: repo, errors: helpers.NewError(nil), now: func() time.Time { return now },
		secret: secret, accessTTL: 30 * time.Minute, refreshTTL: 7 * 24 * time.Hour,
	}
}

func hash(t *testing.T, pw string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.MinCost)
	require.NoError(t, err)
	return string(h)
}

func parentCred(t *testing.T) *models.Credential {
	uid := "user-1"
	return &models.Credential{ID: "cred-1", Email: "parent@example.com", PasswordHash: hash(t, "pw-123456"), AccountType: models.AccountParent, UserID: &uid}
}

func adminCred(t *testing.T) *models.Credential {
	aid := "admin-1"
	return &models.Credential{ID: "cred-2", Email: "admin@example.com", PasswordHash: hash(t, "pw-123456"), AccountType: models.AccountAdmin, AdminID: &aid}
}

func sha(s string) string { sum := sha256.Sum256([]byte(s)); return hex.EncodeToString(sum[:]) }

func TestLogin_ValidParent_ReturnsTokensAndStoresHashedRefresh(t *testing.T) {
	repo := new(repository_mock.AccountRepository)
	repo.On("GetCredentialByEmail", "parent@example.com").Return(parentCred(t), nil)
	var stored *models.RefreshToken
	repo.On("CreateRefreshToken", mock.Anything).Run(func(a mock.Arguments) { stored = a.Get(0).(*models.RefreshToken) }).Return(nil)
	repo.On("TouchLastLogin", "cred-1", now).Return(nil)

	got, err := newSvc(repo).Login("  Parent@Example.com ", "pw-123456", models.AccountParent)
	require.NoError(t, err)

	assert.Equal(t, 1800, got.ExpiresIn, "access token lives 30 minutes")
	assert.Equal(t, "Bearer", got.TokenType)
	assert.Equal(t, models.AccountParent, got.AccountType)
	assert.NotEmpty(t, got.AccessToken)

	require.NotNil(t, stored)
	assert.NotEmpty(t, got.RefreshToken)
	assert.Equal(t, sha(got.RefreshToken), stored.TokenHash, "only the hash is stored")
	assert.Equal(t, "cred-1", stored.CredentialID)
	assert.Equal(t, now.Add(7*24*time.Hour), stored.ExpiresAt, "refresh token lives 7 days")
	repo.AssertExpectations(t)
}

func TestLogin_AccessToken_CarriesAccountClaims(t *testing.T) {
	repo := new(repository_mock.AccountRepository)
	repo.On("GetCredentialByEmail", "parent@example.com").Return(parentCred(t), nil)
	repo.On("CreateRefreshToken", mock.Anything).Return(nil)
	repo.On("TouchLastLogin", mock.Anything, mock.Anything).Return(nil)
	svc := newSvc(repo)
	svc.now = time.Now

	got, err := svc.Login("parent@example.com", "pw-123456", models.AccountParent)
	require.NoError(t, err)
	claims, err := helpers.ParseAccessToken(secret, got.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, "cred-1", claims.Subject)
	assert.Equal(t, "user-1", claims.UserID)
	assert.Equal(t, models.AccountParent, claims.AccountType)
	assert.WithinDuration(t, time.Now().Add(30*time.Minute), claims.ExpiresAt.Time, 5*time.Second)
}

func TestLogin_UnknownEmail_ReturnsInvalidCredentials(t *testing.T) {
	repo := new(repository_mock.AccountRepository)
	repo.On("GetCredentialByEmail", "nobody@example.com").Return(nil, gorm.ErrRecordNotFound)
	_, err := newSvc(repo).Login("nobody@example.com", "pw", models.AccountParent)
	assert.Equal(t, respConst.INVALID_CREDENTIALS, helpers.ErrCode(err))
}

func TestLogin_WrongPassword_ReturnsInvalidCredentials(t *testing.T) {
	repo := new(repository_mock.AccountRepository)
	repo.On("GetCredentialByEmail", "parent@example.com").Return(parentCred(t), nil)
	_, err := newSvc(repo).Login("parent@example.com", "wrong", models.AccountParent)
	assert.Equal(t, respConst.INVALID_CREDENTIALS, helpers.ErrCode(err))
	repo.AssertNotCalled(t, "CreateRefreshToken", mock.Anything)
}

func TestLogin_ParentAccountOnBackOffice_ReturnsForbidden(t *testing.T) {
	repo := new(repository_mock.AccountRepository)
	repo.On("GetCredentialByEmail", "parent@example.com").Return(parentCred(t), nil)
	_, err := newSvc(repo).Login("parent@example.com", "pw-123456", models.AccountAdmin)
	assert.Equal(t, respConst.FORBIDDEN, helpers.ErrCode(err))
	repo.AssertNotCalled(t, "CreateRefreshToken", mock.Anything)
}

func TestLogin_AdminAccountOnLibraryApp_ReturnsForbidden(t *testing.T) {
	repo := new(repository_mock.AccountRepository)
	repo.On("GetCredentialByEmail", "admin@example.com").Return(adminCred(t), nil)
	_, err := newSvc(repo).Login("admin@example.com", "pw-123456", models.AccountParent)
	assert.Equal(t, respConst.FORBIDDEN, helpers.ErrCode(err))
}

func TestLogin_WrongPasswordOnWrongApp_ReturnsInvalidCredentialsNotForbidden(t *testing.T) {
	repo := new(repository_mock.AccountRepository)
	repo.On("GetCredentialByEmail", "parent@example.com").Return(parentCred(t), nil)
	_, err := newSvc(repo).Login("parent@example.com", "wrong", models.AccountAdmin)
	assert.Equal(t, respConst.INVALID_CREDENTIALS, helpers.ErrCode(err), "do not reveal account type before password is verified")
}

func TestLogin_DBDown_ReturnsGeneralError(t *testing.T) {
	repo := new(repository_mock.AccountRepository)
	repo.On("GetCredentialByEmail", "parent@example.com").Return(nil, errors.New("conn refused"))
	_, err := newSvc(repo).Login("parent@example.com", "pw", models.AccountParent)
	assert.Equal(t, respConst.GENERAL_ERROR, helpers.ErrCode(err))
}

func TestLogin_StoreRefreshFails_ReturnsGeneralError(t *testing.T) {
	repo := new(repository_mock.AccountRepository)
	repo.On("GetCredentialByEmail", "parent@example.com").Return(parentCred(t), nil)
	repo.On("CreateRefreshToken", mock.Anything).Return(errors.New("disk full"))
	_, err := newSvc(repo).Login("parent@example.com", "pw-123456", models.AccountParent)
	assert.Equal(t, respConst.GENERAL_ERROR, helpers.ErrCode(err))
}

func TestRefresh_ValidToken_ReturnsNewAccessTokenOnly(t *testing.T) {
	repo := new(repository_mock.AccountRepository)
	repo.On("GetRefreshToken", sha("rt-1")).Return(&models.RefreshToken{CredentialID: "cred-1", ExpiresAt: now.Add(time.Hour)}, nil)
	repo.On("GetCredentialByID", "cred-1").Return(adminCred(t), nil)
	got, err := newSvc(repo).Refresh("rt-1")
	require.NoError(t, err)
	assert.NotEmpty(t, got.AccessToken)
	assert.Empty(t, got.RefreshToken)
	assert.Equal(t, 1800, got.ExpiresIn)
	assert.Equal(t, models.AccountAdmin, got.AccountType)
}

// refresh_tokens is shared with storybook_library_app (ADR-010); its parent tokens must not
// mint access tokens here (STORY-4, F3).
func TestRefresh_ParentRefreshToken_ReturnsUnauthorized(t *testing.T) {
	repo := new(repository_mock.AccountRepository)
	repo.On("GetRefreshToken", sha("rt-1")).Return(&models.RefreshToken{CredentialID: "cred-1", ExpiresAt: now.Add(time.Hour)}, nil)
	repo.On("GetCredentialByID", "cred-1").Return(parentCred(t), nil)
	got, err := newSvc(repo).Refresh("rt-1")
	assert.Nil(t, got)
	assert.Equal(t, respConst.UNAUTHORIZED, helpers.ErrCode(err))
}

func TestRefresh_UnknownToken_ReturnsUnauthorized(t *testing.T) {
	repo := new(repository_mock.AccountRepository)
	repo.On("GetRefreshToken", sha("rt-x")).Return(nil, gorm.ErrRecordNotFound)
	_, err := newSvc(repo).Refresh("rt-x")
	assert.Equal(t, respConst.UNAUTHORIZED, helpers.ErrCode(err))
}

func TestRefresh_RevokedToken_ReturnsUnauthorized(t *testing.T) {
	revoked := now.Add(-time.Minute)
	repo := new(repository_mock.AccountRepository)
	repo.On("GetRefreshToken", sha("rt-1")).Return(&models.RefreshToken{CredentialID: "cred-1", ExpiresAt: now.Add(time.Hour), RevokedAt: &revoked}, nil)
	_, err := newSvc(repo).Refresh("rt-1")
	assert.Equal(t, respConst.UNAUTHORIZED, helpers.ErrCode(err))
}

func TestRefresh_ExpiredAfter7Days_ReturnsRefreshTokenExpired(t *testing.T) {
	repo := new(repository_mock.AccountRepository)
	repo.On("GetRefreshToken", sha("rt-1")).Return(&models.RefreshToken{CredentialID: "cred-1", ExpiresAt: now}, nil)
	_, err := newSvc(repo).Refresh("rt-1")
	assert.Equal(t, respConst.REFRESH_TOKEN_EXPIRED, helpers.ErrCode(err))
}

func TestRefresh_DBDown_ReturnsGeneralError(t *testing.T) {
	repo := new(repository_mock.AccountRepository)
	repo.On("GetRefreshToken", sha("rt-1")).Return(nil, errors.New("conn refused"))
	_, err := newSvc(repo).Refresh("rt-1")
	assert.Equal(t, respConst.GENERAL_ERROR, helpers.ErrCode(err))
}

func TestLogout_RevokesCallersTokenHash(t *testing.T) {
	repo := new(repository_mock.AccountRepository)
	repo.On("RevokeRefreshToken", sha("rt-1"), "cred-1", now).Return(nil)
	require.NoError(t, newSvc(repo).Logout("cred-1", "rt-1"))
	repo.AssertExpectations(t)
}

func TestLogout_DBDown_ReturnsGeneralError(t *testing.T) {
	repo := new(repository_mock.AccountRepository)
	repo.On("RevokeRefreshToken", mock.Anything, mock.Anything, mock.Anything).Return(errors.New("conn refused"))
	assert.Equal(t, respConst.GENERAL_ERROR, helpers.ErrCode(newSvc(repo).Logout("cred-1", "rt-1")))
}
