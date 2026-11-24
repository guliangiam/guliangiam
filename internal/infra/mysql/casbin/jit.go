package casbin

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type jitMetadata struct {
	Source      string `json:"source"`
	LifecycleID string `json:"lifecycleId"`
	StartsAt    string `json:"startsAt"`
}

// jitTimeLayouts 覆盖 IGA 写入（RFC3339Nano）与 MySQL DATETIME 文本两种形态。
var jitTimeLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02 15:04:05.999999999",
	"2006-01-02 15:04:05",
}

func parseJITTime(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	for _, layout := range jitTimeLayouts {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed.UTC(), true
		}
	}
	return time.Time{}, false
}

// EnforceJIT 在请求期评估带时间窗的 JIT 策略。
//
// MySQL 的 v4（失效时间）/ v5（JSON 元数据）都是 VARCHAR，因此不复刻 PG 的
// timestamptz/jsonb 运算，改为取候选行后在 Go 内求值：lifecycle 必须匹配、
// startsAt <= now < expiresAt。
func (s Store) EnforceJIT(ctx context.Context, enforcerID, lifecycleID string, rvals ...interface{}) (bool, error) {
	if strings.TrimSpace(lifecycleID) == "" || len(rvals) < 4 {
		return false, nil
	}
	v0, ok0 := rvals[0].(string)
	v1, ok1 := rvals[1].(string)
	v2, ok2 := rvals[2].(string)
	v3, ok3 := rvals[3].(string)
	if !ok0 || !ok1 || !ok2 || !ok3 {
		return false, nil
	}
	vals := [4]string{v0, v1, v2, v3}

	rows, err := s.db.QueryContext(ctx, qSelectJITCandidates,
		enforcerID, vals[0], vals[1], vals[2], vals[3])
	if err != nil {
		return false, apperrors.WrapInternalf(err, "query JIT policy candidates")
	}
	defer rows.Close()

	now := time.Now().UTC()
	for rows.Next() {
		var v0, v1, v2, v3, v4, v5 string
		if err := rows.Scan(&v0, &v1, &v2, &v3, &v4, &v5); err != nil {
			return false, apperrors.WrapInternalf(err, "scan JIT policy candidate")
		}
		var meta jitMetadata
		if err := json.Unmarshal([]byte(v5), &meta); err != nil {
			// 元数据不是合法 JSON 的 JIT 行不可用于授权，跳过而不是放行。
			continue
		}
		if meta.LifecycleID != lifecycleID {
			continue
		}
		startsAt, ok := parseJITTime(meta.StartsAt)
		if !ok || now.Before(startsAt) {
			continue
		}
		expiresAt, ok := parseJITTime(v4)
		if !ok || !now.Before(expiresAt) {
			continue
		}
		return true, nil
	}
	if err := rows.Err(); err != nil {
		return false, apperrors.WrapInternalf(err, "iterate JIT policy candidates")
	}
	return false, nil
}
