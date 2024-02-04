package main

import (
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/alexedwards/scs/sqlite3store"
	"github.com/alexedwards/scs/v2"
	"github.com/alexedwards/scs/v2/memstore"
	"github.com/dys2p/btcpay"
	"github.com/dys2p/digitalgoods"
	"github.com/dys2p/digitalgoods/db"
	"github.com/dys2p/digitalgoods/html"
	"github.com/dys2p/digitalgoods/html/sites"
	"github.com/dys2p/digitalgoods/html/static"
	"github.com/dys2p/digitalgoods/userdb"
	"github.com/dys2p/eco/captcha"
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
	"github.com/julienschmidt/httprouter"
	_ "github.com/mattn/go-sqlite3"
)

var database *db.DB
var custSessions *scs.SessionManager
var staffSessions *scs.SessionManager
var btcpayStore btcpay.Store
var paymentMethods []payment.Method
var ratesHistory *rates.History
var users userdb.Authenticator
var langs = lang.MakeLanguages("de", "en")
var emailer email.Emailer

var staffLang, _ = langs.FromPath("de")

func NotifyPaymentReceived(p *digitalgoods.Purchase) error {
	const subject = "digitalgoods.proxysto.re payment received"
	const msg = "We have received your payment. Please download your vouchers within the next 30 days."

	switch p.NotifyProto {
	case "email":
		err := emailer.Send(p.NotifyAddr, subject, []byte(msg))
		if err != nil {
			return fmt.Errorf("sending email notification: %w", err)
		}
	case "ntfysh":
		err := ntfysh.Publish(p.NotifyAddr, subject, msg)
		if err != nil {
			return fmt.Errorf("sending ntfysh notification: %w", err)
		}
	}

	if p.Status == digitalgoods.StatusFinalized {
		p.NotifyProto = ""
		p.NotifyAddr = ""
		err := database.SetNotify(p)
		if err != nil {
			return fmt.Errorf("removing notify data from database: %w", err)
		}
	}
	return nil
}

func main() {

	log.SetFlags(0)

	// os flags

	var test = flag.Bool("test", false, "use btcpay dummy store")
	flag.Parse()

	var err error
	database, err = db.OpenDB()
	if err != nil {
		log.Printf("error opening database: %v", err)
		return
	}

	// btcpay config

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

	// captcha
	captcha.Initialize(filepath.Join(os.Getenv("STATE_DIRECTORY"), "captcha.sqlite3"))

	// emailer
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

	ratesHistory = &rates.History{
		Currencies:  []string{"AUD", "BGN", "CAD", "CHF", "CNY", "CZK", "DKK", "GBP", "ILS", "ISK", "JPY", "NOK", "NZD", "PLN", "RON", "RSD", "SEK", "TWD", "USD"},
		GetBuyRates: GetBuyRates,
		Repository:  ratesDB,
	}

	go ratesHistory.RunDaemon()

	// users

	users, err = userdb.Open(filepath.Join(os.Getenv("CONFIGURATION_DIRECTORY"), "users.json"))
	if err != nil {
		log.Printf("error opening userdb: %v", err)
		return
	}

	var stop = make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// payment methods

	paymentMethods = []payment.Method{
		payment.BTCPay{
			Purchases:    Purchases{database},
			RedirectPath: "/by-cookie",
			Store:        btcpayStore,
		},
		payment.Cash{
			AddressHTML: addressHTML,
		},
		payment.CashForeign{
			AddressHTML: addressHTML,
			History:     ratesHistory,
			Purchases:   Purchases{database},
		},
		payment.SEPA{
			Account:   sepaAccount,
			Purchases: Purchases{database},
		},
	}

	// customer http server

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

	custSessions = scs.New()
	custSessions.Cookie.SameSite = http.SameSiteLaxMode // prevent CSRF
	custSessions.Lifetime = 8 * time.Hour
	custSessions.Store = sqlite3store.New(custSessionsDB)

	var custRtr = httprouter.New()

	for _, l := range langs {
		custRtr.Handler(http.MethodGet, "/"+l.Prefix, httputil.HandlerFunc(custOrderGet))
		custRtr.Handler(http.MethodPost, "/"+l.Prefix, httputil.HandlerFunc(custOrderPost))
		custRtr.Handler(http.MethodGet, "/"+l.Prefix+"/order/:id/:access-key", httputil.HandlerFunc(custPurchaseGet))
		custRtr.Handler(http.MethodPost, "/"+l.Prefix+"/order/:id/:access-key", httputil.HandlerFunc(custPurchasePost))
		custRtr.Handler(http.MethodGet, "/"+l.Prefix+"/order/:id/:access-key/:payment", httputil.HandlerFunc(custPurchaseGet))
		custRtr.Handler(http.MethodPost, "/"+l.Prefix+"/order/:id/:access-key/:payment", httputil.HandlerFunc(custPurchasePost))

		custRtr.Handler(http.MethodGet, "/"+l.Prefix+"/terms.html", httputil.HandlerFunc(siteGet))
		custRtr.Handler(http.MethodGet, "/"+l.Prefix+"/privacy.html", httputil.HandlerFunc(siteGet))
		custRtr.Handler(http.MethodGet, "/"+l.Prefix+"/imprint.html", httputil.HandlerFunc(siteGet))
		custRtr.Handler(http.MethodGet, "/"+l.Prefix+"/contact.html", httputil.HandlerFunc(siteGet))
		custRtr.Handler(http.MethodGet, "/"+l.Prefix+"/payment.html", httputil.HandlerFunc(siteGet))
		custRtr.Handler(http.MethodGet, "/"+l.Prefix+"/cancellation-policy.html", httputil.HandlerFunc(siteGet))
		custRtr.Handler(http.MethodGet, "/"+l.Prefix+"/cancellation-form.html", httputil.HandlerFunc(siteGet))
	}

	// non-localized stuff
	for _, method := range paymentMethods {
		custRtr.Handler(http.MethodPost, fmt.Sprintf("/payment/%s/*path", method.ID()), method)
	}
	custRtr.ServeFiles("/static/*filepath", http.FS(static.Files))
	custRtr.HandlerFunc(http.MethodGet, "/by-cookie", byCookie)
	custRtr.Handler(http.MethodGet, "/captcha/:fn", captcha.Handler())
	custRtr.Handler(http.MethodGet, "/payment-health", health.Server{
		BTCPay: btcpayStore,
		Rates:  ratesHistory,
	})
	custRtr.NotFound = http.HandlerFunc(langs.Redirect)

	shutdownCust := httputil.ListenAndServe(":9002", custSessions.LoadAndSave(custRtr), stop)
	defer shutdownCust()

	log.Println("listening to port 9002")

	// staff http server

	staffSessions = scs.New()
	staffSessions.Cookie.SameSite = http.SameSiteLaxMode // prevent CSRF
	staffSessions.Store = memstore.New()

	var staffAuthRouter = httprouter.New()
	staffAuthRouter.HandlerFunc(http.MethodGet, "/", showErr(staffIndexGet))
	staffAuthRouter.HandlerFunc(http.MethodGet, "/logout", showErr(staffLogoutGet))
	staffAuthRouter.HandlerFunc(http.MethodGet, "/view", showErr(staffViewGet))
	staffAuthRouter.HandlerFunc(http.MethodPost, "/view", showErr(staffViewPost))
	staffAuthRouter.HandlerFunc(http.MethodGet, "/mark-paid/:id", showErr(staffMarkPaidGet))
	staffAuthRouter.HandlerFunc(http.MethodPost, "/mark-paid/:id", showErr(staffMarkPaidPost))
	staffAuthRouter.HandlerFunc(http.MethodGet, "/upload", showErr(staffSelectGet))
	staffAuthRouter.HandlerFunc(http.MethodGet, "/upload/:articleid/:country", showErr(staffUploadGet))
	staffAuthRouter.HandlerFunc(http.MethodGet, "/upload/:articleid/:country/image", showErr(staffUploadImageGet))
	staffAuthRouter.HandlerFunc(http.MethodPost, "/upload/:articleid/:country/image", returnErr(staffUploadImagePost))
	staffAuthRouter.HandlerFunc(http.MethodGet, "/upload/:articleid/:country/text", showErr(staffUploadTextGet))
	staffAuthRouter.HandlerFunc(http.MethodPost, "/upload/:articleid/:country/text", returnErr(staffUploadTextPost))

	var staffRtr = httprouter.New()
	staffRtr.ServeFiles("/static/*filepath", http.FS(static.Files))
	staffRtr.HandlerFunc(http.MethodGet, "/login", showErr(staffLoginGet))
	staffRtr.HandlerFunc(http.MethodPost, "/login", showErr(staffLoginPost))
	staffRtr.NotFound = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if staffSessions.Exists(r.Context(), "username") {
			staffAuthRouter.ServeHTTP(w, r)
		} else {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
		}
	})

	shutdownStaff := httputil.ListenAndServe("127.0.0.1:9003", staffSessions.LoadAndSave(staffRtr), stop)
	defer shutdownStaff()

	// cleanup bot

	var wg sync.WaitGroup
	defer wg.Wait()

	go func() {
		for ; true; <-time.Tick(12 * time.Hour) {
			wg.Add(1)
			if err := database.Cleanup(); err != nil {
				log.Printf("error cleaning up database: %v", err)
			}
			wg.Done()
		}
	}()

	// notify us
	if err := emailer.Send(emailFrom, "digitalgoods service started", []byte("the digitalgoods service has been started")); err != nil {
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
func frontendErr(err error, message string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		l, _ := langs.FromPath(r.URL.Path)
		fmt.Println(l.BCP47)
		w.WriteHeader(http.StatusInternalServerError)
		html.Error.Execute(w, struct {
			lang.Lang
			Message string
		}{
			Lang:    l,
			Message: message,
		})
		log.Printf("internal server error: %v", err)
		ntfysh.Publish(ntfyshLog, "digitalgoods error", err.Error())
	})
}

// frontend notfound handler, logs err and displays a message
func frontendNotFound(message string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		l, _ := langs.FromPath(r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
		html.Error.Execute(w, struct {
			lang.Lang
			Message string
		}{
			Lang:    l,
			Message: message,
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
func showErr(f func(http.ResponseWriter, *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l, _ := langs.FromPath(r.URL.Path)
		if err := f(w, r); err != nil {
			html.Error.Execute(w, struct {
				lang.Lang
				Message string
			}{
				Lang:    l,
				Message: err.Error(),
			})
		}
	}
}

func custOrderGet(w http.ResponseWriter, r *http.Request) http.Handler {
	l, _ := langs.FromPath(r.URL.Path)

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

	stock, err := database.GetStock()
	if err != nil {
		return frontendErr(err, l.Tr("Error getting stock from database. Please try again later."))
	}

	err = html.CustOrder.Execute(w, &html.CustOrderData{
		AvailableEUCountries: countries.TranslateAndSort(l, availableEUCountries),
		AvailableNonEU:       availableNonEU,
		Catalog:              catalog,
		Stock:                stock,

		Area: area,
		Captcha: captcha.TemplateData{
			ID: captcha.New(),
		},
		Lang: l,
	})
	if err != nil {
		return frontendErr(err, l.Tr("Error displaying website. Please try again later."))
	}
	return nil
}

func custOrderPost(w http.ResponseWriter, r *http.Request) http.Handler {
	l, _ := langs.FromPath(r.URL.Path)

	availableEUCountries, availableNonEU, err := detect.Countries(r)
	if err != nil {
		log.Printf("error detecting countries: %v", err)
	}

	stock, err := database.GetStock()
	if err != nil {
		return frontendErr(err, l.Tr("Error getting stock from database. Please try again later."))
	}

	// read user input

	co := &html.CustOrderData{
		AvailableEUCountries: countries.TranslateAndSort(l, availableEUCountries),
		AvailableNonEU:       availableNonEU,
		Catalog:              catalog,
		Stock:                stock,

		Captcha: captcha.TemplateData{
			Answer: r.PostFormValue("captcha-answer"),
			ID:     r.PostFormValue("captcha-id"),
		},
		Cart:         &digitalgoods.Cart{},
		OtherCountry: make(map[string]string),
		Area:         r.PostFormValue("area"),
		EUCountry:    r.PostFormValue("eu-country"),
		Lang:         l,
	}

	variants := catalog.Variants()

	order := digitalgoods.Order{} // in case of no errors, TODO: create order from cart

	// same logic as in order template
	for _, variant := range variants {
		// featured countries
		for _, countryID := range stock.FeaturedCountryIDs(variant) {
			val := r.PostFormValue(variant.ID + "-" + countryID)
			if val == "" {
				continue
			}
			quantity, _ := strconv.Atoi(val)
			if max := stock.Max(variant, countryID); quantity > max {
				quantity = max // client must check their order before payment
			}
			if quantity > 0 {
				co.Cart.Add(variant.ID, countryID, quantity)
				order = append(order, digitalgoods.OrderRow{
					Quantity:  quantity,
					VariantID: variant.ID,
					CountryID: countryID,
					ItemPrice: variant.Price,
				})
			}
		}
		// other country
		if quantity, _ := strconv.Atoi(r.PostFormValue(variant.ID + "-other-quantity")); quantity > 0 {
			countryID := r.PostFormValue(variant.ID + "-other-country")
			if countryID == "" || !digitalgoods.IsISOCountryCode(countryID) {
				continue
			}
			if max := stock.Max(variant, countryID); quantity > max {
				quantity = max // client must check their order before payment
			}
			if quantity > 0 {
				co.Cart.Add(variant.ID, "other", quantity)
				co.OtherCountry[variant.ID] = countryID
				order = append(order, digitalgoods.OrderRow{
					Quantity:  quantity,
					VariantID: variant.ID,
					CountryID: countryID,
					ItemPrice: variant.Price,
				})
			}
		}
	}

	// validate user input

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

	// VerifyString probably invalidates the captcha, so we check this last
	if !captcha.Verify(co.Captcha.ID, co.Captcha.Answer) {
		co.Captcha.Answer = ""
		co.Captcha.ID = captcha.New()
		co.Captcha.Err = true
		html.CustOrder.Execute(w, co)
		return nil
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

	if err := database.InsertPurchase(purchase); err != nil {
		return frontendErr(err, l.Tr("Error inserting purchase into database. Please try again later."))
	}

	// set cookie
	redirectPath := path.Join("/", l.Prefix, "order", purchase.ID, purchase.AccessKey)
	custSessions.Put(r.Context(), "redirect-path", redirectPath)
	return http.RedirectHandler(redirectPath, http.StatusSeeOther)
}

func custPurchaseGet(w http.ResponseWriter, r *http.Request) http.Handler {
	l, _ := langs.FromPath(r.URL.Path)
	params := httprouter.ParamsFromContext(r.Context())
	purchase, err := database.GetPurchaseByIDAndAccessKey(params.ByName("id"), params.ByName("access-key"))
	if err != nil {
		return frontendNotFound(l.Tr("There is no such purchase, or it has been deleted."))
	}

	paymentMethod, err := payment.Get(paymentMethods, params.ByName("payment"))
	if err != nil {
		return frontendNotFound(l.Tr("Payment method not found."))
	}

	err = html.CustPurchase.Execute(w, &html.CustPurchaseData{
		GroupedOrder:   catalog.GroupOrder(purchase.Ordered),
		Purchase:       purchase,
		PaymentMethod:  paymentMethod,
		URL:            absHost(r) + path.Join("/", l.Prefix, "order", purchase.ID, purchase.AccessKey),
		PreferOnion:    strings.HasSuffix(r.Host, ".onion") || strings.Contains(r.Host, ".onion:"),
		Lang:           l,
		ActiveTab:      paymentMethod.ID(),
		PaymentMethods: paymentMethods,
	})
	if err != nil {
		return frontendErr(err, l.Tr("Error displaying website. Please try again later."))
	}
	return nil
}

func custPurchasePost(w http.ResponseWriter, r *http.Request) http.Handler {
	l, _ := langs.FromPath(r.URL.Path)
	params := httprouter.ParamsFromContext(r.Context())
	purchase, err := database.GetPurchaseByIDAndAccessKey(params.ByName("id"), params.ByName("access-key"))
	if err != nil {
		return frontendNotFound(l.Tr("There is no such purchase, or it has been deleted."))
	}

	notifyProto := r.PostFormValue("notify-proto")
	notifyAddr := r.PostFormValue("notify-addr")
	if len(notifyAddr) > 1024 {
		notifyAddr = notifyAddr[:1024]
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
	if err := database.SetNotify(purchase); err != nil {
		return frontendErr(err, l.Tr("Error saving notify information. Please try again later."))
	}

	return http.RedirectHandler(r.URL.Path+"#notify", http.StatusSeeOther)
}

func byCookie(w http.ResponseWriter, r *http.Request) {
	// TODO maybe save language in cookie and redirect to user's locale
	if redirectPath := custSessions.GetString(r.Context(), "redirect-path"); redirectPath != "" {
		http.Redirect(w, r, redirectPath, http.StatusSeeOther)
	} else {
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func siteGet(w http.ResponseWriter, r *http.Request) http.Handler {
	l, _ := langs.FromPath(r.URL.Path)
	name := strings.TrimSuffix(path.Base(r.URL.Path), ".html")

	file, err := sites.Files.Open(filepath.Join(l.Prefix, name+".md"))
	if err != nil {
		return frontendErr(err, l.Tr("Error loading site content. Please try again later."))
	}

	content, err := io.ReadAll(file)
	if err != nil {
		return frontendErr(err, l.Tr("Error loading site content. Please try again later."))
	}

	if err := html.Site.Execute(w, struct {
		lang.Lang
		Content string
	}{
		Lang:    l,
		Content: string(content),
	}); err != nil {
		return frontendErr(err, l.Tr("Error displaying site content. Please try again later."))
	}
	return nil
}

func staffIndexGet(w http.ResponseWriter, r *http.Request) error {
	underdelivered, err := database.GetPurchases(digitalgoods.StatusUnderdelivered)
	if err != nil {
		return err
	}
	return html.StaffIndex.Execute(w, struct {
		lang.Lang
		Underdelivered []string
	}{
		Lang:           staffLang,
		Underdelivered: underdelivered,
	})
}

func staffLoginGet(w http.ResponseWriter, r *http.Request) error {
	return html.StaffLogin.Execute(w, staffLang)
}

func staffLoginPost(w http.ResponseWriter, r *http.Request) error {
	username := r.PostFormValue("username")
	password := r.PostFormValue("password")
	if err := users.Authenticate(username, password); err != nil {
		return err
	}
	staffSessions.Put(r.Context(), "username", username)
	http.Redirect(w, r, "/", http.StatusSeeOther)
	return nil
}

func staffLogoutGet(w http.ResponseWriter, r *http.Request) error {
	staffSessions.Destroy(r.Context())
	http.Redirect(w, r, "/", http.StatusSeeOther)
	return nil
}

func staffViewGet(w http.ResponseWriter, r *http.Request) error {
	return html.StaffView.Execute(w, staffLang)
}

func staffViewPost(w http.ResponseWriter, r *http.Request) error {
	id := strings.ToUpper(strings.TrimSpace(r.PostFormValue("id")))
	purchase, err := database.GetPurchaseByID(id)
	if err != nil {
		return err
	}
	http.Redirect(w, r, fmt.Sprintf("/mark-paid/%s", purchase.ID), http.StatusSeeOther)
	return nil
}

func staffMarkPaidGet(w http.ResponseWriter, r *http.Request) error {
	id := strings.ToUpper(strings.TrimSpace(httprouter.ParamsFromContext(r.Context()).ByName("id")))
	purchase, err := database.GetPurchaseByID(id)
	if err != nil {
		return err
	}
	currencyOptions, _ := ratesHistory.Get(purchase.CreateDate, float64(purchase.Ordered.Sum())/100.0)

	return html.StaffMarkPaid.Execute(w, struct {
		lang.Lang
		*digitalgoods.Purchase
		GroupedOrder    []digitalgoods.OrderedArticle
		CurrencyOptions []rates.Option
		EUCountries     []countries.CountryWithName
		DB              *db.DB
	}{
		staffLang,
		purchase,
		catalog.GroupOrder(purchase.Ordered),
		currencyOptions,
		countries.TranslateAndSort(staffLang, countries.EuropeanUnion),
		database,
	})
}

func staffMarkPaidPost(w http.ResponseWriter, r *http.Request) error {
	if r.PostFormValue("confirm") == "" {
		return errors.New("You did not confirm.")
	}
	id := r.PostFormValue("id")
	purchase, err := database.GetPurchaseByID(id)
	if err != nil {
		return err
	}
	countryCode := r.PostFormValue("country")
	if purchase.CountryCode != countryCode {
		if err := database.SetCountry(purchase, countryCode); err != nil {
			return err
		}
	}
	if err := database.SetSettled(purchase); err != nil {
		return err
	}
	if err := NotifyPaymentReceived(purchase); err != nil {
		return err
	}
	http.Redirect(w, r, fmt.Sprintf("/mark-paid/%s", purchase.ID), http.StatusSeeOther)
	return nil
}

type staffSelect struct {
	lang.Lang
	Stock          digitalgoods.Stock
	Variants       []digitalgoods.Variant
	Underdelivered map[string]int // key: articleID-countryID
}

func (s *staffSelect) ISOCountryCodes() []string {
	return digitalgoods.ISOCountryCodes[:]
}

func staffSelectGet(w http.ResponseWriter, r *http.Request) error {
	variants := catalog.Variants()

	underdeliveredPurchaseIDs, err := database.GetPurchases(digitalgoods.StatusUnderdelivered)
	if err != nil {
		return err
	}
	underdelivered := make(map[string]int)
	for _, purchaseID := range underdeliveredPurchaseIDs {
		purchase, err := database.GetPurchaseByID(purchaseID)
		if err != nil {
			return err
		}
		unfulfilled, err := purchase.GetUnfulfilled()
		if err != nil {
			return err
		}
		for _, uf := range unfulfilled {
			underdelivered[uf.VariantID+"-"+uf.CountryID] += uf.Quantity
		}
	}

	stock, err := database.GetStock()
	if err != nil {
		return err
	}

	return html.StaffSelect.Execute(w, &staffSelect{
		staffLang,
		stock,
		variants,
		underdelivered,
	})
}

func staffUploadGet(w http.ResponseWriter, r *http.Request) error {
	// redirect to image upload
	http.Redirect(w, r, fmt.Sprintf("/upload/%s/%s/image", httprouter.ParamsFromContext(r.Context()).ByName("articleid"), httprouter.ParamsFromContext(r.Context()).ByName("country")), http.StatusSeeOther)
	return nil
}

func staffUploadImageGet(w http.ResponseWriter, r *http.Request) error {
	variant, err := catalog.Variant(httprouter.ParamsFromContext(r.Context()).ByName("articleid"))
	if err != nil {
		return err
	}
	countryID := httprouter.ParamsFromContext(r.Context()).ByName("country")
	stock, err := database.GetStock()
	if err != nil {
		return err
	}

	return html.StaffUploadImage.Execute(w, struct {
		lang.Lang
		digitalgoods.Variant
		Country string
		Stock   int
	}{
		staffLang,
		variant,
		countryID,
		stock.Get(variant, countryID),
	})
}

func staffUploadImagePost(w http.ResponseWriter, r *http.Request) error {

	file, header, err := r.FormFile("file") // name="file" from dropzonejs
	if err != nil {
		return err
	}

	if header.Size > 100*1024 {
		return errors.New("file too large")
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	var articleID = httprouter.ParamsFromContext(r.Context()).ByName("articleid")
	var countryID = httprouter.ParamsFromContext(r.Context()).ByName("country")

	if err := database.AddToStock(articleID, countryID, header.Filename, data); err != nil {
		return err
	}

	log.Printf("added image to stock: %s %s %s", articleID, countryID, digitalgoods.Mask(header.Filename))

	return database.FulfilUnderdelivered()
}

func staffUploadTextGet(w http.ResponseWriter, r *http.Request) error {
	variant, err := catalog.Variant(httprouter.ParamsFromContext(r.Context()).ByName("articleid"))
	if err != nil {
		return err
	}
	countryID := httprouter.ParamsFromContext(r.Context()).ByName("country")
	stock, err := database.GetStock()
	if err != nil {
		return err
	}

	return html.StaffUploadText.Execute(w, struct {
		lang.Lang
		digitalgoods.Variant
		Country string
		Stock   int
	}{
		staffLang,
		variant,
		countryID,
		stock.Get(variant, countryID),
	})
}

func staffUploadTextPost(w http.ResponseWriter, r *http.Request) error {

	var articleID = httprouter.ParamsFromContext(r.Context()).ByName("articleid")
	var countryID = httprouter.ParamsFromContext(r.Context()).ByName("country")

	for _, code := range strings.Fields(r.PostFormValue("codes")) {
		if err := database.AddToStock(articleID, countryID, code, nil); err == nil {
			log.Printf("added code to stock: %s %s %s", articleID, countryID, digitalgoods.Mask(code))
		} else {
			log.Println(err)
			return err
		}
	}

	if err := database.FulfilUnderdelivered(); err != nil {
		return err
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
	return nil
}

type Purchases struct {
	db *db.DB
}

func (purchases Purchases) PurchaseCreationDate(id, paymentKey string) (string, error) {
	purchase, err := purchases.db.GetPurchaseByIDAndPaymentKey(id, paymentKey)
	if err != nil {
		return "", err
	}
	if purchase.CreateDate == "" {
		purchase.CreateDate = time.Now().Format("2006-01-02") // TODO
	}
	return purchase.CreateDate, nil
}

func (purchases Purchases) PurchaseSumCents(id, paymentKey string) (int, error) {
	purchase, err := purchases.db.GetPurchaseByIDAndPaymentKey(id, paymentKey)
	if err != nil {
		return 0, err
	}
	return purchase.Ordered.Sum(), nil
}

func (purchases Purchases) SetPurchasePaid(id, paymentKey string) error {
	purchase, err := purchases.db.GetPurchaseByIDAndPaymentKey(id, paymentKey)
	if err != nil {
		return err
	}
	if err := purchases.db.SetSettled(purchase); err != nil {
		return err
	}
	return NotifyPaymentReceived(purchase)
}

func (purchases Purchases) SetPurchaseProcessing(id, paymentKey string) error {
	purchase, err := purchases.db.GetPurchaseByIDAndPaymentKey(id, paymentKey)
	if err != nil {
		return err
	}
	return purchases.db.SetProcessing(purchase)
}

// absHost returns the scheme and host part of an HTTP request. It uses a heuristic for the scheme.
//
// If you use nginx as a reverse proxy, make sure you have set "proxy_set_header Host $host;" besides proxy_pass in your configuration.
func absHost(r *http.Request) string {
	var proto = "https"
	if strings.HasPrefix(r.Host, "127.0.") || strings.HasPrefix(r.Host, "[::1]") || strings.HasSuffix(r.Host, ".onion") || strings.Contains(r.Host, ".onion:") { // if running locally or through TOR
		proto = "http"
	}
	return fmt.Sprintf("%s://%s", proto, r.Host)
}
