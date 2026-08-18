package gofast

import (
	"encoding/json"
	"log"
	"net/http"
)

// ErrorCode identifies the category of an error returned by
// GoFast, used both in JSON responses and for programmatic
// handling by clients.
type ErrorCode string

const (
	ErrCodeValidation       ErrorCode = "VALIDATION_ERROR"
	ErrCodeBadRequest       ErrorCode = "BAD_REQUEST"
	ErrCodeInternal         ErrorCode = "INTERNAL_ERROR"
	ErrCodeNotFound         ErrorCode = "NOT_FOUND"
	ErrCodeConflict         ErrorCode = "CONFLICT"
	ErrCodeForbidden        ErrorCode = "FORBIDDEN"
	ErrCodeUnsupportedMedia ErrorCode = "UNSUPPORTED_MEDIA_TYPE"
)

// ErrorBody is the JSON shape of every error response GoFast
// returns.
type ErrorBody struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
	Details any       `json:"details,omitempty"`
}

type errorResponse struct {
	Error ErrorBody `json:"error"`
}

// BusinessError marks an error as safe to expose to API clients.
// Errors that do not implement BusinessError are treated as
// internal: logged server-side and reported to the client with a
// generic message, never with their original details.
type BusinessError interface {
	error
	Code() ErrorCode
	Status() int
}

type businessError struct {
	code    ErrorCode
	status  int
	message string
}

func (e *businessError) Error() string {
	return e.message
}

func (e *businessError) Code() ErrorCode {
	return e.code
}

func (e *businessError) Status() int {
	return e.status
}

// NewBusinessError creates an error that is safe to expose to API
// clients as-is, with the given error code, HTTP status, and
// message.
func NewBusinessError(code ErrorCode, status int, message string) BusinessError {
	return &businessError{code: code, status: status, message: message}
}

func writeError(w http.ResponseWriter, status int, code ErrorCode, message string, details any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(errorResponse{
		Error: ErrorBody{
			Code:    code,
			Message: message,
			Details: details,
		},
	}); err != nil {
		log.Printf("[gofast] failed to encode error response: %v", err)
	}
}

func handleFuncError(w http.ResponseWriter, err error) {
	if be, ok := err.(BusinessError); ok {
		writeError(w, be.Status(), be.Code(), be.Error(), nil)
		return
	}

	log.Printf("[gofast] internal error: %v", err)
	writeError(w, http.StatusInternalServerError, ErrCodeInternal, "an unexpected error occurred", nil)
}
