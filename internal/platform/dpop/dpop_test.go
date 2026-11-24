package dpop

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestValidateDPoPProof_EmptyProof(t *testing.T) {
	tests := []struct {
		name  string
		proof string
	}{
		{"empty string", ""},
		{"whitespace only", "   "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateDPoPProof(tt.proof, "POST", "https://example.com", 60*time.Second)
			if err == nil {
				t.Error("expected error for empty proof")
			}
		})
	}
}

func TestValidateDPoPProof_InvalidFormat(t *testing.T) {
	tests := []struct {
		name  string
		proof string
	}{
		{"one part", "abc"},
		{"two parts", "abc.def"},
		{"four parts", "a.b.c.d"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateDPoPProof(tt.proof, "POST", "https://example.com", 60*time.Second)
			if err == nil {
				t.Error("expected error for invalid format")
			}
		})
	}
}

func TestValidateDPoPProof_InvalidHeaderBase64(t *testing.T) {
	_, err := ValidateDPoPProof("!!!invalid!!!.eyJ0eXAiOiJkcG9wK2p3dCJ9.c2ln", "POST", "https://example.com", 60*time.Second)
	if err == nil {
		t.Error("expected error for invalid header base64")
	}
}

func TestValidateDPoPProof_InvalidHeaderJSON(t *testing.T) {
	headerB64 := base64.RawURLEncoding.EncodeToString([]byte("not-json"))
	claimsB64 := base64.RawURLEncoding.EncodeToString([]byte("{}"))
	_, err := ValidateDPoPProof(headerB64+"."+claimsB64+".c2ln", "POST", "https://example.com", 60*time.Second)
	if err == nil {
		t.Error("expected error for invalid header JSON")
	}
}

func TestValidateDPoPProof_InvalidTyp(t *testing.T) {
	header := map[string]any{"typ": "jwt", "alg": "RS256", "jwk": map[string]string{"kty": "RSA"}}
	headerJSON, _ := json.Marshal(header)
	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	claimsB64 := base64.RawURLEncoding.EncodeToString([]byte("{}"))
	_, err := ValidateDPoPProof(headerB64+"."+claimsB64+".c2ln", "POST", "https://example.com", 60*time.Second)
	if err == nil {
		t.Error("expected error for invalid typ")
	}
	appErr, ok := err.(*apperrors.Error)
	if ok && appErr.Message != "DPoP proof header typ must be dpop+jwt" {
		t.Errorf("unexpected error message: %s", appErr.Message)
	}
}

func TestValidateDPoPProof_MissingAlg(t *testing.T) {
	header := map[string]any{"typ": "dpop+jwt", "jwk": map[string]string{"kty": "RSA"}}
	headerJSON, _ := json.Marshal(header)
	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	claimsB64 := base64.RawURLEncoding.EncodeToString([]byte("{}"))
	_, err := ValidateDPoPProof(headerB64+"."+claimsB64+".c2ln", "POST", "https://example.com", 60*time.Second)
	if err == nil {
		t.Error("expected error for missing alg")
	}
}

func TestValidateDPoPProof_MissingJWK(t *testing.T) {
	header := map[string]any{"typ": "dpop+jwt", "alg": "RS256"}
	headerJSON, _ := json.Marshal(header)
	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	claimsB64 := base64.RawURLEncoding.EncodeToString([]byte("{}"))
	_, err := ValidateDPoPProof(headerB64+"."+claimsB64+".c2ln", "POST", "https://example.com", 60*time.Second)
	if err == nil {
		t.Error("expected error for missing jwk")
	}
}

func TestValidateDPoPProof_MissingRequiredClaims(t *testing.T) {
	header := map[string]any{"typ": "dpop+jwt", "alg": "RS256", "jwk": map[string]string{"kty": "RSA", "n": "abc", "e": "AQAB"}}
	headerJSON, _ := json.Marshal(header)
	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)

	tests := []struct {
		name   string
		claims map[string]any
		errMsg string
	}{
		{"missing jti", map[string]any{"htm": "POST", "htu": "https://example.com", "iat": time.Now().Unix()}, "jti"},
		{"missing htm", map[string]any{"jti": "abc", "htu": "https://example.com", "iat": time.Now().Unix()}, "htm"},
		{"missing htu", map[string]any{"jti": "abc", "htm": "POST", "iat": time.Now().Unix()}, "htu"},
		{"missing iat", map[string]any{"jti": "abc", "htm": "POST", "htu": "https://example.com"}, "iat"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claimsJSON, _ := json.Marshal(tt.claims)
			claimsB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)
			_, err := ValidateDPoPProof(headerB64+"."+claimsB64+".c2ln", "POST", "https://example.com", 60*time.Second)
			if err == nil {
				t.Errorf("expected error for %s", tt.errMsg)
			}
		})
	}
}

func TestValidateDPoPProof_MethodMismatch(t *testing.T) {
	header := map[string]any{"typ": "dpop+jwt", "alg": "RS256", "jwk": map[string]string{"kty": "RSA", "n": "abc", "e": "AQAB"}}
	headerJSON, _ := json.Marshal(header)
	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)

	claims := map[string]any{"jti": "abc", "htm": "GET", "htu": "https://example.com", "iat": time.Now().Unix()}
	claimsJSON, _ := json.Marshal(claims)
	claimsB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)

	_, err := ValidateDPoPProof(headerB64+"."+claimsB64+".c2ln", "POST", "https://example.com", 60*time.Second)
	if err == nil {
		t.Error("expected error for method mismatch")
	}
}

func TestValidateDPoPProof_UrlMismatch(t *testing.T) {
	header := map[string]any{"typ": "dpop+jwt", "alg": "RS256", "jwk": map[string]string{"kty": "RSA", "n": "abc", "e": "AQAB"}}
	headerJSON, _ := json.Marshal(header)
	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)

	claims := map[string]any{"jti": "abc", "htm": "POST", "htu": "https://other.com", "iat": time.Now().Unix()}
	claimsJSON, _ := json.Marshal(claims)
	claimsB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)

	_, err := ValidateDPoPProof(headerB64+"."+claimsB64+".c2ln", "POST", "https://example.com", 60*time.Second)
	if err == nil {
		t.Error("expected error for URL mismatch")
	}
}

func TestValidateDPoPProof_TokenTooOld(t *testing.T) {
	header := map[string]any{"typ": "dpop+jwt", "alg": "RS256", "jwk": map[string]string{"kty": "RSA", "n": "abc", "e": "AQAB"}}
	headerJSON, _ := json.Marshal(header)
	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)

	claims := map[string]any{"jti": "abc", "htm": "POST", "htu": "https://example.com", "iat": time.Now().Add(-120 * time.Second).Unix()}
	claimsJSON, _ := json.Marshal(claims)
	claimsB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)

	_, err := ValidateDPoPProof(headerB64+"."+claimsB64+".c2ln", "POST", "https://example.com", 60*time.Second)
	if err == nil {
		t.Error("expected error for token too old")
	}
}

func TestValidateDPoPProof_TokenInFuture(t *testing.T) {
	header := map[string]any{"typ": "dpop+jwt", "alg": "RS256", "jwk": map[string]string{"kty": "RSA", "n": "abc", "e": "AQAB"}}
	headerJSON, _ := json.Marshal(header)
	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)

	claims := map[string]any{"jti": "abc", "htm": "POST", "htu": "https://example.com", "iat": time.Now().Add(120 * time.Second).Unix()}
	claimsJSON, _ := json.Marshal(claims)
	claimsB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)

	_, err := ValidateDPoPProof(headerB64+"."+claimsB64+".c2ln", "POST", "https://example.com", 60*time.Second)
	if err == nil {
		t.Error("expected error for token in future")
	}
}

func TestUrlMatches(t *testing.T) {
	tests := []struct {
		name       string
		htu        string
		requestURL string
		want       bool
	}{
		{"exact match", "https://example.com/api", "https://example.com/api", true},
		{"trailing slash", "https://example.com/api/", "https://example.com/api", true},
		{"both trailing slashes", "https://example.com/api/", "https://example.com/api/", true},
		{"different paths", "https://example.com/api", "https://example.com/other", false},
		{"case insensitive scheme", "HTTPS://example.com/api", "https://example.com/api", true},
		{"with whitespace", "  https://example.com/api  ", "https://example.com/api", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := urlMatches(tt.htu, tt.requestURL)
			if got != tt.want {
				t.Errorf("urlMatches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDPoPValidator_DisallowedAlgorithm(t *testing.T) {
	v := NewDPoPValidator()
	v.AllowedAlgorithms = map[string]struct{}{"ES256": {}}

	header := map[string]any{"typ": "dpop+jwt", "alg": "RS256", "jwk": map[string]string{"kty": "RSA", "n": "abc", "e": "AQAB"}}
	headerJSON, _ := json.Marshal(header)
	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)

	claims := map[string]any{"jti": "abc123", "htm": "POST", "htu": "https://example.com", "iat": time.Now().Unix()}
	claimsJSON, _ := json.Marshal(claims)
	claimsB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)

	_, err := v.Validate(context.Background(), headerB64+"."+claimsB64+".c2ln", "POST", "https://example.com")
	if err == nil {
		t.Error("expected error for disallowed algorithm")
	}
}

func TestReplayCache_Check(t *testing.T) {
	rc := NewReplayCache()
	defer func() { rc.seen = nil }()

	tests := []struct {
		name string
		jti  string
		want bool
	}{
		{"first check", "jti-1", true},
		{"duplicate check", "jti-1", false},
		{"different jti", "jti-2", true},
		{"another duplicate", "jti-2", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := rc.Check(context.Background(), tt.jti)
			if err != nil {
				t.Fatalf("Check() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("Check() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReplayCache_FullCacheRejectsWithoutEvictingLiveEntries(t *testing.T) {
	now := time.Now()
	rc := &ReplayCache{
		seen: map[string]time.Time{
			"existing-1": now,
			"existing-2": now,
		},
		maxAge:     time.Minute,
		maxEntries: 2,
	}

	if accepted, err := rc.Check(context.Background(), "new-proof"); accepted || err == nil {
		t.Fatal("a full cache must reject a new proof instead of evicting live replay records")
	}
	if accepted, err := rc.Check(context.Background(), "existing-1"); accepted || err != nil {
		t.Fatal("an existing jti must remain rejected while its replay window is live")
	}
	if len(rc.seen) != rc.maxEntries {
		t.Fatalf("cache size = %d, want bounded size %d", len(rc.seen), rc.maxEntries)
	}
}

func TestReplayCache_FullCacheAcceptsAfterExpiredEntriesAreRemoved(t *testing.T) {
	now := time.Now()
	rc := &ReplayCache{
		seen: map[string]time.Time{
			"expired-1": now.Add(-2 * time.Minute),
			"expired-2": now.Add(-2 * time.Minute),
		},
		maxAge:     time.Minute,
		maxEntries: 2,
	}

	if accepted, err := rc.Check(context.Background(), "new-proof"); !accepted || err != nil {
		t.Fatal("a new proof should be accepted after expired replay records are removed")
	}
	if _, exists := rc.seen["expired-1"]; exists {
		t.Fatal("expired replay record was not removed")
	}
}

func TestComputeJWKThumbprint(t *testing.T) {
	tests := []struct {
		name    string
		jwkJSON string
		wantErr bool
	}{
		{
			"RSA key",
			`{"kty":"RSA","e":"AQAB","n":"abc123"}`,
			false,
		},
		{
			"EC key",
			`{"kty":"EC","crv":"P-256","x":"abc","y":"def"}`,
			false,
		},
		{
			"OKP key",
			`{"kty":"OKP","crv":"Ed25519","x":"abc"}`,
			false,
		},
		{
			"unsupported kty",
			`{"kty":"unknown"}`,
			true,
		},
		{
			"invalid json",
			`not-json`,
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			thumbprint, err := ComputeJWKThumbprint(tt.jwkJSON)
			if (err != nil) != tt.wantErr {
				t.Errorf("ComputeJWKThumbprint() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && thumbprint == "" {
				t.Error("expected non-empty thumbprint")
			}
		})
	}
}

func TestComputeJWKThumbprint_Consistent(t *testing.T) {
	jwk := `{"kty":"RSA","e":"AQAB","n":"abc123"}`
	tp1, err := ComputeJWKThumbprint(jwk)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tp2, err := ComputeJWKThumbprint(jwk)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tp1 != tp2 {
		t.Errorf("thumbprints should be consistent: %s != %s", tp1, tp2)
	}
}

func TestBindTokenToDPoP(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]any
		thumb    string
	}{
		{"nil metadata", nil, "thumb1"},
		{"existing metadata", map[string]any{"key": "value"}, "thumb2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BindTokenToDPoP(tt.metadata, tt.thumb)
			if result["tokenBinding"] != "DPoP" {
				t.Error("expected tokenBinding=DPoP")
			}
			cnf, ok := result["cnf"].(map[string]any)
			if !ok {
				t.Fatal("expected cnf to be map[string]any")
			}
			if cnf["jkt"] != tt.thumb {
				t.Errorf("cnf.jkt = %v, want %v", cnf["jkt"], tt.thumb)
			}
		})
	}
}

// parseJWKPublicKey 必须按 SEC1 未压缩点解析 EC 公钥：拒绝不在曲线上的点，
// 并接受由权威编码器产出的合法点。
func TestParseJWKPublicKey_EC_ValidP256(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	point, err := key.PublicKey.Bytes()
	if err != nil {
		t.Fatalf("encode public key: %v", err)
	}
	if len(point) != 65 || point[0] != 0x04 {
		t.Fatalf("unexpected SEC1 point length %d", len(point))
	}
	raw, err := json.Marshal(map[string]string{
		"kty": "EC",
		"crv": "P-256",
		"x":   base64.RawURLEncoding.EncodeToString(point[1:33]),
		"y":   base64.RawURLEncoding.EncodeToString(point[33:]),
	})
	if err != nil {
		t.Fatalf("marshal JWK: %v", err)
	}

	pub, err := parseJWKPublicKey(raw)
	if err != nil {
		t.Fatalf("parseJWKPublicKey() error = %v", err)
	}
	ec, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		t.Fatalf("parseJWKPublicKey() type = %T, want *ecdsa.PublicKey", pub)
	}
	if !key.PublicKey.Equal(ec) {
		t.Error("parsed key does not match the generated key")
	}
}

func TestParseJWKPublicKey_EC_RejectsPointOffCurve(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	point, err := key.PublicKey.Bytes()
	if err != nil {
		t.Fatalf("encode public key: %v", err)
	}
	bogusY := make([]byte, 32)
	for i := range bogusY {
		bogusY[i] = 0xff
	}
	raw, err := json.Marshal(map[string]string{
		"kty": "EC",
		"crv": "P-256",
		"x":   base64.RawURLEncoding.EncodeToString(point[1:33]),
		"y":   base64.RawURLEncoding.EncodeToString(bogusY),
	})
	if err != nil {
		t.Fatalf("marshal JWK: %v", err)
	}

	if _, err := parseJWKPublicKey(raw); err == nil {
		t.Fatal("parseJWKPublicKey() error = nil, want off-curve rejection")
	}
}
