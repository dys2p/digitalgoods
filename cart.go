package digitalgoods

type Cart struct {
	CountryID string
	Units     map[CartItem]int // item -> quantity
}

func (cart *Cart) Add(variantID, variantCountry string, quantity int) {
	if cart.Units == nil {
		cart.Units = make(map[CartItem]int)
	}
	cart.Units[CartItem{variantID, variantCountry}] += quantity
}

func (cart *Cart) Get(variantID, variantCountry string) int {
	if cart == nil {
		return 0
	}
	return cart.Units[CartItem{variantID, variantCountry}]
}

func (cart *Cart) Has(article Article) bool {
	if cart == nil {
		return false
	}
	// O(n^2) search
	for _, variant := range article.Variants {
		for item := range cart.Units {
			if item.VariantID == variant.ID {
				return true
			}
		}
	}
	return false
}

type CartItem struct {
	VariantID      string
	VariantCountry string
}
