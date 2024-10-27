package digitalgoods

type Stock map[string]int // variant => count

// Max returns the max value which can be ordered (like the HTML input max attribute). The stock quantity should be displayed separately, so users know how many items can be delivered instantly.
func (stock Stock) Max(variant Variant) int {
	max := stock[variant.ID]
	if variant.OnDemand {
		max += 100
	}
	return max
}

func (stock Stock) OnDemandOnly(variant Variant) bool {
	return stock[variant.ID] == 0 && variant.OnDemand
}
