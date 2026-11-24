// Copyright (c) 2026 Guliang Tech. All rights reserved.
package webhooks

import "encoding/json"

// buildWebhookHeaders 构造 outbox.Headers JSON — DeliveryOutboxHandler 从中
// 读取 event_id（重放路径：outbox 新行指回原 webhook_events 台账行；扇出路径
// 两表同主键，eventID 传空省略）。url/secret/method 仅作冗余快照，投递时以
// 系统作用域事务内重新加载的 webhook 配置为准。secret 为空时不做 HMAC 签名。
func buildWebhookHeaders(url, secret, method string, extra map[string]string, eventID string) []byte {
	hdrs := map[string]interface{}{
		"url":    url,
		"secret": secret,
		"method": method,
	}
	if len(extra) > 0 {
		hdrs["headers"] = extra
	}
	if eventID != "" {
		hdrs["event_id"] = eventID
	}
	b, _ := json.Marshal(hdrs)
	return b
}
