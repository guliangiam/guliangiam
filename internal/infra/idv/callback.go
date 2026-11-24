package idv

import (
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// jumioCallbackReference extracts the scan reference from a Jumio callback
// body. The reference is the tenant lookup key, not a trusted claim: the caller
// verifies the signature against the owning tenant's webhook secret before
// applying any of the other fields.
func jumioCallbackReference(payload []byte) (string, error) {
	var body struct {
		JumioIDScanReference string `json:"jumioIdScanReference"`
		ScanReference        string `json:"scanReference"`
	}
	if err := decodeJSON(jumioProviderName, payload, &body); err != nil {
		return "", err
	}
	reference := strings.TrimSpace(body.JumioIDScanReference)
	if reference == "" {
		reference = strings.TrimSpace(body.ScanReference)
	}
	if reference == "" {
		return "", apperrors.NewInvalidArgument("jumio: callback is missing the scan reference")
	}
	return reference, nil
}

// aliyunCallbackReference extracts CertifyId from a CloudAuth callback body.
func aliyunCallbackReference(payload []byte) (string, error) {
	var body struct {
		CertifyID string `json:"CertifyId"`
	}
	if err := decodeJSON(aliyunProviderName, payload, &body); err != nil {
		return "", err
	}
	reference := strings.TrimSpace(body.CertifyID)
	if reference == "" {
		return "", apperrors.NewInvalidArgument("aliyun_idv: callback is missing CertifyId")
	}
	return reference, nil
}

// tencentCallbackReference extracts BizToken from a Tencent faceid 核身
// callback body. Tencent pushes no native callback; the reference is the
// tenant lookup key and the caller verifies the gateway signature first.
func tencentCallbackReference(payload []byte) (string, error) {
	var body struct {
		BizToken string `json:"BizToken"`
	}
	if err := decodeJSON(tencentIDVProviderName, payload, &body); err != nil {
		return "", err
	}
	reference := strings.TrimSpace(body.BizToken)
	if reference == "" {
		return "", apperrors.NewInvalidArgument("tencent_idv: callback is missing BizToken")
	}
	return reference, nil
}

// onfidoCallbackReference extracts the workflow-run id from an Onfido webhook
// envelope ({"action":...,"data":{"id":"<run id>",...}}).
func onfidoCallbackReference(payload []byte) (string, error) {
	var body struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := decodeJSON(onfidoProviderName, payload, &body); err != nil {
		return "", err
	}
	reference := strings.TrimSpace(body.Data.ID)
	if reference == "" {
		return "", apperrors.NewInvalidArgument("onfido: callback is missing data.id")
	}
	return reference, nil
}

// veriffCallbackReference extracts the session id from a Veriff decision
// webhook, tolerating both the flat JSON shape and the nested "payload" object
// used by older integrations.
func veriffCallbackReference(payload []byte) (string, error) {
	var body struct {
		ID      string `json:"id"`
		Payload struct {
			ID string `json:"id"`
		} `json:"payload"`
	}
	if err := decodeJSON(veriffProviderName, payload, &body); err != nil {
		return "", err
	}
	reference := strings.TrimSpace(body.ID)
	if reference == "" {
		reference = strings.TrimSpace(body.Payload.ID)
	}
	if reference == "" {
		return "", apperrors.NewInvalidArgument("veriff: callback is missing the session id")
	}
	return reference, nil
}

// personaCallbackReference extracts the inquiry id from a Persona webhook
// envelope (data.relationships.inquiry.data.id).
func personaCallbackReference(payload []byte) (string, error) {
	var body struct {
		Data struct {
			Relationships struct {
				Inquiry struct {
					Data struct {
						ID string `json:"id"`
					} `json:"data"`
				} `json:"inquiry"`
			} `json:"relationships"`
		} `json:"data"`
	}
	if err := decodeJSON(personaProviderName, payload, &body); err != nil {
		return "", err
	}
	reference := strings.TrimSpace(body.Data.Relationships.Inquiry.Data.ID)
	if reference == "" {
		return "", apperrors.NewInvalidArgument("persona: callback is missing the inquiry id")
	}
	return reference, nil
}
