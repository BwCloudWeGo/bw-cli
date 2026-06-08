package errors

import (
	stderrors "errors"
	"fmt"
	"net/http"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Kind 标识应用错误的稳定类别。
type Kind string

const (
	KindInvalidArgument Kind = "INVALID_ARGUMENT"
	KindUnauthorized    Kind = "UNAUTHORIZED"
	KindNotFound        Kind = "NOT_FOUND"
	KindConflict        Kind = "CONFLICT"
	KindInternal        Kind = "INTERNAL"
)

// AppError 是 HTTP 和 gRPC 适配器共用的跨层业务错误。
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

// New 创建不包装底层原因的应用错误。
func New(kind Kind, code string, message string) *AppError {
	return &AppError{Kind: kind, Code: code, Message: message}
}

// Wrap 创建应用错误并保留底层原因。
func Wrap(kind Kind, code string, message string, cause error) *AppError {
	return &AppError{Kind: kind, Code: code, Message: message, Cause: cause}
}

// InvalidArgument 表示客户端输入无效。
func InvalidArgument(code string, message string) *AppError {
	return New(KindInvalidArgument, code, message)
}

// Unauthorized 表示认证缺失或无效。
func Unauthorized(code string, message string) *AppError {
	return New(KindUnauthorized, code, message)
}

// NotFound 表示资源不存在。
func NotFound(code string, message string) *AppError {
	return New(KindNotFound, code, message)
}

// Conflict 表示状态冲突，例如唯一数据重复。
func Conflict(code string, message string) *AppError {
	return New(KindConflict, code, message)
}

// Internal 表示非预期的服务端失败。
func Internal(code string, message string) *AppError {
	return New(KindInternal, code, message)
}

// As 从错误链中提取 AppError。
func As(err error) (*AppError, bool) {
	var appErr *AppError
	if stderrors.As(err, &appErr) {
		return appErr, true
	}
	return nil, false
}

// HTTPStatus 将应用错误映射为 HTTP 状态码。
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

// GRPCCode 将应用错误映射为 gRPC 状态码。
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

// ToGRPC 将应用错误转换为 gRPC status 错误。
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

// FromGRPC 将 gRPC status 错误还原为应用错误。
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
