package digitalgoods

type Cart struct {
	CountryID string
	Units     map[string]int // variant id => quantity
}

func (cart *Cart) Get(variantID string) int {
	if cart == nil {
		return 0
	}
	return cart.Units[variantID]
}

func (cart *Cart) Has(article Article) bool {
	if cart == nil {
		return false
	}
	for _, variant := range article.Variants {
		if cart.Units[variant.ID] > 0 {
			return true
		}
	}
	return false
}
