package html

import (
	"embed"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strings"

	"github.com/dys2p/digitalgoods"
	"github.com/dys2p/eco/payment"
	"gitlab.com/golang-commonmark/markdown"
	"golang.org/x/text/language"
)

//go:embed *
var files embed.FS

var md = markdown.New(markdown.HTML(true), markdown.Linkify(false))

type LangTemplate map[string]*template.Template

func (t LangTemplate) Execute(w io.Writer, lang string, data any) error {
	// prepare matcher
	keys := []string{}
	tags := []language.Tag{}
	for key := range t {
		keys = append(keys, key)
		tags = append(tags, language.Make(key))
	}
	matcher := language.NewMatcher(tags)
	// match
	_, i := language.MatchStrings(matcher, lang)
	return t[keys[i]].Execute(w, data)
}

func parse(fn ...string) *template.Template {
	return template.Must(template.New(fn[0]).Funcs(template.FuncMap{
		"AlertContextualClass": func(status string) string {
			switch status {
			case digitalgoods.StatusNew:
				return "alert-primary"
			case "btcpay-created": // temp, backward compatibility
				return "alert-primary"
			case "btcpay-processing": // temp, backward compatibility
				return "alert-success"
			case digitalgoods.StatusPaymentProcessing:
				return "alert-success"
			case "btcpay-expired": // temp, backward compatibility
				return "alert-danger"
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
	}).ParseFS(files, fn...))
}

var (
	Error     = parse("layout.en.html", "error.html")
	CustOrder = LangTemplate{
		"en": parse("layout.en.html", "customer/order.html"),
		"de": parse("layout.de.html", "customer/order.html"),
	}
	CustPurchase = LangTemplate{
		"en": parse("layout.en.html", "customer/purchase.html"),
		"de": parse("layout.de.html", "customer/purchase.html"),
	}
	Site = LangTemplate{
		"en": parse("layout.en.html", "site.html"),
		"de": parse("layout.de.html", "site.html"),
	}
	StaffIndex       = parse("layout.de.html", "staff.html", "staff/index.html")
	StaffView        = parse("layout.de.html", "staff.html", "staff/view.html")
	StaffMarkPaid    = parse("layout.de.html", "staff.html", "staff/mark-paid.html")
	StaffLogin       = parse("layout.de.html", "staff/login.html")
	StaffSelect      = parse("layout.de.html", "staff.html", "staff/select.html")
	StaffUploadImage = parse("layout.de.html", "staff.html", "staff/upload-image.html")
	StaffUploadText  = parse("layout.de.html", "staff.html", "staff/upload-text.html")
)

type CustOrderData struct {
	ArticlesByCategory func(category *digitalgoods.Category) ([]digitalgoods.Article, error)
	Categories         func() ([]*digitalgoods.Category, error)
	EUCountryCodes     []string

	CaptchaAnswer string
	CaptchaErr    bool
	CaptchaID     string
	Cart          map[string]int    // user input: HTML input name -> amount
	OtherCountry  map[string]string // user input: article ID -> country ID
	CountryAnswer string
	CountryErr    bool
	OrderErr      bool
	Language
}

type CustPurchaseData struct {
	GroupedOrder func(order digitalgoods.Order) ([]digitalgoods.OrderGroup, error)

	Purchase       *digitalgoods.Purchase
	PaymentMethod  payment.Method
	PaymentMethods []payment.Method
	URL            string
	PaysrvErr      error
	PreferOnion    bool
	Language
	HTTPRequest *http.Request
	ActiveTab   string
	TabBTCPay   string
	TabCash     string
	TabSepa     string
}
