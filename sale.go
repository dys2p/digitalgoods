package digitalgoods

type Sale struct {
	ID        string
	Country   string
	PayDate   string
	Name      string
	Quantity  int
	GrossSum  int // for all items
	Difftax   int
	IsService bool
	VATRate   string
}
