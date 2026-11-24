package kerberos

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

func Routes() []route.Definition {
	return []route.Definition{
		{
			Method:      "GET",
			Pattern:     "/api/v1/auth/kerberos/negotiate",
			OperationID: "KerberosNegotiate",
			Summary:     "Kerberos SPNEGO negotiate",
			Permission:  "kerberos.auth",
			Surface:     route.SurfaceProtocol,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/auth/kerberos/login",
			OperationID: "KerberosLogin",
			Summary:     "Kerberos login with ticket",
			Permission:  "kerberos.auth",
			Surface:     route.SurfaceProtocol,
		},
	}
}
