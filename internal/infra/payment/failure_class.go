package payment

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	"github.com/stripe/stripe-go/v82"
)

// 渠道失败分类的唯一生产点。适配器在错误点调用 newUpstreamStatusErrorf /
// newTransportErrorf / configFailure；ChannelAdapter 再从错误链提取
// domain.ChannelError 的 Category / RetryAfter。
//
// 词表与 contracts/registries/errors.yaml 的 category 对齐：rate_limited →
// rate_limit、transient → system、auth → auth、permanent/config → request。

func categoryForStatus(status int) domain.ErrorCategory {
	switch {
	case status == http.StatusTooManyRequests:
		return domain.ErrorCategoryRateLimited
	case status == http.StatusUnauthorized, status == http.StatusForbidden:
		return domain.ErrorCategoryAuth
	case status == http.StatusRequestTimeout, status >= 500 && status <= 599:
		return domain.ErrorCategoryTransient
	case status >= 400 && status <= 499:
		return domain.ErrorCategoryPermanent
	default:
		return domain.ErrorCategoryUnknown
	}
}

// codeForStatus 保证管理面使用的错误码与 errors.yaml 完全同词表。
func codeForStatus(status int) apperrors.Code {
	switch status {
	case http.StatusBadRequest:
		return apperrors.CodeInvalidArgument
	case http.StatusUnauthorized:
		return apperrors.CodeUnauthorized
	case http.StatusForbidden:
		return apperrors.CodeForbidden
	case http.StatusNotFound:
		return apperrors.CodeNotFound
	case http.StatusRequestTimeout:
		return apperrors.CodeUnavailable
	case http.StatusConflict:
		return apperrors.CodeConflict
	case http.StatusGone:
		return apperrors.CodeGone
	case http.StatusRequestEntityTooLarge:
		return apperrors.CodeRequestEntityTooLarge
	case http.StatusUnprocessableEntity:
		return apperrors.CodeUnprocessableEntity
	case http.StatusTooManyRequests:
		return apperrors.CodeTooManyRequests
	}
	if status >= 500 && status <= 599 {
		return apperrors.CodeUnavailable
	}
	return apperrors.CodeInternal
}

// newUpstreamStatusErrorf 为上游非 2xx 响应构造平台错误，并写入分类与上游
// Retry-After（若有）。
func newUpstreamStatusErrorf(status int, header http.Header, format string, args ...any) *apperrors.Error {
	appErr := apperrors.WithRetryable(apperrors.Newf(codeForStatus(status), format, args...), categoryForStatus(status).Retryable())
	if seconds, ok := retryAfterSeconds(header); ok {
		appErr = apperrors.WithRetryAfterSeconds(appErr, seconds)
	}
	return appErr
}

// newTransportErrorf 为 client.Do / 读体失败构造平台错误：传输问题默认可重试。
func newTransportErrorf(err error, format string, args ...any) *apperrors.Error {
	appErr := apperrors.Wrapf(apperrors.CodeUnavailable, err, format, args...)
	if isTransportFailure(err) {
		return apperrors.WithRetryable(appErr, true)
	}
	return appErr
}

// configFailure 把配置类拒绝标记为不可重试（重试无法生成缺失的配置值）。
func configFailure(err error) error {
	if err == nil {
		return nil
	}
	var appErr *apperrors.Error
	if errors.As(err, &appErr) {
		return apperrors.WithRetryable(appErr, false)
	}
	return err
}

// retryAfterSeconds 只接受 delta-seconds 形式；HTTP-date 与畸形值一律丢弃。
func retryAfterSeconds(header http.Header) (int, bool) {
	if header == nil {
		return 0, false
	}
	seconds, err := strconv.Atoi(strings.TrimSpace(header.Get("Retry-After")))
	if err != nil || seconds <= 0 {
		return 0, false
	}
	return seconds, true
}

// classifyError 处理未经构造函数的错误链（SDK 错误、网关内部错误）。
func classifyError(err error) domain.ErrorCategory {
	if err == nil {
		return domain.ErrorCategoryUnknown
	}
	var appErr *apperrors.Error
	if errors.As(err, &appErr) {
		if category := categoryForCode(appErr.Code); category != domain.ErrorCategoryUnknown {
			return category
		}
	}
	var stripeErr *stripe.Error
	if errors.As(err, &stripeErr) && stripeErr.HTTPStatusCode > 0 {
		return categoryForStatus(stripeErr.HTTPStatusCode)
	}
	if isTransportFailure(err) {
		return domain.ErrorCategoryTransient
	}
	return domain.ErrorCategoryUnknown
}

func categoryForCode(code apperrors.Code) domain.ErrorCategory {
	switch code {
	case apperrors.CodeTooManyRequests:
		return domain.ErrorCategoryRateLimited
	case apperrors.CodeUnavailable:
		return domain.ErrorCategoryTransient
	case apperrors.CodeUnauthorized, apperrors.CodeForbidden:
		return domain.ErrorCategoryAuth
	case apperrors.CodeNotFound:
		return domain.ErrorCategoryChannelOrderNotFound
	case apperrors.CodeInvalidArgument, apperrors.CodeUnprocessableEntity, apperrors.CodeConflict,
		apperrors.CodeAlreadyExists, apperrors.CodeGone, apperrors.CodeRequestEntityTooLarge,
		apperrors.CodeUnimplemented:
		return domain.ErrorCategoryPermanent
	default:
		return domain.ErrorCategoryUnknown
	}
}

// isTransportFailure 识别超时与连接级失败；调用方取消（context.Canceled）不可重试。
func isTransportFailure(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	var timeout interface{ Timeout() bool }
	if errors.As(err, &timeout) && timeout.Timeout() {
		return true
	}
	var opErr *net.OpError
	return errors.As(err, &opErr)
}

// newChannelError 构造 domain 错误，并保留原始错误链（ChannelError.Unwrap）。
func newChannelError(code string, err error) *domain.ChannelError {
	chErr := &domain.ChannelError{Code: code, Message: err.Error(), Category: classifyError(err), Cause: err}
	if seconds, ok := apperrors.RetryAfterSeconds(err); ok {
		after := time.Duration(seconds) * time.Second
		chErr.RetryAfter = &after
	}
	return chErr
}
