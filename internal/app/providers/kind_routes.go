package providers

import domain "github.com/guliangiam/guliangiam/internal/domain/providers"

// ProviderKindRoute is the runtime view of contracts/registries/provider-kinds.yaml
// for kinds whose subsystem has not landed yet. Every entry stays
// routable=false; the subsystem batch removes it from this table and flips the
// registry to routable=true when its consumer actually lands.
//
// Messaging kinds are no longer listed here: they resolve through
// MessagingResolver, and the kind -> channel table in
// internal/domain/providers/messaging.go is their runtime source.
type ProviderKindRoute struct {
	Kind      domain.Kind
	Subsystem string
	Batch     string
	Routable  bool
}

const (
	subsystemCaptcha = "captcha"
	subsystemFaceID  = "faceid"
	subsystemScan    = "scan"
	subsystemLogs    = "logs"
)

// providerKindRoutes is the pending half of the registry. The check-provider-kinds
// gate holds it equal to the routable=false entries in the YAML.
var providerKindRoutes = []ProviderKindRoute{
	{Kind: identityKindAliyunFaceBody, Subsystem: subsystemFaceID, Batch: "B4", Routable: false},
	{Kind: identityKindWechatScan, Subsystem: subsystemScan, Batch: "B5", Routable: false},
	{Kind: identityKindDingtalkScan, Subsystem: subsystemScan, Batch: "B5", Routable: false},
	{Kind: identityKindElasticsearch, Subsystem: subsystemLogs, Batch: "B6", Routable: false},
	{Kind: identityKindLoki, Subsystem: subsystemLogs, Batch: "B6", Routable: false},
}

// PendingKindRoutes returns a copy of the pending routing table for gates and
// readers that need to inspect the not-yet-wired subsystems.
func PendingKindRoutes() []ProviderKindRoute {
	return append([]ProviderKindRoute(nil), providerKindRoutes...)
}
