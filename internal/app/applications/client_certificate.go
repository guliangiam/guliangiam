package applications

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformmtls "github.com/guliangiam/guliangiam/internal/platform/mtls"
)

// ClientCertificateInput 是一次登记的输入：调用方必须同时提交公钥证书本体，
// 服务端重算指纹后与 FingerprintSHA256 比对。只报指纹的请求一律拒绝，
// 否则可以把一个自己并不持有的证书指纹绑到应用上，造成无法使用的配置。
type ClientCertificateInput struct {
	// FingerprintSHA256 是 RFC 8705 x5t#S256：43 字符 base64url(SHA-256(DER))。
	FingerprintSHA256 string
	// Certificate 是单张公有证书，PEM（CERTIFICATE 块）或 base64 DER。
	Certificate string
	// SelfSigned 由调用方显式声明，必须与当前 token_endpoint_auth_method 一致。
	SelfSigned bool
}

// ClientCertificateView 是读侧投影：只暴露指纹与元数据，绝不返回证书原文。
type ClientCertificateView struct {
	FingerprintSHA256 string     `json:"fingerprintSha256"`
	NotAfter          *time.Time `json:"notAfter"`
	SelfSigned        bool       `json:"selfSigned"`
	Status            string     `json:"status"`
}

// ReplaceClientCertificatesCommand 是替换式写入：本次集合即最终活跃集合。
type ReplaceClientCertificatesCommand struct {
	ID             string
	OrganizationID string
	ActorID        string
	Certificates   []ClientCertificateInput
}

// ListClientCertificates 返回应用当前活跃的客户端证书元数据。
// 管理面读取：租户必填，且先按 org 读应用，跨租户直接 404。
func (s *Service) ListClientCertificates(ctx context.Context, organizationID, applicationID string) ([]ClientCertificateView, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}

	var views []ClientCertificateView
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		application, err := tx.Applications().GetByIDForOrganization(ctx, applicationID, organizationID)
		if err != nil {
			return err
		}
		certificates, err := tx.Applications().ListActiveClientCertificates(ctx, application.OrganizationID, application.ID)
		if err != nil {
			return err
		}
		views = projectClientCertificates(certificates)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return views, nil
}

// ReplaceClientCertificates 用请求集合替换应用的全部活跃证书绑定。
//
// 校验全部发生在写入前：认证方式必须是 mTLS 之一、每张证书可解析且指纹与
// 登记值一致、self_signed 标记与认证方式一致。任一条失败即整体回滚，不会留下
// 部分替换的证书集合。
func (s *Service) ReplaceClientCertificates(ctx context.Context, command ReplaceClientCertificatesCommand) error {
	if err := requireOrganizationID(command.OrganizationID); err != nil {
		return err
	}

	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		application, err := tx.Applications().GetByIDForOrganization(ctx, command.ID, command.OrganizationID)
		if err != nil {
			return err
		}
		method := application.EffectiveTokenEndpointAuthMethod()
		if !isMTLSMethod(method) {
			return apperrors.NewUnprocessableEntity(
				"client certificates are only accepted for tls_client_auth or self_signed_tls_client_auth",
			)
		}
		return s.replaceClientCertificatesTx(ctx, tx, application, method, command.Certificates, command.ActorID, s.Clock.Now())
	})
}

// replaceClientCertificatesTx 校验并整体替换证书集合。调用方负责确认 method
// 是 mTLS 之一；本函数保证集合非空，因为 mTLS 应用没有证书永远无法认证。
func (s *Service) replaceClientCertificatesTx(
	ctx context.Context,
	tx appuow.UnitOfWork,
	application *domain.Application,
	method string,
	inputs []ClientCertificateInput,
	actorID string,
	now time.Time,
) error {
	if len(inputs) == 0 {
		return clientCertificatesRequiredError(method)
	}
	if len(inputs) > domain.ClientCertificateMaxPerApplication {
		return apperrors.NewUnprocessableEntityf(
			"at most %d client certificates can be registered per application",
			domain.ClientCertificateMaxPerApplication,
		)
	}

	rows, prefixes, selfSignedCount, err := s.buildClientCertificateRows(application, method, inputs, now)
	if err != nil {
		return err
	}

	if err := tx.Applications().ReplaceClientCertificates(ctx, application.OrganizationID, application.ID, rows, now); err != nil {
		return err
	}

	// 审计只记录指纹前缀与计数：证书原文、完整指纹都不进审计与 outbox。
	return s.embedAuditEvent(
		tx,
		ctx,
		application.OrganizationID,
		actorID,
		"application.client_certificate.replaced",
		application.ID,
		"application.client_certificate.replaced",
		map[string]any{
			"fingerprintPrefixes": prefixes,
			"count":               len(rows),
			"selfSignedCount":     selfSignedCount,
		},
	)
}

func isMTLSMethod(method string) bool {
	return method == domain.TokenEndpointAuthMethodTLSClientAuth ||
		method == domain.TokenEndpointAuthMethodSelfSignedTLSClientAuth
}

// clientCertificatesRequiredError 是 422：mTLS 应用没有任何证书时，token
// endpoint 必然拒绝认证，因此不接受这种持久化状态。
func clientCertificatesRequiredError(method string) error {
	return apperrors.NewWithMeta(
		apperrors.CodeUnprocessableEntity,
		"at least one client certificate is required when tokenEndpointAuthMethod is tls_client_auth or self_signed_tls_client_auth",
		map[string]any{
			"field":    "clientCertificates",
			"value":    method,
			"expected": "non_empty_client_certificates",
		},
	)
}

func (s *Service) buildClientCertificateRows(
	application *domain.Application,
	method string,
	inputs []ClientCertificateInput,
	now time.Time,
) ([]*domain.ClientCertificate, []string, int, error) {
	rows := make([]*domain.ClientCertificate, 0, len(inputs))
	prefixes := make([]string, 0, len(inputs))
	seen := make(map[string]struct{}, len(inputs))
	selfSignedCount := 0

	for index, input := range inputs {
		if input.SelfSigned != (method == domain.TokenEndpointAuthMethodSelfSignedTLSClientAuth) {
			return nil, nil, 0, apperrors.NewUnprocessableEntityf(
				"certificate %d: selfSigned must match token_endpoint_auth_method %s", index+1, method,
			)
		}

		certificate, err := parseUploadedClientCertificate(input.Certificate)
		if err != nil {
			return nil, nil, 0, err
		}
		thumbprint, err := platformmtls.Thumbprint(certificate)
		if err != nil {
			return nil, nil, 0, apperrors.NewUnprocessableEntityf("certificate %d: fingerprint cannot be computed", index+1)
		}
		if strings.TrimSpace(input.FingerprintSHA256) != thumbprint {
			// 同时覆盖“只报指纹不报证书”的漏报：空值必然不等于重算值。
			return nil, nil, 0, apperrors.NewUnprocessableEntityf(
				"certificate %d: fingerprint does not match the supplied certificate", index+1,
			)
		}
		if _, duplicate := seen[thumbprint]; duplicate {
			return nil, nil, 0, apperrors.NewUnprocessableEntityf(
				"certificate %d: duplicate fingerprint", index+1,
			)
		}
		seen[thumbprint] = struct{}{}

		id, err := s.UUID.NewString()
		if err != nil {
			return nil, nil, 0, apperrors.WrapInternal(err, "generate client certificate id")
		}
		notBefore, notAfter := certificate.NotBefore.UTC(), certificate.NotAfter.UTC()
		rows = append(rows, &domain.ClientCertificate{
			ID:                id,
			ApplicationID:     application.ID,
			OrganizationID:    application.OrganizationID,
			AuthMethod:        method,
			CertificatePEM:    encodeCertificatePEM(certificate),
			FingerprintSHA256: thumbprint,
			SubjectDN:         certificate.Subject.String(),
			IssuerDN:          certificate.Issuer.String(),
			NotBefore:         &notBefore,
			NotAfter:          &notAfter,
			Status:            domain.ClientCertificateStatusActive,
			CreatedAt:         now,
			UpdatedAt:         now,
			Revision:          1,
		})
		prefix := thumbprint
		if len(prefix) > 12 {
			prefix = prefix[:12]
		}
		prefixes = append(prefixes, prefix)
		if input.SelfSigned {
			selfSignedCount++
		}
	}
	return rows, prefixes, selfSignedCount, nil
}

// parseUploadedClientCertificate 接受 PEM（恰好一个 CERTIFICATE 块）或 base64 DER。
// 私钥块显式拒绝：静默跳过会让调用方以为服务端保存了私钥。
func parseUploadedClientCertificate(raw string) (*x509.Certificate, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, apperrors.NewUnprocessableEntity("certificate is required")
	}
	if len(trimmed) > domain.ClientCertificateMaxPEMBytes {
		return nil, apperrors.NewUnprocessableEntity("certificate exceeds the maximum size")
	}
	if strings.Contains(trimmed, "PRIVATE KEY") {
		return nil, apperrors.NewUnprocessableEntity("private key material is not accepted; submit the public certificate only")
	}

	if block, rest := pem.Decode([]byte(trimmed)); block != nil {
		if block.Type != "CERTIFICATE" {
			return nil, apperrors.NewUnprocessableEntity("PEM block must be a CERTIFICATE")
		}
		if remainder := strings.TrimSpace(string(rest)); remainder != "" {
			return nil, apperrors.NewUnprocessableEntity("exactly one CERTIFICATE block is expected")
		}
		certificate, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, apperrors.NewUnprocessableEntityf("parse certificate: %v", err)
		}
		return certificate, nil
	}

	der, err := base64.StdEncoding.DecodeString(trimmed)
	if err != nil {
		der, err = base64.RawStdEncoding.DecodeString(trimmed)
	}
	if err != nil {
		return nil, apperrors.NewUnprocessableEntity("certificate must be PEM or base64 DER")
	}
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, apperrors.NewUnprocessableEntityf("parse certificate: %v", err)
	}
	return certificate, nil
}

func encodeCertificatePEM(certificate *x509.Certificate) string {
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate.Raw}))
}

func projectClientCertificates(certificates []*domain.ClientCertificate) []ClientCertificateView {
	views := make([]ClientCertificateView, 0, len(certificates))
	for _, certificate := range certificates {
		if certificate == nil {
			continue
		}
		views = append(views, ClientCertificateView{
			FingerprintSHA256: certificate.FingerprintSHA256,
			NotAfter:          certificate.NotAfter,
			SelfSigned:        certificate.AuthMethod == domain.TokenEndpointAuthMethodSelfSignedTLSClientAuth,
			Status:            string(certificate.Status),
		})
	}
	return views
}
