package auth

import (
	"context"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	tokendomain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	domainworkload "github.com/guliangiam/guliangiam/internal/domain/workload"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

// FederationTrustResolver returns every active trust candidate for an issuer.
// A candidate is selected only after full JWT verification and subject matching.
type FederationTrustResolver interface {
	FindActiveTrustsByIssuer(ctx context.Context, issuer string) ([]*domainworkload.FederationTrust, error)
}

// ExternalJWTVerifier 验证外部 IdP 签发的 JWT（由 infra/federation 实现）。
type ExternalJWTVerifier interface {
	Verify(tokenString string, trust *domainworkload.FederationTrust) (map[string]interface{}, error)
}

type federationUOWResolver struct {
	runner appuow.TxRunner
}

// ConfigureFederation enables external JWT verification using the UOW-backed
// trust resolver. A nil verifier disables federation resolution.
func (s *Service) ConfigureFederation(verifier ExternalJWTVerifier) {
	s.federationVerifier = verifier
	s.federationResolver = nil
	if verifier == nil || s.UOW == nil {
		return
	}
	s.federationResolver = &federationUOWResolver{runner: s.UOW}
}

func (r *federationUOWResolver) FindActiveTrustsByIssuer(ctx context.Context, issuer string) ([]*domainworkload.FederationTrust, error) {
	if r == nil || r.runner == nil {
		return nil, nil
	}
	var trusts []*domainworkload.FederationTrust
	// This read-only system transaction has one minimum boundary: resolving an
	// issuer to its globally distributed trust candidates. Verification happens
	// after it closes, and token/audit writes keep the caller's organization and
	// original authorization context.
	err := r.runner.WithinTransaction(tenantctx.System(ctx), func(ctx context.Context, tx appuow.UnitOfWork) error {
		if tx.Workload() == nil {
			return apperrors.NewInternal("workload repository is not configured")
		}
		var err error
		trusts, err = tx.Workload().FindActiveTrustsByIssuer(ctx, issuer)
		return err
	})
	if err != nil {
		return nil, err
	}
	return trusts, nil
}

func (s *Service) resolveFederationJWTSubjectWithClaims(
	ctx context.Context,
	organizationID string,
	tokenString string,
	claims map[string]interface{},
	resolver FederationTrustResolver,
	verifier ExternalJWTVerifier,
) (map[string]interface{}, *domainworkload.FederationTrust, error) {
	if resolver == nil || verifier == nil {
		return nil, nil, nil
	}
	issuer, _ := claims["iss"].(string)
	issuer = strings.TrimSpace(issuer)
	if issuer == "" {
		return nil, nil, nil
	}

	trusts, err := resolver.FindActiveTrustsByIssuer(ctx, issuer)
	if err != nil {
		return nil, nil, apperrors.WrapInternalf(err, "lookup federation trust")
	}
	if len(trusts) == 0 {
		return nil, nil, nil
	}
	if organizationID != "" {
		// The issuer is read before signature verification, so the repository must
		// resolve candidates globally. Bind them back to the authenticated caller's
		// organization before trying any keys or accepting any external claims.
		scoped := make([]*domainworkload.FederationTrust, 0, len(trusts))
		for _, trust := range trusts {
			if trust != nil && trust.OrganizationID == organizationID {
				scoped = append(scoped, trust)
			}
		}
		trusts = scoped
		if len(trusts) == 0 {
			return nil, nil, nil
		}
	}

	reason := "no_matching_trust"
	resourceID := issuer
	for _, trust := range trusts {
		if trust == nil {
			reason = "invalid_trust_candidate"
			continue
		}
		if trust.Status != domainworkload.TrustStatusActive {
			reason = "trust_inactive"
			continue
		}
		if !hasUsableFederationAudience(trust.Audience) {
			reason = "audience_empty"
			continue
		}
		if len(trust.Subjects) == 0 {
			reason = "subjects_empty"
			continue
		}

		verifiedClaims, verifyErr := verifier.Verify(tokenString, trust)
		if verifyErr != nil {
			reason = "jwt_verification_failed"
			continue
		}
		subject, _ := verifiedClaims["sub"].(string)
		subject = strings.TrimSpace(subject)
		if subject == "" {
			reason = "subject_missing"
			continue
		}
		if !domainworkload.MatchSubject(trust.Subjects, subject) {
			reason = "subject_not_allowed"
			continue
		}
		return verifiedClaims, trust, nil
	}

	if err := s.recordFederationRejection(ctx, organizationID, issuer, resourceID, reason); err != nil {
		return nil, nil, err
	}
	return nil, nil, apperrors.NewUnauthorized("federation JWT verification failed")
}

func hasUsableFederationAudience(audiences []string) bool {
	if len(audiences) == 0 {
		return false
	}
	for _, audience := range audiences {
		if strings.TrimSpace(audience) == "" {
			return false
		}
	}
	return true
}

func (s *Service) recordFederationRejection(ctx context.Context, organizationID, issuer, resourceID, reason string) error {
	if s.UOW == nil || s.Clock == nil {
		return nil
	}
	// Commit the audit transaction before returning the authentication error;
	// returning that error from inside the closure would roll back both records.
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		return s.embedAuditOutboxPlain(
			tx,
			ctx,
			"",
			"",
			"protocol",
			organizationID,
			"",
			"wli.federation.rejected",
			"federation_trust",
			resourceID,
			"federation_trust",
			"wli.federation.rejected",
			map[string]any{"issuer": issuer, "reason": reason},
		)
	})
	if err != nil {
		return apperrors.WrapInternal(err, "record federation rejection")
	}
	return nil
}

// syntheticFederationToken 把 federation JWT claims 映射到 tokendomain.Token。
// Federation JWT 不在本地 Token 表，所以 Token ID 是合成的 "fed:<sub>"。
func (s *Service) syntheticFederationToken(claims map[string]interface{}) (*tokendomain.Token, error) {
	sub, _ := claims["sub"].(string)
	if strings.TrimSpace(sub) == "" {
		return nil, apperrors.NewUnauthorized("federation JWT missing sub claim")
	}
	var expiresAt time.Time
	if exp, ok := claims["exp"].(float64); ok {
		expiresAt = time.Unix(int64(exp), 0).UTC()
	}
	var scopes []string
	if scopeStr, ok := claims["scope"].(string); ok {
		scopes = strings.Fields(scopeStr)
	}
	var audience []string
	switch aud := claims["aud"].(type) {
	case string:
		audience = []string{aud}
	case []interface{}:
		for _, a := range aud {
			if value, ok := a.(string); ok {
				audience = append(audience, value)
			}
		}
	}
	return &tokendomain.Token{
		ID:        "fed:" + sub,
		UserID:    sub,
		Scopes:    scopes,
		Audience:  audience,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now().UTC(),
		Metadata:  map[string]any{"federation": true},
	}, nil
}
