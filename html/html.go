package html

import (
	"embed"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
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
	t = template.Must(t.ParseGlob(filepath.Join(os.Getenv("CONFIGURATION_DIRECTORY"), "*.html")))
	return t
}

var (
	CustError    = parse("digitalgoods.proxysto.re/*.html", "customer.html", "customer/error.html")
	CustOrder    = parse("digitalgoods.proxysto.re/*.html", "customer.html", "customer/order.html")
	CustPurchase = parse("digitalgoods.proxysto.re/*.html", "customer.html", "customer/purchase.html")
	CustSite     = parse("digitalgoods.proxysto.re/*.html", "customer.html")

	StaffError            = parse("staff.html", "staff/error.html")
	StaffIndex            = parse("staff.html", "staff/index.html")
	StaffView             = parse("staff.html", "staff/view.html")
	StaffMarkPaid         = parse("staff.html", "staff/mark-paid.html")
	StaffLogin            = parse("staff.html", "staff/login.html")
	StaffPurchaseNotFound = parse("staff.html", "staff/purchase-not-found.html")
	StaffSelect           = parse("staff.html", "staff/select.html")
	StaffUpload           = parse("staff.html", "staff/upload.html")
)

type CustErrorData struct {
	ssg.TemplateData
	Message string
}

type CustOrderData struct {
	ssg.TemplateData

	Articles             func() ([]*digitalgoods.Article, error)
	AvailableEUCountries []countries.CountryOption
	AvailableNonEU       bool
	Catalog              digitalgoods.Catalog
	Stock                digitalgoods.Stock

	Cart       *digitalgoods.Cart
	Area       string // tri-state: "eu", "non-eu" or empty
	EUCountry  string
	CountryErr bool
	OrderErr   bool
}

type CustPurchaseData struct {
	ssg.TemplateData

	ActivePaymentMethod string
	GroupedOrder        []digitalgoods.OrderedArticle
	PaymentMethods      []payment.Method
	Purchase            *digitalgoods.Purchase
	URL                 string
}
