package digitalgoods

type Cart struct {
	CountryID string
	Units     map[string]int // variant id => quantity
}

func (cart *Cart) Get(articleID, variantID string) int {
	if cart == nil {
		return 0
	}
	return cart.Units[articleID+"-"+variantID] + cart.Units[variantID] // backwards compatibility
}

func (cart *Cart) Has(article Article) bool {
	if cart == nil {
		return false
	}
	for _, variant := range article.Variants {
		if cart.Units[article.ID+"-"+variant.ID] > 0 || cart.Units[variant.ID] > 0 { // backwards compatibility
			return true
		}
	}
	return false
}
