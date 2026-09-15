package digitalgoods

import (
	"cmp"
	"slices"
	"strings"
)

type UploadCatalog []UploadBrand

type UploadBrand struct {
	Brand string
	Units []UploadStockUnit
}

type UploadStockUnit struct {
	StockID  string
	Variants []Variant
}

// MakeUploadCatalog creates a catalog for the backend upload view. It collects stock units by brand.
func MakeUploadCatalog(catalog Catalog) UploadCatalog {
	var ucatalog UploadCatalog
	for a := range catalog.Articles() {
		// get or insert brand at index b
		b := slices.IndexFunc(ucatalog, func(brand UploadBrand) bool { return brand.Brand == a.Brand })
		if b < 0 {
			ucatalog = append(ucatalog, UploadBrand{Brand: a.Brand})
			b = len(ucatalog) - 1
		}
		// insert or append unit for variant
		for _, v := range a.Variants {
			i := slices.IndexFunc(ucatalog[b].Units, func(unit UploadStockUnit) bool { return unit.StockID == v.StockID() })
			if i < 0 {
				ucatalog[b].Units = append(ucatalog[b].Units, UploadStockUnit{StockID: v.StockID(), Variants: []Variant{v}})
			} else {
				ucatalog[b].Units[i].Variants = append(ucatalog[b].Units[i].Variants, v)
			}
		}
	}
	slices.SortFunc(ucatalog, func(a, b UploadBrand) int { return cmp.Compare(strings.ToLower(a.Brand), strings.ToLower(b.Brand)) })
	return ucatalog
}

func (ucatalog UploadCatalog) UploadStockUnit(id string) (UploadStockUnit, bool) {
	for _, article := range ucatalog {
		for _, unit := range article.Units {
			if unit.StockID == id {
				return unit, true
			}
		}
	}
	return UploadStockUnit{}, false
}
