package db

type Article struct {
	ID         string
	CategoryID string
	Name       string
	Price      int            // euro cents
	Stock      map[string]int // key is country or "all"
	OnDemand   bool
	Hide       bool // see Portfolio() for precedence
	HasCountry bool
}

// Max returns the max value which can be ordered (like the HTML input max attribute). The stock quantity should be displayed separately, so users know how many items can be delivered instantly.
func (a Article) Max(countryID string) int {
	max := a.Stock[countryID]
	if a.OnDemand {
		max += 100
	}
	return max
}

// Portfolio determines whether an article is shown in the portfolio. It might be still sold out at the moment.
func (a Article) Portfolio() bool {
	if len(a.Stock) > 0 || a.OnDemand {
		return true
	}
	return !a.Hide
}
