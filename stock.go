package digitalgoods

type Stock map[string]map[string]int // variant => country (or "all") => count

func (stock Stock) Get(variant Variant, country string) int {
	v := stock[variant.ID]
	if v == nil {
		return 0
	}
	return v[country]
}

// Max returns the max value which can be ordered (like the HTML input max attribute). The stock quantity should be displayed separately, so users know how many items can be delivered instantly.
func (stock Stock) Max(variant Variant, country string) int {
	max := stock.Get(variant, country)
	if variant.OnDemand {
		max += 100
	}
	return max
}

func (stock Stock) OnDemandOnly(variant Variant, country string) bool {
	return stock.Get(variant, country) == 0 && variant.OnDemand
}

// TODO FeaturedCountryIDs: check s.Underdelivered[variant.ID+"-"+countryID] ?

func (stock Stock) FeaturedCountryIDs(variant Variant) []string {
	if !variant.HasCountry {
		return []string{"all"}
	}
	ids := []string{}
	for _, country := range ISOCountryCodes {
		if stock.Get(variant, country) > 0 {
			ids = append(ids, country)
		}
	}
	return ids
}

func (stock Stock) OtherCountryIDs(variant Variant) []string {
	if !variant.HasCountry {
		return nil
	}
	if !variant.OnDemand {
		return nil
	}
	ids := []string{}
	for _, country := range ISOCountryCodes {
		if stock.Get(variant, country) == 0 {
			ids = append(ids, country)
		}
	}
	return ids
}
