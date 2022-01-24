package db

type Article struct {
	ID         string
	CategoryID string
	Name       string
	Price      int // euro cents
	Stock      int
	Hide       bool
}
