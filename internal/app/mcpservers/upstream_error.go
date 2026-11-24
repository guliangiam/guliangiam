package mcpservers

import (
	"context"
	"errors"
	"net"
	"net/url"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/redact"
)

// Stable in-band codes carried next to the message so a client can branch
// without parsing text. The platform Code still drives any HTTP mapping.
const (
	ErrorCodeMCPToolFailed          = "mcp_tool_failed"
	ErrorCodeMCPSyncFailed          = "mcp_sync_failed"
	ErrorCodeMCPUpstreamUnavailable = "mcp_upstream_unavailable"
	ErrorCodeMCPUpstreamTimeout     = "mcp_upstream_timeout"
	ErrorCodeMCPServerNotFound      = "mcp_server_not_found"
	ErrorCodeMCPRequestRejected     = "mcp_server_request_rejected"
)

// Fixed messages. Upstream text stays inside failure.wrapped for errors.Is/As.
const (
	messageMCPToolFailed      = "mcp tool execution failed"
	messageMCPSyncFailed      = "mcp tool sync failed"
	messageMCPUnavailable     = "mcp upstream unavailable"
	messageMCPTimeout         = "mcp upstream timeout"
	messageMCPServerNotFound  = "mcp server not found"
	messageMCPRequestRejected = "mcp server request rejected"
)

type mcpOperation int

const (
	mcpOperationTool mcpOperation = iota
	mcpOperationSync
)

type mcpFailure struct {
	Code      apperrors.Code
	ErrorCode string
	Message   string
	// wrapped keeps the original error for errors.Is/As only; never rendered.
	wrapped *apperrors.Error
}

// classifyMCPFailure maps an upstream error to a stable code/message pair.
// Timeout is checked before *url.Error because net/http wraps it in one.
func classifyMCPFailure(operation mcpOperation, err error) mcpFailure {
	code := apperrors.CodeInternal
	errorCode := ErrorCodeMCPToolFailed
	message := messageMCPToolFailed
	if operation == mcpOperationSync {
		errorCode = ErrorCodeMCPSyncFailed
		message = messageMCPSyncFailed
	}

	switch {
	case errors.Is(err, context.DeadlineExceeded):
		code, errorCode, message = apperrors.CodeUnavailable, ErrorCodeMCPUpstreamTimeout, messageMCPTimeout
	default:
		var netErr net.Error
		var urlErr *url.Error
		var appErr *apperrors.Error
		switch {
		case errors.As(err, &netErr) && netErr.Timeout():
			code, errorCode, message = apperrors.CodeUnavailable, ErrorCodeMCPUpstreamTimeout, messageMCPTimeout
		case errors.As(err, &urlErr):
			code, errorCode, message = apperrors.CodeUnavailable, ErrorCodeMCPUpstreamUnavailable, messageMCPUnavailable
		case errors.As(err, &appErr):
			switch appErr.Code {
			case apperrors.CodeNotFound:
				code, errorCode, message = appErr.Code, ErrorCodeMCPServerNotFound, messageMCPServerNotFound
			case apperrors.CodeInvalidArgument:
				code, errorCode, message = appErr.Code, ErrorCodeMCPRequestRejected, messageMCPRequestRejected
			case apperrors.CodeUnavailable:
				code, errorCode, message = appErr.Code, ErrorCodeMCPUpstreamUnavailable, messageMCPUnavailable
			}
		}
	}

	return mcpFailure{
		Code:      code,
		ErrorCode: errorCode,
		Message:   redact.Bound(message),
		wrapped:   apperrors.Wrap(code, redact.WrapError(err), message),
	}
}

// PlatformError exposes the typed error for callers that propagate the failure.
func (f mcpFailure) PlatformError() *apperrors.Error { return f.wrapped }

// SanitizeToolError maps a tool-call failure to a typed, fixed-message error.
// The original error stays reachable through errors.Is/As.
func SanitizeToolError(err error) error {
	if err == nil {
		return nil
	}
	return classifyMCPFailure(mcpOperationTool, err).PlatformError()
}
