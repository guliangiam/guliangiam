// Retry witness carried on platform errors.
//
// 生产者（infra 渠道适配器）与消费者（transport route.WriteError）唯一的合法
// 交汇点是 platform/errors；否则 transport 必须 import infra（违反分层）。
package errors

import "errors"

// Error.Meta 键：生产者与写端只允许使用这两个常量，避免键名漂移。
const (
	MetaRetryable         = "retryable"
	MetaRetryAfterSeconds = "retryAfterSeconds"
)

// WithRetryable records whether the failure may be retried;其他 Meta 键保留。
func WithRetryable(err *Error, retryable bool) *Error {
	if err == nil {
		return nil
	}
	meta(err)[MetaRetryable] = retryable
	return err
}

// WithRetryAfterSeconds records the upstream Retry-After verbatim (整秒)。
// 调用方只传已解析成功的值；解析失败由生产者在源头丢弃，写端绝不补默认值。
func WithRetryAfterSeconds(err *Error, seconds int) *Error {
	if err == nil || seconds <= 0 {
		return err
	}
	meta(err)[MetaRetryAfterSeconds] = seconds
	return err
}

// IsRetryable reports the explicit marker. ok=false 表示生产者未分类，
// 调用方不得据此猜测。
func IsRetryable(err error) (retryable bool, ok bool) {
	value, exists := metaOf(err)[MetaRetryable]
	if !exists {
		return false, false
	}
	retryable, ok = value.(bool)
	return retryable, ok
}

// RetryAfterSeconds returns the recorded upstream delay; ok=false 表示上游没有
// 给出可解析的值，写端应省略响应头。
func RetryAfterSeconds(err error) (seconds int, ok bool) {
	seconds, ok = metaOf(err)[MetaRetryAfterSeconds].(int)
	return seconds, ok && seconds > 0
}

func meta(err *Error) map[string]any {
	if err.Meta == nil {
		err.Meta = map[string]any{}
	}
	return err.Meta
}

func metaOf(err error) map[string]any {
	var appErr *Error
	if !errors.As(err, &appErr) || appErr.Meta == nil {
		return nil
	}
	return appErr.Meta
}
