package html

import (
	"embed"
	"fmt"
	"html/template"
	"strings"

	"github.com/dys2p/digitalgoods"
	"github.com/dys2p/eco/captcha"
	"github.com/dys2p/eco/countries"
	"github.com/dys2p/eco/lang"
	"github.com/dys2p/eco/payment"
	"github.com/dys2p/eco/payment/health"
	"gitlab.com/golang-commonmark/markdown"
)

//go:embed *
var files embed.FS

var md = markdown.New(markdown.HTML(true), markdown.Linkify(false))

func parse(fn ...string) *template.Template {
	t := template.New(fn[0]).Funcs(template.FuncMap{
		"AlertContextualClass": func(status digitalgoods.Status) string {
			switch status {
			case digitalgoods.StatusNew:
				return "alert-primary"
			case digitalgoods.StatusPaymentProcessing:
				return "alert-success"
			case digitalgoods.StatusUnderdelivered:
				return "alert-warning"
			case digitalgoods.StatusFinalized:
				return "alert-success"
			default:
				return "alert-primary"
			}
		},
		"FmtEuro": func(cents int) template.HTML {
			return template.HTML(strings.Replace(fmt.Sprintf("%.2f&nbsp;€", float64(cents)/100.0), ".", ",", 1))
		},
		"IsURL": func(s string) bool {
			return strings.HasPrefix(s, "https://")
		},
		"Markdown": func(input string) template.HTML {
			return template.HTML(md.RenderToString([]byte(input)))
		},
	})
	t = template.Must(t.Parse(captcha.TemplateString))
	t = template.Must(t.Parse(health.TemplateString))
	t = template.Must(t.ParseFS(files, fn...))
	return t
}

var (
	Error            = parse("template.html", "layout.html", "error.html")
	CustOrder        = parse("template.html", "layout.html", "customer/order.html")
	CustPurchase     = parse("template.html", "layout.html", "customer/purchase.html")
	Site             = parse("template.html", "layout.html", "site.html")
	StaffIndex       = parse("template.html", "layout.html", "staff.html", "staff/index.html")
	StaffView        = parse("template.html", "layout.html", "staff.html", "staff/view.html")
	StaffMarkPaid    = parse("template.html", "layout.html", "staff.html", "staff/mark-paid.html")
	StaffLogin       = parse("template.html", "layout.html", "staff/login.html")
	StaffSelect      = parse("template.html", "layout.html", "staff.html", "staff/select.html")
	StaffUploadImage = parse("template.html", "layout.html", "staff.html", "staff/upload-image.html")
	StaffUploadText  = parse("template.html", "layout.html", "staff.html", "staff/upload-text.html")
)

type CustOrderData struct {
	Articles             func() ([]*digitalgoods.Article, error)
	AvailableEUCountries []countries.CountryWithName
	AvailableNonEU       bool
	Catalog              digitalgoods.Catalog
	Stock                digitalgoods.Stock

	Captcha      captcha.TemplateData
	Cart         *digitalgoods.Cart
	OtherCountry map[string]string // user input: variant ID -> country ID
	Area         string
	EUCountry    string
	CountryErr   bool
	OrderErr     bool
	lang.Lang
}

type CustPurchaseData struct {
	GroupedOrder   []digitalgoods.OrderedArticle
	Purchase       *digitalgoods.Purchase
	PaymentMethod  payment.Method
	PaymentMethods []payment.Method
	URL            string
	PaysrvErr      error
	PreferOnion    bool
	lang.Lang
	ActiveTab string
	TabBTCPay string
	TabCash   string
	TabSepa   string
}
