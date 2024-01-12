package digitalgoods

import (
	"fmt"
	"html/template"

	"github.com/dys2p/eco/lang"
)

type Catalog []Category

func (catalog Catalog) Variant(id string) (Variant, error) {
	for _, category := range catalog {
		for _, article := range category.Articles {
			for _, variant := range article.Variants {
				if variant.ID == id {
					return variant, nil
				}
			}
		}
	}
	return Variant{}, fmt.Errorf("variant not found: %s", id)
}

func (catalog Catalog) Variants() []Variant {
	var variants []Variant
	for _, category := range catalog {
		for _, article := range category.Articles {
			variants = append(variants, article.Variants...)
		}
	}
	return variants
}

type Category struct {
	Name     map[string]string
	Articles []Article
}

// only supports langs which exist as Name key, TODO: language.Matcher
func (cat *Category) TranslateName(l lang.Lang) template.HTML {
	return template.HTML(cat.Name[string(l)])
}

type Article struct {
	Name        string // not translated
	Alert       map[string]string
	Description map[string]string
	Variants    []Variant
}

func (article Article) NameHTML() template.HTML {
	return template.HTML(article.Name)
}

// only supports langs which exist as Alert key, TODO: language.Matcher
func (article Article) TranslateAlert(l lang.Lang) template.HTML {
	return template.HTML(article.Alert[string(l)])
}

// only supports langs which exist as Description key, TODO: language.Matcher
func (article Article) TranslateDescription(l lang.Lang) template.HTML {
	return template.HTML(article.Description[string(l)])
}

// groups order by article
func (catalog Catalog) GroupOrder(order Order) []OrderedArticle {
	var rowsByVariantID = make(map[string][]OrderRow)
	for _, row := range order {
		rowsByVariantID[row.VariantID] = append(rowsByVariantID[row.VariantID], row)
	}

	var orderedArticles []OrderedArticle
	for _, category := range catalog {
		for _, article := range category.Articles {
			var orderedVariants []OrderedVariant
			for _, variant := range article.Variants {
				if rows := rowsByVariantID[variant.ID]; len(rows) > 0 {
					orderedVariants = append(orderedVariants, OrderedVariant{
						Variant: variant,
						Rows:    rows,
					})
				}
			}
			if len(orderedVariants) > 0 {
				orderedArticles = append(orderedArticles, OrderedArticle{
					Article:  article,
					Variants: orderedVariants,
				})
			}
		}
	}

	// don't check the unlikely case that no article is found because this is just the "ordered" section and not the "delivered goods" section

	return orderedArticles
}

type OrderedArticle struct {
	Article
	Variants []OrderedVariant
}

type OrderedVariant struct {
	Variant
	Rows []OrderRow // TODO just one OrderRow?
}
