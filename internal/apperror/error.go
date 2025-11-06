package apperror

import (
	"errors"
	"fmt"
)

type ErrorCode string

const (
	CodeBadRequest         ErrorCode = "BAD_REQUEST"
	CodeUnauthorized       ErrorCode = "UNAUTHORIZED"
	CodeForbidden          ErrorCode = "FORBIDDEN"
	CodeNotFound           ErrorCode = "NOT_FOUND"
	CodeConflict           ErrorCode = "CONFLICT"
	CodeTooManyRequests    ErrorCode = "TOO_MANY_REQUESTS"
	CodeInternalError      ErrorCode = "INTERNAL_ERROR"
	CodeDatabaseError      ErrorCode = "DATABASE_ERROR"
	CodeValidationError    ErrorCode = "VALIDATION_ERROR"
	CodeTokenError         ErrorCode = "TOKEN_ERROR"
	CodeInvalidCredentials ErrorCode = "INVALID_CREDENTIALS"
	CodeAccountInactive    ErrorCode = "ACCOUNT_INACTIVE"
	CodeEmailExists        ErrorCode = "EMAIL_ALREADY_EXISTS"
)

type AppError struct {
	Code       ErrorCode
	Message    string
	HTTPStatus int
	Err        error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s (%v)", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error {
	return e.Err
}

func New(code ErrorCode, message string, httpStatus int) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		HTTPStatus: httpStatus,
	}
}

func Wrap(code ErrorCode, message string, httpStatus int, err error) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		HTTPStatus: httpStatus,
		Err:        err,
	}
}

func BadRequest(message string) *AppError {
	return New(CodeBadRequest, message, 400)
}

func Unauthorized(message string) *AppError {
	return New(CodeUnauthorized, message, 401)
}

func Forbidden(message string) *AppError {
	return New(CodeForbidden, message, 403)
}

func NotFound(message string) *AppError {
	return New(CodeNotFound, message, 404)
}

func Conflict(message string) *AppError {
	return New(CodeConflict, message, 409)
}

func TooManyRequests(message string) *AppError {
	return New(CodeTooManyRequests, message, 429)
}

func InternalError(message string, err error) *AppError {
	return Wrap(CodeInternalError, message, 500, err)
}

func InvalidCredentials() *AppError {
	return New(CodeInvalidCredentials, "Invalid email or password", 401)
}

func AccountInactive() *AppError {
	return New(CodeAccountInactive, "Account is not active", 401)
}

func EmailAlreadyExists() *AppError {
	return New(CodeEmailExists, "Email address already registered", 409)
}

func AsAppError(err error) *AppError {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr
	}
	return InternalError("unexpected error", err)
}
