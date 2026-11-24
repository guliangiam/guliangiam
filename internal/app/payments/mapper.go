package payments

import (
	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
)

func toListItem(p *domain.Payment) PaymentListItem {
	return PaymentListItem{ID: p.ID, OrganizationID: p.OrganizationID, Name: p.Name, DisplayName: p.DisplayName,
		Provider: p.Provider, Type: p.Type, User: p.User, Price: p.Price, Currency: p.Currency, State: p.State,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt}
}
func toDetail(p *domain.Payment) PaymentDetail {
	return PaymentDetail{PaymentListItem: toListItem(p), Products: append([]string(nil), p.Products...),
		ProductName: p.ProductName, ProductDisplayName: p.ProductDisplayName, Detail: p.Detail,
		PersonName: p.PersonName, PersonIdCard: p.PersonIdCard, PersonEmail: p.PersonEmail, PersonPhone: p.PersonPhone,
		InvoiceType: p.InvoiceType, InvoiceTitle: p.InvoiceTitle, InvoiceTaxId: p.InvoiceTaxId,
		InvoiceRemark: p.InvoiceRemark, InvoiceUrl: p.InvoiceUrl,
		Order: p.Order, OutOrderId: p.OutOrderId, PayUrl: p.PayUrl, SuccessUrl: p.SuccessUrl,
		Message: p.Message, Revision: p.Revision}
}
