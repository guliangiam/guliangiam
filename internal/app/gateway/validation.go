package gateway

import (
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ─── Design note ───────────────────────────────────────────────────────────
// Protocol / Status / SslMode 字段使用纯 TEXT 类型（无 DB CHECK 约束，
// 前端 Zod Schema 也未声明 enum 闭集），且前端 UI 选项值与数据库默认值
// 存在历史差异（前端选 sse/stdio/streamable-http，DB 默认 https）。
// 因此这里只做 必填字段 + TrimSpace 校验，枚举值域由上游（前端/领域层）
// 按各自语义自行约束——避免后端凭猜测强行闭集阻断合法请求。

// ─── Server ────────────────────────────────────────────────────────────────

func validateCreateServerCommand(command CreateServerCommand) error {
	if strings.TrimSpace(command.OrganizationID) == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if strings.TrimSpace(command.Name) == "" {
		return apperrors.NewInvalidArgument("name is required")
	}
	if strings.TrimSpace(command.Endpoint) == "" {
		return apperrors.NewInvalidArgument("endpoint is required")
	}
	return nil
}

func validateUpdateServerCommand(command UpdateServerCommand) error {
	if strings.TrimSpace(command.ID) == "" {
		return apperrors.NewInvalidArgument("id is required")
	}
	if command.Name != "" && strings.TrimSpace(command.Name) == "" {
		return apperrors.NewInvalidArgument("name must not be blank")
	}
	if command.Endpoint != "" && strings.TrimSpace(command.Endpoint) == "" {
		return apperrors.NewInvalidArgument("endpoint must not be blank")
	}
	return nil
}

// ─── Site ──────────────────────────────────────────────────────────────────

func validateCreateSiteCommand(command CreateSiteCommand) error {
	if strings.TrimSpace(command.OrganizationID) == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if strings.TrimSpace(command.Name) == "" {
		return apperrors.NewInvalidArgument("name is required")
	}
	if strings.TrimSpace(command.Host) == "" {
		return apperrors.NewInvalidArgument("host is required")
	}
	return nil
}

func validateUpdateSiteCommand(command UpdateSiteCommand) error {
	if strings.TrimSpace(command.ID) == "" {
		return apperrors.NewInvalidArgument("id is required")
	}
	if command.Name != "" && strings.TrimSpace(command.Name) == "" {
		return apperrors.NewInvalidArgument("name must not be blank")
	}
	if command.Host != "" && strings.TrimSpace(command.Host) == "" {
		return apperrors.NewInvalidArgument("host must not be blank")
	}
	return nil
}
