package saml

import (
	"strings"

	"github.com/beevik/etree"
)

// IdPIssuerFromMetadata extracts the IdP entityID from a SAML metadata
// EntityDescriptor. It returns "" when the document is not metadata or has no
// entityID; callers treat that as "issuer check not configured".
func IdPIssuerFromMetadata(metadata string) string {
	metadata = strings.TrimSpace(metadata)
	if metadata == "" {
		return ""
	}
	doc := etree.NewDocument()
	if err := doc.ReadFromString(metadata); err != nil {
		return ""
	}
	root := doc.Root()
	if root == nil || root.Tag != "EntityDescriptor" {
		return ""
	}
	return strings.TrimSpace(root.SelectAttrValue("entityID", ""))
}
