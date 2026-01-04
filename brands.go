package digitalgoods

import "strings"

type BrandCatalog struct {
	Name       string
	Categories []Category
}

// MakeBrandCatalogs creates a catalog for the backend upload view. It collects stock units by brand.
func MakeBrandCatalogs(catalog Catalog) map[string]BrandCatalog {
	// collect brands
	var brands = make(map[string]any)
	for a := range catalog.Articles() {
		brands[a.Brand] = Catalog{}
	}

	var m = make(map[string]BrandCatalog)
	for brand := range brands {
		// filter articles by brand, then skip empty categories
		var categories []Category
		for _, category := range catalog {
			var articles []Article
			for _, a := range category.Articles {
				if a.Brand == brand {
					articles = append(articles, a)
				}
			}
			if len(articles) > 0 {
				categories = append(categories, Category{
					Name:     category.Name,
					Articles: articles,
				})
			}
		}
		m[strings.ToLower(brand)] = BrandCatalog{
			Name:       brand,
			Categories: categories,
		}
	}
	return m
}
