package errors

import (
	"errors"
	"fmt"
	"net/http"
)

// Sentinel domain errors.
var (
	ErrNotFound       = errors.New("resource not found")
	ErrAlreadyExists  = errors.New("resource already exists")
	ErrUnauthorized   = errors.New("unauthorized")
	ErrForbidden      = errors.New("forbidden")
	ErrValidation     = errors.New("validation error")
	ErrInternal       = errors.New("internal error")
	ErrBadRequest     = errors.New("bad request")
	ErrConflict       = errors.New("conflict")
	ErrRateLimited    = errors.New("rate limited")
	ErrGone           = errors.New("gone")
	ErrNotImplemented = errors.New("not implemented")
	ErrEventPast      = errors.New("event is past")
	ErrPaidEvent      = errors.New("event is paid")
	ErrSoldOut        = errors.New("event is sold out")
)

// DomainError is a structured error with an underlying cause and a message.
type DomainError struct {
	Kind    error  // One of the sentinel errors above.
	Code    string // Stable machine-readable error code.
	Message string // Human-readable message.
	Err     error  // Optional wrapped error for debugging.
}

// Error implements the error interface.
func (e *DomainError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Kind, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Kind, e.Message)
}

// Unwrap returns the underlying sentinel error for errors.Is.
func (e *DomainError) Unwrap() error {
	return e.Kind
}

// New creates a new DomainError.
func New(kind error, message string, err error) *DomainError {
	return &DomainError{
		Kind:    kind,
		Message: message,
		Err:     err,
	}
}

// NewWithCode creates a domain error with a stable client-facing code.
func NewWithCode(kind error, code, message string, err error) *DomainError {
	return &DomainError{
		Kind:    kind,
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// ErrorCode returns a stable client-facing code when one is present.
func ErrorCode(err error) string {
	var domainError *DomainError
	if errors.As(err, &domainError) {
		return domainError.Code
	}
	return ""
}

// HTTPStatusCode maps a domain error to an HTTP status code.
func HTTPStatusCode(err error) int {
	switch {
	case errors.Is(err, ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrAlreadyExists):
		return http.StatusConflict
	case errors.Is(err, ErrUnauthorized):
		return http.StatusUnauthorized
	case errors.Is(err, ErrForbidden):
		return http.StatusForbidden
	case errors.Is(err, ErrValidation):
		return http.StatusUnprocessableEntity
	case errors.Is(err, ErrBadRequest):
		return http.StatusBadRequest
	case errors.Is(err, ErrConflict):
		return http.StatusConflict
	case errors.Is(err, ErrRateLimited):
		return http.StatusTooManyRequests
	case errors.Is(err, ErrGone):
		return http.StatusGone
	case errors.Is(err, ErrNotImplemented):
		return http.StatusNotImplemented
	case errors.Is(err, ErrPaidEvent):
		return http.StatusUnprocessableEntity
	case errors.Is(err, ErrEventPast), errors.Is(err, ErrSoldOut):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

// ErrorResponse is the JSON structure returned to API clients.
type ErrorResponse struct {
	Error     string `json:"error"`
	ErrorCode string `json:"error_code,omitempty"`
	Message   string `json:"message"`
	Code      int    `json:"code"`
}
