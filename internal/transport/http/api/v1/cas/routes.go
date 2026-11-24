package cas

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

func Routes() []route.Definition {
	return []route.Definition{
		{
			Method:      "GET",
			Pattern:     "/cas/{organizationSlug}/{applicationSlug}/validate",
			OperationID: "CasValidate",
			Summary:     "CAS 1.0 validate",
			Permission:  "cas.validate",
			Surface:     route.SurfaceProtocol,
		},
		{
			Method:      "GET",
			Pattern:     "/cas/{organizationSlug}/{applicationSlug}/serviceValidate",
			OperationID: "CasServiceValidate",
			Summary:     "CAS 2.0 serviceValidate",
			Permission:  "cas.validate",
			Surface:     route.SurfaceProtocol,
		},
		{
			Method:      "GET",
			Pattern:     "/cas/{organizationSlug}/{applicationSlug}/proxyValidate",
			OperationID: "CasProxyValidate",
			Summary:     "CAS 2.0 proxyValidate",
			Permission:  "cas.validate",
			Surface:     route.SurfaceProtocol,
		},
		{
			Method:      "GET",
			Pattern:     "/cas/{organizationSlug}/{applicationSlug}/proxy",
			OperationID: "CasProxy",
			Summary:     "CAS 2.0 proxy",
			Permission:  "cas.validate",
			Surface:     route.SurfaceProtocol,
		},
		{
			Method:      "GET",
			Pattern:     "/cas/{organizationSlug}/{applicationSlug}/p3/serviceValidate",
			OperationID: "CasP3ServiceValidate",
			Summary:     "CAS 3.0 serviceValidate",
			Permission:  "cas.validate",
			Surface:     route.SurfaceProtocol,
		},
		{
			Method:      "GET",
			Pattern:     "/cas/{organizationSlug}/{applicationSlug}/p3/proxyValidate",
			OperationID: "CasP3ProxyValidate",
			Summary:     "CAS 3.0 proxyValidate",
			Permission:  "cas.validate",
			Surface:     route.SurfaceProtocol,
		},
		{
			Method:      "POST",
			Pattern:     "/cas/{organizationSlug}/{applicationSlug}/samlValidate",
			OperationID: "CasSamlValidate",
			Summary:     "CAS SAML validate",
			Permission:  "cas.validate",
			Surface:     route.SurfaceProtocol,
		},
	}
}
