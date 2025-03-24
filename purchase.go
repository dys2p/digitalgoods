package digitalgoods

import (
	"fmt"

	"github.com/dys2p/eco/lang"
)

const DateFmt = "2006-01-02"

const (
	StatusNew               Status = "new"            // unpaid
	StatusPaymentProcessing Status = "processing"     // e.g. btcpay: "InvoiceProcessing Webhook: Triggers when an invoice is fully paid, but doesn't have the required amount of confirmations on the blockchain yet according to your store's settings."
	StatusUnderdelivered    Status = "underdelivered" // payment settled, but we had not had enough items in stock
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
		return string(s)
	}
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
	NotifyProto string
	NotifyAddr  string
	Ordered     Order
	Delivered   Delivery
	CreateDate  string // yyyy-mm-dd, for foreign currency rates
	DeleteDate  string // yyyy-mm-dd
	CountryCode string // EU country
}

func (p *Purchase) GetUnfulfilled() (Order, error) {
	// copy ordered
	var unfulfilled = make(Order, len(p.Ordered))
	copy(unfulfilled, p.Ordered)
	// decrement
	for _, d := range p.Delivered {
		if err := unfulfilled.Decrement(d.VariantID); err != nil {
			return nil, fmt.Errorf("decrementing order %s: %w", p.ID, err)
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
		if row.Quantity > 0 {
			return false
		}
	}
	return true
}

func (order *Order) Decrement(variantID string) error {
	for i := range *order {
		if (*order)[i].VariantID == variantID {
			(*order)[i].Quantity--
			return nil
		}
	}
	return fmt.Errorf("variant %s not found in order", variantID)
}

func (order Order) Sum() int {
	var sum = 0
	for _, o := range order {
		sum += o.Sum()
	}
	return sum
}

type OrderRow struct {
	Quantity  int    `json:"amount"`     // legacy json id
	VariantID string `json:"article-id"` // legacy json id
	CountryID string `json:"country-id"`
	ItemPrice int    `json:"item-price"` // euro cents, price at order time
}

func (o OrderRow) Sum() int {
	return o.Quantity * o.ItemPrice
}

type Delivery []DeliveredItem

type DeliveredItem struct {
	VariantID    string `json:"article-id"` // legacy json id
	CountryID    string `json:"country-id"`
	Payload      string `json:"id"`
	DeliveryDate string `json:"delivery-date"`
}
