package errors

import (
	stderrors "errors"
	"fmt"
	"net/http"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Kind identifies the stable class of an application error.
type Kind string

const (
	KindInvalidArgument Kind = "INVALID_ARGUMENT"
	KindUnauthorized    Kind = "UNAUTHORIZED"
	KindNotFound        Kind = "NOT_FOUND"
	KindConflict        Kind = "CONFLICT"
	KindInternal        Kind = "INTERNAL"
)

// AppError is the cross-layer business error used by HTTP and gRPC adapters.
type AppError struct {
	Kind    Kind   `json:"kind"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Cause   error  `json:"-"`
}

func (e *AppError) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// New creates an application error without a wrapped cause.
func New(kind Kind, code string, message string) *AppError {
	return &AppError{Kind: kind, Code: code, Message: message}
}

// Wrap creates an application error while preserving the lower-level cause.
func Wrap(kind Kind, code string, message string, cause error) *AppError {
	return &AppError{Kind: kind, Code: code, Message: message, Cause: cause}
}

// InvalidArgument reports invalid client input.
func InvalidArgument(code string, message string) *AppError {
	return New(KindInvalidArgument, code, message)
}

// Unauthorized reports missing or invalid authentication.
func Unauthorized(code string, message string) *AppError {
	return New(KindUnauthorized, code, message)
}

// NotFound reports a missing resource.
func NotFound(code string, message string) *AppError {
	return New(KindNotFound, code, message)
}

// Conflict reports a state conflict such as duplicate unique data.
func Conflict(code string, message string) *AppError {
	return New(KindConflict, code, message)
}

// Internal reports an unexpected server-side failure.
func Internal(code string, message string) *AppError {
	return New(KindInternal, code, message)
}

// As extracts an AppError from an error chain.
func As(err error) (*AppError, bool) {
	var appErr *AppError
	if stderrors.As(err, &appErr) {
		return appErr, true
	}
	return nil, false
}

// HTTPStatus maps an application error to an HTTP status code.
func HTTPStatus(err error) int {
	appErr, ok := As(err)
	if !ok {
		return http.StatusInternalServerError
	}
	switch appErr.Kind {
	case KindInvalidArgument:
		return http.StatusBadRequest
	case KindUnauthorized:
		return http.StatusUnauthorized
	case KindNotFound:
		return http.StatusNotFound
	case KindConflict:
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

// GRPCCode maps an application error to a gRPC status code.
func GRPCCode(err error) codes.Code {
	appErr, ok := As(err)
	if !ok {
		return codes.Internal
	}
	switch appErr.Kind {
	case KindInvalidArgument:
		return codes.InvalidArgument
	case KindUnauthorized:
		return codes.Unauthenticated
	case KindNotFound:
		return codes.NotFound
	case KindConflict:
		return codes.AlreadyExists
	default:
		return codes.Internal
	}
}

// ToGRPC converts an application error to a gRPC status error.
func ToGRPC(err error) error {
	if err == nil {
		return nil
	}
	appErr, ok := As(err)
	if !ok {
		appErr = Internal("internal_error", "internal error")
	}
	return status.Error(GRPCCode(appErr), appErr.Code+"|"+appErr.Message)
}

// FromGRPC converts a gRPC status error back to an application error.
func FromGRPC(err error) *AppError {
	if err == nil {
		return nil
	}
	st, ok := status.FromError(err)
	if !ok {
		return Internal("internal_error", "internal error")
	}

	code, message := splitGRPCMessage(st.Message())
	return &AppError{
		Kind:    kindFromGRPCCode(st.Code()),
		Code:    code,
		Message: message,
	}
}

func splitGRPCMessage(message string) (string, string) {
	parts := strings.SplitN(message, "|", 2)
	if len(parts) != 2 {
		return "internal_error", message
	}
	return parts[0], parts[1]
}

func kindFromGRPCCode(code codes.Code) Kind {
	switch code {
	case codes.InvalidArgument:
		return KindInvalidArgument
	case codes.Unauthenticated:
		return KindUnauthorized
	case codes.NotFound:
		return KindNotFound
	case codes.AlreadyExists:
		return KindConflict
	default:
		return KindInternal
	}
}
