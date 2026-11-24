package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"strings"

	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	paymentsdomain "github.com/guliangiam/guliangiam/internal/domain/payments"
	"github.com/guliangiam/guliangiam/internal/domain/providers"
	infrafaceid "github.com/guliangiam/guliangiam/internal/infra/faceid"
	"github.com/guliangiam/guliangiam/internal/infra/idp"
	infraldap "github.com/guliangiam/guliangiam/internal/infra/ldap"
	infrapayment "github.com/guliangiam/guliangiam/internal/infra/payment"
	postgresuow "github.com/guliangiam/guliangiam/internal/infra/postgres/uow"
	"github.com/guliangiam/guliangiam/internal/infra/saml"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// exchangeSocialCodeWithIDP returns the adapter that wires the app layer's
// social login to the actual IDP infrastructure. This function satisfies the
// appauth.ExchangeSocialCodeFunc signature and is set via
// appauth.SetSocialLoginAdapters during bootstrap. replayStore carries the
// process/shared SAML Response-ID consumption state.
func exchangeSocialCodeWithIDP(replayStore saml.ReplayStore) appauth.ExchangeSocialCodeFunc {
	return func(ctx context.Context, config appauth.SocialProviderConfig, code string) (*appauth.SocialUserInfo, error) {
		factory := idp.NewFactory()
		domainProvider := socialDomainProvider(config)

		idpProvider, err := factory.GetProviderWithOptions(domainProvider, idp.ProviderOptions{
			CodeVerifier:         config.CodeVerifier,
			SAMLState:            config.State,
			SAMLReplayStore:      replayStore,
			SAMLReplayOrgID:      config.OrganizationID,
			SAMLReplayProviderID: config.ProviderID,
		})
		if err != nil {
			return nil, apperrors.WrapInternalf(err, "create idp provider")
		}
		if binder, ok := idpProvider.(idp.NonceBinder); ok && strings.TrimSpace(config.Nonce) != "" {
			binder.SetExpectedNonce(config.Nonce)
		}

		oauthToken, err := idpProvider.ExchangeCode(ctx, code)
		if err != nil {
			return nil, err
		}

		userInfo, err := idpProvider.GetUserInfo(ctx, oauthToken)
		if err != nil {
			// A rejected nonce is an authentication failure, not an internal
			// error; keep its 401 semantics so the app layer reports it.
			var appErr *apperrors.Error
			if errors.As(err, &appErr) && appErr.Code == apperrors.CodeUnauthorized {
				return nil, err
			}
			return nil, apperrors.WrapInternalf(err, "get user info")
		}

		return &appauth.SocialUserInfo{
			ID:           userInfo.ID,
			Username:     userInfo.Username,
			DisplayName:  userInfo.DisplayName,
			Email:        userInfo.Email,
			Phone:        userInfo.Phone,
			AvatarURL:    userInfo.AvatarURL,
			AuthnContext: userInfo.AuthnContext,
			AccessToken:  oauthToken.AccessToken,
		}, nil
	}
}

// getSocialRedirectURLWithIDP is the adapter that generates the redirect URL
// for a social login provider using the IDP infrastructure.
func getSocialRedirectURLWithIDP(ctx context.Context, config appauth.SocialProviderConfig, state string) (string, error) {
	factory := idp.NewFactory()
	domainProvider := socialDomainProvider(config)

	idpProvider, err := factory.GetProvider(domainProvider)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "create idp provider")
	}

	options := idp.AuthorizationRequestOptions{
		Nonce:               config.Nonce,
		CodeChallenge:       config.CodeChallenge,
		CodeChallengeMethod: config.CodeChallengeMethod,
	}
	if requestProvider, ok := idpProvider.(idp.AuthorizationRequestProvider); ok && (options.Nonce != "" || options.CodeChallenge != "") {
		return requestProvider.GetRedirectURLWithOptions(state, options)
	}
	return idpProvider.GetRedirectURL(state)
}

// socialDomainProvider maps the app-layer config back to the domain row shape
// the IDP factory reads. Without the auth/token/user-info/issuer fields,
// host-based adapters (Okta/Auth0/Casdoor) built relative URLs. The SAML
// columns (SP entity ID, SSO URL, certificate, metadata) must survive the
// round-trip or the SP adapter cannot enforce audience/recipient/signature.
func socialDomainProvider(config appauth.SocialProviderConfig) *providers.Provider {
	return &providers.Provider{
		Category:     providers.ProviderCategory(config.Category),
		ProviderType: providers.ProviderType(config.ProviderType),
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		ProviderURL:  config.RedirectURI,
		CallbackUrl:  config.RedirectURI,
		IssuerUrl:    config.IssuerURL,
		AuthUrl:      config.AuthURL,
		TokenUrl:     config.TokenURL,
		UserInfoUrl:  config.UserInfoURL,
		Host:         config.Host,
		Scopes:       append([]string{}, config.Scopes...),
		EnablePkce:   config.EnablePkce,
		EnableProxy:  config.EnableProxy,
		HttpHeaders:  config.HTTPHeaders,
		IdPEntityId:  config.IdPEntityID,
		SsoUrl:       config.SsoURL,
		Cert:         config.SAMLCert,
		Metadata:     config.SAMLMetadata,
	}
}

// ldapBindAndSearch is the adapter that connects the app layer's LDAP login
// to the actual LDAP client infrastructure. It performs a user bind + search
// to authenticate and retrieve user info from an external LDAP directory.
func ldapBindAndSearch(ctx context.Context, host string, port int, bindDN, bindPassword, baseDN string, useTLS bool, username, password, filter, attributeName string) (*appauth.LdapUserInfo, error) {
	client := infraldap.NewClient()

	// Step 1: Connect with the service account (bindDN) to search for the user.
	conn, err := client.Connect(host, port, bindDN, bindPassword, useTLS)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "LDAP service bind failed")
	}
	defer conn.Close()

	// Step 2: Build the search filter to find the user.
	searchFilter := fmt.Sprintf("(%s=%s)", attributeName, username)
	if filter != "" {
		searchFilter = fmt.Sprintf("(&%s%s)", filter, searchFilter)
	}

	// Step 3: Search for the user entry.
	users, err := client.SearchUsers(conn, baseDN, searchFilter, nil)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "LDAP user search failed")
	}
	if len(users) == 0 {
		return nil, apperrors.NewNotFoundf("LDAP user not found: %s", username)
	}

	ldapUser := users[0]

	// Step 4: Re-bind as the user to verify their password.
	userConn, err := client.Connect(host, port, ldapUser.DN, password, useTLS)
	if err != nil {
		return nil, apperrors.NewInvalidArgument("LDAP user bind failed: invalid credentials")
	}
	userConn.Close()

	// Step 5: Return the user info.
	return &appauth.LdapUserInfo{
		UID:         ldapUser.UID,
		CommonName:  ldapUser.CommonName,
		Email:       ldapUser.Email,
		Phone:       ldapUser.Telephone,
		Mobile:      ldapUser.Mobile,
		Department:  ldapUser.Department,
		Title:       ldapUser.Title,
		EmployeeNum: ldapUser.EmployeeNumber,
		DN:          ldapUser.DN,
	}, nil
}

// createPaymentWithGateway is the adapter that wires the app layer's order service
// to the actual payment gateway infrastructure. It creates a payment request with
// the payment provider and returns the payment URL and trade number.
func createPaymentWithGateway(ctx context.Context, orderID string, amount float64, currency, subject, notifyURL, returnUrl string) (paymentURL, tradeNo string, err error) {
	providerType := envOrEmpty("PAYMENT_PROVIDER")
	if providerType == "" {
		return "", "", apperrors.NewUnavailable("payment gateway not configured: PAYMENT_PROVIDER env var not set")
	}

	config := map[string]string{
		"appId":                envOrEmpty("PAYMENT_APP_ID"),
		"privateKey":           envOrEmpty("PAYMENT_PRIVATE_KEY"),
		"publicKey":            envOrEmpty("PAYMENT_PUBLIC_KEY"),
		"mchId":                envOrEmpty("PAYMENT_MCH_ID"),
		paymentConfigAPIKey:    envOrEmpty("PAYMENT_API_KEY"),
		paymentConfigIsSandbox: envOrEmpty("PAYMENT_IS_SANDBOX"),
	}

	gateway, err := infrapayment.NewGateway(providerType, config)
	if err != nil {
		return "", "", apperrors.WrapInternalf(err, "create payment gateway")
	}

	result, err := gateway.CreatePayment(ctx, orderID, amount, currency, subject, notifyURL, returnUrl)
	if err != nil {
		return "", "", apperrors.WrapInternalf(err, "create payment")
	}

	return result.PaymentURL, result.TradeNo, nil
}

// queryPaymentWithGateway is the adapter that wires the app layer's order service
// to the actual payment gateway infrastructure for querying payment status.
func queryPaymentWithGateway(ctx context.Context, outTradeNo string) (paymentsdomain.Outcome, error) {
	providerType := envOrEmpty("PAYMENT_PROVIDER")
	if providerType == "" {
		return paymentsdomain.OutcomeUnknown, apperrors.NewUnavailable("payment gateway not configured: PAYMENT_PROVIDER env var not set")
	}

	config := map[string]string{
		"appId":                envOrEmpty("PAYMENT_APP_ID"),
		"privateKey":           envOrEmpty("PAYMENT_PRIVATE_KEY"),
		"publicKey":            envOrEmpty("PAYMENT_PUBLIC_KEY"),
		"mchId":                envOrEmpty("PAYMENT_MCH_ID"),
		paymentConfigAPIKey:    envOrEmpty("PAYMENT_API_KEY"),
		paymentConfigIsSandbox: envOrEmpty("PAYMENT_IS_SANDBOX"),
	}

	gateway, err := infrapayment.NewGateway(providerType, config)
	if err != nil {
		return paymentsdomain.OutcomeUnknown, apperrors.WrapInternalf(err, "create payment gateway")
	}

	result, err := gateway.QueryPayment(ctx, outTradeNo)
	if err != nil {
		return paymentsdomain.OutcomeUnknown, apperrors.WrapInternalf(err, "query payment")
	}

	// The channel speaks its own status vocabulary; this boundary is the only
	// place that knows it, so the translation happens here and the app layer
	// receives a domain Outcome.
	return paymentsdomain.TranslateTradeStatus(providerType, result.Status), nil
}

// newFaceVerifier selects the FaceVerifier for a provider row. The Local
// UniFace service performs comparison only; its adapter fails closed for
// liveness. Unknown/legacy types keep the Aliyun implementation so existing
// rows are unaffected.
func newFaceVerifier(providerType, accessKeyID, accessKeySecret, endpoint string) infrafaceid.FaceVerifier {
	if isLocalUniFaceType(providerType) {
		return infrafaceid.NewLocalUniFaceVerifier(endpoint, accessKeySecret)
	}
	return infrafaceid.NewAliyunVerifier(accessKeyID, accessKeySecret, endpoint)
}

func isLocalUniFaceType(providerType string) bool {
	normalized := strings.ToLower(strings.NewReplacer(" ", "", "-", "", "_", "").Replace(providerType))
	return normalized == "localuniface" || normalized == "uniface"
}

// verifyFaceIdLiveness is the adapter that wires the app layer's Face ID login
// to the actual FaceVerifier infrastructure.
func verifyFaceIdLiveness(ctx context.Context, providerType, accessKeyID, accessKeySecret, endpoint, imageData, deviceInfo string) (isLive bool, score float64, sessionID string, err error) {
	verifier := newFaceVerifier(providerType, accessKeyID, accessKeySecret, endpoint)
	result, err := verifier.VerifyLiveness(ctx, imageData, deviceInfo)
	if err != nil {
		return false, 0, "", err
	}
	return result.IsLive, result.Score, result.SessionID, nil
}

// compareFaceIdWithTemplate is the adapter that wires the app layer's Face ID
// login to the actual FaceVerifier infrastructure.
func compareFaceIdWithTemplate(ctx context.Context, providerType, accessKeyID, accessKeySecret, endpoint, imageData, storedTemplate string) (matched bool, score float64, err error) {
	verifier := newFaceVerifier(providerType, accessKeyID, accessKeySecret, endpoint)
	result, err := verifier.CompareFace(ctx, imageData, storedTemplate)
	if err != nil {
		return false, 0, err
	}
	return result.IsMatch, result.Confidence, nil
}

var (
	_ appuow.UnitOfWork  = (*postgresuow.UnitOfWork)(nil)
	_ appuow.IdentityUoW = (*postgresuow.UnitOfWork)(nil)
	_ appuow.AuthUoW     = (*postgresuow.UnitOfWork)(nil)
	_ appuow.AuthnUoW    = (*postgresuow.UnitOfWork)(nil)
	_ appuow.AuditUoW    = (*postgresuow.UnitOfWork)(nil)
	_ appuow.CommerceUoW = (*postgresuow.UnitOfWork)(nil)
	_ appuow.InfraUoW    = (*postgresuow.UnitOfWork)(nil)
	_ appuow.UnitOfWork  = postgresuow.UnitOfWork{}
	_ appuow.IdentityUoW = postgresuow.UnitOfWork{}
	_ appuow.AuthUoW     = postgresuow.UnitOfWork{}
	_ appuow.AuthnUoW    = postgresuow.UnitOfWork{}
	_ appuow.AuditUoW    = postgresuow.UnitOfWork{}
	_ appuow.CommerceUoW = postgresuow.UnitOfWork{}
	_ appuow.InfraUoW    = postgresuow.UnitOfWork{}
)
