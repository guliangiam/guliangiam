package jwt

import (
	domaintokens "github.com/guliangiam/guliangiam/internal/domain/tokens"
)

type AccessTokenClaims = domaintokens.AccessTokenClaims

type IDTokenClaims = domaintokens.IDTokenClaims

type IssueAccessTokenParams = domaintokens.IssueAccessTokenParams

type IssueIDTokenParams = domaintokens.IssueIDTokenParams
