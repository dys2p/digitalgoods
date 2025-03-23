package digitalgoods

import (
	"fmt"
	"html/template"

	"github.com/dys2p/eco/lang"
	"github.com/dys2p/eco/productfeed"
)

type Stock map[string]int // variant => quantity

type Variant struct {
	ID        string
	Name      string
	ImageLink string
	Price     int // euro cents
}

func (variant Variant) NameHTML() template.HTML {
	return template.HTML(variant.Name)
}

type Description struct {
	Alert string
	About string
	Howto string
	Legal string
}

type Article struct {
	Brand     string
	Name      string // not translated
	Hide      bool
	ImageLink string
	ID        string // for <details> and #anchor
	Desc      map[string]Description
	Variants  []Variant
}

func (article Article) NameHTML() template.HTML {
	return template.HTML(article.Name)
}

// only supports langs which exist as key, TODO: language.Matcher
func (article Article) TranslateAlert(l lang.Lang) template.HTML {
	return template.HTML(article.Desc[l.Prefix].Alert)
}

// only supports langs which exist as key, TODO: language.Matcher
func (article Article) TranslateAbout(l lang.Lang) template.HTML {
	return template.HTML(article.Desc[l.Prefix].About)
}

// only supports langs which exist as key, TODO: language.Matcher
func (article Article) TranslateHowto(l lang.Lang) template.HTML {
	return template.HTML(article.Desc[l.Prefix].Howto)
}

// only supports langs which exist as key, TODO: language.Matcher
func (article Article) TranslateLegal(l lang.Lang) template.HTML {
	return template.HTML(article.Desc[l.Prefix].Legal)
}

type Category struct {
	Name     map[string]string
	Articles []Article
}

// only supports langs which exist as Name key, TODO: language.Matcher
func (cat *Category) TranslateName(l lang.Lang) template.HTML {
	return template.HTML(cat.Name[l.Prefix])
}

type Catalog []Category

// assumes that catalog contains every article exactly once
func (catalog Catalog) Products() []productfeed.Product {
	var products []productfeed.Product
	for _, category := range catalog {
		for _, article := range category.Articles {
			if article.Hide {
				continue
			}

			for _, variant := range article.Variants {
				var imageLink = variant.ImageLink
				if imageLink == "" {
					imageLink = article.ImageLink // fallback
				}

				products = append(products, productfeed.Product{
					Availability: "in stock",
					Brand:        article.Brand,
					Condition:    "new",
					Description:  productfeed.HTMLtoText(article.Desc["en"].About), // TODO match request language?
					Id:           variant.ID,
					ImageLink:    imageLink,
					ItemGroupId:  article.ID,
					Link:         "https://digitalgoods.proxysto.re/#" + article.ID,
					Price:        fmt.Sprintf("%.2f EUR", float64(variant.Price)/100.0),
					Title:        variant.Name,
				})
			}
		}
	}
	return products
}

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

type PurchaseArticle struct {
	Article
	Variants []PurchaseVariant // shadows Article.Variants
}

type PurchaseVariant struct {
	Variant
	Rows []OrderRow // TODO just one OrderRow?
}

func MakePurchaseArticles(catalog Catalog, order Order) []PurchaseArticle {
	var rowsByVariantID = make(map[string][]OrderRow)
	for _, row := range order {
		rowsByVariantID[row.VariantID] = append(rowsByVariantID[row.VariantID], row)
	}

	var articles []PurchaseArticle
	for _, category := range catalog {
		for _, article := range category.Articles {
			var variants []PurchaseVariant
			for _, variant := range article.Variants {
				if rows := rowsByVariantID[variant.ID]; len(rows) > 0 {
					variants = append(variants, PurchaseVariant{
						Variant: variant,
						Rows:    rows,
					})
				}
			}
			if len(variants) > 0 {
				articles = append(articles, PurchaseArticle{
					Article:  article,
					Variants: variants,
				})
			}
		}
	}

	// don't check the unlikely case that no article is found because this is just the "ordered" section and not the "delivered goods" section

	return articles
}
