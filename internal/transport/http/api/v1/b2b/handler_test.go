package b2b

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	appb2b "github.com/guliangiam/guliangiam/internal/app/b2b"
	"github.com/stretchr/testify/require"
)

func TestClaimDomainRequestValidate(t *testing.T) {
	require.NoError(t, (ClaimDomainRequest{Domain: "corp.example.com"}).Validate())
	require.Error(t, (ClaimDomainRequest{Domain: " "}).Validate())
	require.Error(t, (ClaimDomainRequest{Domain: strings.Repeat("a", 254)}).Validate())
}

func TestListDomainsRejectsInvalidQuery(t *testing.T) {
	handler := Handler{}
	tests := []ListDomainsRequest{
		{Page: 0, PageSize: 20},
		{Page: 1, PageSize: 0},
		{Page: 1, PageSize: 101},
		{Page: 1, PageSize: 20, Status: "closed"},
	}
	for _, request := range tests {
		_, err := handler.ListDomains(context.Background(), request)
		require.Error(t, err)
	}
}

func TestDomainResponseVerifiedAt(t *testing.T) {
	verifiedAt := time.Date(2026, time.October, 27, 8, 30, 0, 0, time.UTC)

	withoutVerification, err := json.Marshal(toDomainResponse(appb2b.DomainDTO{}))
	require.NoError(t, err)
	require.NotContains(t, string(withoutVerification), `"verifiedAt"`)

	withVerification, err := json.Marshal(toDomainResponse(appb2b.DomainDTO{VerifiedAt: verifiedAt}))
	require.NoError(t, err)
	require.Contains(t, string(withVerification), `"verifiedAt":"2026-10-27T08:30:00Z"`)
}
