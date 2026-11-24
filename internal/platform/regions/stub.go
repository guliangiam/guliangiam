// Package regions 提供 ADR-031 Decision 1 + Decision 3 的平台层支撑。
//
// StubRegionResolver: Decision 1 — RegionResolver 的内存实现（跨 region 检查用）
// StubRegionKeyResolver: Decision 3 — RegionKeyResolver 的内存实现（KEK 路由用）
package regions

import (
	"context"
	"fmt"
	"sync"
	"time"

	domaincrypto "github.com/guliangiam/guliangiam/internal/domain/crypto"
	domainregions "github.com/guliangiam/guliangiam/internal/domain/regions"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ─══════════════════════════════════════════════════
//  StubRegionResolver  (Decision 1 — RegionResolver)
// ─══════════════════════════════════════════════════

// StubRegionResolver 是 RegionResolver 的内存实现。
type StubRegionResolver struct {
	mu   sync.Mutex
	data map[string]string // orgID → region_code
}

func NewStubRegionResolver() *StubRegionResolver {
	return &StubRegionResolver{data: make(map[string]string)}
}

func NewStubRegionResolverFromMap(m map[string]string) *StubRegionResolver {
	cp := make(map[string]string, len(m))
	for k, v := range m {
		cp[k] = v
	}
	return &StubRegionResolver{data: cp}
}

func (s *StubRegionResolver) Bind(orgID, regionCode string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[orgID] = regionCode
}

func (s *StubRegionResolver) Unbind(orgID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, orgID)
}

func (s *StubRegionResolver) Resolve(ctx context.Context, orgID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	code, ok := s.data[orgID]
	if !ok {
		return "", apperrors.NewNotFoundf("region binding for org %s", orgID)
	}
	return code, nil
}

func (s *StubRegionResolver) ResolveBatch(ctx context.Context, orgIDs []string) (map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make(map[string]string, len(orgIDs))
	for _, id := range orgIDs {
		if code, ok := s.data[id]; ok {
			result[id] = code
		}
	}
	return result, nil
}

var _ domainregions.RegionResolver = (*StubRegionResolver)(nil)

// ─══════════════════════════════════════════════════
//  StubRegionKeyResolver  (Decision 3 — RegionKeyResolver)
// ─══════════════════════════════════════════════════

// StubRegionKeyResolver 是 RegionKeyResolver 的内存实现。
// 不做真正的 DB JOIN — 直接构造 KEK map（测试注入用）。
type StubRegionKeyResolver struct {
	mu sync.Mutex
	// orgID → purpose → version → EncryptionKey
	data map[string]map[string]map[int]*domaincrypto.EncryptionKey
	// orgID → purpose → current_version
	current map[string]map[string]int
}

func NewStubRegionKeyResolver() *StubRegionKeyResolver {
	return &StubRegionKeyResolver{
		data:    make(map[string]map[string]map[int]*domaincrypto.EncryptionKey),
		current: make(map[string]map[string]int),
	}
}

// AddKEK 给 (orgID, purpose) 注入一个 KEK 版本。
// isCurrent=true 设为当前版本。
func (s *StubRegionKeyResolver) AddKEK(orgID, purpose string, key *domaincrypto.EncryptionKey, isCurrent bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.data[orgID]; !ok {
		s.data[orgID] = make(map[string]map[int]*domaincrypto.EncryptionKey)
		s.current[orgID] = make(map[string]int)
	}
	if _, ok := s.data[orgID][purpose]; !ok {
		s.data[orgID][purpose] = make(map[int]*domaincrypto.EncryptionKey)
	}
	s.data[orgID][purpose][key.KeyVersion] = key
	if isCurrent {
		s.current[orgID][purpose] = key.KeyVersion
	}
}

// RevokeKEK 把一个 KEK 版本标记为 revoked（测试吊销场景）。
func (s *StubRegionKeyResolver) RevokeKEK(orgID, purpose string, keyVersion int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if keys, ok := s.data[orgID][purpose]; ok {
		if k, ok := keys[keyVersion]; ok {
			k.Status = "revoked"
			now := time.Now()
			k.RevokedAt = &now
		}
	}
}

func (s *StubRegionKeyResolver) ResolveKEKs(ctx context.Context, orgID, purpose string) (keysOut map[int]*domaincrypto.EncryptionKey, versionOut int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	purposes, ok := s.data[orgID]
	if !ok {
		return nil, 0, nil
	}
	keys, ok := purposes[purpose]
	if !ok {
		return nil, 0, nil
	}
	cur := s.current[orgID][purpose]

	// Experience 100007470: 解密层需要 revoked KEK 来解旧 token，所以返回所有 IsUsableForDecrypt 的
	usable := make(map[int]*domaincrypto.EncryptionKey)
	for v, k := range keys {
		if k.IsUsableForDecrypt() {
			usable[v] = k
		}
	}
	return usable, cur, nil
}

func (s *StubRegionKeyResolver) CurrentKEKVersion(ctx context.Context, orgID, purpose string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	purposes, ok := s.current[orgID]
	if !ok {
		return 0, apperrors.NewNotFoundf("no KEKs for org %s", orgID)
	}
	cur, ok := purposes[purpose]
	if !ok {
		return 0, apperrors.NewNotFoundf("no KEK for purpose %s org %s", purpose, orgID)
	}
	return cur, nil
}

// _ unused fmt guard
var _ = fmt.Sprintf
