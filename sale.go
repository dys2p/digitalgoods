package digitalgoods

type Sale struct {
	ID          string
	Country     string
	B2BVATID    string
	PayDate     string
	VariantID   string
	Quantity    int
	ItemPrice   int
	ItemDifftax int
	IsService   bool
	VATRate     string
}
