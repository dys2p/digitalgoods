package db

type Article struct {
	ID         string
	CategoryID string
	Name       string
	Price      int // euro cents
	Stock      int
	Hide       bool
}

func (a Article) PriceEUR() float64 {
	return float64(a.Price) / 100.0
}
