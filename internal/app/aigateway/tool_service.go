// tool_service.go — AI Gateway tool_use runtime (设计文档 §7).
//
// 职责：
//  1. 扫描上游 CanonicalResponse.Content 中的 ContentBlockToolUse 块
//  2. 从原始请求的 CanonicalRequest.Tools[] 中查找对应的 MCPBinding
//  3. 通过 ToolExecutor 端口执行每个工具调用
//  4. 将结果转换为 ContentBlockToolResult 块
//
// Clean Architecture：本文件 **只依赖 Domain 层的 ToolExecutor Port**。
// 具体实现（mcpservers.Service 等）在 bootstrap 层注入。
package aigateway

import (
	"context"
	"errors"
	"log/slog"

	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/redact"
)

// ExecuteTools 扫描 content 中的 tool_use 块，通过 exec 执行后
// 返回替换为 tool_result 块的新 Content 切片。
//
// orgID / providerID 用于 Prometheus 指标标签（RecordToolExecution）。
// 传空字符串时指标不会 panic（promauto 允许任意 label values），但建议
// 调用方始终传入真实值以便聚合分析。
//
// bindings 是 CanonicalRequest.Tools[] 的子集——用于从工具声明里取出
// 运行时 MCPBinding（.MCPTarget），让 ToolExecutor 知道调到哪个 MCP server。
// 找不到 binding 的工具调用会跳过执行（content 原样保留该 tool_use 块）。
//
// 规则：
//   - exec == nil → 直接返回原 content（Fail-Open）
//   - 找不到 MCPBinding 的工具 → 原样保留 tool_use 块
//   - 单个工具执行失败 → 对应 tool_result 块里带 error 文本，不阻断其他
//   - 原始 content 不会被修改（调用方可安全重用）
func ExecuteTools(
	ctx context.Context,
	exec domain.ToolExecutor,
	content []domain.CanonicalContentBlock,
	bindings []domain.CanonicalTool,
	orgID, providerID string,
) []domain.CanonicalContentBlock {
	if exec == nil || len(content) == 0 {
		return content
	}

	out := make([]domain.CanonicalContentBlock, 0, len(content))
	for _, block := range content {
		if block.Type != domain.ContentBlockToolUse {
			out = append(out, block)
			continue
		}

		// 从请求 Tools[] 里查找对应 MCPBinding
		binding := findMCPBinding(bindings, block.ToolName)
		if binding == nil {
			// 无运行时绑定 — 原样保留 tool_use 块让调用方手动执行
			slog.DebugContext(ctx, "aigateway tool: no MCP binding for tool_use, leaving to caller",
				"tool", block.ToolName)
			out = append(out, block)
			continue
		}

		result, err := exec.ExecuteTool(ctx, binding, block.ToolName, block.ToolInput)
		if err != nil {
			// The tool_result block is relayed back to the model: never echo the
			// upstream error text. Typed errors (MCP failures) already carry a
			// fixed message; anything else is sanitized and bounded here.
			detail := toolErrorText(err)
			slog.WarnContext(ctx, "aigateway tool execution failed",
				"tool", block.ToolName, "tool_call_id", block.ToolCallID,
				"server", binding.ServerName, "err", detail)
			RecordToolExecution(orgID, providerID, block.ToolName, "failure")
			out = append(out, domain.CanonicalContentBlock{
				Type:       domain.ContentBlockToolResult,
				ToolCallID: block.ToolCallID,
				ToolName:   block.ToolName,
				ToolResult: map[string]any{"error": detail},
			})
			continue
		}

		RecordToolExecution(orgID, providerID, block.ToolName, "success")
		out = append(out, domain.CanonicalContentBlock{
			Type:       domain.ContentBlockToolResult,
			ToolCallID: block.ToolCallID,
			ToolName:   block.ToolName,
			ToolResult: result,
		})
	}
	return out
}

// toolErrorText returns the text that may enter a tool_result block: the fixed
// Message of a typed platform error, otherwise a sanitized/bounded rendering of
// the error. The cause chain is not rendered here; it never leaves the process.
func toolErrorText(err error) string {
	if err == nil {
		return ""
	}
	var appErr *apperrors.Error
	if errors.As(err, &appErr) && appErr.Message != "" {
		return redact.SanitizeText(appErr.Message)
	}
	return redact.SanitizeText(err.Error())
}

// HasToolUseBlocks returns true if any content block has Type == ContentBlockToolUse.
func HasToolUseBlocks(content []domain.CanonicalContentBlock) bool {
	for _, b := range content {
		if b.Type == domain.ContentBlockToolUse {
			return true
		}
	}
	return false
}

// findMCPBinding looks up the MCPBinding for a given tool name from the
// request's CanonicalTools declarations. Returns nil if no tool with that
// name has an MCPTarget set.
func findMCPBinding(bindings []domain.CanonicalTool, toolName string) *domain.MCPBinding {
	for _, t := range bindings {
		if t.Name == toolName && t.MCPTarget != nil {
			return t.MCPTarget
		}
	}
	return nil
}
