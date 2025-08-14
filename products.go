package digitalgoods

import (
	"fmt"
	"html/template"
	"iter"

	"github.com/dys2p/eco/lang"
	"github.com/dys2p/eco/productfeed"
)

type Stock map[string]int // variant => quantity

type Variant struct {
	ID        string
	Name      string
	ImageLink string
	Price     int // euro cents
	WarnStock int
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

func (catalog Catalog) Articles() iter.Seq[Article] {
	return func(yield func(Article) bool) {
		for _, category := range catalog {
			for _, article := range category.Articles {
				if !yield(article) {
					return
				}
			}
		}
	}
}

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

func (catalog Catalog) Variant(id string) (Variant, bool) {
	for _, category := range catalog {
		for _, article := range category.Articles {
			for _, variant := range article.Variants {
				if variant.ID == id {
					return variant, true
				}
			}
		}
	}
	return Variant{}, false
}

type PurchaseArticle struct {
	Article
	Variants []PurchaseVariant // shadows Article.Variants
}

func (pa PurchaseArticle) AnythingDelivered() bool {
	for _, v := range pa.Variants {
		if len(v.Delivered) > 0 {
			return true
		}
	}
	return false
}

type PurchaseVariant struct {
	Variant
	Quantity   int
	GrossPrice int // in case Variant.Price has changed
	Delivered  []DeliveredItem
}

// MakePurchaseArticles runs in O(n^2). Only use it for small catalogs.
func MakePurchaseArticles(catalog Catalog, purchase *Purchase) []PurchaseArticle {
	// filter catalog by purchase.Ordered
	var purchaseArticles []PurchaseArticle
	for article := range catalog.Articles() {
		var purchaseVariants []PurchaseVariant
		for _, variant := range article.Variants {
			var purchaseVariant PurchaseVariant
			for _, row := range purchase.Ordered {
				if row.VariantID == variant.ID {
					purchaseVariant.Variant = variant
					purchaseVariant.GrossPrice = row.ItemPrice
					purchaseVariant.Quantity += row.Quantity
				}
			}
			if purchaseVariant.Quantity > 0 {
				purchaseVariants = append(purchaseVariants, purchaseVariant)
			}
		}
		if len(purchaseVariants) > 0 {
			purchaseArticles = append(purchaseArticles, PurchaseArticle{
				Article:  article,
				Variants: purchaseVariants,
			})
		}
	}

	// add purchase.Delivered
nextItem:
	for _, item := range purchase.Delivered {
		// linear search in purchaseArticles
		for i := range purchaseArticles {
			for j := range purchaseArticles[i].Variants {
				if purchaseArticles[i].Variants[j].ID == item.VariantID {
					purchaseArticles[i].Variants[j].Delivered = append(purchaseArticles[i].Variants[j].Delivered, item)
					continue nextItem
				}
			}
		}
		// variant not found, add new article
		//
		// get Quantity and GrossPrice from first matching row in purchase.Ordered
		var quantity int
		var grossPrice int
		for _, row := range purchase.Ordered {
			if row.VariantID == item.VariantID {
				quantity = row.Quantity
				grossPrice = row.ItemPrice
			}
		}
		purchaseArticles = append(purchaseArticles, PurchaseArticle{
			Variants: []PurchaseVariant{{
				Variant: Variant{
					ID:   item.VariantID,
					Name: item.VariantID,
				},
				Quantity:   quantity,
				GrossPrice: grossPrice,
				Delivered:  []DeliveredItem{item},
			}},
		})
	}

	return purchaseArticles
}

func (pv PurchaseVariant) GrossSum() int {
	return pv.Quantity * pv.GrossPrice
}
