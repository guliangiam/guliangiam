package auth

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

func registerQrcodeRoutes(router chi.Router, handler Handler) {
	router.Post("/qrcode", func(w http.ResponseWriter, r *http.Request) {
		var request CreateQRCodeRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		result, err := handler.CreateQRCode(r.Context(), request.toCommand())
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusCreated, result)
	})
	router.Post("/qrcode/scan", func(w http.ResponseWriter, r *http.Request) {
		var request ScanQRCodeRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		sessionUsername, sessionOrganizationID := sessionSubjectFromRequest(r)
		result, sessionID, err := handler.ScanQRCode(
			r.Context(),
			request.toCommand(sessionUsername, sessionOrganizationID),
		)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		// 匿名扫码自动注册时扫码端本来没有会话：不种 cookie 它无法完成 confirm。
		if sessionID != "" {
			SetOpaqueSessionCookie(w, sessionID)
		}

		route.WriteJSON(w, http.StatusOK, result)
	})
	router.Post("/qrcode/confirm", func(w http.ResponseWriter, r *http.Request) {
		var request ConfirmQRCodeRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		// 确认者身份取自会话（cookie 或 Bearer JWT 都经 SessionContextMiddleware
		// 注入），不再用只认 JWT 的 sessionContextFromHeaders —— 否则 cookie
		// 会话的手机端永远匹配不上扫码身份。
		sessionUsername, sessionOrganizationID := sessionSubjectFromRequest(r)
		if err := handler.ConfirmQRCode(
			r.Context(),
			request.toCommand(sessionUsername, sessionOrganizationID),
		); err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("ok", "operation succeeded"))
	})
	router.Get("/qrcode/{codeId}/status", func(w http.ResponseWriter, r *http.Request) {
		codeID := chi.URLParam(r, "codeId")

		result, err := handler.PollQRCodeStatus(r.Context(), codeID)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, result)
	})
	router.Post("/qrcode/exchange", func(w http.ResponseWriter, r *http.Request) {
		var request ExchangeQRCodeRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		command := request.toCommand()
		command.PreviousSessionID = ExtractPreviousSessionIDFromRequest(r)
		// 会话元数据由服务端从连接与请求头解析，不接受客户端自报。
		command.IPAddress = clientIPFromRequest(r)
		command.UserAgent = r.UserAgent()

		response, sessionID, err := handler.ExchangeQRCode(r.Context(), command)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		// 会话在 exchange 签发：发起端是匿名 PC，这里种下 cookie 才算登录完成。
		if sessionID != "" {
			SetOpaqueSessionCookie(w, sessionID)
		}

		route.WriteJSON(w, http.StatusOK, response)
	})
}
