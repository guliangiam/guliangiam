package cas

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
)

// Handler handles CAS protocol endpoints.
type Handler struct {
	AuthService *appauth.Service
}

// RegisterChiRoutes registers CAS protocol routes.
func RegisterChiRoutes(router chi.Router, handler *Handler) {
	// CAS 1.0
	router.Get("/cas/{organizationSlug}/{applicationSlug}/validate", func(w http.ResponseWriter, r *http.Request) {
		handler.casValidate(w, r)
	})

	// CAS 2.0
	router.Get("/cas/{organizationSlug}/{applicationSlug}/serviceValidate", func(w http.ResponseWriter, r *http.Request) {
		handler.casServiceValidate(w, r)
	})
	router.Get("/cas/{organizationSlug}/{applicationSlug}/proxyValidate", func(w http.ResponseWriter, r *http.Request) {
		handler.casProxyValidate(w, r)
	})
	router.Get("/cas/{organizationSlug}/{applicationSlug}/proxy", func(w http.ResponseWriter, r *http.Request) {
		handler.casProxy(w, r)
	})

	// CAS 3.0
	router.Get("/cas/{organizationSlug}/{applicationSlug}/p3/serviceValidate", func(w http.ResponseWriter, r *http.Request) {
		handler.casP3ServiceValidate(w, r)
	})
	router.Get("/cas/{organizationSlug}/{applicationSlug}/p3/proxyValidate", func(w http.ResponseWriter, r *http.Request) {
		handler.casP3ProxyValidate(w, r)
	})

	// SAML 1.1
	router.Post("/cas/{organizationSlug}/{applicationSlug}/samlValidate", func(w http.ResponseWriter, r *http.Request) {
		handler.samlValidate(w, r)
	})
}

func (h Handler) casValidate(w http.ResponseWriter, r *http.Request) {
	ticket := strings.TrimSpace(r.URL.Query().Get("ticket"))
	service := strings.TrimSpace(r.URL.Query().Get("service"))

	if ticket == "" || service == "" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "no\n\n")
		return
	}

	orgSlug := chi.URLParam(r, "organizationSlug")
	appSlug := chi.URLParam(r, "applicationSlug")

	result, err := h.AuthService.CASValidate(r.Context(), appauth.CASServiceValidateCommand{
		OrganizationSlug: orgSlug,
		ApplicationSlug:  appSlug,
		Ticket:           ticket,
		Service:          service,
	})
	if err != nil || !result.Success {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "no\n\n")
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "yes\n%s\n", result.Username)
}

func (h Handler) casServiceValidate(w http.ResponseWriter, r *http.Request) {
	h.casValidateXML(w, r, true)
}

func (h Handler) casProxyValidate(w http.ResponseWriter, r *http.Request) {
	h.casValidateXML(w, r, false)
}

func (h Handler) casP3ServiceValidate(w http.ResponseWriter, r *http.Request) {
	h.casValidateXML(w, r, true)
}

func (h Handler) casP3ProxyValidate(w http.ResponseWriter, r *http.Request) {
	h.casValidateXML(w, r, false)
}

func (h Handler) casValidateXML(w http.ResponseWriter, r *http.Request, requireST bool) {
	ticket := strings.TrimSpace(r.URL.Query().Get("ticket"))
	service := strings.TrimSpace(r.URL.Query().Get("service"))
	format := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("format")))

	if ticket == "" || service == "" {
		writeCASError(w, "INVALID_REQUEST", "ticket and service parameters are required", format)
		return
	}

	orgSlug := chi.URLParam(r, "organizationSlug")
	appSlug := chi.URLParam(r, "applicationSlug")

	result, err := h.AuthService.CASServiceValidate(r.Context(), appauth.CASServiceValidateCommand{
		OrganizationSlug: orgSlug,
		ApplicationSlug:  appSlug,
		Ticket:           ticket,
		Service:          service,
	})
	if err != nil || !result.Success {
		writeCASError(w, "INVALID_TICKET", fmt.Sprintf("Ticket %s not recognized", ticket), format)
		return
	}

	writeCASSuccess(w, result, format)
}

func (h Handler) casProxy(w http.ResponseWriter, r *http.Request) {
	pgt := strings.TrimSpace(r.URL.Query().Get("pgt"))
	targetService := strings.TrimSpace(r.URL.Query().Get("targetService"))
	format := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("format")))

	if pgt == "" || targetService == "" {
		writeCASError(w, "INVALID_REQUEST", "pgt and targetService parameters are required", format)
		return
	}

	// Validate the PGT and generate a proxy ticket.
	proxyTicket, err := h.AuthService.CASGenerateProxyTicket(r.Context(), pgt, targetService)
	if err != nil {
		writeCASError(w, "UNAUTHORIZED_SERVICE", err.Error(), format)
		return
	}

	writeCASProxySuccess(w, proxyTicket, format)
}

func (h Handler) samlValidate(w http.ResponseWriter, r *http.Request) {
	// CAS SAML 1.1 validation endpoint (POST with SOAP body).
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeCASError(w, "INVALID_REQUEST", "failed to read request body", "XML")
		return
	}
	defer r.Body.Close()

	ticket := extractSAMLTicket(string(body))
	service := strings.TrimSpace(r.URL.Query().Get("TARGET"))
	if service == "" {
		service = strings.TrimSpace(r.URL.Query().Get("target"))
	}

	if ticket == "" || service == "" {
		writeSAMLFailure(w, "INVALID_REQUEST", "ticket and service parameters are required")
		return
	}

	orgSlug := chi.URLParam(r, "organizationSlug")
	appSlug := chi.URLParam(r, "applicationSlug")

	result, err := h.AuthService.CASServiceValidate(r.Context(), appauth.CASServiceValidateCommand{
		OrganizationSlug: orgSlug,
		ApplicationSlug:  appSlug,
		Ticket:           ticket,
		Service:          service,
	})
	if err != nil || !result.Success {
		writeSAMLFailure(w, "INVALID_TICKET", fmt.Sprintf("Ticket %s not recognized", ticket))
		return
	}

	writeSAMLSuccess(w, result)
}

func writeCASSuccess(w http.ResponseWriter, result *appauth.CASValidateResult, format string) {
	if format == "JSON" {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(map[string]any{
			"serviceResponse": map[string]any{
				"authenticationSuccess": map[string]any{
					"user":       result.Username,
					"attributes": result.Attrs,
				},
			},
		})
		return
	}

	data := appauth.BuildCASXMLSuccessResponse(result.Username, result.UserID, result.Attrs)
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.Write(data)
}

func writeCASError(w http.ResponseWriter, code, message, format string) {
	if format == "JSON" {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(map[string]any{
			"serviceResponse": map[string]any{
				"authenticationFailure": map[string]any{
					"code":    code,
					"message": message,
				},
			},
		})
		return
	}

	data := appauth.BuildCASXMLFailureResponse(code, message)
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.Write(data)
}

func writeCASProxySuccess(w http.ResponseWriter, proxyTicket string, format string) {
	if format == "JSON" {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(map[string]any{
			"serviceResponse": map[string]any{
				"proxySuccess": map[string]any{
					"proxyTicket": proxyTicket,
				},
			},
		})
		return
	}

	response := appauth.CASServiceResponse{
		XMLNS: "http://www.yale.edu/tp/cas",
		ProxySuccess: &appauth.CASProxySuccess{
			ProxyTicket: proxyTicket,
		},
	}
	data, _ := xml.Marshal(response)
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.Write(data)
}

// extractSAMLTicket extracts the artifact (ticket) from a SAML 1.1 SOAP request.
func extractSAMLTicket(body string) string {
	// Look for <samlp:Artifact> content in the SOAP body.
	// Simple extraction without full XML parsing.
	startTag := "<samlp:Artifact>"
	endTag := "</samlp:Artifact>"
	start := strings.Index(body, startTag)
	if start == -1 {
		// Also try without namespace prefix.
		startTag = "<Artifact>"
		endTag = "</Artifact>"
		start = strings.Index(body, startTag)
	}
	if start == -1 {
		return ""
	}
	start += len(startTag)
	end := strings.Index(body[start:], endTag)
	if end == -1 {
		return ""
	}
	return strings.TrimSpace(body[start : start+end])
}

// writeSAMLSuccess writes a CAS SAML 1.1 success response.
func writeSAMLSuccess(w http.ResponseWriter, result *appauth.CASValidateResult) {
	now := time.Now().UTC()
	notBefore := now.Format(time.RFC3339)
	notOnOrAfter := now.Add(5 * time.Minute).Format(time.RFC3339)

	response := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<SOAP-ENV:Envelope xmlns:SOAP-ENV="http://schemas.xmlsoap.org/soap/envelope/">
<SOAP-ENV:Body>
<samlp:Response xmlns:samlp="urn:oasis:names:tc:SAML:1.0:protocol" MajorVersion="1" MinorVersion="1" ResponseID="%s" IssueInstant="%s">
<samlp:Status><samlp:StatusCode Value="samlp:Success"/></samlp:Status>
<samlp:Assertion xmlns:saml="urn:oasis:names:tc:SAML:1.0:assertion" MajorVersion="1" MinorVersion="1" AssertionID="%s" Issuer="guliangiam" IssueInstant="%s" NotBefore="%s" NotOnOrAfter="%s">
<saml:Conditions NotBefore="%s" NotOnOrAfter="%s"/>
<saml:AuthenticationStatement AuthenticationMethod="urn:oasis:names:tc:SAML:1.0:am:password" AuthenticationInstant="%s">
<saml:Subject><saml:NameIdentifier>%s</saml:NameIdentifier></saml:Subject>
</saml:AuthenticationStatement>
</samlp:Assertion>
</samlp:Response>
</SOAP-ENV:Body>
</SOAP-ENV:Envelope>`,
		result.UserID,
		now.Format(time.RFC3339),
		result.UserID+"-assertion",
		now.Format(time.RFC3339),
		notBefore, notOnOrAfter,
		notBefore, notOnOrAfter,
		now.Format(time.RFC3339),
		result.Username,
	)

	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(response))
}

// writeSAMLFailure writes a CAS SAML 1.1 failure response.
func writeSAMLFailure(w http.ResponseWriter, code, message string) {
	now := time.Now().UTC().Format(time.RFC3339)

	response := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<SOAP-ENV:Envelope xmlns:SOAP-ENV="http://schemas.xmlsoap.org/soap/envelope/">
<SOAP-ENV:Body>
<samlp:Response xmlns:samlp="urn:oasis:names:tc:SAML:1.0:protocol" MajorVersion="1" MinorVersion="1" ResponseID="failure" IssueInstant="%s">
<samlp:Status><samlp:StatusCode Value="samlp:RequestDenied"/></samlp:Status>
</samlp:Response>
</SOAP-ENV:Body>
</SOAP-ENV:Envelope>`, now)

	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(response))
}
