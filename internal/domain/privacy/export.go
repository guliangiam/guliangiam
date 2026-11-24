package privacy

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ExportRecord 是导出证据包中的一条数据记录：类别（如 user/profile/consent）、
// 主体与可序列化载荷；载荷内容不入审计，只记录导出清单与校验和。
type ExportRecord struct {
	Category  string         `json:"category"`
	SubjectID string         `json:"subjectId"`
	Payload   map[string]any `json:"payload"`
}

// EvidencePackage 是 DSAR 导出结果：JSON Lines 记录 + 清单 + 校验和。
type EvidencePackage struct {
	GeneratedAt time.Time
	Region      string
	Records     []ExportRecord
	Checksum    string
	LineCount   int
}

// BuildEvidencePackage 归一化记录顺序并生成确定性 JSON Lines 与 SHA-256 校验和：
// 相同输入（与同 region）产生相同 checksum，便于接收方独立核验。
func BuildEvidencePackage(records []ExportRecord, region string, now time.Time) (*EvidencePackage, error) {
	if region == "" {
		return nil, apperrors.NewInvalidArgument("region is required")
	}
	normalized := make([]ExportRecord, len(records))
	copy(normalized, records)
	sort.SliceStable(normalized, func(i, j int) bool {
		if normalized[i].Category != normalized[j].Category {
			return normalized[i].Category < normalized[j].Category
		}
		if normalized[i].SubjectID != normalized[j].SubjectID {
			return normalized[i].SubjectID < normalized[j].SubjectID
		}
		left, _ := json.Marshal(normalized[i].Payload)
		right, _ := json.Marshal(normalized[j].Payload)
		return bytes.Compare(left, right) < 0
	})
	var buf bytes.Buffer
	for _, record := range normalized {
		if record.Category == "" {
			return nil, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "export record category is required",
				map[string]any{"field": "category"})
		}
		if record.SubjectID == "" {
			return nil, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "export record subjectId is required",
				map[string]any{"field": "subjectId"})
		}
		line, err := json.Marshal(record)
		if err != nil {
			return nil, apperrors.WrapInternalf(err, "marshal export record")
		}
		buf.Write(line)
		buf.WriteByte('\n')
	}
	sum := sha256.Sum256(buf.Bytes())
	return &EvidencePackage{
		GeneratedAt: now,
		Region:      region,
		Records:     normalized,
		Checksum:    hex.EncodeToString(sum[:]),
		LineCount:   len(normalized),
	}, nil
}

// JSONLines 返回可下载的 JSON Lines 内容（与 Checksum 对应）。
func (p EvidencePackage) JSONLines() ([]byte, error) {
	var buf bytes.Buffer
	for _, record := range p.Records {
		line, err := json.Marshal(record)
		if err != nil {
			return nil, apperrors.WrapInternalf(err, "marshal export record")
		}
		buf.Write(line)
		buf.WriteByte('\n')
	}
	return buf.Bytes(), nil
}

// EvidenceManifest 是随包清单：记录数、region、生成时间与校验和。
type EvidenceManifest struct {
	Region      string    `json:"region"`
	GeneratedAt time.Time `json:"generatedAt"`
	LineCount   int       `json:"lineCount"`
	Checksum    string    `json:"checksum"`
}

// Manifest 返回导出清单。
func (p EvidencePackage) Manifest() EvidenceManifest {
	return EvidenceManifest{Region: p.Region, GeneratedAt: p.GeneratedAt, LineCount: p.LineCount, Checksum: p.Checksum}
}
