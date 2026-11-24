package payments

import (
	"time"
)

type ListPaymentsQuery struct {
	Page, PageSize                int
	OrganizationID, Keyword, User string
	State                         *string
	CreatedFrom, CreatedTo        *time.Time // batch reconciliation range
}
type CreatePaymentCommand struct {
	OrganizationID, Name, DisplayName, Provider, Type                  string
	Products                                                           []string
	ProductName, ProductDisplayName, Detail, Currency                  string
	Price                                                              float64
	User, PersonName, PersonIdCard, PersonEmail, PersonPhone           string
	InvoiceType, InvoiceTitle, InvoiceTaxId, InvoiceRemark, InvoiceUrl string
	Order, OutOrderId, PayUrl, SuccessUrl, State, Message              string
}
type PatchPaymentCommand struct {
	ID                             string
	DisplayName, Provider, Type    *string
	State, Message                 *string
	PayUrl, SuccessUrl, OutOrderId *string
	InvoiceUrl                     *string
	Revision                       int64
}

type InvoicePaymentCommand struct {
	ID                          string
	InvoiceType, InvoiceTitle   string
	InvoiceTaxId, InvoiceRemark string
	PersonName, PersonIdCard    string
	PersonEmail, PersonPhone    string
	Revision                    int64
}

type PaymentResult struct {
	ID           string    `json:"id"`
	State        string    `json:"state"`
	Price        float64   `json:"price"`
	Currency     string    `json:"currency,omitempty"`
	ProductName  string    `json:"productName,omitempty"`
	ProviderName string    `json:"providerName,omitempty"`
	PayUrl       string    `json:"payUrl,omitempty"`
	InvoiceUrl   string    `json:"invoiceUrl,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type PaymentListItem struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organizationId"`
	Name           string    `json:"name"`
	DisplayName    string    `json:"displayName"`
	Provider       string    `json:"provider,omitempty"`
	Type           string    `json:"type,omitempty"`
	User           string    `json:"user,omitempty"`
	Price          float64   `json:"price"`
	Currency       string    `json:"currency,omitempty"`
	State          string    `json:"state"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}
type PaymentDetail struct {
	PaymentListItem
	Products           []string `json:"products,omitempty"`
	ProductName        string   `json:"productName,omitempty"`
	ProductDisplayName string   `json:"productDisplayName,omitempty"`
	Detail             string   `json:"detail,omitempty"`
	PersonName         string   `json:"personName,omitempty"`
	PersonIdCard       string   `json:"personIdCard,omitempty"`
	PersonEmail        string   `json:"personEmail,omitempty"`
	PersonPhone        string   `json:"personPhone,omitempty"`
	InvoiceType        string   `json:"invoiceType,omitempty"`
	InvoiceTitle       string   `json:"invoiceTitle,omitempty"`
	InvoiceTaxId       string   `json:"invoiceTaxId,omitempty"`
	InvoiceRemark      string   `json:"invoiceRemark,omitempty"`
	InvoiceUrl         string   `json:"invoiceUrl,omitempty"`
	Order              string   `json:"order,omitempty"`
	OutOrderId         string   `json:"outOrderId,omitempty"`
	PayUrl             string   `json:"payUrl,omitempty"`
	SuccessUrl         string   `json:"successUrl,omitempty"`
	Message            string   `json:"message,omitempty"`
	Revision           int64    `json:"revision"`
}
