package orders

import (
	"context"
	"math"
	"strings"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/orders"
)

// moneyTolerance is half a cent. Prices travel as DECIMAL in the catalog and as
// float64 in the API, so an exact equality test would reject honest baskets
// because of binary rounding; anything larger than half a cent is a real
// disagreement about money and must not be waved through.
const moneyTolerance = 0.005

// catalogPricing is what the products aggregate says about one basket: the
// snapshots to freeze onto the order and the price/currency the buyer owes.
type catalogPricing struct {
	Infos    []domain.ProductInfo
	Price    float64
	Currency string
}

// resolveCatalog resolves a basket of product identifiers against the products
// aggregate.
//
// The catalog is the single source of truth for what a product is and what it
// costs, so the snapshot an order carries is read from the aggregate here rather
// than accepted from the caller: a client that could post its own `productInfos`
// could post its own history, and the order would then be its own evidence. For
// the same reason the client's price is only a claim to be checked — a basket
// that claims a price the catalog does not agree with is rejected outright,
// which closes the `price: 0.01` tampering path. A zero claim means "you price
// it", which is what every console caller that has no money field relies on.
//
// A product that is unknown or not `active` fails the whole basket. Sellability
// has exactly one definition (`registries/state-machine-registry.md` §1.5) and
// buying is not a partial operation: half an order is not an order.
//
// Every catalog read is scoped to the order's organization: a product id alone
// must not resolve to another tenant's row, or the order would carry a foreign
// product snapshot even though the order itself is written locally.
func resolveCatalog(ctx context.Context, tx appuow.UnitOfWork, organizationID string, ids []string, claimedPrice float64, claimedCurrency string) (catalogPricing, error) {
	infos := make([]domain.ProductInfo, 0, len(ids))
	total := 0.0
	catalogCurrency := ""

	for _, id := range ids {
		p, err := tx.Products().GetByIDForOrganization(ctx, id, organizationID)
		if err != nil {
			return catalogPricing{}, err
		}
		if err := validateSellableProduct(p); err != nil {
			return catalogPricing{}, err
		}
		// An order cannot mix currencies: the total below would be a number with
		// no unit, and the payment channel would be asked to charge it anyway.
		if err := validateSingleCurrency(catalogCurrency, p.Currency); err != nil {
			return catalogPricing{}, err
		}
		catalogCurrency = p.Currency
		infos = append(infos, domain.ProductInfo{
			Owner:       p.OrganizationID,
			Name:        p.Name,
			DisplayName: p.DisplayName,
			Image:       p.Image,
			Detail:      p.Detail,
			Price:       p.Price,
			Currency:    p.Currency,
			IsRecharge:  p.IsRecharge,
			Quantity:    1,
		})
		total += p.Price
	}

	if err := validateClaimedCurrency(claimedCurrency, catalogCurrency); err != nil {
		return catalogPricing{}, err
	}
	if claimedPrice != 0 && math.Abs(claimedPrice-total) > moneyTolerance {
		return catalogPricing{}, validateClaimedPrice(claimedPrice, total)
	}

	price := total
	if claimedPrice != 0 {
		price = claimedPrice
	}
	return catalogPricing{Infos: infos, Price: price, Currency: catalogCurrency}, nil
}

// catalogOrderName derives the order's business key when the caller has none.
//
// `uq_orders_org_name` makes `name` unique per organization, so a buyer-driven
// checkout cannot invent a stable one — two carts of the same product would
// collide. The `suffix` is the freshly generated order id, which makes the
// derived name unique by construction and keeps it traceable back to the order.
func catalogOrderName(infos []domain.ProductInfo, suffix string) string {
	base := "order"
	if len(infos) > 0 && strings.TrimSpace(infos[0].Name) != "" {
		base = infos[0].Name
	}
	short := strings.ReplaceAll(suffix, "-", "")
	if len(short) > 8 {
		short = short[:8]
	}
	return base + "-" + short
}

// catalogOrderDisplayName is the human label for a basket the caller did not
// label. It falls back to the business key so an order is never nameless in the
// console.
func catalogOrderDisplayName(infos []domain.ProductInfo, fallback string) string {
	if len(infos) > 0 {
		if infos[0].DisplayName != "" {
			return infos[0].DisplayName
		}
		if infos[0].Name != "" {
			return infos[0].Name
		}
	}
	return fallback
}
