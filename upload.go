package digitalgoods

import (
	"cmp"
	"maps"
	"slices"
)

type UploadStockUnit struct {
	StockID  string
	Variants []Variant
}

func (unit UploadStockUnit) contains(variant Variant) bool {
	return slices.ContainsFunc(unit.Variants, func(v Variant) bool {
		return v.ID == variant.ID
	})
}

type UploadBrand struct {
	Brand string
	Units []UploadStockUnit
}

type UploadCatalog []UploadBrand

// MakeUploadCatalog creates a catalog for the backend upload view. It collects stock units by brand.
func MakeUploadCatalog(catalog Catalog) UploadCatalog {
	var m = make(map[string][]UploadStockUnit)
	for a := range catalog.Articles() {
		for _, v := range a.Variants {
			i := slices.IndexFunc(m[a.Brand], func(unit UploadStockUnit) bool { return unit.StockID == v.StockID() })
			if i < 0 {
				m[a.Brand] = append(m[a.Brand], UploadStockUnit{StockID: v.StockID()})
				i = len(m[a.Brand]) - 1
			}
			if !m[a.Brand][i].contains(v) {
				m[a.Brand][i].Variants = append(m[a.Brand][i].Variants, v)
			}
		}
	}
	var result UploadCatalog
	for _, brand := range slices.Sorted(maps.Keys(m)) {
		var units = m[brand]
		slices.SortFunc(units, func(a, b UploadStockUnit) int { return cmp.Compare(a.StockID, b.StockID) })
		result = append(result, UploadBrand{
			Brand: brand,
			Units: units,
		})
	}
	return result
}

func (ucatalog UploadCatalog) UploadStockUnit(id string) (UploadStockUnit, bool) {
	for _, brand := range ucatalog {
		for _, unit := range brand.Units {
			if unit.StockID == id {
				return unit, true
			}
		}
	}
	return UploadStockUnit{}, false
}
