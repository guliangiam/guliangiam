package payment

import (
	"strings"

	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// NewGateway creates a PaymentGateway instance based on provider type and configuration.
func NewGateway(providerType string, config map[string]string) (PaymentGateway, error) {
	switch providerType {
	case channelAlipay:
		gw, err := NewAlipayGateway(
			config["appId"],
			config["privateKey"],
			config["publicKey"],
			sandboxEnabled(config),
		)
		if err != nil {
			return nil, apperrors.WrapInternalf(err, "create alipay gateway")
		}
		return gw, nil
	case channelWeChatPay:
		// WeChat Pay APIv3 signs every request with the merchant API certificate
		// private key and verifies responses against the platform certificate
		// bundle (see the official signature-generation doc). Both arrive as PEM
		// strings in the channel config.
		var missing []string
		for key, name := range map[string]string{
			"appId":           "appId",
			"mchId":           "mchId",
			"serialNo":        "serialNo (merchant API certificate serial number)",
			"privateKey":      "privateKey (apiclient_key.pem)",
			"platformCertPEM": "platformCertPEM",
		} {
			if strings.TrimSpace(config[key]) == "" {
				missing = append(missing, name)
			}
		}
		if len(missing) > 0 {
			return nil, apperrors.NewUnavailablef("wechat_pay: missing config: %s", strings.Join(missing, ", "))
		}

		privateKey, err := platformcrypto.ParseRSAPrivateKeyPEM(config["privateKey"])
		if err != nil {
			return nil, apperrors.WrapInvalidArgumentf(err, "wechat_pay privateKey")
		}
		platformCerts, err := platformcrypto.ParseCertificatePEMBundle(config["platformCertPEM"])
		if err != nil {
			return nil, apperrors.WrapInvalidArgumentf(err, "wechat_pay platformCertPEM")
		}
		return NewWeChatPayGateway(
			config["appId"],
			config["mchId"],
			config["serialNo"],
			privateKey,
			platformCerts,
			sandboxEnabled(config),
		)
	case "stripe":
		return NewStripeGateway(config["apiKey"]), nil
	case "paypal":
		return NewPayPalGateway(
			config["clientId"],
			config["clientSecret"],
			sandboxEnabled(config),
		)
	case "balance":
		return NewBalanceGateway(), nil
	case "adyen":
		return NewAdyenGateway(
			config["apiKey"],
			config["merchantId"],
			config["hmacKey"],
			sandboxEnabled(config),
		), nil
	case "airwallex":
		return NewAirwallexGateway(
			config["clientId"],
			config["apiKey"],
			config["webhookSecret"],
			sandboxEnabled(config),
		), nil
	case "checkout":
		secretKey := config["apiKey"]
		if secretKey == "" {
			return nil, apperrors.NewInvalidArgument("checkout: apiKey (secret key) is required")
		}
		return NewCheckoutGateway(secretKey, config["webhookSecret"], sandboxEnabled(config)), nil

	case "square":
		accessToken := config["accessToken"]
		locationID := config["locationId"]
		if accessToken == "" || locationID == "" {
			return nil, apperrors.NewInvalidArgument("square: accessToken and locationId are required")
		}
		return NewSquareGateway(accessToken, locationID, config["webhookKey"], config["apiVersion"], sandboxEnabled(config)), nil

	case "paddle":
		return NewPaddleGateway(
			config["apiToken"],
			config["webhookSecret"],
			sandboxEnabled(config),
		), nil
	case "polar":
		return NewPolarGateway(
			config["accessToken"],
			config["webhookSecret"],
			sandboxEnabled(config),
		), nil
	case "dummy":
		return NewDummyGateway(), nil
	case "gc":
		return NewGCGateway(
			config["xmpch"],
			config["secretKey"],
			config["host"],
		), nil
	case "fastspring":
		return NewFastSpringGateway(
			config["apiUsername"],
			config["apiPassword"],
			config["storefrontPath"],
		), nil
	case "lemonsqueezy":
		return NewLemonSqueezyGateway(
			config["apiKey"],
			config["storeId"],
			config["webhookSecret"],
		), nil
	default:
		return nil, apperrors.NewInvalidArgumentf("unsupported payment provider: %s", providerType)
	}
}

// sandboxEnabled reports whether the provider config points at the sandbox endpoint.
// Provider configs are stored as string maps, so "true" is the only accepted spelling.
func sandboxEnabled(config map[string]string) bool {
	return config["isSandbox"] == "true"
}
