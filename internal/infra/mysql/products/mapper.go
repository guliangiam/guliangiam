package products

import (
	"encoding/json"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	domain "github.com/guliangiam/guliangiam/internal/domain/products"
)

// scanProduct 扫描一行到 domain.Product，处理 recharge_options / providers JSON 反序列化。
func scanProduct(sc interface{ Scan(...interface{}) error }) (*domain.Product, error) {
	var p domain.Product
	var rechargeOpts, providers []byte
	err := sc.Scan(&p.ID, &p.OrganizationID, &p.Name, &p.DisplayName, &p.Image, &p.Detail, &p.Description, &p.Tag, &p.Currency, &p.Price, &p.Quantity, &p.Sold, &p.IsRecharge, &rechargeOpts, &p.DisableCustomRecharge, &providers, &p.SuccessUrl, &p.State, &p.CreatedAt, &p.UpdatedAt, &p.Revision)
	if err != nil {
		return nil, err
	}
	if len(rechargeOpts) > 0 && string(rechargeOpts) != "null" {
		if err := json.Unmarshal(rechargeOpts, &p.RechargeOptions); err != nil {
			return nil, apperrors.WrapInternalf(err, "unmarshal recharge_options")
		}
	}
	if len(providers) > 0 && string(providers) != "null" {
		if err := json.Unmarshal(providers, &p.Providers); err != nil {
			return nil, apperrors.WrapInternalf(err, "unmarshal providers")
		}
	}
	if p.RechargeOptions == nil {
		p.RechargeOptions = []float64{}
	}
	if p.Providers == nil {
		p.Providers = []string{}
	}
	return &p, nil
}
