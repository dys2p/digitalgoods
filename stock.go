package digitalgoods

type Stock map[string]int // variant => count

func (stock Stock) OnDemandOnly(variant Variant) bool {
	return stock[variant.ID] == 0 && variant.OnDemand
}
