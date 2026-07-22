package application

import (
	"context"
	"errors"
	"fmt"
)

// ErrorCode is stable across the Go, gRPC, and MCP surfaces.
type ErrorCode string

const (
	CodeInvalidArgument   ErrorCode = "invalid_argument"
	CodeNotFound          ErrorCode = "not_found"
	CodeUnsupported       ErrorCode = "unsupported"
	CodeResourceExhausted ErrorCode = "resource_exhausted"
	CodePermissionDenied  ErrorCode = "permission_denied"
	CodeCanceled          ErrorCode = "canceled"
	CodeInternal          ErrorCode = "internal"
)

// OpError adds a machine-readable code and operation to an underlying error.
type OpError struct {
	Op   string
	Code ErrorCode
	Err  error
}

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

func (e *OpError) Unwrap() error { return e.Err }

func opError(op string, code ErrorCode, err error) error {
	if err == nil {
		return nil
	}
	return &OpError{Op: op, Code: code, Err: err}
}

// CodeOf returns the public error code for err.
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
