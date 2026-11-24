package auth

import (
	"html/template"
	"net/http"

	"github.com/go-chi/chi/v5"

	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// frontChannelPage renders the hidden iframes that notify relying parties of a
// logout (OpenID Connect Front-Channel Logout 1.0 §2). The page is same-origin
// with the console and only rendered after the OP session is revoked.
var frontChannelPage = template.Must(template.New("front-channel-logout").Parse(
	`<!DOCTYPE html><html lang="en"><head><meta charset="utf-8"><title>Logged out</title></head>
<body>{{range .Targets}}<iframe title="front-channel logout" style="width:0;height:0;border:0" src="{{.URL}}"></iframe>{{end}}
<p>Logged out.</p></body></html>`))

func registerFrontChannelRoutes(router chi.Router, handler Handler) {
	router.Get("/frontchannel-logout", func(w http.ResponseWriter, r *http.Request) {
		sessionID, ok := OpaqueSessionIDFromRequest(r)
		if !ok {
			route.WriteError(w, r, apperrors.NewUnauthorized("session id is required"))
			return
		}
		result, err := handler.BuildFrontChannelLogout(r.Context(), sessionID, "")
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		// Revoke the OP session after targets are collected; this also triggers
		// the existing back-channel notifications (ADR-013).
		if err := handler.Logout(r.Context(), appauth.LogoutCommand{SessionID: sessionID}); err != nil {
			route.WriteError(w, r, err)
			return
		}
		ClearSessionCookie(w)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		_ = frontChannelPage.Execute(w, result)
	})
}
