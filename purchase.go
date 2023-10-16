package digitalgoods

import (
	"encoding/base64"
	"fmt"
	"html/template"
	"net/http"
	"time"

	"github.com/dys2p/eco/countries"
	"github.com/dys2p/eco/lang"
)

const DateFmt = "2006-01-02"

const (
	StatusNew               Status = "new"            // unpaid
	StatusPaymentProcessing Status = "processing"     // e.g. btcpay: "InvoiceProcessing Webhook: Triggers when an invoice is fully paid, but doesn't have the required amount of confirmations on the blockchain yet according to your store's settings."
	StatusUnderdelivered    Status = "underdelivered" // payment settled, but we had not had enough items on stock
	StatusFinalized         Status = "finalized"      // payment settled, codes delivered
)

type Status string

func (s Status) TranslateDescription(l lang.Lang) string {
	switch s {
	case StatusNew:
		return l.Tr("We are waiting for your payment.")
	case StatusPaymentProcessing:
		return l.Tr("A payment is on the way, but we're still waiting for the required amount of confirmations on the blockchain.")
	case StatusUnderdelivered:
		return l.Tr("We have received your payment, but have gone out of stock meanwhile. You will receive the missing codes here as soon as possible. Sorry for the inconvenience.")
	case StatusFinalized:
		return l.Tr("Your codes have been delivered.")
	default:
		return ""
	}
}

func (s Status) TranslateName(l lang.Lang) string {
	switch s {
	case StatusNew:
		return l.Tr("New")
	case StatusPaymentProcessing:
		return l.Tr("Payment processing")
	case StatusUnderdelivered:
		return l.Tr("Underdelivered")
	case StatusFinalized:
		return l.Tr("Finalized")
	default:
		return ""
	}
}

func IsValidCountryCode(s string) bool {
	if s == "non-EU" {
		return true
	}
	for _, c := range countries.EuropeanUnion {
		if c.ID == s {
			return true
		}
	}
	return false
}

// Mask replaces all but the last six letters of a string by asterisks.
func Mask(s string) string {
	r := []rune(s)
	for i := 0; i < len(s)-6; i++ {
		r[i] = '*'
	}
	return string(r)
}

type Purchase struct {
	ID          string
	AccessKey   string
	PaymentKey  string
	Status      Status
	Ordered     Order
	Delivered   Delivery
	CreateDate  string // yyyy-mm-dd
	DeleteDate  string // yyyy-mm-dd
	CountryCode string // EU country
}

func (p *Purchase) GetUnfulfilled() (Order, error) {
	// copy ordered
	var unfulfilled = make(Order, len(p.Ordered))
	copy(unfulfilled, p.Ordered)
	// decrement
	for _, d := range p.Delivered {
		if d.CountryID == "" {
			// backwards compatibility: rather fail than risk double fulfilment
			return nil, nil
		}
		if err := unfulfilled.Decrement(d.ArticleID, d.CountryID); err != nil {
			return nil, err
		}
	}
	return unfulfilled, nil
}

func (p *Purchase) Underdelivered() bool {
	return p.Status == StatusUnderdelivered
}

func (p *Purchase) Unpaid() bool {
	return p.Status == StatusNew
}

func (p *Purchase) Waiting() bool {
	return p.Status == StatusNew || p.Status == StatusPaymentProcessing || p.Status == StatusUnderdelivered
}

type Order []OrderRow

func (order Order) Empty() bool {
	for _, row := range order {
		if row.Amount > 0 {
			return false
		}
	}
	return true
}

func (order *Order) Decrement(articleID, countryID string) error {
	for i := range *order {
		if (*order)[i].ArticleID == articleID && (*order)[i].CountryID == countryID {
			(*order)[i].Amount--
			return nil
		}
	}
	return fmt.Errorf("article %s with country %s not found in order", articleID, countryID)
}

func (order Order) Sum() int {
	var sum = 0
	for _, o := range order {
		sum += o.Sum()
	}
	return sum
}

type OrderRow struct {
	Amount    int    `json:"amount"`
	ArticleID string `json:"article-id"`
	CountryID string `json:"country-id"`
	ItemPrice int    `json:"item-price"` // euro cents, price at order time
}

func (o OrderRow) Sum() int {
	return o.Amount * o.ItemPrice
}

type Delivery []DeliveredItem

type DeliveredItem struct {
	ArticleID    string `json:"article-id"`
	CountryID    string `json:"country-id"`
	ID           string `json:"id"` // can be the code, but not necessarily
	Image        []byte `json:"image"`
	DeliveryDate string `json:"delivery-date"`
}

func (item *DeliveredItem) ParseDeliveryDate() (time.Time, error) {
	return time.Parse(DateFmt, string(item.DeliveryDate))
}

func (item *DeliveredItem) ImageSrc() template.URL {
	return template.URL(fmt.Sprintf("data:%s;base64,%s", http.DetectContentType(item.Image), base64.StdEncoding.EncodeToString(item.Image)))
}

// Higher-level data structures:

type OrderArticle struct {
	OrderRow
	Article *Article
}

type OrderGroup struct {
	Category *Category
	Rows     []OrderArticle
}
