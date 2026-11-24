// Package hibp implements the Have I Been Pwned k-anonymity range lookup
// (AUTH-009, OPEN-API-85). Only the first 5 hex chars of the local SHA-1 hash
// leave the process; the plaintext and the full hash are never sent, stored
// or logged.
package hibp

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"io"
	"net/http"
	"strings"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

const (
	defaultBaseURL = "https://api.pwnedpasswords.com"
	// rangeResponseLimit bounds the origin body. A padded range response is
	// ~1000 lines (~45KB); 2MB is generous headroom, not a memory vector.
	rangeResponseLimit = 2 << 20
	// RangeCacheTTL balances corpus freshness against origin load. The HIBP
	// corpus updates infrequently; 24h keeps at most one fetch per prefix per day.
	RangeCacheTTL = 24 * time.Hour
)

// RangeCache stores origin range bodies keyed by the 5-char SHA-1 prefix.
// A cache error must never fail the lookup by itself: the checker falls
// through to the origin, and origin failure fails open at the app layer.
type RangeCache interface {
	Get(ctx context.Context, prefix string) (body string, found bool, err error)
	Set(ctx context.Context, prefix, body string) error
}

// RangeChecker is the app/auth.BreachChecker implementation.
type RangeChecker struct {
	BaseURL string
	Client  *http.Client
	Cache   RangeCache
}

func NewRangeChecker(baseURL string, timeout time.Duration, cache RangeCache) *RangeChecker {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultBaseURL
	}
	return &RangeChecker{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Client:  &http.Client{Timeout: timeout},
		Cache:   cache,
	}
}

// IsBreached hashes the password locally and compares the hash suffix against
// the range body for its prefix. SHA-1 is fixed by the HIBP protocol; it is
// used for this lookup only, never for credential storage.
func (c *RangeChecker) IsBreached(ctx context.Context, password string) (bool, error) {
	sum := sha1.Sum([]byte(password))
	hashHex := strings.ToUpper(hex.EncodeToString(sum[:]))
	prefix, suffix := hashHex[:5], hashHex[5:]

	body, err := c.rangeBody(ctx, prefix)
	if err != nil {
		return false, err
	}
	for _, line := range strings.Split(body, "\n") {
		suffixPart, countPart, ok := strings.Cut(strings.TrimSpace(line), ":")
		// Padding entries carry count 0; matching one would be a false positive.
		if ok && suffixPart == suffix && strings.TrimSpace(countPart) != "0" {
			return true, nil
		}
	}
	return false, nil
}

func (c *RangeChecker) rangeBody(ctx context.Context, prefix string) (string, error) {
	if c.Cache != nil {
		if body, found, err := c.Cache.Get(ctx, prefix); err == nil && found {
			return body, nil
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/range/"+prefix, nil)
	if err != nil {
		return "", apperrors.WrapInternal(err, "build HIBP range request")
	}
	req.Header.Set("User-Agent", "GuliangIAM")
	// Add-Padding obscures the response size from network observers (HIBP API
	// recommendation); padded lines carry count 0 and never match a real hash.
	req.Header.Set("Add-Padding", "true")

	resp, err := c.Client.Do(req)
	if err != nil {
		return "", apperrors.WrapUnavailable(err, "HIBP range request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", apperrors.NewUnavailablef("HIBP range status %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, rangeResponseLimit))
	if err != nil {
		return "", apperrors.WrapUnavailable(err, "read HIBP range body")
	}
	body := string(raw)
	if c.Cache != nil {
		_ = c.Cache.Set(ctx, prefix, body)
	}
	return body, nil
}
