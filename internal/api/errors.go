package api

import (
	"fmt"
	"net/http"
)

// StatusError is an error with an HTTP status code
type StatusError struct {
	StatusCode   int    `json:"-"`
	ErrorMessage string `json:"error"`
}

func (e *StatusError) Error() string { return e.ErrorMessage }

// ErrBadRequest creates a 400 Bad Request error
func ErrBadRequest(msg string) *StatusError {
	return &StatusError{
		StatusCode:   http.StatusBadRequest,
		ErrorMessage: msg,
	}
}

// ErrNotFound creates a 404 Not Found error
func ErrNotFound(msg string) *StatusError {
	return &StatusError{
		StatusCode:   http.StatusNotFound,
		ErrorMessage: msg,
	}
}

// ErrInternalServer creates a 500 Internal Server Error
func ErrInternalServer(msg string) *StatusError {
	return &StatusError{
		StatusCode:   http.StatusInternalServerError,
		ErrorMessage: msg,
	}
}

// ErrBadGateway creates a 502 Bad Gateway error
func ErrBadGateway(msg string) *StatusError {
	return &StatusError{
		StatusCode:   http.StatusBadGateway,
		ErrorMessage: msg,
	}
}

// WrapError wraps an existing error into a StatusError
func WrapError(err error, code int, msg string) *StatusError {
	fullMsg := msg
	if err != nil {
		fullMsg = fmt.Sprintf("%s: %v", msg, err)
	}
	return &StatusError{
		StatusCode:   code,
		ErrorMessage: fullMsg,
	}
}
