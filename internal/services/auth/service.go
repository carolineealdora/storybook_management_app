package auth

// SECURITY: manual review required — login, token issuance and revocation.

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	respConst "codebase/internal/constant/response"
	helpers "codebase/internal/helper"
	"codebase/internal/repositories/account"
	"codebase/models"
	"codebase/models/dto"
	"codebase/pkg/shared"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type (
	Service interface {
		Login(email, password, accountType string) (*dto.TokenResponse, error)
		Refresh(refreshToken string) (*dto.TokenResponse, error)
		Logout(credentialID, refreshToken string) error
	}

	ServiceImpl struct {
		repo       account.Repository
		errors     helpers.Error
		now        shared.Now
		secret     []byte
		accessTTL  time.Duration
		refreshTTL time.Duration
	}
)

func NewService(holder *models.SharedHolder) Service {
	return &ServiceImpl{
		repo:       account.NewRepository(holder),
		errors:     helpers.NewError(holder),
		now:        time.Now,
		secret:     []byte(holder.Config.JWTSecret),
		accessTTL:  holder.Config.AccessTokenTTL,
		refreshTTL: holder.Config.RefreshTokenTTL,
	}
}

// dummyHash keeps login timing the same whether or not the email exists.
var dummyHash = func() []byte {
	h, err := bcrypt.GenerateFromPassword([]byte("timing-equalizer"), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	return h
}()

func passwordMatches(hash []byte, password string) bool {
	return bcrypt.CompareHashAndPassword(hash, []byte(password)) == nil
}

func (s *ServiceImpl) Login(email, password, accountType string) (*dto.TokenResponse, error) {
	cred, err := s.repo.GetCredentialByEmail(strings.ToLower(strings.TrimSpace(email)))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		passwordMatches(dummyHash, password) // timing only
		return nil, s.errors.Wrap(respConst.INVALID_CREDENTIALS, "unknown email")
	}
	if err != nil {
		return nil, s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}
	if !passwordMatches([]byte(cred.PasswordHash), password) {
		return nil, s.errors.Wrap(respConst.INVALID_CREDENTIALS, "wrong password for credential "+cred.ID)
	}
	if cred.AccountType != accountType {
		return nil, s.errors.Wrap(respConst.FORBIDDEN, cred.AccountType+" account used on "+accountType+" app")
	}

	now := s.now()
	access, err := s.accessToken(cred, now)
	if err != nil {
		return nil, s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}
	refresh, err := randomToken()
	if err != nil {
		return nil, s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}
	if err := s.repo.CreateRefreshToken(&models.RefreshToken{
		CredentialID: cred.ID, TokenHash: hashToken(refresh), ExpiresAt: now.Add(s.refreshTTL), CreatedAt: now,
	}); err != nil {
		return nil, s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}
	if err := s.repo.TouchLastLogin(cred.ID, now); err != nil {
		return nil, s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}

	resp := s.tokenResponse(access, cred.AccountType)
	resp.RefreshToken = refresh
	return resp, nil
}

func (s *ServiceImpl) Refresh(refreshToken string) (*dto.TokenResponse, error) {
	rt, err := s.repo.GetRefreshToken(hashToken(refreshToken))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, s.errors.Wrap(respConst.UNAUTHORIZED, "unknown refresh token")
	}
	if err != nil {
		return nil, s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}
	if rt.RevokedAt != nil {
		return nil, s.errors.Wrap(respConst.UNAUTHORIZED, "refresh token revoked")
	}
	now := s.now()
	if !now.Before(rt.ExpiresAt) {
		return nil, s.errors.Wrap(respConst.REFRESH_TOKEN_EXPIRED, "refresh token expired")
	}
	cred, err := s.repo.GetCredentialByID(rt.CredentialID)
	if err != nil {
		return nil, s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}
	// SECURITY: manual review required
	// refresh_tokens is shared with storybook_library_app (ADR-010). Its parent tokens must not
	// mint access tokens here; answer as for an unknown token (ADR-011).
	if cred.AccountType != models.AccountAdmin {
		return nil, s.errors.Wrap(respConst.UNAUTHORIZED, "refresh: "+cred.AccountType+" token on the back-office API")
	}
	access, err := s.accessToken(cred, now)
	if err != nil {
		return nil, s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}
	return s.tokenResponse(access, cred.AccountType), nil
}

// Logout revokes the caller's refresh token. Unknown or already revoked tokens are a no-op.
func (s *ServiceImpl) Logout(credentialID, refreshToken string) error {
	if err := s.repo.RevokeRefreshToken(hashToken(refreshToken), credentialID, s.now()); err != nil {
		return s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}
	return nil
}

func (s *ServiceImpl) accessToken(cred *models.Credential, now time.Time) (string, error) {
	claims := helpers.Claims{
		AccountType: cred.AccountType,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   cred.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.accessTTL)),
		},
	}
	if cred.UserID != nil {
		claims.UserID = *cred.UserID
	}
	if cred.AdminID != nil {
		claims.AdminID = *cred.AdminID
	}
	return helpers.SignAccessToken(s.secret, claims)
}

func (s *ServiceImpl) tokenResponse(access, accountType string) *dto.TokenResponse {
	return &dto.TokenResponse{AccessToken: access, TokenType: "Bearer", ExpiresIn: int(s.accessTTL.Seconds()), AccountType: accountType}
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
