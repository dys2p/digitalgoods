package html

import (
	"embed"
	"fmt"
	"html/template"
	"strings"

	"github.com/dys2p/digitalgoods"
	"github.com/dys2p/eco/countries"
	"github.com/dys2p/eco/payment"
	"github.com/dys2p/eco/payment/health"
	"github.com/dys2p/eco/ssg"
	"gitlab.com/golang-commonmark/markdown"
)

//go:embed *
var Files embed.FS

var md = markdown.New(markdown.HTML(true), markdown.Linkify(false))

func parse(fn ...string) *template.Template {
	t := template.New("html").Funcs(template.FuncMap{
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
	t = template.Must(t.Parse(health.TemplateString))
	t = template.Must(t.ParseFS(Files, fn...))
	return t
}

var (
	Error            = parse("digitalgoods.proxysto.re/*.html", "layout.html", "error.html")
	CustOrder        = parse("digitalgoods.proxysto.re/*.html", "layout.html", "customer/order.html")
	CustPurchase     = parse("digitalgoods.proxysto.re/*.html", "layout.html", "customer/purchase.html")
	Layout           = parse("layout.html")
	StaffIndex       = parse("digitalgoods.proxysto.re/*.html", "layout.html", "staff.html", "staff/index.html")
	StaffView        = parse("digitalgoods.proxysto.re/*.html", "layout.html", "staff.html", "staff/view.html")
	StaffMarkPaid    = parse("digitalgoods.proxysto.re/*.html", "layout.html", "staff.html", "staff/mark-paid.html")
	StaffLogin       = parse("digitalgoods.proxysto.re/*.html", "layout.html", "staff/login.html")
	StaffSelect      = parse("digitalgoods.proxysto.re/*.html", "layout.html", "staff.html", "staff/select.html")
	StaffUploadImage = parse("digitalgoods.proxysto.re/*.html", "layout.html", "staff.html", "staff/upload-image.html")
	StaffUploadText  = parse("digitalgoods.proxysto.re/*.html", "layout.html", "staff.html", "staff/upload-text.html")
)

type CustOrderData struct {
	ssg.TemplateData

	Articles             func() ([]*digitalgoods.Article, error)
	AvailableEUCountries []countries.CountryWithName
	AvailableNonEU       bool
	Catalog              digitalgoods.Catalog
	Stock                digitalgoods.Stock

	Cart         *digitalgoods.Cart
	OtherCountry map[string]string // user input: variant ID -> country ID
	Area         string
	EUCountry    string
	CountryErr   bool
	OrderErr     bool
}

type CustPurchaseData struct {
	ssg.TemplateData
	GroupedOrder   []digitalgoods.OrderedArticle
	Purchase       *digitalgoods.Purchase
	PaymentMethod  payment.Method
	PaymentMethods []payment.Method
	URL            string
	PaysrvErr      error
	PreferOnion    bool
	ActiveTab      string
	TabBTCPay      string
	TabCash        string
	TabSepa        string
}
