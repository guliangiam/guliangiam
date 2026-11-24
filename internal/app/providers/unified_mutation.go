package providers

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domainai "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	domainpayment "github.com/guliangiam/guliangiam/internal/domain/payments"
	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	audit "github.com/guliangiam/guliangiam/internal/platform/outbox/providers/audit"
)

func (s *UnifiedService) Patch(ctx context.Context, command PatchProviderCommand) (*Provider, error) {
	if err := validatePatchProviderCommand(command); err != nil {
		return nil, err
	}
	if command.Secret == nil && command.ClientSecret != nil {
		command.Secret = &RotateProviderSecretCommand{SecretType: secretTypeClientSecret, Plaintext: *command.ClientSecret}
	}
	if command.Secret == nil && command.Payment != nil {
		if command.Payment.Config != "" {
			command.Secret = &RotateProviderSecretCommand{SecretType: secretTypeConfig, Plaintext: command.Payment.Config}
		} else if command.Payment.ConfigSealed != "" {
			plaintext, err := s.cipher().DecryptField(command.Payment.ConfigSealed)
			if err != nil {
				return nil, validateConfigSealed()
			}
			command.Secret = &RotateProviderSecretCommand{SecretType: secretTypeConfig, Plaintext: plaintext}
		}
	}
	var updated *Provider
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		provider, err := s.loadProviderTx(ctx, tx, command.ID, command.Family, command.OrganizationID)
		if err != nil {
			return err
		}
		// Ownership is bound by the scoped read; the assertion keeps the
		// not-found shape before revision checks, native detail writes, secret
		// rotation and audit/outbox embeds.
		if err := ensureProviderTenantScope(ctx, provider.Registration.OrganizationID, command.OrganizationID); err != nil {
			return err
		}
		if err := validateDelegatedIdentityPatch(provider, command); err != nil {
			return err
		}
		// 字段取值校验必须在加载持久记录之后：只有本次真正变化的字段才校验，
		// 存量脏值不会阻断无关字段的 PATCH。
		if err := validateProviderPatchWrite(provider, command); err != nil {
			return err
		}
		if command.Revision > 0 && provider.Registration.Revision != command.Revision {
			return apperrors.NewConflict("provider revision mismatch")
		}
		if err := applyProviderPatch(provider, command); err != nil {
			return err
		}
		legacyIdentity := provider.legacy && provider.Registration.Family == domain.FamilyIdentity
		if !legacyIdentity {
			strategy, err := s.strategy(provider.Registration.Family)
			if err != nil {
				return err
			}
			if err := strategy.Validate(provider); err != nil {
				return err
			}
		}
		provider.Registration.UpdatedAt = s.Clock.Now()
		provider.Registration.Revision++
		if command.Secret != nil {
			secretCommand := *command.Secret
			secretCommand.ID = provider.Registration.ID
			secretCommand.Family = string(provider.Registration.Family)
			if secretCommand.SecretType == "" {
				secretCommand.SecretType = defaultSecretType(provider.Registration.Family)
			}
			version, err := s.nextSecretVersion(ctx, tx, provider.Registration.ID, secretCommand.SecretType)
			if err != nil {
				return err
			}
			if err := s.rotateNativeSecret(ctx, tx, provider, secretCommand); err != nil {
				return err
			}
			if err := s.persistSecretStatus(ctx, tx, provider, version, secretCommand.GracePeriod); err != nil {
				return err
			}
		} else if provider.Registration.Family != domain.FamilyIdentity {
			if err := s.updateNativeDetail(ctx, tx, provider); err != nil {
				return err
			}
		}
		if err := s.updateRegistration(ctx, tx, provider); err != nil {
			return err
		}
		if err := s.saveProviderSettings(ctx, tx, provider, true); err != nil {
			return err
		}
		if err := s.embedAudit(ctx, tx, provider, command.ActorID, "updated", nil); err != nil {
			return err
		}
		if command.Secret != nil {
			if err := s.embedAudit(ctx, tx, provider, command.ActorID, "secret.rotated", map[string]any{
				metaKeySecretType: provider.secretType, "fingerprint": provider.fingerprint,
			}); err != nil {
				return err
			}
		}
		updated = provider
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.invalidateProviderCache(updated)
	return updated, nil
}

func (s *UnifiedService) ChangeStatus(ctx context.Context, command ChangeProviderStatusCommand) (*Provider, error) {
	target := domain.Status(command.Status)
	if err := validateChangeStatus(target); err != nil {
		return nil, err
	}
	var changed *Provider
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		provider, err := s.loadProviderTx(ctx, tx, command.ID, command.Family, command.OrganizationID)
		if err != nil {
			return err
		}
		if command.OrganizationID != "" {
			if err := ensureProviderTenantScope(ctx, provider.Registration.OrganizationID, command.OrganizationID); err != nil {
				return err
			}
		}
		if target == domain.StatusActive {
			if err := rejectDelegatedIdentityProvider(provider); err != nil {
				return err
			}
		}
		if command.Revision > 0 && provider.Registration.Revision != command.Revision {
			return apperrors.NewConflict("provider revision mismatch")
		}
		if provider.Registration.Status == target {
			changed = provider
			return nil
		}
		// 显式「启用」允许从 draft / disabled 直接进入 active：探测阶段不再落库状态，
		// 由操作者在探测成功后主动触发。这里补一次 draft|disabled → testing 的合法跃迁，
		// 让状态机与落库结果保持一致，而不是绕过 TransitionTo。
		if target == domain.StatusActive && (provider.Registration.Status == domain.StatusDraft || provider.Registration.Status == domain.StatusDisabled) {
			if err := provider.Registration.TransitionTo(domain.StatusTesting); err != nil {
				return err
			}
		}
		if err := provider.Registration.TransitionTo(target); err != nil {
			return err
		}
		strategy, err := s.strategy(provider.Registration.Family)
		if err != nil {
			return err
		}
		if target == domain.StatusActive {
			if err := strategy.BeforeActivate(ctx, tx, provider); err != nil {
				return err
			}
		}
		now := s.Clock.Now()
		provider.Registration.UpdatedAt = now
		provider.Registration.Revision++
		switch detail := provider.Detail.(type) {
		case *domainpayment.PayChannelConfig:
			if target == domain.StatusActive {
				detail.ActivatedAt = &now
			} else {
				detail.DisabledAt = &now
			}
		case *domainai.LlmProvider:
			detail.UpdatedAt = now
		}
		if provider.Registration.Family != domain.FamilyIdentity {
			if err := s.updateNativeDetail(ctx, tx, provider); err != nil {
				return err
			}
		}
		if err := s.updateRegistration(ctx, tx, provider); err != nil {
			return err
		}
		action := "disabled"
		if target == domain.StatusActive {
			action = "activated"
		}
		if err := s.embedAudit(ctx, tx, provider, command.ActorID, action, map[string]any{metaKeyStatus: target}); err != nil {
			return err
		}
		changed = provider
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.invalidateProviderCache(changed)
	return changed, nil
}

// TestConnection runs the family strategy as a pure probe. It resolves the
// provider through a tenant-scoped read and writes nothing: no lifecycle
// transition, no native detail update, no audit/outbox. Activation is a
// separate, explicit operator action (ChangeStatus).
func (s *UnifiedService) TestConnection(ctx context.Context, id, family, organizationID, actorID string) (*TestConnectionResult, error) {
	var result *TestConnectionResult
	var probeErr error
	metricFamily := "unknown"
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		provider, err := s.loadProviderTx(ctx, tx, id, family, organizationID)
		if err != nil {
			return err
		}
		// Ownership is bound by the scoped read; the assertion keeps the
		// not-found shape before the connection probe and any lifecycle/audit
		// write.
		if err := ensureProviderTenantScope(ctx, provider.Registration.OrganizationID, organizationID); err != nil {
			return err
		}
		if err := rejectDelegatedIdentityProvider(provider); err != nil {
			return err
		}
		metricFamily = string(provider.Registration.Family)
		if detail, ok := provider.Detail.(*domain.Provider); ok &&
			strings.EqualFold(string(detail.Category), string(domain.CategoryIDVerification)) {
			stored, err := tx.Providers().GetConfig(ctx, detail.ID)
			if err != nil {
				return err
			}
			if stored != nil {
				provider.config = stored.Config
			}
		}
		strategy, err := s.strategy(provider.Registration.Family)
		if err != nil {
			return err
		}
		result, probeErr = strategy.TestConnection(ctx, provider)
		if probeErr != nil {
			result = &TestConnectionResult{Connected: false, Error: "provider connection test failed"}
		}
		result.ProviderID = provider.Registration.ID
		// 探测阶段必须只读：不推进生命周期状态、不写 native detail、不产生
		// audit/outbox。启用由独立的 enabled 操作显式完成（见 ChangeStatus）。
		return nil
	})
	if err != nil {
		return nil, err
	}
	connected := result != nil && result.Connected
	recordProviderConnectionTest(metricFamily, connected)
	if !connected {
		slog.WarnContext(ctx, "provider connection test failed", "provider_id", id, "family", metricFamily)
	}
	// The probe is read-only. Only classified retryable failures (upstream 429
	// or 5xx) are raised as errors, so the management face can answer with
	// error.meta.retryable and the upstream Retry-After; every other probe
	// failure keeps the legacy connected=false envelope.
	if retryable, ok := apperrors.IsRetryable(probeErr); ok && retryable {
		return nil, probeErr
	}
	return result, nil
}

func (s *UnifiedService) Delete(ctx context.Context, id, family, organizationID, actorID string) error {
	var deleted *Provider
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		provider, err := s.loadProviderTx(ctx, tx, id, family, organizationID)
		if err != nil {
			return err
		}
		// Ownership is bound by the scoped read; the assertion keeps the
		// not-found shape before the active-status guard, native detail delete
		// and audit/outbox embed.
		if err := ensureProviderTenantScope(ctx, provider.Registration.OrganizationID, organizationID); err != nil {
			return err
		}
		if provider.Registration.Status == domain.StatusActive || provider.Registration.IsRoutable() {
			return apperrors.NewConflict("active provider cannot be deleted; disable it first")
		}
		if provider.Registration.Family != domain.FamilyIdentity {
			if err := s.deleteNativeDetail(ctx, tx, provider); err != nil {
				return err
			}
		}
		if err := tx.Providers().Delete(ctx, provider.Registration.ID); err != nil {
			return err
		}
		deleted = provider
		return s.embedAudit(ctx, tx, provider, actorID, "deleted", nil)
	})
	if err != nil {
		return err
	}
	s.invalidateProviderCache(deleted)
	return nil
}

func (s *UnifiedService) RotateSecret(ctx context.Context, command RotateProviderSecretCommand) (*RotateProviderSecretResult, error) {
	var result *RotateProviderSecretResult
	rotatedOrganizationID := ""
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		provider, err := s.loadProviderTx(ctx, tx, command.ID, command.Family, "")
		if err != nil {
			return err
		}
		if err := rejectDelegatedIdentityProvider(provider); err != nil {
			return err
		}
		rotatedOrganizationID = provider.Registration.OrganizationID
		if command.Revision > 0 && provider.Registration.Revision != command.Revision {
			return apperrors.NewConflict("provider revision mismatch")
		}
		if command.SecretType == "" {
			command.SecretType = defaultSecretType(provider.Registration.Family)
		}
		version, err := s.nextSecretVersion(ctx, tx, provider.Registration.ID, command.SecretType)
		if err != nil {
			return err
		}
		provider.Registration.UpdatedAt = s.Clock.Now()
		provider.Registration.Revision++
		if err := s.rotateNativeSecret(ctx, tx, provider, command); err != nil {
			return err
		}
		if err := s.persistSecretStatus(ctx, tx, provider, version, command.GracePeriod); err != nil {
			return err
		}
		if err := s.updateRegistration(ctx, tx, provider); err != nil {
			return err
		}
		if err := s.embedAudit(ctx, tx, provider, command.ActorID, "secret.rotated", map[string]any{
			metaKeySecretType: provider.secretType, "fingerprint": provider.fingerprint, "version": version,
		}); err != nil {
			return err
		}
		result = &RotateProviderSecretResult{
			ProviderID: provider.Registration.ID, SecretType: provider.secretType,
			SecretSet: true, SecretMasked: maskedCredential(provider.fingerprint),
			Fingerprint: provider.fingerprint, Version: version,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if s.CacheInvalidator != nil && rotatedOrganizationID != "" {
		s.CacheInvalidator.Invalidate(rotatedOrganizationID)
	}
	return result, nil
}

func defaultSecretType(family domain.Family) string {
	switch family {
	case domain.FamilyAI:
		return secretTypeAPIKey
	case domain.FamilyPayment:
		return secretTypeConfig
	case domain.FamilyNotification:
		return secretTypeAccessToken
	case domain.FamilyIdentity:
		return secretTypeClientSecret
	default:
		return secretTypeClientSecret
	}
}

func (s *UnifiedService) embedAudit(ctx context.Context, tx appuow.UnitOfWork, provider *Provider, actorID, action string, payload map[string]any) error {
	if payload == nil {
		payload = map[string]any{}
	}
	eventType := fmt.Sprintf("provider.%s.%s", provider.Registration.Family, action)
	return audit.EmbedAuditEvent(
		s.Clock, tx, ctx,
		"", "", "", "", "",
		provider.Registration.OrganizationID, actorID,
		eventType, "provider", provider.Registration.ID,
		"provider", eventType, payload,
	)
}
