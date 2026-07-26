package application

import (
	"context"
	"errors"
	"fmt"
)

// ErrorCode 表示在 Go、gRPC 与 MCP 接口之间保持稳定的应用错误代码 / ErrorCode represents an application error code that remains stable across the Go, gRPC, and MCP surfaces
type ErrorCode string

const (
	// CodeInvalidArgument 表示调用参数无效 / CodeInvalidArgument indicates invalid call arguments
	CodeInvalidArgument ErrorCode = "invalid_argument"
	// CodeNotFound 表示请求的资源不存在 / CodeNotFound indicates that the requested resource does not exist
	CodeNotFound ErrorCode = "not_found"
	// CodeUnsupported 表示请求的操作或格式不受支持 / CodeUnsupported indicates that the requested operation or format is unsupported
	CodeUnsupported ErrorCode = "unsupported"
	// CodeResourceExhausted 表示操作超出资源限制 / CodeResourceExhausted indicates that an operation exceeded a resource limit
	CodeResourceExhausted ErrorCode = "resource_exhausted"
	// CodePermissionDenied 表示调用方没有执行操作所需的权限 / CodePermissionDenied indicates that the caller lacks permission to perform the operation
	CodePermissionDenied ErrorCode = "permission_denied"
	// CodeCanceled 表示操作被取消或超过截止时间 / CodeCanceled indicates that an operation was canceled or exceeded its deadline
	CodeCanceled ErrorCode = "canceled"
	// CodeInternal 表示未分类的内部错误 / CodeInternal indicates an unclassified internal error
	CodeInternal ErrorCode = "internal"
)

// OpError 为底层错误附加机器可读代码和操作名称 / OpError attaches a machine-readable code and operation name to an underlying error
type OpError struct {
	// Op 是发生错误的操作名称 / Op is the name of the operation that failed
	Op string
	// Code 是供调用方判断的稳定错误代码 / Code is the stable error code exposed to callers
	Code ErrorCode
	// Err 是原始底层错误 / Err is the original underlying error
	Err error
}

// Error 返回包含操作名称和底层原因的错误文本
// Error returns error text containing the operation name and underlying cause
func (e *OpError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Err == nil {
		if e.Op == "" {
			return "application error"
		}
		return e.Op
	}
	if e.Op == "" {
		return e.Err.Error()
	}
	return fmt.Sprintf("%s: %v", e.Op, e.Err)
}

// Unwrap 返回可供标准错误链检查的底层错误
// Unwrap returns the underlying error for standard error-chain inspection
func (e *OpError) Unwrap() error { return e.Err }

// opError 在错误非空时使用应用错误代码和操作名称包装错误
// opError wraps a non-nil error with an application error code and operation name
func opError(op string, code ErrorCode, err error) error {
	if err == nil {
		return nil
	}
	return &OpError{Op: op, Code: code, Err: err}
}

// CodeOf 返回错误对应的公开应用错误代码
// CodeOf returns the public application error code associated with an error
func CodeOf(err error) ErrorCode {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return CodeCanceled
	}
	var target *OpError
	if errors.As(err, &target) {
		return target.Code
	}
	return CodeInternal
}
