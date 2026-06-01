package digitalgoods

type Cart struct {
	CountryID string
	Units     map[string]int // key: article id + "-" + variant id because multiple articles can have a common variant, and the cart must remember which article has been used
}

func (cart *Cart) Get(articleID, variantID string) int {
	if cart == nil {
		return 0
	}
	return cart.Units[articleID+"-"+variantID]
}

func (cart *Cart) Has(article Article) bool {
	if cart == nil {
		return false
	}
	for _, variant := range article.Variants {
		if cart.Units[article.ID+"-"+variant.ID] > 0 {
			return true
		}
	}
	return false
}
