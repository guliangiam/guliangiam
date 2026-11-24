// Package jwksuri fetches a client's remote JWKS under strict SSRF, timeout,
// size and cache controls (OPEN-API-61).
package jwksuri

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	"github.com/guliangiam/guliangiam/internal/platform/ssrf"
)

const (
	defaultTimeout     = 5 * time.Second
	defaultMaxBodySize = 64 << 10 // 64 KiB
	defaultTTL         = 5 * time.Minute
)

type cacheEntry struct {
	keys      []*applicationdomain.ClientJWK
	expiresAt time.Time
}

// Fetcher retrieves remote JWKS documents. The HTTP client must be an
// SSRF-safe client; tests may inject one with an explicit loopback allowlist.
type Fetcher struct {
	client      *http.Client
	maxBodySize int64
	ttl         time.Duration
	now         func() time.Time

	mu    sync.RWMutex
	cache map[string]cacheEntry
}

func NewFetcher(client *http.Client) *Fetcher {
	if client == nil {
		client = ssrf.SafeHTTPClient(defaultTimeout)
	}
	return &Fetcher{
		client:      client,
		maxBodySize: defaultMaxBodySize,
		ttl:         defaultTTL,
		now:         time.Now,
		cache:       make(map[string]cacheEntry),
	}
}

// Fetch returns the verified-shape keys of the remote JWKS. Only https URLs are
// accepted; failures never return cached data (fail-closed).
func (f *Fetcher) Fetch(ctx context.Context, jwksURI string) ([]*applicationdomain.ClientJWK, error) {
	trimmed := strings.TrimSpace(jwksURI)
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return nil, fmt.Errorf("jwks_uri must be an absolute HTTPS URL")
	}
	if err := ssrf.ValidateURL(trimmed); err != nil {
		return nil, err
	}

	now := f.now()
	f.mu.RLock()
	entry, ok := f.cache[trimmed]
	f.mu.RUnlock()
	if ok && now.Before(entry.expiresAt) {
		return entry.keys, nil
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, trimmed, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	response, err := f.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch jwks_uri: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch jwks_uri: unexpected status %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, f.maxBodySize+1))
	if err != nil {
		return nil, fmt.Errorf("read jwks_uri: %w", err)
	}
	if int64(len(body)) > f.maxBodySize {
		return nil, fmt.Errorf("jwks_uri response exceeds %d bytes", f.maxBodySize)
	}

	var document struct {
		Keys []map[string]any `json:"keys"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return nil, fmt.Errorf("decode jwks_uri: %w", err)
	}
	keys := make([]*applicationdomain.ClientJWK, 0, len(document.Keys))
	for _, raw := range document.Keys {
		kid, _ := raw["kid"].(string)
		if strings.TrimSpace(kid) == "" {
			continue
		}
		alg, _ := raw["alg"].(string)
		serialized, err := json.Marshal(raw)
		if err != nil {
			continue
		}
		keys = append(keys, &applicationdomain.ClientJWK{Kid: kid, Alg: strings.TrimSpace(alg), JWK: string(serialized)})
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("jwks_uri document contains no usable keys")
	}

	f.mu.Lock()
	f.cache[trimmed] = cacheEntry{keys: keys, expiresAt: now.Add(f.ttl)}
	f.mu.Unlock()
	return keys, nil
}
