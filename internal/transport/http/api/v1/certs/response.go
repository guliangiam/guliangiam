package certs

import (
	appcerts "github.com/guliangiam/guliangiam/internal/app/certs"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type ListCertsResponse = presenter.ListResponse[appcerts.CertListItem]
type CertResponse = appcerts.CertDetail
