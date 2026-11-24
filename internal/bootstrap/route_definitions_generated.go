package bootstrap

import (
	routes0 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/actions"
	routes1 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/adapters"
	routes2 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/agents"
	routes3 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/analytics"
	routes4 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/applications"
	routes5 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	routes6 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/b2b"
	routes7 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/batch"
	routes8 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/captchaproviders"
	routes9 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/cas"
	routes10 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/certs"
	routes11 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/coupons"
	routes12 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/dashboard"
	routes13 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/dcr"
	routes14 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/enforcers"
	routes15 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/entries"
	routes16 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/faceidproviders"
	routes17 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/forms"
	routes18 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/gateway"
	routes19 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/groups"
	routes20 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/i18n"
	routes21 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/identitypools"
	routes22 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/idv"
	routes23 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/iga"
	routes24 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/invitations"
	routes25 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/kerberos"
	routes26 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/keys"
	routes27 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/ldaps"
	routes28 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/ldapsync"
	routes29 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/mcpservers"
	routes30 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/messages"
	routes31 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/metering"
	routes32 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/models"
	routes33 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/notificationproviders"
	routes34 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/notifications"
	routes35 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/orders"
	routes36 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/organizations"
	routes37 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/payments"
	routes38 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/permissions"
	routes39 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/plans"
	routes40 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/pricings"
	routes41 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/privacy"
	routes42 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/products"
	routes43 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/providers"
	routes44 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/radius"
	routes45 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/radiusaccountings"
	routes46 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/radiusconfigs"
	routes47 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/rebac"
	routes48 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/reconciliation"
	routes49 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/records"
	routes50 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/resources"
	routes51 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/risk"
	routes52 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/roles"
	routes53 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/rules"
	routes54 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/saml"
	routes55 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/scim"
	routes69 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/search"
	routes56 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/sessions"
	routes57 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/subscriptions"
	routes58 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/syncers"
	routes59 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/system"
	routes60 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/tickets"
	routes61 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/tokens"
	routes62 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/transactions"
	routes70 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/usernotifications"
	routes63 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/users"
	routes64 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/verifications"
	routes65 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/webauthn"
	routes66 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/webhookevents"
	routes67 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/webhooks"
	routes68 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/wellknown"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// allRouteDefinitions aggregates every package-level route declaration so the
// startup contract check sees the same set the OpenAPI/route gates see.
func allRouteDefinitions() []route.Definition {
	var out []route.Definition
	out = append(out, routes0.Routes()...)
	out = append(out, routes1.Routes()...)
	out = append(out, routes2.Routes()...)
	out = append(out, routes3.Routes()...)
	out = append(out, routes4.Routes()...)
	out = append(out, routes5.Routes()...)
	out = append(out, routes6.Routes()...)
	out = append(out, routes7.Routes()...)
	out = append(out, routes8.Routes()...)
	out = append(out, routes9.Routes()...)
	out = append(out, routes10.Routes()...)
	out = append(out, routes11.Routes()...)
	out = append(out, routes12.Routes()...)
	out = append(out, routes13.Routes()...)
	out = append(out, routes14.Routes()...)
	out = append(out, routes15.Routes()...)
	out = append(out, routes16.Routes()...)
	out = append(out, routes17.Routes()...)
	out = append(out, routes18.Routes()...)
	out = append(out, routes19.Routes()...)
	out = append(out, routes20.Routes()...)
	out = append(out, routes21.Routes()...)
	out = append(out, routes22.Routes()...)
	out = append(out, routes23.Routes()...)
	out = append(out, routes24.Routes()...)
	out = append(out, routes25.Routes()...)
	out = append(out, routes26.Routes()...)
	out = append(out, routes27.Routes()...)
	out = append(out, routes28.Routes()...)
	out = append(out, routes29.Routes()...)
	out = append(out, routes30.Routes()...)
	out = append(out, routes31.Routes()...)
	out = append(out, routes32.Routes()...)
	out = append(out, routes33.Routes()...)
	out = append(out, routes34.Routes()...)
	out = append(out, routes35.Routes()...)
	out = append(out, routes36.Routes()...)
	out = append(out, routes37.Routes()...)
	out = append(out, routes38.Routes()...)
	out = append(out, routes39.Routes()...)
	out = append(out, routes40.Routes()...)
	out = append(out, routes41.Routes()...)
	out = append(out, routes42.Routes()...)
	out = append(out, routes43.Routes()...)
	out = append(out, routes44.Routes()...)
	out = append(out, routes45.Routes()...)
	out = append(out, routes46.Routes()...)
	out = append(out, routes47.Routes()...)
	out = append(out, routes48.Routes()...)
	out = append(out, routes49.Routes()...)
	out = append(out, routes50.Routes()...)
	out = append(out, routes51.Routes()...)
	out = append(out, routes52.Routes()...)
	out = append(out, routes53.Routes()...)
	out = append(out, routes54.Routes()...)
	out = append(out, routes55.Routes()...)
	out = append(out, routes56.Routes()...)
	out = append(out, routes57.Routes()...)
	out = append(out, routes58.Routes()...)
	out = append(out, routes59.Routes()...)
	out = append(out, routes60.Routes()...)
	out = append(out, routes61.Routes()...)
	out = append(out, routes62.Routes()...)
	out = append(out, routes63.Routes()...)
	out = append(out, routes64.Routes()...)
	out = append(out, routes65.Routes()...)
	out = append(out, routes66.Routes()...)
	out = append(out, routes67.Routes()...)
	out = append(out, routes68.Routes()...)
	out = append(out, routes69.Routes()...)
	out = append(out, routes70.Routes()...)
	out = append(out, routes10.GlobalRoutes()...)
	return out
}
