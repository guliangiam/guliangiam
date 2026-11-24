package crypto

import (
	"encoding/pem"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/emmansun/gmsm/sm2"
	"github.com/emmansun/gmsm/smx509"

	"github.com/guliangiam/guliangiam/internal/platform/config"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// gm_tls 是部署层能力位：本进程只校验双证书材料，不实现 TLCP 握手。
// 开关为 true 不等于可用；材料缺失、证书/私钥非法或不匹配、过期、CA 链
// 校验失败都必须落 unavailable，并在 Detail/degrade_reason 给出具体原因。
const (
	gmTLSSignCertFileEnv = "GM_TLS_SIGN_CERT_FILE"
	gmTLSSignKeyFileEnv  = "GM_TLS_SIGN_KEY_FILE"
	gmTLSEncCertFileEnv  = "GM_TLS_ENC_CERT_FILE"
	gmTLSEncKeyFileEnv   = "GM_TLS_ENC_KEY_FILE"
	gmTLSCAFileEnv       = "GM_TLS_CA_FILE"
)

type gmTLSFile struct {
	env  string
	path string
}

type gmTLSMaterial struct {
	signCert gmTLSFile
	signKey  gmTLSFile
	encCert  gmTLSFile
	encKey   gmTLSFile
	ca       gmTLSFile
}

func loadGMTLSMaterial() gmTLSMaterial {
	return gmTLSMaterial{
		signCert: gmTLSFile{gmTLSSignCertFileEnv, config.GMTLSSignCertFile()},
		signKey:  gmTLSFile{gmTLSSignKeyFileEnv, config.GMTLSSignKeyFile()},
		encCert:  gmTLSFile{gmTLSEncCertFileEnv, config.GMTLSEncCertFile()},
		encKey:   gmTLSFile{gmTLSEncKeyFileEnv, config.GMTLSEncKeyFile()},
		ca:       gmTLSFile{gmTLSCAFileEnv, config.GMTLSCAFile()},
	}
}

func (m gmTLSMaterial) files() []gmTLSFile {
	return []gmTLSFile{m.signCert, m.signKey, m.encCert, m.encKey, m.ca}
}

func (m gmTLSMaterial) envNames() []string {
	files := m.files()
	names := make([]string, 0, len(files))
	for _, file := range files {
		names = append(names, file.env)
	}
	return names
}

func (m gmTLSMaterial) configured() bool {
	return m.signCert.path != "" ||
		m.signKey.path != "" ||
		m.encCert.path != "" ||
		m.encKey.path != "" ||
		m.ca.path != ""
}

func (m gmTLSMaterial) missing() []string {
	missing := make([]string, 0, len(m.files()))
	for _, file := range m.files() {
		if file.path == "" {
			missing = append(missing, file.env)
		}
	}
	return missing
}

func probeGMTLS() error {
	material := loadGMTLSMaterial()
	if !config.GMTLSEnabled() {
		if !material.configured() {
			return apperrors.NewUnavailable(
				"GM_TLS_ENABLED is not true and no GM_TLS_* certificate material is configured")
		}
		// 开关关闭但材料已注入：仍执行校验以给出具体原因；即便材料有效也不得计为
		// available，因为部署层没有声明启用该能力。
		if err := validateGMTLSMaterial(material, time.Now()); err != nil {
			return apperrors.WrapUnavailable(err, "GM_TLS_ENABLED is not true")
		}
		return apperrors.NewUnavailable(
			"GM_TLS_ENABLED is not true; GM TLS material is valid but the capability is disabled")
	}

	if missing := material.missing(); len(missing) > 0 {
		return apperrors.NewUnavailablef(
			"GM_TLS_ENABLED=true requires %s; missing: %s",
			strings.Join(material.envNames(), ", "), strings.Join(missing, ", "))
	}
	return validateGMTLSMaterial(material, time.Now())
}

func validateGMTLSMaterial(material gmTLSMaterial, now time.Time) error {
	signCert, signIntermediates, err := loadSM2LeafCertificate(material.signCert, "sign", now)
	if err != nil {
		return err
	}
	signKey, err := loadSM2PrivateKey(material.signKey)
	if err != nil {
		return err
	}
	if !signKey.PublicKey.Equal(signCert.PublicKey) {
		return apperrors.NewUnavailablef(
			"%s does not match %s", material.signKey.env, material.signCert.env)
	}

	encCert, encIntermediates, err := loadSM2LeafCertificate(material.encCert, "encrypt", now)
	if err != nil {
		return err
	}
	encKey, err := loadSM2PrivateKey(material.encKey)
	if err != nil {
		return err
	}
	if !encKey.PublicKey.Equal(encCert.PublicKey) {
		return apperrors.NewUnavailablef(
			"%s does not match %s", material.encKey.env, material.encCert.env)
	}

	roots, err := loadSM2CertPool(material.ca)
	if err != nil {
		return err
	}
	if err := verifySM2Chain(signCert, signIntermediates, roots, material.signCert, now); err != nil {
		return err
	}
	return verifySM2Chain(encCert, encIntermediates, roots, material.encCert, now)
}

func loadSM2LeafCertificate(file gmTLSFile, usage string, now time.Time) (*smx509.Certificate, []*smx509.Certificate, error) {
	if file.path == "" {
		return nil, nil, apperrors.NewUnavailablef("%s is not set", file.env)
	}
	data, err := readGMFile(file)
	if err != nil {
		return nil, nil, err
	}
	certs, err := parsePEMCertificates(data)
	if err != nil {
		return nil, nil, apperrors.WrapUnavailablef(err, "%s", file.env)
	}
	if len(certs) == 0 {
		return nil, nil, apperrors.NewUnavailablef(
			"%s contains no PEM certificate block", file.env)
	}
	leaf := certs[0]
	if err := validateSM2LeafCertificate(leaf, file, usage, now); err != nil {
		return nil, nil, err
	}
	return leaf, certs[1:], nil
}

func validateSM2LeafCertificate(cert *smx509.Certificate, file gmTLSFile, usage string, now time.Time) error {
	if cert.IsCA {
		return apperrors.NewUnavailablef(
			"%s: expected a leaf certificate, got a CA certificate", file.env)
	}
	if !sm2.IsSM2PublicKey(cert.PublicKey) {
		return apperrors.NewUnavailablef(
			"%s: certificate public key is not SM2 (want OID 1.2.156.10197.1.301)", file.env)
	}
	if cert.SignatureAlgorithm != smx509.SM2WithSM3 {
		return apperrors.NewUnavailablef(
			"%s: certificate signature algorithm %s is not SM2-SM3 (want OID 1.2.156.10197.1.501)",
			file.env, cert.SignatureAlgorithm)
	}
	switch usage {
	case "sign":
		if cert.KeyUsage != 0 && cert.KeyUsage&smx509.KeyUsageDigitalSignature == 0 {
			return apperrors.NewUnavailablef(
				"%s: certificate key usage does not permit digital signature", file.env)
		}
	case "encrypt":
		encipherOnly := smx509.KeyUsageKeyEncipherment | smx509.KeyUsageDataEncipherment
		if cert.KeyUsage != 0 && cert.KeyUsage&encipherOnly == 0 {
			return apperrors.NewUnavailablef(
				"%s: certificate key usage does not permit key encipherment", file.env)
		}
	}
	if now.Before(cert.NotBefore) {
		return apperrors.NewUnavailablef(
			"%s: certificate is not valid before %s", file.env, cert.NotBefore.Format(time.RFC3339))
	}
	if now.After(cert.NotAfter) {
		return apperrors.NewUnavailablef(
			"%s: certificate expired at %s", file.env, cert.NotAfter.Format(time.RFC3339))
	}
	return nil
}

func loadSM2PrivateKey(file gmTLSFile) (*sm2.PrivateKey, error) {
	if file.path == "" {
		return nil, apperrors.NewUnavailablef("%s is not set", file.env)
	}
	data, err := readGMFile(file)
	if err != nil {
		return nil, err
	}
	der, err := privateKeyDER(data)
	if err != nil {
		return nil, apperrors.WrapUnavailablef(err, "%s", file.env)
	}
	key, err := parseSM2PrivateKey(der)
	if err != nil {
		return nil, apperrors.WrapUnavailablef(err, "%s", file.env)
	}
	return key, nil
}

func privateKeyDER(data []byte) ([]byte, error) {
	for {
		block, rest := pem.Decode(data)
		if block == nil {
			return nil, errors.New("no PEM private key block found")
		}
		if strings.Contains(block.Type, "PRIVATE KEY") {
			return block.Bytes, nil
		}
		data = rest
	}
}

func parseSM2PrivateKey(der []byte) (*sm2.PrivateKey, error) {
	parsed, pkcs8Err := smx509.ParsePKCS8PrivateKey(der)
	if pkcs8Err != nil {
		parsed, sec1Err := smx509.ParseTypedECPrivateKey(der)
		if sec1Err != nil {
			return nil, apperrors.NewUnavailablef(
				"parse PKCS#8 private key failed (%v); parse SEC1 private key failed (%v)",
				pkcs8Err, sec1Err)
		}
		return asSM2PrivateKey(parsed)
	}
	return asSM2PrivateKey(parsed)
}

func asSM2PrivateKey(parsed any) (*sm2.PrivateKey, error) {
	key, ok := parsed.(*sm2.PrivateKey)
	if !ok {
		return nil, apperrors.NewUnavailablef("private key type %T is not SM2", parsed)
	}
	return key, nil
}

func loadSM2CertPool(file gmTLSFile) (*smx509.CertPool, error) {
	if file.path == "" {
		return nil, apperrors.NewUnavailablef("%s is not set", file.env)
	}
	data, err := readGMFile(file)
	if err != nil {
		return nil, err
	}
	certs, err := parsePEMCertificates(data)
	if err != nil {
		return nil, apperrors.WrapUnavailablef(err, "%s", file.env)
	}
	pool := smx509.NewCertPool()
	added := 0
	for _, cert := range certs {
		// 只把 SM2 证书放进受信池；混入标准算法根不会让国密链变得可用。
		if sm2.IsSM2PublicKey(cert.PublicKey) {
			pool.AddCert(cert)
			added++
		}
	}
	if added == 0 {
		return nil, apperrors.NewUnavailablef(
			"%s contains no SM2 CA certificate", file.env)
	}
	return pool, nil
}

func verifySM2Chain(cert *smx509.Certificate, intermediates []*smx509.Certificate, roots *smx509.CertPool, file gmTLSFile, now time.Time) error {
	options := smx509.VerifyOptions{
		Roots:         roots,
		CurrentTime:   now,
		KeyUsages:     []smx509.ExtKeyUsage{smx509.ExtKeyUsageAny},
		Intermediates: smx509.NewCertPool(),
	}
	for _, intermediate := range intermediates {
		options.Intermediates.AddCert(intermediate)
	}
	if _, err := cert.Verify(options); err != nil {
		return apperrors.WrapUnavailablef(
			err, "%s: certificate chain verification against %s failed", file.env, gmTLSCAFileEnv)
	}
	return nil
}

func readGMFile(file gmTLSFile) ([]byte, error) {
	data, err := os.ReadFile(file.path) // #nosec G304 -- operator-supplied certificate path from env, not request input
	if err != nil {
		// /healthz 是公开端点：降级原因只披露 env 名与底层错误，不回显文件路径。
		var pathErr *os.PathError
		if errors.As(err, &pathErr) {
			return nil, apperrors.WrapUnavailablef(pathErr.Err, "%s: read failed", file.env)
		}
		return nil, apperrors.WrapUnavailablef(err, "%s: read failed", file.env)
	}
	return data, nil
}

func parsePEMCertificates(data []byte) ([]*smx509.Certificate, error) {
	var certs []*smx509.Certificate
	for {
		block, rest := pem.Decode(data)
		if block == nil {
			break
		}
		if block.Type == "CERTIFICATE" {
			cert, err := smx509.ParseCertificate(block.Bytes)
			if err != nil {
				return nil, apperrors.NewUnavailablef("parse certificate: %v", err)
			}
			certs = append(certs, cert)
		}
		data = rest
	}
	return certs, nil
}
