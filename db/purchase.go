package db

import (
	"encoding/base64"
	"fmt"
	"html/template"
	"net/http"
	"time"
)

const (
	StatusNew                  string = "new" // no BTCPay invoice created yet
	StatusBTCPayInvoiceCreated string = "btcpay-created"
	StatusBTCPayInvoiceExpired string = "btcpay-expired" // not paid properly
	StatusUnderdelivered       string = "underdelivered" // payment settled, but we had not had enough items on stock
	StatusFinalized            string = "finalized"      // payment settled, codes delivered
)

type Purchase struct {
	ID              string
	BTCPayInvoiceID string // defined by BTCPay server
	PayID           string // defined by us
	Status          string
	Ordered         Order
	Delivered       Delivery
	DeleteDate      string
	CountryCode     string
}

func (p *Purchase) DeleteDateStr() (string, error) {
	var t, err = time.Parse(DateFmt, string(p.DeleteDate))
	return t.Format("02.01.2006"), err
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
	return p.Status == StatusNew || p.Status == StatusBTCPayInvoiceCreated || p.Status == StatusBTCPayInvoiceExpired
}

func (p *Purchase) WaitingForBTCPayment() bool {
	return p.Status == StatusBTCPayInvoiceCreated
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

func (item *DeliveredItem) DeliveryDateStr() (string, error) {
	var t, err = time.Parse(DateFmt, string(item.DeliveryDate))
	return t.Format("02.01.2006"), err
}

func (item *DeliveredItem) ImageSrc() template.URL {
	return template.URL(fmt.Sprintf("data:%s;base64,%s", http.DetectContentType(item.Image), base64.StdEncoding.EncodeToString(item.Image)))
}
