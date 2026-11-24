package auth

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	mw "github.com/guliangiam/guliangiam/internal/platform/middleware"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

func registerFaceidRoutes(router chi.Router, handler Handler) {
	router.Post("/faceid", func(w http.ResponseWriter, r *http.Request) {
		var request InitiateFaceIDRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		result, err := handler.InitiateFaceID(r.Context(), request.toCommand())
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, result)
	})
	router.Post("/faceid/verify", func(w http.ResponseWriter, r *http.Request) {
		var request VerifyFaceIDRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		result, err := handler.VerifyFaceID(r.Context(), request.toCommand())
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, result)
	})
	router.Post("/faceid/complete", func(w http.ResponseWriter, r *http.Request) {
		var request CompleteFaceIDLoginRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		faceidCmd := request.toCommand()
		faceidCmd.PreviousSessionID = ExtractPreviousSessionIDFromRequest(r)
		// 会话元数据由服务端从连接与请求头解析，不接受客户端自报。
		faceidCmd.IPAddress = clientIPFromRequest(r)
		faceidCmd.UserAgent = r.UserAgent()
		response, sessionID, err := handler.CompleteFaceIDLogin(r.Context(), faceidCmd)
		if err != nil {
			mw.RecordLoginError("", "faceid", "login_failed")
			route.WriteError(w, r, err)
			return
		}

		mw.RecordLoginAttempt("", "faceid", "success")

		if sessionID != "" {
			SetOpaqueSessionCookie(w, sessionID)
		}
		route.WriteJSON(w, http.StatusOK, response)
	})
}
