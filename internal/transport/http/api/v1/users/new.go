package users

import (
	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
	apporders "github.com/guliangiam/guliangiam/internal/app/orders"
	apppayments "github.com/guliangiam/guliangiam/internal/app/payments"
	appsessions "github.com/guliangiam/guliangiam/internal/app/sessions"
	appusers "github.com/guliangiam/guliangiam/internal/app/users"
	"github.com/guliangiam/guliangiam/internal/platform/scopeauthorizer"
)

func NewHandler(service appusers.Service, ordersService apporders.Service, paymentsService apppayments.Service, sessionService appsessions.Service, scopeAuthorizer scopeauthorizer.ScopeAuthorizer, authServices ...*appauth.Service) Handler {
	handler := Handler{
		Service:         service,
		OrdersService:   ordersService,
		PaymentsService: paymentsService,
		SessionService:  sessionService,
		ScopeAuthorizer: scopeAuthorizer,
	}
	if len(authServices) > 0 {
		handler.AuthService = authServices[0]
	}
	return handler
}
