package auth

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	mw "github.com/guliangiam/guliangiam/internal/platform/middleware"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
	"github.com/guliangiam/guliangiam/internal/transport/http/util"
)

func registerPasswordRoutes(router chi.Router, handler Handler) {
	router.With(mw.RateLimit(mw.PasswordResetRateLimit)).Post("/password/reset", func(w http.ResponseWriter, r *http.Request) {
		var request ResetPasswordRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}
		if err := request.Validate(); err != nil {
			route.WriteError(w, r, err)
			return
		}

		cmd := request.toCommand()
		cmd.ClientIP = util.ClientIP(r)
		cmd.PreviousSessionID = ExtractPreviousSessionIDFromRequest(r)
		result, sessionID, err := handler.ResetPassword(r.Context(), cmd)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		if sessionID != "" {
			SetOpaqueSessionCookie(w, sessionID)
		}

		route.WriteJSON(w, http.StatusOK, result)
	})
	router.With(mw.RateLimit(mw.SignupRateLimit)).Post("/signup", func(w http.ResponseWriter, r *http.Request) {
		var request SignupRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}
		if err := request.Validate(); err != nil {
			route.WriteError(w, r, err)
			return
		}

		cmd := request.toCommand()
		cmd.ClientIP = util.ClientIP(r)
		result, err := handler.Signup(r.Context(), cmd)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusCreated, result)
	})
	router.With(mw.RateLimit(mw.SignupRateLimit)).Post("/reset-password", func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			OrganizationSlug string `json:"organizationSlug"`
			Username         string `json:"username"`
			VerificationCode string `json:"verificationCode"`
			NewPassword      string `json:"newPassword"`
			CaptchaToken     string `json:"captchaToken,omitempty"`
			CaptchaID        string `json:"captchaId,omitempty"`
		}
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}
		command := appauth.ResetPasswordCommand{
			OrganizationSlug:  request.OrganizationSlug,
			Username:          request.Username,
			VerificationCode:  request.VerificationCode,
			NewPassword:       request.NewPassword,
			CaptchaToken:      request.CaptchaToken,
			CaptchaID:         request.CaptchaID,
			ClientIP:          util.ClientIP(r),
			PreviousSessionID: ExtractPreviousSessionIDFromRequest(r),
		}
		result, sessionID, err := handler.ResetPassword(r.Context(), command)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		if sessionID != "" {
			SetOpaqueSessionCookie(w, sessionID)
		}
		route.WriteJSON(w, http.StatusOK, result)
	})
	router.Get("/captcha-status", func(w http.ResponseWriter, r *http.Request) {
		orgSlug := r.URL.Query().Get("organizationId")
		result, err := handler.GetCaptchaStatus(r.Context(), orgSlug)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, result)
	})
	router.Get("/captcha", func(w http.ResponseWriter, r *http.Request) {
		orgID := r.URL.Query().Get("organizationId")
		result, err := handler.GenerateCaptcha(r.Context(), orgID)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, result)
	})
	router.Post("/captcha/verify", func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			OrganizationID string `json:"organizationId"`
			CaptchaID      string `json:"captchaId"`
			CaptchaValue   string `json:"captchaValue"`
		}
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}
		// For default provider, token format is "captchaId:captchaValue"
		token := request.CaptchaID + ":" + request.CaptchaValue
		remoteIP := r.RemoteAddr
		result, err := handler.VerifyCaptcha(r.Context(), request.OrganizationID, token, remoteIP)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, result)
	})
	router.Post("/verify-code", func(w http.ResponseWriter, r *http.Request) {
		var request VerifyCodeRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}
		if err := request.Validate(); err != nil {
			route.WriteError(w, r, err)
			return
		}

		if err := handler.VerifyCode(r.Context(), request.toCommand()); err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("ok", "verification code is valid"))
	})
	router.With(mw.RateLimit(mw.PasswordResetRateLimit)).Post("/password/check", func(w http.ResponseWriter, r *http.Request) {
		sessionCtx, ok := SessionContextFromContext(r.Context())
		if !ok {
			route.WriteError(w, r, apperrors.NewUnauthorized("unauthorized"))
			return
		}

		var request CheckPasswordRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		cmd := request.toCommand()
		// Override with session context — caller-supplied org/username ignored.
		// Without this the endpoint is a cross-tenant password-guessing oracle
		// (R2-B02); it mirrors the /password/change guard below.
		cmd.OrganizationSlug = sessionCtx.OrganizationSlug
		cmd.Username = sessionCtx.Username
		result, err := handler.CheckPassword(r.Context(), cmd)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, result)
	})
	router.With(mw.RateLimit(mw.PasswordResetRateLimit)).Post("/password/change", func(w http.ResponseWriter, r *http.Request) {
		sessionCtx, ok := SessionContextFromContext(r.Context())
		if !ok {
			route.WriteError(w, r, apperrors.NewUnauthorized("unauthorized"))
			return
		}

		var request ChangePasswordRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}
		if err := request.Validate(); err != nil {
			route.WriteError(w, r, err)
			return
		}

		cmd := request.toCommand()
		// Override with session context — caller-supplied org/username ignored.
		cmd.OrganizationSlug = sessionCtx.OrganizationSlug
		cmd.Username = sessionCtx.Username
		cmd.PreviousSessionID = ExtractPreviousSessionIDFromRequest(r)
		result, sessionID, err := handler.ChangePassword(r.Context(), cmd)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		if sessionID != "" {
			SetOpaqueSessionCookie(w, sessionID)
		}

		route.WriteJSON(w, http.StatusOK, result)
	})
}
