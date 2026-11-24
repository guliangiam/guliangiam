package payment

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// testWeChatCredentials builds a throwaway merchant key and platform certificate
// so the APIv3 construction path is exercised without real merchant secrets.
func testWeChatCredentials(t *testing.T) (keyPEM, certPEM string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal pkcs8 key: %v", err)
	}
	keyPEM = string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "wechat-pay-platform"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	certPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}))
	return keyPEM, certPEM
}

func wechatConfig(keyPEM, certPEM string) map[string]string {
	return map[string]string{
		"appId":           "wx1234567890abcdef",
		"mchId":           "1900007291",
		"serialNo":        "408B07E79B8269FEC3D5D3E6AB8ED163A6A380DB",
		"privateKey":      keyPEM,
		"platformCertPEM": certPEM,
		"isSandbox":       "false",
	}
}

func TestNewGateway_WeChatPayWithPEMCredentials(t *testing.T) {
	keyPEM, certPEM := testWeChatCredentials(t)

	gw, err := NewGateway(channelWeChatPay, wechatConfig(keyPEM, certPEM))
	if err != nil {
		t.Fatalf("NewGateway(wechat_pay) error = %v", err)
	}
	if gw == nil {
		t.Fatal("expected a gateway instance")
	}
}

func TestNewGateway_WeChatPayAcceptPKCS1Key(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	pkcs1PEM := string(pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}))
	_, certPEM := testWeChatCredentials(t)

	if _, err := NewGateway(channelWeChatPay, wechatConfig(pkcs1PEM, certPEM)); err != nil {
		t.Fatalf("PKCS#1 key should be accepted, got %v", err)
	}
}

func TestNewGateway_WeChatPayMissingCredentialsIsUnavailable(t *testing.T) {
	_, err := NewGateway(channelWeChatPay, map[string]string{})
	if err == nil {
		t.Fatal("expected an error for an empty config")
	}
	if code := apperrors.FromError(err).Code; code != apperrors.CodeUnavailable {
		t.Errorf("code = %q, want %q", code, apperrors.CodeUnavailable)
	}
}

func TestNewGateway_WeChatPayMalformedPEMIsInvalidArgument(t *testing.T) {
	_, certPEM := testWeChatCredentials(t)
	cfg := wechatConfig("not-a-pem", certPEM)
	_, err := NewGateway(channelWeChatPay, cfg)
	if err == nil {
		t.Fatal("expected an error for a malformed private key")
	}
	if code := apperrors.FromError(err).Code; code != apperrors.CodeInvalidArgument {
		t.Errorf("code = %q, want %q", code, apperrors.CodeInvalidArgument)
	}

	keyPEM, _ := testWeChatCredentials(t)
	cfg = wechatConfig(keyPEM, "not-a-cert")
	_, err = NewGateway(channelWeChatPay, cfg)
	if err == nil {
		t.Fatal("expected an error for a malformed platform certificate bundle")
	}
	if code := apperrors.FromError(err).Code; code != apperrors.CodeInvalidArgument {
		t.Errorf("certificate code = %q, want %q", code, apperrors.CodeInvalidArgument)
	}
}
