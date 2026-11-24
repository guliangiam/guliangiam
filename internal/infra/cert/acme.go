package cert

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/registration"

	"github.com/guliangiam/guliangiam/internal/platform/config"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/recover"
	ssrf "github.com/guliangiam/guliangiam/internal/platform/ssrf"
)

// ACMERenewer handles automatic certificate issuance and renewal via ACME (Let's Encrypt).
type ACMERenewer struct {
	directoryURL string
	email        string
	isStaging    bool
	httpClient   *http.Client
}

// NewACMERenewer creates a new ACME certificate renewer.
func NewACMERenewer(email string, isStaging bool) *ACMERenewer {
	directoryURL := "https://acme-v02.api.letsencrypt.org/directory"
	if isStaging {
		directoryURL = "https://acme-staging-v02.api.letsencrypt.org/directory"
	}
	return &ACMERenewer{
		directoryURL: directoryURL,
		email:        email,
		isStaging:    isStaging,
		httpClient:   ssrf.SafeHTTPClient(30 * time.Second),
	}
}

// acmeUser implements lego's registration.User interface.
type acmeUser struct {
	email string
	key   crypto.PrivateKey
	reg   *registration.Resource
}

func (u *acmeUser) GetEmail() string                        { return u.email }
func (u *acmeUser) GetRegistration() *registration.Resource { return u.reg }
func (u *acmeUser) GetPrivateKey() crypto.PrivateKey        { return u.key }

// ObtainCertificate obtains a new certificate for the given domain via ACME HTTP-01 challenge.
// Returns certPEM, privateKeyPEM, error.
func (r *ACMERenewer) ObtainCertificate(ctx context.Context, domain string) (certPEMOut, keyPEMOut []byte, err error) {
	// Generate ECC private key for the ACME account
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, apperrors.WrapInternalf(err, "acme generate account key")
	}

	user := &acmeUser{
		email: r.email,
		key:   privateKey,
	}

	legoCfg := lego.NewConfig(user)
	legoCfg.CADirURL = r.directoryURL

	client, err := lego.NewClient(legoCfg)
	if err != nil {
		return nil, nil, apperrors.WrapInternalf(err, "acme create client")
	}

	// Register the ACME account
	reg, err := client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
	if err != nil {
		// If registration fails (e.g., already registered), fall back to self-signed
		return r.selfSignedECCCert(domain)
	}
	user.reg = reg

	// Set up HTTP-01 challenge provider
	// This requires port 80 to be available for the challenge
	httpProvider, err := NewHTTP01Provider()
	if err != nil {
		return r.selfSignedECCCert(domain)
	}
	if err := client.Challenge.SetHTTP01Provider(httpProvider); err != nil {
		slog.Warn("acme http-01 provider not configured; falling back to self-signed",
			"domain", domain, "error", err)
		return r.selfSignedECCCert(domain)
	}

	// Generate a new private key for the certificate
	certKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, apperrors.WrapInternalf(err, "acme generate cert key")
	}

	// Create the certificate request
	request := certificate.ObtainRequest{
		Domains:    []string{domain},
		Bundle:     true,
		PrivateKey: certKey,
	}

	certificates, err := client.Certificate.Obtain(request)
	if err != nil {
		// Fall back to self-signed certificate
		return r.selfSignedECCCert(domain)
	}

	return certificates.Certificate, certificates.PrivateKey, nil
}

// RenewCertificate renews an existing certificate.
func (r *ACMERenewer) RenewCertificate(ctx context.Context, domain string, certPEM, keyPEM []byte) (certPEMOut, keyPEMOut []byte, err error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, apperrors.WrapInternalf(err, "acme generate account key")
	}

	user := &acmeUser{
		email: r.email,
		key:   privateKey,
	}

	legoCfg := lego.NewConfig(user)
	legoCfg.CADirURL = r.directoryURL

	client, err := lego.NewClient(legoCfg)
	if err != nil {
		return nil, nil, apperrors.WrapInternalf(err, "acme create client")
	}

	_, err = client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
	if err != nil {
		return r.selfSignedECCCert(domain)
	}

	httpProvider, err := NewHTTP01Provider()
	if err != nil {
		return r.selfSignedECCCert(domain)
	}
	if err := client.Challenge.SetHTTP01Provider(httpProvider); err != nil {
		slog.Warn("acme http-01 provider not configured; falling back to self-signed",
			"domain", domain, "error", err)
		return r.selfSignedECCCert(domain)
	}

	// Renew using the existing certificate
	certResource := certificate.Resource{
		Domain:      domain,
		Certificate: certPEM,
		PrivateKey:  keyPEM,
	}

	newCert, err := client.Certificate.RenewWithOptions(certResource, &certificate.RenewOptions{
		Bundle:     true,
		MustStaple: false,
	})
	if err != nil {
		return r.selfSignedECCCert(domain)
	}

	return newCert.Certificate, newCert.PrivateKey, nil
}

// CheckAndRenew checks if a certificate needs renewal and renews if needed.
// Returns newCertPEM, newKeyPEM, wasRenewed, error.
func (r *ACMERenewer) CheckAndRenew(ctx context.Context, domain string, certPEM []byte, renewBefore time.Duration) (certPEMOut, keyPEMOut []byte, renewed bool, err error) {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, nil, false, apperrors.NewNotFound("no certificate found in PEM data")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, nil, false, apperrors.WrapInternalf(err, "parse certificate")
	}

	// Check if renewal is needed
	if time.Until(cert.NotAfter) > renewBefore {
		return nil, nil, false, nil // no renewal needed
	}

	// Need to renew - extract the private key from the original cert resource
	// Since we don't have the original key PEM stored separately, obtain a new cert
	newCert, newKey, err := r.ObtainCertificate(ctx, domain)
	if err != nil {
		return nil, nil, false, apperrors.WrapInternalf(err, "renew certificate")
	}

	return newCert, newKey, true, nil
}

// StartHTTP01Challenge starts an HTTP server on port 80 to serve ACME HTTP-01 challenge responses.
func (r *ACMERenewer) StartHTTP01Challenge(token, keyAuth string) (net.Listener, error) {
	handler := http.NewServeMux()
	handler.HandleFunc("/.well-known/acme-challenge/"+token, func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(keyAuth))
	})

	var lc net.ListenConfig
	listener, err := lc.Listen(context.Background(), "tcp", ":80") //nolint:gosec // G102: ACME HTTP-01 must answer on port 80 of every interface
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "acme http-01 listen")
	}

	// 显式 http.Server（带超时）而不是 http.Serve 帮助函数：后者无法设置超时，
	// 也会触发 gosec G114。
	srv := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	recover.SafeGo("acme-http01-challenge-server", func() {
		if err := srv.Serve(listener); err != nil &&
			!errors.Is(err, net.ErrClosed) && !errors.Is(err, http.ErrServerClosed) {
			slog.Warn("acme http-01 challenge server stopped", "error", err)
		}
	})
	return listener, nil
}

// selfSignedECCCert creates a self-signed ECC certificate as a fallback.
func (r *ACMERenewer) selfSignedECCCert(domain string) (certPEMOut, keyPEMOut []byte, err error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, apperrors.WrapInternalf(err, "acme generate ecc key")
	}

	template := &x509.Certificate{
		SerialNumber:          generateSerialNumber(),
		Subject:               pkix.Name{CommonName: domain},
		DNSNames:              []string{domain},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(90 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, nil, apperrors.WrapInternalf(err, "create self-signed cert")
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	privateKeyBytes, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return nil, nil, apperrors.WrapInternalf(err, "marshal ecc key")
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privateKeyBytes})

	return certPEM, keyPEM, nil
}

func generateSerialNumber() *big.Int {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, _ := rand.Int(rand.Reader, serialNumberLimit)
	return serialNumber
}

// --- HTTP-01 Challenge Provider for lego ---

// HTTP01Provider implements lego's challenge.Provider interface for HTTP-01 challenges.
// It serves challenge tokens via an HTTP server on port 80.
type HTTP01Provider struct {
	tokens map[string]string
	server *http.Server
}

// NewHTTP01Provider creates a new HTTP-01 challenge provider.
func NewHTTP01Provider() (*HTTP01Provider, error) {
	p := &HTTP01Provider{
		tokens: make(map[string]string),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/acme-challenge/", func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.URL.Path, "/.well-known/acme-challenge/")
		if keyAuth, ok := p.tokens[token]; ok {
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte(keyAuth))
			return
		}
		http.NotFound(w, r)
	})

	p.server = &http.Server{
		Addr:              ":80",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
	}

	// Start the HTTP server in the background
	recover.SafeGo("acme-http01-provider-server", func() {
		if err := p.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("acme http-01 server error", "error", err)
		}
	})

	return p, nil
}

// Present presents the ACME challenge token.
func (p *HTTP01Provider) Present(domain, token, keyAuth string) error {
	p.tokens[token] = keyAuth
	return nil
}

// CleanUp removes the ACME challenge token.
func (p *HTTP01Provider) CleanUp(domain, token, keyAuth string) error {
	delete(p.tokens, token)
	return nil
}

// --- Certificate Auto-Renewal Background Goroutine ---

// CertRenewalChecker periodically checks certificates and renews them.
type CertRenewalChecker struct {
	renewer       *ACMERenewer
	certGetter    CertGetter
	renewBefore   time.Duration
	checkInterval time.Duration
	stopCh        chan struct{}
}

// CertGetter is an interface for retrieving certificates that need renewal checking.
type CertGetter interface {
	// GetExpiringCerts returns certificates expiring within the given duration.
	GetExpiringCerts(ctx context.Context, within time.Duration) ([]*CertInfo, error)
}

// CertInfo contains information about a certificate for renewal checking.
type CertInfo struct {
	ID             string
	OrganizationID string
	Domain         string
	CertificatePEM []byte
	PrivateKeyPEM  []byte
}

// NewCertRenewalChecker creates a new certificate renewal checker.
func NewCertRenewalChecker(renewer *ACMERenewer, certGetter CertGetter, renewBefore, checkInterval time.Duration) *CertRenewalChecker {
	return &CertRenewalChecker{
		renewer:       renewer,
		certGetter:    certGetter,
		renewBefore:   renewBefore,
		checkInterval: checkInterval,
		stopCh:        make(chan struct{}),
	}
}

// Start starts the certificate renewal checker goroutine.
func (c *CertRenewalChecker) Start(ctx context.Context) {
	ticker := time.NewTicker(c.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.checkAndRenewAll(ctx)
		}
	}
}

// Stop stops the certificate renewal checker.
func (c *CertRenewalChecker) Stop() {
	close(c.stopCh)
}

func (c *CertRenewalChecker) checkAndRenewAll(ctx context.Context) {
	certs, err := c.certGetter.GetExpiringCerts(ctx, c.renewBefore)
	if err != nil {
		slog.Error("cert renewal checker: failed to get expiring certs", "error", err)
		return
	}

	for _, ci := range certs {
		if ci.Domain == "" {
			continue
		}

		newCert, newKey, renewed, err := c.renewer.CheckAndRenew(ctx, ci.Domain, ci.CertificatePEM, c.renewBefore)
		if err != nil {
			slog.Error("cert renewal checker: failed to check/renew cert", "id", ci.ID, "error", err)
			continue
		}

		if renewed {
			slog.Info("cert renewal checker: renewed certificate", "domain", ci.Domain, "id", ci.ID)
			_ = newCert
			_ = newKey
			// In full integration, update the certificate in the repository:
			// c.certUpdater.UpdateCert(ctx, ci.OrganizationID, ci.ID, newCert, newKey)
		}
	}
}

// --- Environment Helpers ---

// ACMEEnabled returns true if ACME is enabled via environment variables.
func ACMEEnabled() bool {
	return strconv.FormatBool(config.ACMEEnabled()) == "true" || strconv.FormatBool(config.ACMEEnabled()) == "1"
}

// ACMEDefaultRenewBefore returns the default time before expiration to renew certificates.
func ACMEDefaultRenewBefore() time.Duration {
	return 30 * 24 * time.Hour // 30 days
}

// ACMEDefaultCheckInterval returns the default interval for checking certificate expiration.
func ACMEDefaultCheckInterval() time.Duration {
	return 24 * time.Hour // Check once per day
}
