package main

import (
	"database/sql"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/alexedwards/scs/sqlite3store"
	"github.com/alexedwards/scs/v2"
	"github.com/alexedwards/scs/v2/memstore"
	"github.com/dys2p/digitalgoods"
	"github.com/dys2p/digitalgoods/db"
	"github.com/dys2p/digitalgoods/html"
	"github.com/dys2p/digitalgoods/userdb"
	"github.com/dys2p/eco/countries"
	"github.com/dys2p/eco/countries/detect"
	"github.com/dys2p/eco/email"
	"github.com/dys2p/eco/httputil"
	"github.com/dys2p/eco/id"
	"github.com/dys2p/eco/lang"
	"github.com/dys2p/eco/ntfysh"
	"github.com/dys2p/eco/payment"
	"github.com/dys2p/eco/payment/health"
	"github.com/dys2p/eco/payment/rates"
	"github.com/dys2p/eco/productfeed"
	"github.com/dys2p/eco/ssg"
	"github.com/dys2p/go-btcpay"
	"github.com/julienschmidt/httprouter"
	_ "github.com/mattn/go-sqlite3"
)

type Shop struct {
	Btcpay           btcpay.Store
	CustomerSessions *scs.SessionManager
	Database         *db.DB
	Emailer          email.Emailer
	Langs            lang.Languages
	PaymentMethods   []payment.Method
	ProductFeed      productfeed.Feed
	RatesHistory     *rates.History
	StaffSessions    *scs.SessionManager
	StaffUsers       userdb.Authenticator
	VATRate          func(digitalgoods.Sale) string
}

var CatalogUpdated string // go build -ldflags "-X main.CatalogUpdated=$(date --iso-8601=seconds --utc -r path/to/product-catalog.go)"

var staffLang, _, _ = lang.MakeLanguages(nil, "de", "en").FromPath("de")

func main() {
	log.SetFlags(0)

	// test mode
	var test = flag.Bool("test", false, "use btcpay dummy store")
	flag.Parse()

	// order db
	database, err := db.OpenDB()
	if err != nil {
		log.Printf("error opening database: %v", err)
		return
	}

	// btcpay
	var btcpayStore btcpay.Store
	if *test {
		btcpayStore = btcpay.NewDummyStore()
		log.Println("\033[33m" + "warning: using btcpay dummy store" + "\033[0m")
	} else {
		btcpayStore, err = btcpay.Load(filepath.Join(os.Getenv("CONFIGURATION_DIRECTORY"), "btcpay.json"))
		if err != nil {
			log.Printf("error loading btcpay store: %v", err)
			return
		}
	}

	// emailer
	var emailer email.Emailer
	if *test {
		emailer = email.DummyMailer{}
		log.Println("\033[33m" + "warning: using dummy emailer" + "\033[0m")
	} else {
		emailer = email.Sendmail{
			From: emailFrom,
		}
	}

	// foreign currency cash
	ratesDB, err := rates.OpenDB(filepath.Join(os.Getenv("STATE_DIRECTORY"), "rates.sqlite3"))
	if err != nil {
		log.Printf("error opening rates db: %v", err)
		return
	}
	ratesHistory := &rates.History{
		Database:    ratesDB,
		GetBuyRates: GetBuyRates,
	}
	go ratesHistory.RunDaemon()

	// staff users
	staffUsers, err := userdb.Open(filepath.Join(os.Getenv("CONFIGURATION_DIRECTORY"), "users.json"))
	if err != nil {
		log.Printf("error opening userdb: %v", err)
		return
	}

	// customer sessions
	custSessionsDB, err := sql.Open("sqlite3", filepath.Join(os.Getenv("STATE_DIRECTORY"), "customer-sessions.sqlite3"))
	if err != nil {
		log.Printf("error opening customer session database: %v", err)
		return
	}
	defer custSessionsDB.Close()
	if _, err = custSessionsDB.Exec(`
		CREATE TABLE IF NOT EXISTS sessions (
			token TEXT PRIMARY KEY,
			data BLOB NOT NULL,
			expiry REAL NOT NULL
		);
		CREATE INDEX IF NOT EXISTS sessions_expiry_idx ON sessions(expiry);
	`); err != nil {
		log.Printf("error creating customer sessions table: %v", err)
		return
	}
	custSessions := scs.New()
	custSessions.Cookie.SameSite = http.SameSiteLaxMode // prevent CSRF
	custSessions.Lifetime = 8 * time.Hour
	custSessions.Store = sqlite3store.New(custSessionsDB)

	// staff sessions
	staffSessions := scs.New()
	staffSessions.Cookie.SameSite = http.SameSiteLaxMode // prevent CSRF
	staffSessions.Store = memstore.New()

	// shop
	s := &Shop{
		Btcpay:           btcpayStore,
		Database:         database,
		Emailer:          emailer,
		Langs:            lang.MakeLanguages(nil, "de", "en"),
		RatesHistory:     ratesHistory,
		CustomerSessions: custSessions,
		StaffSessions:    staffSessions,
		StaffUsers:       staffUsers,
		VATRate:          vatRate,
	}

	s.ProductFeed = productfeed.Feed{
		ID:       "https://digitalgoods.proxysto.re",
		Title:    "Digital Goods by ProxyStore",
		Updated:  CatalogUpdated,
		Products: catalog.Products(),
	}

	// payment methods (need shop variable)
	s.PaymentMethods = []payment.Method{
		payment.BTCPay{
			Purchases:    s,
			RedirectPath: "/by-cookie",
			Store:        btcpayStore,
			CreateInvoiceError: func(err error, msg string) http.Handler {
				return s.frontendErr(err, msg)
			},
			WebhookError: func(err error) http.Handler {
				log.Printf("webhook error: %v", err)
				ntfysh.Publish(ntfyshLog, "digitalgoods error", err.Error())
				return nil
			},
		},
		payment.Cash{
			AddressHTML: addressHTML,
		},
		payment.CashForeign{
			AddressHTML: addressHTML,
			History:     ratesHistory,
			Purchases:   s,
		},
		payment.SEPA{
			Account:   sepaAccount,
			Purchases: s,
		},
	}

	s.ListenAndServe()
}

func (s *Shop) ListenAndServe() {

	var stop = make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	siteFiles, err := fs.Sub(html.Files, "digitalgoods.proxysto.re")
	if err != nil {
		log.Fatalf("error opening site dir: %v", err)
	}
	staticFiles, err := fs.Sub(html.Files, "digitalgoods.proxysto.re/static") // for staff router
	if err != nil {
		log.Fatalf("error opening static dir: %v", err)
	}

	staticSites, err := ssg.MakeWebsite(siteFiles, html.CustSite, s.Langs)
	if err != nil {
		log.Fatalf("error making static sites: %v", err)
	}

	healthSrv := &health.Server{
		BTCPay: s.Btcpay,
		Rates:  s.RatesHistory,
	}
	go healthSrv.Run()

	// customer http server

	var custRtr = httprouter.New()
	custRtr.ServeFiles("/static/*filepath", http.FS(httputil.ModTimeFS{staticFiles, time.Now()})) // can be omitted when ssg.Handler sets the modification time
	for _, l := range s.Langs {
		custRtr.Handler(http.MethodGet, "/"+l.Prefix, httputil.HandlerFunc(s.custOrderGet))
		custRtr.Handler(http.MethodPost, "/"+l.Prefix, httputil.HandlerFunc(s.custOrderPost))
		custRtr.Handler(http.MethodGet, "/"+l.Prefix+"/order/:id/:access-key", httputil.HandlerFunc(s.custPurchaseGet))
		custRtr.Handler(http.MethodPost, "/"+l.Prefix+"/order/:id/:access-key", httputil.HandlerFunc(s.custPurchasePost))
		custRtr.Handler(http.MethodGet, "/"+l.Prefix+"/order/:id/:access-key/:payment", httputil.HandlerFunc(s.custPurchaseGet))
		custRtr.Handler(http.MethodPost, "/"+l.Prefix+"/order/:id/:access-key/:payment", httputil.HandlerFunc(s.custPurchasePost))
	}
	for _, method := range s.PaymentMethods {
		custRtr.Handler(http.MethodPost, fmt.Sprintf("/payment/%s/*path", method.ID()), method)
	}
	custRtr.HandlerFunc(http.MethodGet, "/by-cookie", s.byCookie)
	custRtr.HandlerFunc(http.MethodGet, "/productfeed.xml", func(w http.ResponseWriter, r *http.Request) {
		bs, _ := s.ProductFeed.Bytes()
		w.Write(bs)
	})
	custRtr.Handler(http.MethodGet, "/payment-health", healthSrv)
	custRtr.NotFound = staticSites.Handler(nil, s.Langs.RedirectHandler())

	shutdownCust := httputil.ListenAndServe(":9002", s.CustomerSessions.LoadAndSave(custRtr), stop)
	defer shutdownCust()

	log.Println("listening to port 9002")

	// staff http server

	var staffAuthRouter = httprouter.New()
	staffAuthRouter.HandlerFunc(http.MethodGet, "/", s.showErr(s.staffIndexGet))
	staffAuthRouter.HandlerFunc(http.MethodGet, "/logout", s.showErr(s.staffLogoutGet))
	staffAuthRouter.HandlerFunc(http.MethodGet, "/export/:from", s.showErr(s.staffExportGet))
	staffAuthRouter.HandlerFunc(http.MethodGet, "/view", s.showErr(s.staffViewGet))
	staffAuthRouter.HandlerFunc(http.MethodPost, "/view", s.showErr(s.staffViewPost))
	staffAuthRouter.HandlerFunc(http.MethodGet, "/mark-paid/:id", s.showErr(s.staffMarkPaidGet))
	staffAuthRouter.HandlerFunc(http.MethodPost, "/mark-paid/:id", s.showErr(s.staffMarkPaidPost))
	staffAuthRouter.HandlerFunc(http.MethodGet, "/upload", s.showErr(s.staffSelectGet))
	staffAuthRouter.HandlerFunc(http.MethodGet, "/upload/:variant", s.showErr(s.staffUploadGet))
	staffAuthRouter.HandlerFunc(http.MethodPost, "/upload/:variant", returnErr(s.staffUploadPost))

	var staffRtr = httprouter.New()
	staffRtr.ServeFiles("/static/*filepath", http.FS(httputil.ModTimeFS{staticFiles, time.Now()}))
	staffRtr.HandlerFunc(http.MethodGet, "/login", s.showErr(s.staffLoginGet))
	staffRtr.HandlerFunc(http.MethodPost, "/login", s.showErr(s.staffLoginPost))
	staffRtr.NotFound = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.StaffSessions.Exists(r.Context(), "username") {
			staffAuthRouter.ServeHTTP(w, r)
		} else {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
		}
	})

	shutdownStaff := httputil.ListenAndServe("127.0.0.1:9003", s.StaffSessions.LoadAndSave(staffRtr), stop)
	defer shutdownStaff()

	// cleanup bot

	var wg sync.WaitGroup
	defer wg.Wait()

	go func() {
		for ; true; <-time.Tick(12 * time.Hour) {
			wg.Add(1)
			if err := s.Database.Cleanup(); err != nil {
				log.Printf("error cleaning up database: %v", err)
			}
			wg.Done()
		}
	}()

	// notify us
	if err := s.Emailer.Send(emailFrom, "digitalgoods service started", []byte("the digitalgoods service has been started")); err != nil {
		log.Println(err)
	}
	if err := ntfysh.Publish(ntfyshLog, "digitalgoods service started", "the digitalgoods service has been started"); err != nil {
		log.Println(err)
	}

	// run until we receive an interrupt or any of the listeners fails

	log.Printf("running")
	<-stop
	log.Println("shutting down")
}

// frontend error handler, logs err and displays a message
func (s *Shop) frontendErr(err error, message string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		html.CustError.Execute(w, html.CustErrorData{
			TemplateData: s.MakeTemplateData(r),
			Message:      message,
		})

		if err != nil {
			log.Printf("internal server error: %v", err)
			ntfysh.Publish(ntfyshLog, "digitalgoods error", err.Error())
		}
	})
}

// frontend notfound handler, logs err and displays a message
func (s *Shop) frontendNotFound(message string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		html.CustError.Execute(w, html.CustErrorData{
			TemplateData: s.MakeTemplateData(r),
			Message:      message,
		})
	})
}

// middleware for backend POST API
func returnErr(f func(http.ResponseWriter, *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := f(w, r); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
		}
	}
}

// middleware for backend HTML GET only
func (s *Shop) showErr(f func(http.ResponseWriter, *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := f(w, r); err != nil {
			html.StaffError.Execute(w, err.Error())
		}
	}
}

func (s *Shop) custOrderGet(w http.ResponseWriter, r *http.Request) http.Handler {
	l, _, _ := s.Langs.FromPath(r.URL.Path)

	availableEUCountries, availableNonEU, err := detect.Countries(r)
	if err != nil {
		log.Printf("error detecting countries: %v", err)
	}

	// pre-select area if it's known
	var area string
	if len(availableEUCountries) == 0 {
		area = "non-eu"
	}
	if !availableNonEU {
		area = "eu"
	}

	stock, err := s.Database.GetStock()
	if err != nil {
		return s.frontendErr(err, l.Tr("Error getting stock from database. Please try again later."))
	}

	err = html.CustOrder.Execute(w, &html.CustOrderData{
		TemplateData: s.MakeTemplateData(r),

		AvailableEUCountries: countries.TranslateAndSort(l, availableEUCountries, countries.Country("")),
		AvailableNonEU:       availableNonEU,
		Catalog:              catalog,
		Stock:                stock,

		Area: area,
	})
	if err != nil {
		return s.frontendErr(err, l.Tr("Error displaying website. Please try again later."))
	}
	return nil
}

func (s *Shop) custOrderPost(w http.ResponseWriter, r *http.Request) http.Handler {
	l, _, _ := s.Langs.FromPath(r.URL.Path)

	if val := r.PostFormValue("n-o-b-o-t-s"); val != "" {
		return s.frontendErr(nil, l.Tr("Our service thinks that you are a bot. If you are not, please contact us."))
	}

	availableEUCountries, availableNonEU, err := detect.Countries(r)
	if err != nil {
		log.Printf("error detecting countries: %v", err)
	}

	stock, err := s.Database.GetStock()
	if err != nil {
		return s.frontendErr(err, l.Tr("Error getting stock from database. Please try again later."))
	}

	// read user input

	selectedEUCountry, _ := countries.Get(countries.EuropeanUnion, r.PostFormValue("eu-country"))

	// like in order template
	var cart digitalgoods.Cart   // for page reload in case of error
	var order digitalgoods.Order // in case of no errors
	for _, category := range catalog {
		for _, article := range category.Articles {
			for _, variant := range article.Variants {
				quantity, _ := strconv.Atoi(r.PostFormValue(variant.ID))
				if quantity > 100000 { // just to prevent overflow issues
					quantity = 100000
				}

				if quantity > 0 {
					cart.Add(variant.ID, quantity)
					order = append(order, digitalgoods.OrderRow{
						Quantity:  quantity,
						VariantID: variant.ID,
						ItemPrice: variant.Price,
					})
				}
			}
		}
	}

	// validate user input

	co := &html.CustOrderData{
		TemplateData: s.MakeTemplateData(r),

		AvailableEUCountries: countries.TranslateAndSort(l, availableEUCountries, selectedEUCountry),
		AvailableNonEU:       availableNonEU,
		Catalog:              catalog,
		Stock:                stock,

		Cart:      &cart,
		Area:      r.PostFormValue("area"),
		EUCountry: string(selectedEUCountry),
	}

	if len(order) == 0 {
		co.OrderErr = true
		html.CustOrder.Execute(w, co)
		return nil
	}

	var country countries.Country
	if co.Area == "non-eu" {
		country = countries.NonEU
	} else {
		country = countries.Country(co.EUCountry)
		if !countries.InEuropeanUnion(country) {
			co.CountryErr = true
			html.CustOrder.Execute(w, co)
			return nil
		}
	}

	purchase := &digitalgoods.Purchase{
		AccessKey:   id.New(16, id.AlphanumCaseSensitiveDigits), // 16 digits * log2(58) = 94 bits
		PaymentKey:  id.New(16, id.AlphanumCaseSensitiveDigits), // 16 digits * log2(58) = 94 bits
		Status:      digitalgoods.StatusNew,
		Ordered:     order,
		CreateDate:  time.Now().Format("2006-01-02"),
		DeleteDate:  time.Now().AddDate(0, 0, 31).Format("2006-01-02"),
		CountryCode: string(country),
	}

	if err := s.Database.InsertPurchase(purchase); err != nil {
		return s.frontendErr(err, l.Tr("Error inserting purchase into database. Please try again later."))
	}

	// set cookie
	redirectPath := path.Join("/", l.Prefix, "order", purchase.ID, purchase.AccessKey)
	s.CustomerSessions.Put(r.Context(), "redirect-path", redirectPath)
	return http.RedirectHandler(redirectPath, http.StatusSeeOther)
}

func (s *Shop) custPurchaseGet(w http.ResponseWriter, r *http.Request) http.Handler {
	l, _, _ := s.Langs.FromPath(r.URL.Path)
	params := httprouter.ParamsFromContext(r.Context())
	purchase, err := s.Database.GetPurchaseByIDAndAccessKey(params.ByName("id"), params.ByName("access-key"))
	if err != nil {
		return s.frontendNotFound(l.Tr("There is no such purchase, or it has been deleted."))
	}

	err = html.CustPurchase.Execute(w, &html.CustPurchaseData{
		TemplateData: s.MakeTemplateData(r),

		ActivePaymentMethod: params.ByName("payment"),
		GroupedOrder:        catalog.GroupOrder(purchase.Ordered),
		PaymentMethods:      s.PaymentMethods,
		Purchase:            purchase,
		URL:                 httputil.SchemeHost(r) + path.Join("/", l.Prefix, "order", purchase.ID, purchase.AccessKey),
	})
	if err != nil {
		return s.frontendErr(err, l.Tr("Error displaying website. Please try again later."))
	}
	return nil
}

func (s *Shop) custPurchasePost(w http.ResponseWriter, r *http.Request) http.Handler {
	l, _, _ := s.Langs.FromPath(r.URL.Path)
	params := httprouter.ParamsFromContext(r.Context())
	purchase, err := s.Database.GetPurchaseByIDAndAccessKey(params.ByName("id"), params.ByName("access-key"))
	if err != nil {
		return s.frontendNotFound(l.Tr("There is no such purchase, or it has been deleted."))
	}

	notifyProto := r.PostFormValue("notify-proto")
	notifyAddr := strings.TrimSpace(r.PostFormValue("notify-addr"))
	if len(notifyAddr) > 1024 {
		notifyAddr = notifyAddr[:1024]
	}

	// reset proto if addr is empty
	if notifyAddr == "" {
		notifyProto = ""
	}

	// if proto is empty, guess it
	if notifyProto == "" {
		if strings.Contains(notifyAddr, "@") {
			notifyProto = "email"
		}
	}

	switch notifyProto {
	case "email":
		notifyAddr = strings.TrimSpace(notifyAddr)
		if !email.AddressValid(notifyAddr) {
			notifyAddr = ""
		}
	case "ntfysh":
		notifyAddr = ntfysh.ValidateAddress(notifyAddr)
	default:
		notifyAddr = ""
		notifyProto = ""
	}

	purchase.NotifyProto = notifyProto
	purchase.NotifyAddr = notifyAddr
	if err := s.Database.SetNotify(purchase); err != nil {
		return s.frontendErr(err, l.Tr("Error saving notify information. Please try again later."))
	}

	return http.RedirectHandler(r.URL.Path+"#notify", http.StatusSeeOther)
}

func (s *Shop) byCookie(w http.ResponseWriter, r *http.Request) {
	// TODO maybe save language in cookie and redirect to user's locale
	if redirectPath := s.CustomerSessions.GetString(r.Context(), "redirect-path"); redirectPath != "" {
		http.Redirect(w, r, redirectPath, http.StatusSeeOther)
	} else {
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func (s *Shop) staffIndexGet(w http.ResponseWriter, r *http.Request) error {
	underdelivered, err := s.Database.GetPurchases(digitalgoods.StatusUnderdelivered)
	if err != nil {
		return err
	}
	return html.StaffIndex.Execute(w, struct {
		Underdelivered []string
	}{
		Underdelivered: underdelivered,
	})
}

func (s *Shop) staffLoginGet(w http.ResponseWriter, r *http.Request) error {
	return html.StaffLogin.Execute(w, nil)
}

func (s *Shop) staffLoginPost(w http.ResponseWriter, r *http.Request) error {
	username := r.PostFormValue("username")
	password := r.PostFormValue("password")
	if err := s.StaffUsers.Authenticate(username, password); err != nil {
		return err
	}
	s.StaffSessions.Put(r.Context(), "username", username)
	http.Redirect(w, r, "/", http.StatusSeeOther)
	return nil
}

func (s *Shop) staffLogoutGet(w http.ResponseWriter, r *http.Request) error {
	s.StaffSessions.Destroy(r.Context())
	http.Redirect(w, r, "/", http.StatusSeeOther)
	return nil
}

func (s *Shop) staffExportGet(w http.ResponseWriter, r *http.Request) error {
	minDate := httprouter.ParamsFromContext(r.Context()).ByName("from")
	sales, err := s.Database.GetSales(minDate)
	if err != nil {
		return err
	}

	// set VAT rates
	for i := range sales {
		sales[i].VATRate = s.VATRate(sales[i])
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	out := csv.NewWriter(w)
	out.Write([]string{"pay_date", "id", "country", "gross", "vat_rate", "name"})
	for _, sale := range sales {
		out.Write([]string{sale.PayDate, sale.ID, sale.Country, strconv.Itoa(sale.Gross), sale.VATRate, sale.Name})
	}
	out.Flush()
	return nil
}

func (s *Shop) staffViewGet(w http.ResponseWriter, r *http.Request) error {
	return html.StaffView.Execute(w, nil)
}

func (s *Shop) staffViewPost(w http.ResponseWriter, r *http.Request) error {
	id := strings.ToUpper(strings.TrimSpace(r.PostFormValue("id")))
	if purchase, err := s.Database.GetPurchaseByID(id); err == nil {
		http.Redirect(w, r, fmt.Sprintf("/mark-paid/%s", purchase.ID), http.StatusSeeOther)
		return nil
	}

	var didYouMean []string
	if 3 <= len(id) && len(id) <= 5 {
		ids, err := s.Database.GetIDsByPattern("%" + id + "%")
		if err != nil {
			return err
		}
		didYouMean = append(didYouMean, ids...)
	}
	if len(id) == 6 {
		for i := range len(id) {
			ids, err := s.Database.GetIDsByPattern(id[:i] + "_" + id[i+1:])
			if err != nil {
				return err
			}
			didYouMean = append(didYouMean, ids...)
		}
	}
	slices.Sort(didYouMean)
	didYouMean = slices.Compact(didYouMean)
	return html.StaffPurchaseNotFound.Execute(w, didYouMean)
}

func (s *Shop) staffMarkPaidGet(w http.ResponseWriter, r *http.Request) error {
	id := strings.ToUpper(strings.TrimSpace(httprouter.ParamsFromContext(r.Context()).ByName("id")))
	purchase, err := s.Database.GetPurchaseByID(id)
	if err != nil {
		return err
	}
	currencyOptions, _ := s.RatesHistory.Options(purchase.CreateDate, float64(purchase.Ordered.Sum())/100.0)

	return html.StaffMarkPaid.Execute(w, struct {
		*digitalgoods.Purchase
		GroupedOrder    []digitalgoods.OrderedArticle
		CurrencyOptions []rates.Option
		EUCountries     []countries.CountryOption
	}{
		Purchase:        purchase,
		GroupedOrder:    catalog.GroupOrder(purchase.Ordered),
		CurrencyOptions: currencyOptions,
		EUCountries:     countries.TranslateAndSort(staffLang, countries.EuropeanUnion, countries.Country("")),
	})
}

func (s *Shop) staffMarkPaidPost(w http.ResponseWriter, r *http.Request) error {
	if r.PostFormValue("confirm") == "" {
		return errors.New("You did not confirm.")
	}
	id := r.PostFormValue("id")
	purchase, err := s.Database.GetPurchaseByID(id)
	if err != nil {
		return err
	}
	countryCode := r.PostFormValue("country")
	if purchase.CountryCode != countryCode {
		if err := s.Database.SetCountry(purchase, countryCode); err != nil {
			return err
		}
	}
	if err := s.Database.SetSettled(purchase); err != nil {
		return err
	}
	if err := s.NotifyPaymentReceived(purchase); err != nil {
		return err
	}
	http.Redirect(w, r, fmt.Sprintf("/mark-paid/%s", purchase.ID), http.StatusSeeOther)
	return nil
}

func (s *Shop) staffSelectGet(w http.ResponseWriter, r *http.Request) error {

	underdeliveredPurchaseIDs, err := s.Database.GetPurchases(digitalgoods.StatusUnderdelivered)
	if err != nil {
		return err
	}
	underdelivered := make(map[string]int)
	for _, purchaseID := range underdeliveredPurchaseIDs {
		purchase, err := s.Database.GetPurchaseByID(purchaseID)
		if err != nil {
			return err
		}
		unfulfilled, err := purchase.GetUnfulfilled()
		if err != nil {
			return err
		}
		for _, uf := range unfulfilled {
			underdelivered[uf.VariantID] += uf.Quantity
		}
	}

	stock, err := s.Database.GetStock()
	if err != nil {
		return err
	}

	return html.StaffSelect.Execute(w, struct {
		Catalog        digitalgoods.Catalog
		Stock          digitalgoods.Stock
		Underdelivered map[string]int // key: variant id
	}{
		Catalog:        catalog,
		Stock:          stock,
		Underdelivered: underdelivered,
	})
}

func (s *Shop) staffUploadGet(w http.ResponseWriter, r *http.Request) error {
	variant, err := catalog.Variant(httprouter.ParamsFromContext(r.Context()).ByName("variant"))
	if err != nil {
		return err
	}
	stock, err := s.Database.GetStock()
	if err != nil {
		return err
	}

	return html.StaffUpload.Execute(w, struct {
		digitalgoods.Variant
		Stock int
	}{
		Variant: variant,
		Stock:   stock[variant.ID],
	})
}

func (s *Shop) staffUploadPost(w http.ResponseWriter, r *http.Request) error {

	var variantID = httprouter.ParamsFromContext(r.Context()).ByName("variant")

	for _, code := range strings.Fields(r.PostFormValue("codes")) {
		if err := s.Database.AddToStock(variantID, code); err == nil {
			log.Printf("added code to stock: %s %s", variantID, digitalgoods.Mask(code))
		} else {
			log.Println(err)
			return err
		}
	}

	if err := s.Database.FulfilUnderdelivered(); err != nil {
		return err
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
	return nil
}

func (s *Shop) PurchaseCreationDate(id, paymentKey string) (string, error) {
	purchase, err := s.Database.GetPurchaseByIDAndPaymentKey(id, paymentKey)
	if err != nil {
		return "", err
	}
	if purchase.CreateDate == "" {
		purchase.CreateDate = time.Now().Format("2006-01-02") // TODO
	}
	return purchase.CreateDate, nil
}

func (s *Shop) PurchaseSumCents(id, paymentKey string) (int, error) {
	purchase, err := s.Database.GetPurchaseByIDAndPaymentKey(id, paymentKey)
	if err != nil {
		return 0, err
	}
	return purchase.Ordered.Sum(), nil
}

func (s *Shop) SetPurchasePaid(id, paymentKey, methodName string) error {
	purchase, err := s.Database.GetPurchaseByIDAndPaymentKey(id, paymentKey)
	if err != nil {
		return err
	}
	if err := s.Database.SetSettled(purchase); err != nil {
		return err
	}
	return s.NotifyPaymentReceived(purchase)
}

func (s *Shop) SetPurchaseProcessing(id, paymentKey string) error {
	purchase, err := s.Database.GetPurchaseByIDAndPaymentKey(id, paymentKey)
	if err != nil {
		return err
	}
	return s.Database.SetProcessing(purchase)
}

func (s *Shop) NotifyPaymentReceived(purchase *digitalgoods.Purchase) error {
	const subject = "digitalgoods.proxysto.re payment received"
	const msg = "We have received your payment. Please download your vouchers within the next 30 days."

	switch purchase.NotifyProto {
	case "email":
		err := s.Emailer.Send(purchase.NotifyAddr, subject, []byte(msg))
		if err != nil {
			return fmt.Errorf("sending email notification: %w", err)
		}
	case "ntfysh":
		err := ntfysh.Publish(purchase.NotifyAddr, subject, msg)
		if err != nil {
			return fmt.Errorf("sending ntfysh notification: %w", err)
		}
	}

	if purchase.Status == digitalgoods.StatusFinalized {
		purchase.NotifyProto = ""
		purchase.NotifyAddr = ""
		err := s.Database.SetNotify(purchase)
		if err != nil {
			return fmt.Errorf("removing notify data from database: %w", err)
		}
	}
	return nil
}

func (s *Shop) MakeTemplateData(r *http.Request) ssg.TemplateData {
	l, path, _ := s.Langs.FromPath(r.URL.Path)
	return ssg.TemplateData{
		Lang:      l,
		Languages: ssg.LangOptions(s.Langs, l),
		Onion:     strings.HasSuffix(r.Host, ".onion") || strings.Contains(r.Host, ".onion:"),
		Path:      path,
	}
}
