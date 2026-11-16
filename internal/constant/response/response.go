package response

import (
	"net/http"

	"codebase/models"
)

// Error codes are the PRD §8 / §12.4 machine-readable codes; the frontend switches on them.
const (
	VALIDATION_ERROR      = "VALIDATION_ERROR"
	INVALID_FILE_FORMAT   = "INVALID_FILE_FORMAT"
	INVALID_AGE_RANGE     = "INVALID_AGE_RANGE"
	INVALID_PRICE         = "INVALID_PRICE"
	UNAUTHORIZED          = "UNAUTHORIZED"
	TOKEN_EXPIRED         = "TOKEN_EXPIRED"
	REFRESH_TOKEN_EXPIRED = "REFRESH_TOKEN_EXPIRED"
	INVALID_CREDENTIALS   = "INVALID_CREDENTIALS"
	FORBIDDEN             = "FORBIDDEN"
	BOOK_NOT_FOUND        = "BOOK_NOT_FOUND"
	PLAN_NOT_FOUND        = "PLAN_NOT_FOUND"
	NOT_FOUND             = "NOT_FOUND"
	METHOD_NOT_ALLOWED    = "METHOD_NOT_ALLOWED"
	PAYLOAD_TOO_LARGE     = "PAYLOAD_TOO_LARGE"
	GENERAL_ERROR         = "GENERAL_ERROR"
)

var Responses = map[string]models.ResponsePayload{
	VALIDATION_ERROR:    {Desc: "Request validation failed.", HttpCode: http.StatusBadRequest},
	INVALID_FILE_FORMAT: {Desc: "Cover must be .jpg, .jpeg or .png and the book file must be .epub.", HttpCode: http.StatusBadRequest},
	INVALID_AGE_RANGE:   {Desc: "Age must be a whole number and min must be lower than max.", HttpCode: http.StatusBadRequest},
	INVALID_PRICE:       {Desc: "Price must be a whole number above 0.", HttpCode: http.StatusBadRequest},

	UNAUTHORIZED:          {Desc: "Please log in again.", HttpCode: http.StatusUnauthorized},
	TOKEN_EXPIRED:         {Desc: "Access token has expired.", HttpCode: http.StatusUnauthorized},
	REFRESH_TOKEN_EXPIRED: {Desc: "Session has expired. Please log in again.", HttpCode: http.StatusUnauthorized},
	INVALID_CREDENTIALS:   {Desc: "Email or password is incorrect.", HttpCode: http.StatusUnauthorized},

	FORBIDDEN: {Desc: "You do not have access to this resource.", HttpCode: http.StatusForbidden},

	BOOK_NOT_FOUND: {Desc: "Book not found.", HttpCode: http.StatusNotFound},
	PLAN_NOT_FOUND: {Desc: "Subscription plan is not configured.", HttpCode: http.StatusNotFound},
	NOT_FOUND:      {Desc: "Resource not found.", HttpCode: http.StatusNotFound},

	METHOD_NOT_ALLOWED: {Desc: "Method not allowed.", HttpCode: http.StatusMethodNotAllowed},

	PAYLOAD_TOO_LARGE: {Desc: "Uploaded file is too large.", HttpCode: http.StatusRequestEntityTooLarge},

	GENERAL_ERROR: {Desc: "Something went wrong. Please try again.", HttpCode: http.StatusInternalServerError},
}
