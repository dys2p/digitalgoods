package html

import (
	"embed"
	"html/template"
)

//go:embed *
var files embed.FS

func parse(fn ...string) *template.Template {
	fn = append([]string{"layout.html"}, fn...)
	return template.Must(template.Must(template.New("layout.html").ParseFS(files, fn...)).ParseGlob("data/custom.html"))
}

var (
	Error                = parse("error.html")
	CustOrder            = parse("customer/order.html")
	CustPurchase         = parse("customer/purchase.html")
	StaffIndex           = parse("staff.html", "staff/index.html")
	StaffMarkPaid        = parse("staff.html", "staff/mark-paid.html")
	StaffMarkPaidConfirm = parse("staff.html", "staff/mark-paid-confirm.html")
	StaffLogin           = parse("staff/login.html")
	StaffSelect          = parse("staff.html", "staff/select.html")
	StaffUploadImage     = parse("staff.html", "staff/upload-image.html")
	StaffUploadText      = parse("staff.html", "staff/upload-text.html")
)
