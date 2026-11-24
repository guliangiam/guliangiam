package bootstrap

import (
	"context"

	appidv "github.com/guliangiam/guliangiam/internal/app/idv"
	appproviders "github.com/guliangiam/guliangiam/internal/app/providers"
	domainproviders "github.com/guliangiam/guliangiam/internal/domain/providers"
	infraidv "github.com/guliangiam/guliangiam/internal/infra/idv"
)

// wireIDV connects the IDV use cases to the tenant provider resolver, the real
// HTTP adapter factory, the user-facing initiation gateway and the read-only
// connection probe. Nothing here simulates a provider.
func wireIDV(services *AppServices, idvService *appidv.Service) {
	idvService.Providers = services.ProviderService
	factory := infraidv.NewFactory()
	idvService.Factory = factory
	idvService.OCRFactory = factory
	// 认证终态结果（通过/拒绝/过期）同步进本人通知收件箱，provider 回调失败
	// 不影响已提交的认证结果。
	idvService.Notifier = newIDVInboxNotifier(services.UserNotificationService)

	services.UserService.IdentityVerification = idvUserAdapter{idv: idvService}
	services.UserService.DocumentRecognizer = idvDocumentRecognizer{idv: idvService}

	if services.ProviderService.Unified == nil {
		return
	}
	services.ProviderService.Unified.IDVProbe = func(ctx context.Context, provider *domainproviders.Provider, config map[string]any) error {
		credentials, err := appproviders.CredentialsFromProviderConfig(provider, config)
		if err != nil {
			return err
		}
		verifier, err := factory.Build(*credentials)
		if err != nil {
			return err
		}
		return verifier.Probe(ctx)
	}
}
