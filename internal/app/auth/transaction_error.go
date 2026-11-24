package auth

import (
	"fmt"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// wrapTransactionError 保留领域错误码（not_found / unauthorized / conflict 等）
// 以便 transport 层继续按原语义映射 HTTP 状态；只有事务层（begin/set_config/
// commit）的裸基础设施错误才补充上下文，便于日志定位。
func wrapTransactionError(err error, message string) error {
	if err == nil {
		return nil
	}
	if e := apperrors.FromError(err); e.Code != apperrors.CodeInternal {
		return err
	}
	return fmt.Errorf("%s: %w", message, err)
}
