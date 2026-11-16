package dto

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email,max=255"`
	Password string `json:"password" validate:"required,max=72"`
}

func (r LoginRequest) Validate() interface{} { return r }

// RefreshRequest is the body of /auth/refresh and both logout endpoints.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required,max=200"`
}

func (r RefreshRequest) Validate() interface{} { return r }

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"` // seconds
	AccountType  string `json:"account_type"`
}
