package digitalgoods

type Sale struct {
	ID        string
	Country   string
	PayDate   string
	VariantID string
	Quantity  int
	GrossSum  int // for all items
	Difftax   int // for all items
	IsService bool
	VATRate   string
}
