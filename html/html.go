package html

import (
	"embed"
	"fmt"
	"html/template"
	"strings"

	"github.com/dys2p/digitalgoods"
)

//go:embed *
var files embed.FS

func parse(fn ...string) *template.Template {
	fn = append([]string{"layout.html"}, fn...)
	return template.Must(template.Must(template.New("layout.html").Funcs(template.FuncMap{
		"FmtEuro": func(cents int) template.HTML {
			return template.HTML(strings.Replace(fmt.Sprintf("%.2f&nbsp;€", float64(cents)/100.0), ".", ",", 1))
		},
	}).ParseFS(files, fn...)).ParseGlob("data/custom.html"))
}

var (
	Error            = parse("error.html")
	CustOrder        = parse("customer/order.html")
	CustPurchase     = parse("customer/purchase.html")
	StaffIndex       = parse("staff.html", "staff/index.html")
	StaffView        = parse("staff.html", "staff/view.html")
	StaffMarkPaid    = parse("staff.html", "staff/mark-paid.html")
	StaffLogin       = parse("staff/login.html")
	StaffSelect      = parse("staff.html", "staff/select.html")
	StaffUploadImage = parse("staff.html", "staff/upload-image.html")
	StaffUploadText  = parse("staff.html", "staff/upload-text.html")
)

type CustOrderData struct {
	ArticlesByCategory func(category *digitalgoods.Category) ([]digitalgoods.Article, error)
	Categories         func() ([]*digitalgoods.Category, error)
	EUCountryCodes     []string

	CaptchaAnswer string
	CaptchaErr    bool
	CaptchaID     string
	Cart          map[string]int    // user input, in case of errors: HTML input name -> amount
	OtherCountry  map[string]string // user input, in case of errors: article ID -> country ID
	CountryAnswer string
	CountryErr    bool
	OrderErr      bool
	Language
}

type CustPurchaseData struct {
	GetArticle   func(id string) (digitalgoods.Article, error)
	GroupedOrder func(order digitalgoods.Order) ([]digitalgoods.OrderGroup, error)

	Purchase    *digitalgoods.Purchase
	URL         string
	PaysrvErr   error
	PreferOnion bool
	Language
	ActiveTab string
	TabBTCPay string
	TabCash   string
	TabSepa   string
}
