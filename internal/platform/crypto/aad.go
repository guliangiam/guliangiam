package crypto

import (
	"errors"
	"strings"

	"github.com/google/uuid"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ErrAADContext reports a missing or invalid v2 AAD context. A v2 token never
// degrades to v1 or plaintext when the caller cannot supply table/column/PK;
// callers must treat it as a data-integrity error.
var ErrAADContext = errors.New("crypto: v2 envelope AAD context is missing or invalid")

// ErrAADMismatch reports an authenticated v2 envelope that does not match the
// supplied row binding. It covers cross-row/cross-column ciphertext moves and
// any tampering with the token payload; callers must not fall back to v1.
var ErrAADMismatch = errors.New("crypto: v2 envelope does not match the row AAD")

// AADContext is the row identity bound into a v2 envelope. ADR-015 fixes the
// AAD to exactly "v2|{table}|{column}|{primary_key_value}": no tenant, no
// mutable columns and no algorithm, so a ciphertext stays readable only in the
// row and column that produced it.
type AADContext struct {
	Table   string // fully-qualified table, e.g. "guliangiam.providers"
	Column  string // encrypted column name
	PKValue string // normalized primary key text of the owning row
}

// Validate rejects contexts that would make the AAD ambiguous. Table and column
// reuse EncryptedColumn's safe-identifier rule; the primary key must be
// present and must not contain the separator, a newline or NUL. There is no
// escaping: an ambiguous AAD is refused instead of rewritten.
func (c AADContext) Validate() error {
	if !safeIdentifier.MatchString(c.Table) {
		return apperrors.WrapInternalf(ErrAADContext, "unsafe AAD table identifier %q", c.Table)
	}
	if !safeIdentifier.MatchString(c.Column) {
		return apperrors.WrapInternalf(ErrAADContext, "unsafe AAD column identifier %q", c.Column)
	}
	if c.PKValue == "" {
		return apperrors.WrapInternal(ErrAADContext, "AAD primary key value is required")
	}
	if strings.ContainsAny(c.PKValue, "|\n\r\x00") {
		return apperrors.WrapInternal(ErrAADContext, "AAD primary key value contains a reserved separator")
	}
	return nil
}

// Bytes returns the canonical AAD payload shared by the KEK-wrap and
// DEK->plaintext GCM layers. UUID primary keys are normalized to the lowercase
// hyphenated form the database stores; non-UUID keys are used verbatim.
func (c AADContext) Bytes() ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	pk := c.PKValue
	if parsed, err := uuid.Parse(pk); err == nil {
		pk = parsed.String()
	}
	return []byte("v2|" + c.Table + "|" + c.Column + "|" + pk), nil
}
