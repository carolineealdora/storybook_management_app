package response

import (
	"net/http"

	"codebase/models"
)

// Error codes are the PRD §8 / §12.4 machine-readable codes; the frontend switches on them.
const (
	VALIDATION_ERROR        = "VALIDATION_ERROR"
	INVALID_FILE_FORMAT     = "INVALID_FILE_FORMAT"
	INVALID_AGE_RANGE       = "INVALID_AGE_RANGE"
	UNAUTHORIZED            = "UNAUTHORIZED"
	TOKEN_EXPIRED           = "TOKEN_EXPIRED"
	REFRESH_TOKEN_EXPIRED   = "REFRESH_TOKEN_EXPIRED"
	INVALID_CREDENTIALS     = "INVALID_CREDENTIALS"
	INVALID_PIN             = "INVALID_PIN"
	INVALID_PASSWORD        = "INVALID_PASSWORD"
	PAYMENT_FAILED          = "PAYMENT_FAILED"
	FORBIDDEN               = "FORBIDDEN"
	BOOK_LOCKED             = "BOOK_LOCKED"
	BOOK_NOT_FOUND          = "BOOK_NOT_FOUND"
	ORDER_NOT_FOUND         = "ORDER_NOT_FOUND"
	PLAN_NOT_FOUND          = "PLAN_NOT_FOUND"
	NOT_FOUND               = "NOT_FOUND"
	METHOD_NOT_ALLOWED      = "METHOD_NOT_ALLOWED"
	BOOK_ALREADY_OWNED      = "BOOK_ALREADY_OWNED"
	SUBSCRIPTION_ACTIVE     = "SUBSCRIPTION_ACTIVE"
	BOOK_UNAVAILABLE        = "BOOK_UNAVAILABLE"
	DUPLICATE_REQUEST       = "DUPLICATE_REQUEST"
	BOOK_NOT_PURCHASABLE    = "BOOK_NOT_PURCHASABLE"
	PIN_NOT_SET             = "PIN_NOT_SET"
	SUBSCRIPTION_NOT_ACTIVE = "SUBSCRIPTION_NOT_ACTIVE"
	PAYLOAD_TOO_LARGE       = "PAYLOAD_TOO_LARGE"
	TOO_MANY_ATTEMPTS       = "TOO_MANY_ATTEMPTS"
	GENERAL_ERROR           = "GENERAL_ERROR"
)

var Responses = map[string]models.ResponsePayload{
	VALIDATION_ERROR:    {Desc: "Request validation failed.", HttpCode: http.StatusBadRequest},
	INVALID_FILE_FORMAT: {Desc: "Cover must be .jpg, .jpeg or .png and the book file must be .epub.", HttpCode: http.StatusBadRequest},
	INVALID_AGE_RANGE:   {Desc: "Age must be a whole number and min must be lower than max.", HttpCode: http.StatusBadRequest},

	UNAUTHORIZED:          {Desc: "Please log in again.", HttpCode: http.StatusUnauthorized},
	TOKEN_EXPIRED:         {Desc: "Access token has expired.", HttpCode: http.StatusUnauthorized},
	REFRESH_TOKEN_EXPIRED: {Desc: "Session has expired. Please log in again.", HttpCode: http.StatusUnauthorized},
	INVALID_CREDENTIALS:   {Desc: "Email or password is incorrect.", HttpCode: http.StatusUnauthorized},
	INVALID_PIN:           {Desc: "PIN is incorrect.", HttpCode: http.StatusUnauthorized},
	INVALID_PASSWORD:      {Desc: "Password is incorrect.", HttpCode: http.StatusUnauthorized},

	PAYMENT_FAILED: {Desc: "Payment failed. Please try again.", HttpCode: http.StatusPaymentRequired},

	FORBIDDEN:   {Desc: "You do not have access to this resource.", HttpCode: http.StatusForbidden},
	BOOK_LOCKED: {Desc: "Subscribe or purchase this book to read it.", HttpCode: http.StatusForbidden},

	BOOK_NOT_FOUND:  {Desc: "Book not found.", HttpCode: http.StatusNotFound},
	ORDER_NOT_FOUND: {Desc: "Order not found.", HttpCode: http.StatusNotFound},
	PLAN_NOT_FOUND:  {Desc: "Subscription plan is not configured.", HttpCode: http.StatusNotFound},
	NOT_FOUND:       {Desc: "Resource not found.", HttpCode: http.StatusNotFound},

	METHOD_NOT_ALLOWED: {Desc: "Method not allowed.", HttpCode: http.StatusMethodNotAllowed},

	BOOK_ALREADY_OWNED:      {Desc: "You already own this book.", HttpCode: http.StatusConflict},
	SUBSCRIPTION_ACTIVE:     {Desc: "You already have an active subscription.", HttpCode: http.StatusConflict},
	BOOK_UNAVAILABLE:        {Desc: "This book is no longer available.", HttpCode: http.StatusConflict},
	DUPLICATE_REQUEST:       {Desc: "This request was already processed.", HttpCode: http.StatusConflict},
	BOOK_NOT_PURCHASABLE:    {Desc: "This book cannot be purchased.", HttpCode: http.StatusConflict},
	PIN_NOT_SET:             {Desc: "Please set a PIN in Settings first.", HttpCode: http.StatusConflict},
	SUBSCRIPTION_NOT_ACTIVE: {Desc: "There is no active subscription.", HttpCode: http.StatusConflict},

	PAYLOAD_TOO_LARGE: {Desc: "Uploaded file is too large.", HttpCode: http.StatusRequestEntityTooLarge},

	TOO_MANY_ATTEMPTS: {Desc: "Too many wrong attempts. Please try again in 1 minute.", HttpCode: http.StatusTooManyRequests},

	GENERAL_ERROR: {Desc: "Something went wrong. Please try again.", HttpCode: http.StatusInternalServerError},
}
