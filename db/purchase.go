package db

import (
	"encoding/base64"
	"fmt"
	"html/template"
	"net/http"
	"time"
)

const (
	StatusNew            string = "new"            // waiting for payment
	StatusExpired               = "expired"        // not paid properly
	StatusUnderdelivered        = "underdelivered" // payment settled, but we had not had enough items on stock
	StatusFinalized             = "finalized"      // payment settled, codes delivered
)

type Purchase struct {
	InvoiceID  string
	Status     string
	Ordered    Order
	Delivered  Delivery
	DeleteDate string
}

func (p *Purchase) DeleteDateStr() (string, error) {
	var t, err = time.Parse(DateFmt, string(p.DeleteDate))
	return t.Format("02.01.2006"), err
}

// GetUnfulfilled returns the difference of ordered and delivered items.
// Keys are article IDs, values are amounts. Empty assignments are omitted.
func (p *Purchase) GetUnfulfilled() map[string]int {
	var unfulfilled = make(map[string]int)
	// add ordered
	for _, o := range p.Ordered {
		unfulfilled[o.ArticleID] += o.Amount
	}
	// subtract delivered
	for _, d := range p.Delivered {
		unfulfilled[d.ArticleID] -= 1 // there is no d.Amount
	}
	// remove empty items
	for articleID, amount := range unfulfilled {
		if amount == 0 {
			delete(unfulfilled, articleID)
		}
	}
	return unfulfilled
}

type Order []OrderRow

func (order Order) Sum() int {
	var sum = 0
	for _, o := range order {
		sum += o.Sum()
	}
	return sum
}

func (order Order) SumEUR() float64 {
	return float64(order.Sum()) / 100.0
}

type OrderRow struct {
	Amount    int    `json:"amount"`
	ArticleID string `json:"article-id"`
	ItemPrice int    `json:"item-price"` // euro cents, price at order time
}

func (o OrderRow) ItemPriceEUR() float64 {
	return float64(o.ItemPrice) / 100.0
}

func (o OrderRow) Sum() int {
	return o.Amount * o.ItemPrice
}

func (o OrderRow) SumEUR() float64 {
	return float64(o.Sum()) / 100.0
}

type Delivery []DeliveredItem

type DeliveredItem struct {
	ArticleID    string `json:"article-id"`
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
