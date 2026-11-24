package notification

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"

	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	"github.com/guliangiam/guliangiam/internal/platform/ssrf"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// vapidJWTLifetime bounds the VAPID JWT expiry; RFC 8292 caps it at 24h.
const vapidJWTLifetime = 12 * time.Hour

// WebPushNotifier sends notifications via WebPush (W3C Push API).
//
// Calibration (OPEN-API-75 P6, 2026-11-14, 已按官方规范复核，无偏差)：RFC 8291
// aes128gcm 载荷加密、RFC 8030 TTL/Content-Type、RFC 8292 VAPID
// `vapid t=<JWT>, k=<pub>`，JWT claims aud=订阅端点 origin、exp 为 NumericDate
// 且 ≤24h（本实现取 12h）：https://datatracker.ietf.org/doc/html/rfc8292
// Implements RFC 8291 (Message Encryption for Web Push) using
// only the Go standard library (no external webpush dependency).
//
// Requires VAPID keys configured via WEBPUSH_VAPID_PUBLIC_KEY,
// WEBPUSH_VAPID_PRIVATE_KEY, and WEBPUSH_SUBSCRIBER_EMAIL env vars.
type WebPushNotifier struct {
	VAPIDPublicKey  string
	VAPIDPrivateKey string
	SubscriberEmail string
	HTTPClient      *http.Client
}

// NewWebPushNotifier creates a new WebPush notifier without configuration.
func NewWebPushNotifier() *WebPushNotifier {
	return &WebPushNotifier{HTTPClient: ssrf.DefaultHTTPClient}
}

// NewWebPushNotifierWithConfig creates a new WebPush notifier with VAPID configuration.
func NewWebPushNotifierWithConfig(publicKey, privateKey, email string) *WebPushNotifier {
	return &WebPushNotifier{
		VAPIDPublicKey:  publicKey,
		VAPIDPrivateKey: privateKey,
		SubscriberEmail: email,
		HTTPClient:      ssrf.DefaultHTTPClient,
	}
}

func (n *WebPushNotifier) Send(ctx context.Context, notif *Notification) error {
	if n.VAPIDPrivateKey == "" {
		return apperrors.NewUnavailable("webpush notification: VAPID private key is not configured")
	}
	if notif.Recipient == "" {
		return apperrors.NewInvalidArgument("webpush notification: push subscription endpoint is required")
	}

	p256dh := getMetadataString(notif, "p256dh")
	auth := getMetadataString(notif, "auth")
	if p256dh == "" || auth == "" {
		return apperrors.NewInvalidArgument("webpush notification: subscription keys (p256dh, auth) are required in metadata")
	}

	clientPubKey, err := base64.RawURLEncoding.DecodeString(p256dh)
	if err != nil {
		return apperrors.WrapInternalf(err, "webpush notification: invalid p256dh key")
	}
	authKey, err := base64.RawURLEncoding.DecodeString(auth)
	if err != nil {
		return apperrors.WrapInternalf(err, "webpush notification: invalid auth key")
	}

	encryptedBody, err := encryptWebPushPayload([]byte(notif.Content), clientPubKey, authKey)
	if err != nil {
		return apperrors.WrapInternalf(err, "webpush notification: encryption failed")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, notif.Recipient, bytes.NewReader(encryptedBody))
	if err != nil {
		return apperrors.WrapInternalf(err, "webpush notification: create request")
	}
	// RFC 8030: encrypted push bodies are opaque octet streams.
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Content-Encoding", "aes128gcm")
	req.Header.Set("TTL", "86400")
	// RFC 8292: VAPID identifies the application server via an ES256 JWT.
	vapidAuth, err := n.vapidAuthorization(notif.Recipient)
	if err != nil {
		return err
	}
	if vapidAuth != "" {
		req.Header.Set("Authorization", vapidAuth)
	}

	resp, err := n.HTTPClient.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "webpush notification: send request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return apperrors.NewInternalf("webpush notification: unexpected status %d", resp.StatusCode)
	}

	return nil
}

// encryptWebPushPayload implements RFC 8291 aes128gcm content encryption.
func encryptWebPushPayload(payload, clientPubKey, authKey []byte) ([]byte, error) {
	curve := elliptic.P256()

	// Generate ephemeral ECDSA key pair (used only for ECDH)
	epk, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "generate ephemeral key")
	}
	serverPubKey := elliptic.Marshal(curve, epk.PublicKey.X, epk.PublicKey.Y)

	// Parse client's public key
	cx, cy := elliptic.Unmarshal(curve, clientPubKey)
	if cx == nil {
		return nil, apperrors.NewInvalidArgument("invalid client public key")
	}

	// Compute shared secret via ECDH: shared = d * Cx (x-coordinate only)
	sx, _ := curve.ScalarMult(cx, cy, epk.D.Bytes())
	sharedSecret := sx.Bytes()

	// Derive PRK per RFC 8291 Section 3.2
	prk, err := derivePRK(authKey, sharedSecret, clientPubKey, serverPubKey)
	if err != nil {
		return nil, err
	}

	// Derive CEK and nonce with the shared HKDF-SHA256 implementation.
	cek, err := platformcrypto.Expand(prk, []byte("Content-Encoding: aes128gcm\x00"), 16)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "derive webpush CEK")
	}
	nonce, err := platformcrypto.Expand(prk, []byte("Content-Encoding: nonce\x00"), 12)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "derive webpush nonce")
	}

	// Pad: payload || 0x02
	padded := make([]byte, len(payload)+1)
	copy(padded, payload)
	padded[len(payload)] = 0x02

	block, err := aes.NewCipher(cek)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "create AES cipher")
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "create GCM")
	}
	ciphertext := aead.Seal(nil, nonce, padded, nil)

	// Build body: salt(16) | rs(4) | keyid_len(1) | keyid | encrypted_payload
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, apperrors.WrapInternalf(err, "generate salt")
	}
	rs := make([]byte, 4)
	binary.BigEndian.PutUint32(rs, 4096)

	var body []byte
	body = append(body, salt...)
	body = append(body, rs...)
	body = append(body, 0) // empty key ID length
	body = append(body, ciphertext...)

	return body, nil
}

// derivePRK computes the PRK per RFC 8291 Section 3.2 using the shared
// HKDF-SHA256 implementation (Platform Crypto single source of truth).
func derivePRK(authKey, sharedSecret, clientPubKey, serverPubKey []byte) ([]byte, error) {
	// PRK = HKDF-Extract(salt=auth_key, IKM=shared_secret)
	prk := platformcrypto.Extract(sharedSecret, authKey)

	// key_info = "P-256" || 0x00 || len(client)(2) || client || len(server)(2) || server
	keyInfo := []byte("P-256\x00")
	cl := make([]byte, 2)
	binary.BigEndian.PutUint16(cl, uint16(len(clientPubKey))) // #nosec G115 -- ECDH public key is always 65 bytes
	sl := make([]byte, 2)
	binary.BigEndian.PutUint16(sl, uint16(len(serverPubKey))) // #nosec G115 -- ECDH public key is always 65 bytes
	keyInfo = append(keyInfo, cl...)
	keyInfo = append(keyInfo, clientPubKey...)
	keyInfo = append(keyInfo, sl...)
	keyInfo = append(keyInfo, serverPubKey...)

	info := []byte("Content-Encoding: aes128gcm\x00")
	return platformcrypto.Expand(prk, append(info, keyInfo...), 32)
}

// vapidAuthorization builds the RFC 8292 Authorization value
// `vapid t=${JWT}, k=${public_key}` for the endpoint's origin. It returns an
// empty string when no VAPID private key is configured.
func (n *WebPushNotifier) vapidAuthorization(endpoint string) (string, error) {
	if n.VAPIDPrivateKey == "" {
		return "", nil
	}
	priv, err := parseVAPIDPrivateKey(n.VAPIDPrivateKey)
	if err != nil {
		return "", err
	}
	pubBytes, err := n.vapidPublicKeyBytes(priv)
	if err != nil {
		return "", err
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", apperrors.NewInvalidArgument("webpush notification: push subscription endpoint must be an absolute URL")
	}
	aud := u.Scheme + "://" + u.Host

	header, err := json.Marshal(map[string]string{"typ": "JWT", "alg": "ES256"})
	if err != nil {
		return "", apperrors.WrapInternalf(err, "webpush notification: marshal VAPID JWT header")
	}
	// exp is a JWT NumericDate (JSON number), per RFC 8292 §3.1.
	claims := map[string]any{
		"aud": aud,
		"exp": time.Now().Add(vapidJWTLifetime).Unix(),
	}
	if n.SubscriberEmail != "" {
		claims["sub"] = "mailto:" + n.SubscriberEmail
	}
	claimsBytes, err := json.Marshal(claims)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "webpush notification: marshal VAPID JWT claims")
	}

	signingInput := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(claimsBytes)
	digest := sha256.Sum256([]byte(signingInput))
	r, s, err := ecdsa.Sign(rand.Reader, priv, digest[:])
	if err != nil {
		return "", apperrors.WrapInternalf(err, "webpush notification: sign VAPID JWT")
	}
	sig := append(ensureBigBytes(r, 32), ensureBigBytes(s, 32)...)

	t := signingInput + "." + base64.RawURLEncoding.EncodeToString(sig)
	k := base64.RawURLEncoding.EncodeToString(pubBytes)
	return "vapid t=" + t + ", k=" + k, nil
}

// vapidPublicKeyBytes returns the 65-byte uncompressed P-256 point advertised
// in the `k=` parameter, from config or derived from the private key.
func (n *WebPushNotifier) vapidPublicKeyBytes(priv *ecdsa.PrivateKey) ([]byte, error) {
	if n.VAPIDPublicKey != "" {
		raw, err := decodeMaybeBase64(n.VAPIDPublicKey)
		if err != nil {
			return nil, apperrors.WrapInternalf(err, "webpush notification: invalid VAPID public key")
		}
		if len(raw) != 65 || raw[0] != 0x04 {
			return nil, apperrors.NewInvalidArgument("webpush notification: VAPID public key must be a 65-byte uncompressed P-256 point")
		}
		return raw, nil
	}
	return elliptic.Marshal(elliptic.P256(), priv.PublicKey.X, priv.PublicKey.Y), nil
}

// parseVAPIDPrivateKey accepts a PEM SEC1/PKCS8 EC key or a base64url/base64
// encoded raw 32-byte P-256 scalar.
func parseVAPIDPrivateKey(raw string) (*ecdsa.PrivateKey, error) {
	if block, _ := pem.Decode([]byte(raw)); block != nil {
		switch block.Type {
		case "EC PRIVATE KEY":
			key, err := x509.ParseECPrivateKey(block.Bytes)
			if err != nil {
				return nil, apperrors.WrapInvalidArgumentf(err, "webpush notification: parse PEM EC private key")
			}
			return key, nil
		case "PRIVATE KEY":
			parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
			if err != nil {
				return nil, apperrors.WrapInvalidArgumentf(err, "webpush notification: parse PEM PKCS8 private key")
			}
			key, ok := parsed.(*ecdsa.PrivateKey)
			if !ok {
				return nil, apperrors.NewInvalidArgument("webpush notification: PEM private key is not ECDSA")
			}
			return key, nil
		default:
			return nil, apperrors.NewInvalidArgument("webpush notification: unsupported PEM private key type " + block.Type)
		}
	}
	keyBytes, err := decodeMaybeBase64(raw)
	if err != nil {
		return nil, apperrors.WrapInvalidArgumentf(err, "webpush notification: invalid VAPID private key")
	}
	if len(keyBytes) != 32 {
		return nil, apperrors.NewInvalidArgument("webpush notification: raw VAPID private key must be a 32-byte P-256 scalar")
	}
	curve := elliptic.P256()
	d := new(big.Int).SetBytes(keyBytes)
	x, y := curve.ScalarBaseMult(d.Bytes())
	return &ecdsa.PrivateKey{D: d, PublicKey: ecdsa.PublicKey{Curve: curve, X: x, Y: y}}, nil
}

// decodeMaybeBase64 tries base64url (with and without padding) then standard
// base64.
func decodeMaybeBase64(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	for _, enc := range []*base64.Encoding{base64.RawURLEncoding, base64.URLEncoding, base64.StdEncoding} {
		if b, err := enc.DecodeString(s); err == nil {
			return b, nil
		}
	}
	return nil, apperrors.NewInvalidArgument("webpush notification: value is not base64 encoded")
}

// ensureBigBytes pads a big.Int to the expected byte length for P-256 (32 bytes).
func ensureBigBytes(b *big.Int, size int) []byte {
	raw := b.Bytes()
	if len(raw) >= size {
		return raw[len(raw)-size:]
	}
	padded := make([]byte, size)
	copy(padded[size-len(raw):], raw)
	return padded
}

// getMetadataString extracts a string value from Notification metadata.
func getMetadataString(notif *Notification, key string) string {
	if notif.Metadata == nil {
		return ""
	}
	val, ok := notif.Metadata[key]
	if !ok {
		return ""
	}
	str, ok := val.(string)
	if !ok {
		return fmt.Sprintf("%v", val)
	}
	return str
}
