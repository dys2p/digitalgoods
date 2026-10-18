package digitalgoods

import (
	"github.com/dys2p/eco/countries"
)

// zero value is okay
type Customer struct {
	Country  countries.Country
	B2BVATID string // must match country, for tax-free b2b sales only
}
