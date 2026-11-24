package providers

import (
	"fmt"
	"strings"

	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
)

// connectionProbeMode describes how a provider's "test connection" is actually
// executed. It is the single source of truth behind the capability decision:
// a provider type without a real probe must never be reported as connected.
type connectionProbeMode string

const (
	connectionProbeHTTP           connectionProbeMode = "http_probe"
	connectionProbeSMTP           connectionProbeMode = "smtp_auth"
	connectionProbeAlibabaSMS     connectionProbeMode = "alibaba_sms_auth_probe"
	connectionProbeIDV            connectionProbeMode = "idv_auth_probe"
	connectionProbeStructuralOnly connectionProbeMode = "structural_only"
)

// identityConnectionProbeMode maps a detail row to the verification mode that
// the identity family can actually perform today.
func identityConnectionProbeMode(p *domain.Provider) connectionProbeMode {
	if p == nil {
		return connectionProbeStructuralOnly
	}
	switch strings.ToLower(strings.TrimSpace(string(p.Category))) {
	case "email":
		// Only the SMTP adapter has a real auth probe; HTTP-API email kinds
		// stay structural-only instead of dialing a nonexistent SMTP host.
		if strings.EqualFold(strings.TrimSpace(string(p.Kind)), "smtp") ||
			canonicalIdentityKind(string(p.ProviderType)) == domain.Kind("smtp") {
			return connectionProbeSMTP
		}
		return connectionProbeStructuralOnly
	case "sms":
		if canonicalIdentityKind(string(p.ProviderType)) == domain.Kind(identityKindAlibabaSMS) {
			return connectionProbeAlibabaSMS
		}
		return connectionProbeStructuralOnly
	case "idverification":
		// A real read-only credential probe runs for the three IDV adapters that
		// actually have a consumer. Unsupported kinds stay structural-only instead
		// of pretending to be connected.
		kind := strings.ToLower(strings.TrimSpace(string(p.Kind)))
		if kind == idvKindJumio || kind == idvKindAliyunIDV || kind == idvKindTencentIDV {
			return connectionProbeIDV
		}
		return connectionProbeStructuralOnly
	}
	if strings.TrimSpace(p.ProviderURL) != "" {
		return connectionProbeHTTP
	}
	return connectionProbeStructuralOnly
}

// validateSMSProviderConfig returns a human-readable configuration problem, or
// an empty string when the required fields are present. It only checks what can
// be checked locally; it never claims that connectivity was verified.
func validateSMSProviderConfig(p *domain.Provider) string {
	if p == nil {
		return "SMS provider detail is missing"
	}
	kind := canonicalIdentityKind(string(p.ProviderType))
	switch kind {
	case domain.Kind(identityKindAlibabaSMS):
		missing := make([]string, 0, 4)
		if strings.TrimSpace(p.Admin) == "" {
			missing = append(missing, "accessKeyId")
		}
		if strings.TrimSpace(p.Password) == "" {
			missing = append(missing, "accessKeySecret")
		}
		if strings.TrimSpace(p.Sender) == "" {
			missing = append(missing, "signName")
		}
		if strings.TrimSpace(p.Template) == "" {
			missing = append(missing, "templateCode")
		}
		if len(missing) > 0 {
			return fmt.Sprintf("alibaba sms configuration is incomplete: %s required", strings.Join(missing, ", "))
		}
		return ""
	case domain.Kind(identityKindMockSMS):
		// The mock provider intentionally has no upstream; it is not a misconfiguration.
		return ""
	case domain.Kind(identityKindCustomHTTPSMS), domain.Kind(identityKindCustomHTTP), domain.Kind(domain.ProviderCustom):
		if strings.TrimSpace(p.Host) == "" {
			return "custom SMS endpoint (host) is required"
		}
		return ""
	case domain.Kind(identityKindUnismsSMS):
		if strings.TrimSpace(p.Admin) == "" || strings.TrimSpace(p.Password) == "" || strings.TrimSpace(p.AppId) == "" || strings.TrimSpace(p.Template) == "" {
			return "unisms MchId (admin), AppKey (password), AppId and TemplateId are required"
		}
		return ""
	case domain.Kind(identityKindYunpianSMS), domain.Kind(identityKindTelnyxSMS), domain.Kind(identityKindBirdSMS), domain.Kind(identityKindClickatellSMS):
		if strings.TrimSpace(p.Password) == "" || strings.TrimSpace(p.Sender) == "" {
			return "SMS API key (password) and sender are required"
		}
		return ""
	case domain.Kind(identityKindJDCloudSMS):
		if strings.TrimSpace(p.Admin) == "" || strings.TrimSpace(p.Password) == "" || strings.TrimSpace(p.Sender) == "" || strings.TrimSpace(p.Template) == "" {
			return "jdcloud accessKey, secretKey, signId and templateId are required"
		}
		return ""
	case domain.Kind(identityKindSinchSMS):
		if strings.TrimSpace(p.Admin) == "" || strings.TrimSpace(p.Password) == "" || strings.TrimSpace(p.AppId) == "" {
			return "sinch keyId, keySecret and projectId are required"
		}
		return ""
	case domain.Kind(identityKindVonageSMS), domain.Kind(identityKindPlivoSMS), domain.Kind(identityKindAfricastalkingSMS):
		if strings.TrimSpace(p.Admin) == "" || strings.TrimSpace(p.Password) == "" || strings.TrimSpace(p.Sender) == "" {
			return "SMS credentials and sender are required"
		}
		return ""
	default:
		if strings.TrimSpace(p.Admin) == "" || strings.TrimSpace(p.Password) == "" {
			return "SMS credentials (accessKeyId/accessKeySecret) are required"
		}
		return ""
	}
}

// validateEmailProviderConfig mirrors validateSMSProviderConfig for the email
// category: it reports locally checkable configuration problems (missing
// per-kind credentials or endpoints) and never claims connectivity. Field
// naming follows the infra builder's column mapping (admin/password/appId/
// sender/host/region), one entry per kind in the email closed set.
func validateEmailProviderConfig(p *domain.Provider) string {
	if p == nil {
		return "email provider detail is missing"
	}
	kind := canonicalIdentityKind(string(p.ProviderType))
	if kind == domain.Kind(domain.ProviderCustom) {
		// providerType 写法不在闭集时回退到行上的 kind 列。
		kind = domain.Kind(strings.ToLower(strings.TrimSpace(string(p.Kind))))
	}
	switch kind {
	case domain.Kind("smtp"):
		if strings.TrimSpace(p.Host) == "" {
			return "smtp host is required"
		}
		return ""
	case domain.Kind("sendgrid"), domain.Kind("resend"),
		domain.Kind(identityKindMailtrapEmail), domain.Kind(identityKindPostmark),
		domain.Kind(identityKindMailerSend), domain.Kind(identityKindBrevo):
		if strings.TrimSpace(p.Password) == "" {
			return "email API key (password) is required"
		}
		return ""
	case domain.Kind(identityKindAzureACS):
		if strings.TrimSpace(p.Host) == "" {
			return "azure acs endpoint (host) is required"
		}
		if strings.TrimSpace(p.Password) == "" {
			return "azure acs apiKey (password) is required"
		}
		return ""
	case domain.Kind(identityKindSubmailEmail):
		if strings.TrimSpace(p.AppId) == "" || strings.TrimSpace(p.Password) == "" {
			return "submail email appId and appKey (password) are required"
		}
		return ""
	case domain.Kind(identityKindCustomHTTP):
		if strings.TrimSpace(p.Host) == "" {
			return "custom http email endpoint (host) is required"
		}
		return ""
	case domain.Kind(identityKindDirectMail):
		if strings.TrimSpace(p.Admin) == "" || strings.TrimSpace(p.Password) == "" || strings.TrimSpace(p.Sender) == "" {
			return "directmail accessKeyId (admin), accessKeySecret (password) and accountName (sender) are required"
		}
		return ""
	case domain.Kind(identityKindTencentSES):
		if strings.TrimSpace(p.Admin) == "" || strings.TrimSpace(p.Password) == "" || strings.TrimSpace(p.Sender) == "" {
			return "tencent ses SecretId (admin), SecretKey (password) and sender address are required"
		}
		return ""
	case domain.Kind(identityKindSendCloud):
		if strings.TrimSpace(p.Admin) == "" || strings.TrimSpace(p.Password) == "" || strings.TrimSpace(p.Sender) == "" {
			return "sendcloud apiUser (admin), apiKey (password) and from address (sender) are required"
		}
		return ""
	case domain.Kind(identityKindMailgun):
		if strings.TrimSpace(p.Password) == "" {
			return "mailgun private API key (password) is required"
		}
		if strings.TrimSpace(p.Host) == "" {
			return "mailgun sending domain (host) is required"
		}
		return ""
	case domain.Kind(identityKindAmazonSES):
		if strings.TrimSpace(p.Admin) == "" || strings.TrimSpace(p.Password) == "" {
			return "amazon ses accessKeyId (admin) and secretAccessKey (password) are required"
		}
		if strings.TrimSpace(p.Region) == "" {
			return "amazon ses region is required"
		}
		return ""
	case domain.Kind(identityKindZohoMail):
		if strings.TrimSpace(p.Password) == "" {
			return "zoho mail OAuth access token (password) is required"
		}
		if strings.TrimSpace(p.AppId) == "" {
			return "zoho mail accountId (appId) is required"
		}
		return ""
	default:
		if strings.TrimSpace(p.Password) == "" {
			return "email credentials (password) are required"
		}
		return ""
	}
}
