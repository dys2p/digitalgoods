package digitalgoods

type Sale struct {
	ID        string
	Country   string
	PayDate   string
	Name      string
	Gross     int
	Difftax   int
	IsService bool
	VATRate   string
}
