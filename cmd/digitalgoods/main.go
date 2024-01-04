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
	"github.com/dys2p/eco/lang"
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
var langs lang.Languages

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

	langs = lang.Languages([]string{"en", "de"})

	var custRtr = httprouter.New()

	for _, id := range langs {
		custRtr.HandlerFunc(http.MethodGet, "/"+id, custOrderGet)
		custRtr.HandlerFunc(http.MethodPost, "/"+id, custOrderPost)
		custRtr.HandlerFunc(http.MethodGet, "/"+id+"/i/:access-key", custPurchaseGetRedirect)
		custRtr.HandlerFunc(http.MethodGet, "/"+id+"/i/:access-key/:payment", custPurchaseGetPaymentRedirect)
		custRtr.HandlerFunc(http.MethodGet, "/"+id+"/order/:id/:access-key", custPurchaseGet)
		custRtr.HandlerFunc(http.MethodGet, "/"+id+"/order/:id/:access-key/:payment", custPurchaseGet)

		custRtr.HandlerFunc(http.MethodGet, "/"+id+"/terms.html", siteGet)
		custRtr.HandlerFunc(http.MethodGet, "/"+id+"/privacy.html", siteGet)
		custRtr.HandlerFunc(http.MethodGet, "/"+id+"/imprint.html", siteGet)
		custRtr.HandlerFunc(http.MethodGet, "/"+id+"/contact.html", siteGet)
		custRtr.HandlerFunc(http.MethodGet, "/"+id+"/payment.html", siteGet)
		custRtr.HandlerFunc(http.MethodGet, "/"+id+"/cancellation-policy.html", siteGet)
		custRtr.HandlerFunc(http.MethodGet, "/"+id+"/cancellation-form.html", siteGet)
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

	var custSrv = ListenAndServe("tcp", ":9002", custSessions.LoadAndSave(custRtr), stop)
	defer custSrv.Shutdown()

	log.Println("listening to port 9002")

	// staff http server

	staffSessions = scs.New()
	staffSessions.Cookie.SameSite = http.SameSiteLaxMode // prevent CSRF
	staffSessions.Store = memstore.New()

	var staffRtr = httprouter.New()
	staffRtr.ServeFiles("/static/*filepath", http.FS(static.Files))
	staffRtr.HandlerFunc(http.MethodGet, "/login", wrapTmpl(staffLoginGet))
	staffRtr.HandlerFunc(http.MethodPost, "/login", wrapTmpl(staffLoginPost))
	// with authentication:
	staffRtr.HandlerFunc(http.MethodGet, "/", auth(wrapTmpl(staffIndexGet)))
	staffRtr.HandlerFunc(http.MethodGet, "/logout", auth(wrapTmpl(staffLogoutGet)))
	staffRtr.HandlerFunc(http.MethodGet, "/view", auth(wrapTmpl(staffViewGet)))
	staffRtr.HandlerFunc(http.MethodPost, "/view", auth(wrapTmpl(staffViewPost)))
	staffRtr.HandlerFunc(http.MethodGet, "/mark-paid/:id", auth(wrapTmpl(staffMarkPaidGet)))
	staffRtr.HandlerFunc(http.MethodPost, "/mark-paid/:id", auth(wrapTmpl(staffMarkPaidPost)))
	staffRtr.HandlerFunc(http.MethodGet, "/upload", auth(wrapTmpl(staffSelectGet)))
	staffRtr.HandlerFunc(http.MethodGet, "/upload/:articleid/:country", auth(wrapTmpl(staffUploadGet)))
	staffRtr.HandlerFunc(http.MethodGet, "/upload/:articleid/:country/image", auth(wrapTmpl(staffUploadImageGet)))
	staffRtr.HandlerFunc(http.MethodPost, "/upload/:articleid/:country/image", auth(wrapAPI(staffUploadImagePost)))
	staffRtr.HandlerFunc(http.MethodGet, "/upload/:articleid/:country/text", auth(wrapTmpl(staffUploadTextGet)))
	staffRtr.HandlerFunc(http.MethodPost, "/upload/:articleid/:country/text", auth(wrapAPI(staffUploadTextPost)))

	var staffSrv = ListenAndServe("tcp", "127.0.0.1:9003", staffSessions.LoadAndSave(staffRtr), stop)
	defer staffSrv.Shutdown()

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

	// run until we receive an interrupt or any of the listeners fails

	log.Printf("running")
	<-stop
	log.Println("shutting down")
}

func custOrderGet(w http.ResponseWriter, r *http.Request) {
	lang := langs.ByPath(r)

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
		http.Error(w, err.Error(), http.StatusInternalServerError) // TODO
		return
	}

	err = html.CustOrder.Execute(w, string(lang), &html.CustOrderData{
		AvailableEUCountries: countries.TranslateAndSort(lang.String(), availableEUCountries),
		AvailableNonEU:       availableNonEU,
		Catalog:              catalog,
		Stock:                stock,

		Area: area,
		Captcha: captcha.TemplateData{
			ID: captcha.New(),
		},
		Lang: lang,
	})
	if err != nil {
		log.Println(err)
	}
}

func custOrderPost(w http.ResponseWriter, r *http.Request) {
	lang := langs.ByPath(r)

	availableEUCountries, availableNonEU, err := detect.Countries(r)
	if err != nil {
		log.Printf("error detecting countries: %v", err)
	}

	stock, err := database.GetStock()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError) // TODO
		return
	}

	// read user input

	co := &html.CustOrderData{
		AvailableEUCountries: countries.TranslateAndSort(lang.String(), availableEUCountries),
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
		Lang:         lang,
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
		html.CustOrder.Execute(w, string(lang), co)
		return
	}

	var country countries.Country
	if co.Area == "non-eu" {
		country = countries.NonEU
	} else {
		country = countries.Country(co.EUCountry)
		if !countries.InEuropeanUnion(country) {
			co.CountryErr = true
			html.CustOrder.Execute(w, string(lang), co)
			return
		}
	}

	// VerifyString probably invalidates the captcha, so we check this last
	if !captcha.Verify(co.Captcha.ID, co.Captcha.Answer) {
		co.Captcha.Answer = ""
		co.Captcha.ID = captcha.New()
		co.Captcha.Err = true
		html.CustOrder.Execute(w, string(lang), co)
		return
	}

	purchase := &digitalgoods.Purchase{
		AccessKey:   digitalgoods.NewKey(),
		PaymentKey:  digitalgoods.NewKey(),
		Status:      digitalgoods.StatusNew,
		Ordered:     order,
		CreateDate:  time.Now().Format("2006-01-02"),
		DeleteDate:  time.Now().AddDate(0, 0, 31).Format("2006-01-02"),
		CountryCode: string(country),
	}

	if err := database.InsertPurchase(purchase); err != nil {
		html.ErrorInternal.Execute(w, lang)
		return
	}

	// set cookie
	redirectPath := lang.Path("/order/%s/%s", purchase.ID, purchase.AccessKey)
	custSessions.Put(r.Context(), "redirect-path", redirectPath)
	http.Redirect(w, r, redirectPath, http.StatusSeeOther)
}

func custPurchaseGetRedirect(w http.ResponseWriter, r *http.Request) {
	lang := langs.ByPath(r)
	params := httprouter.ParamsFromContext(r.Context())

	accessKey := params.ByName("access-key")
	purchase, err := database.GetPurchaseByAccessKey(accessKey)
	if err != nil {
		html.ErrorInternal.Execute(w, lang)
		return
	}

	redirectPath := lang.Path("/order/%s/%s", purchase.ID, purchase.AccessKey)
	http.Redirect(w, r, redirectPath, http.StatusMovedPermanently)
}

func custPurchaseGetPaymentRedirect(w http.ResponseWriter, r *http.Request) {
	lang := langs.ByPath(r)
	params := httprouter.ParamsFromContext(r.Context())

	accessKey := params.ByName("access-key")
	purchase, err := database.GetPurchaseByAccessKey(accessKey)
	if err != nil {
		html.ErrorInternal.Execute(w, lang)
		return
	}

	paymentMethod := params.ByName("payment")

	redirectPath := lang.Path("/order/%s/%s/%s", purchase.ID, purchase.AccessKey, paymentMethod)
	http.Redirect(w, r, redirectPath, http.StatusMovedPermanently)
}

func custPurchaseGet(w http.ResponseWriter, r *http.Request) {
	lang := langs.ByPath(r)
	params := httprouter.ParamsFromContext(r.Context())

	accessKey := params.ByName("access-key")
	purchase, err := database.GetPurchaseByAccessKey(accessKey)
	if err != nil {
		html.ErrorNotFound.Execute(w, lang)
		return
	}

	paymentMethod, err := payment.Get(paymentMethods, params.ByName("payment"))
	if err != nil {
		html.ErrorNotFound.Execute(w, lang)
		return
	}

	err = html.CustPurchase.Execute(w, string(lang), &html.CustPurchaseData{
		GroupedOrder:   catalog.GroupOrder(purchase.Ordered),
		Purchase:       purchase,
		PaymentMethod:  paymentMethod,
		URL:            absHost(r) + lang.Path("order/%s/%s", purchase.ID, purchase.AccessKey),
		PreferOnion:    strings.HasSuffix(r.Host, ".onion") || strings.Contains(r.Host, ".onion:"),
		Lang:           lang,
		ActiveTab:      paymentMethod.ID(),
		PaymentMethods: paymentMethods,
	})
	if err != nil {
		// TODO use eco/middleware
		log.Println(err)
	}
}

func byCookie(w http.ResponseWriter, r *http.Request) {
	// TODO maybe save language in cookie and redirect to user's locale
	if redirectPath := custSessions.GetString(r.Context(), "redirect-path"); redirectPath != "" {
		http.Redirect(w, r, redirectPath, http.StatusSeeOther)
	} else {
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func siteGet(w http.ResponseWriter, r *http.Request) {
	lang := langs.ByPath(r)
	name := strings.TrimSuffix(path.Base(r.URL.Path), ".html")

	file, err := sites.Files.Open(filepath.Join(string(lang), name+".md"))
	if err != nil {
		html.ErrorInternal.Execute(w, lang)
		return
	}

	content, err := io.ReadAll(file)
	if err != nil {
		html.ErrorInternal.Execute(w, lang)
		return
	}

	html.Site.Execute(w, string(lang), string(content))
}

func staffIndexGet(w http.ResponseWriter, r *http.Request) error {
	unfulfilled, err := database.GetPurchases(digitalgoods.StatusUnderdelivered)
	if err != nil {
		return err
	}
	return html.StaffIndex.Execute(w, unfulfilled)
}

func staffLoginGet(w http.ResponseWriter, r *http.Request) error {
	return html.StaffLogin.Execute(w, nil)
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
	return html.StaffView.Execute(w, nil)
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
		*digitalgoods.Purchase
		GroupedOrder    []digitalgoods.OrderedArticle
		CurrencyOptions []rates.Option
		EUCountries     []countries.CountryWithName
		DB              *db.DB
	}{
		purchase,
		catalog.GroupOrder(purchase.Ordered),
		currencyOptions,
		countries.TranslateAndSort("de", countries.EuropeanUnion),
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
	http.Redirect(w, r, fmt.Sprintf("/mark-paid/%s", purchase.ID), http.StatusSeeOther)
	return nil
}

type staffSelect struct {
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
	return html.StaffUploadImage.Execute(w, struct {
		digitalgoods.Variant
		Country string
	}{
		variant,
		countryID,
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
	return html.StaffUploadText.Execute(w, struct {
		digitalgoods.Variant
		Country string
	}{
		variant,
		countryID,
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
	return purchases.db.SetSettled(purchase)
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
