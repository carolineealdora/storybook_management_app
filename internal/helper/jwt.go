package helpers

// SECURITY: manual review required — access token signing and verification.

import (
	"errors"
	"strings"

	respConst "codebase/internal/constant/response"
	"codebase/models"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
)

var (
	ErrAccessTokenExpired = errors.New("access token expired")
	ErrInvalidAccessToken = errors.New("invalid access token")
)

const claimsKey = "auth.claims"

// Claims of the access token. Subject is the credential id.
type Claims struct {
	AccountType string `json:"account_type"`
	UserID      string `json:"user_id,omitempty"`
	AdminID     string `json:"admin_id,omitempty"`
	jwt.RegisteredClaims
}

func SignAccessToken(secret []byte, claims Claims) (string, error) {
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
}

func ParseAccessToken(secret []byte, token string) (*Claims, error) {
	var c Claims
	_, err := jwt.ParseWithClaims(token, &c, func(*jwt.Token) (any, error) { return secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithExpirationRequired())
	switch {
	case err == nil:
		return &c, nil
	case errors.Is(err, jwt.ErrTokenExpired):
		return nil, ErrAccessTokenExpired
	default:
		return nil, ErrInvalidAccessToken
	}
}

// RequireAccount rejects requests without a valid access token of one of the given account types.
func RequireAccount(holder *models.SharedHolder, accountTypes ...string) echo.MiddlewareFunc {
	response, errs := NewResponse(holder), NewError(holder)
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			deny := func(code, info string) error {
				return response.CommonResponse(models.ResponseArgs{Ctx: c, ErrorData: errs.Wrap(code, info)})
			}
			token, ok := strings.CutPrefix(c.Request().Header.Get(echo.HeaderAuthorization), "Bearer ")
			if !ok || token == "" {
				return deny(respConst.UNAUTHORIZED, "missing bearer token")
			}
			claims, err := ParseAccessToken([]byte(holder.Config.JWTSecret), token)
			if errors.Is(err, ErrAccessTokenExpired) {
				return deny(respConst.TOKEN_EXPIRED, err.Error())
			}
			if err != nil {
				return deny(respConst.UNAUTHORIZED, err.Error())
			}
			if !SlicesContains(accountTypes, claims.AccountType) {
				return deny(respConst.FORBIDDEN, "account type "+claims.AccountType+" not allowed")
			}
			SetClaims(c, *claims)
			return next(c)
		}
	}
}

func SetClaims(c echo.Context, claims Claims) { c.Set(claimsKey, claims) }

// ClaimsFrom returns the claims set by RequireAccount (zero value if the route is unprotected).
func ClaimsFrom(c echo.Context) Claims {
	claims, _ := c.Get(claimsKey).(Claims)
	return claims
}
