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
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/alexedwards/scs/sqlite3store"
	"github.com/alexedwards/scs/v2"
	"github.com/alexedwards/scs/v2/memstore"
	"github.com/dchest/captcha"
	"github.com/dys2p/btcpay"
	"github.com/dys2p/digitalgoods"
	"github.com/dys2p/digitalgoods/db"
	"github.com/dys2p/digitalgoods/html"
	"github.com/dys2p/digitalgoods/static"
	"github.com/dys2p/digitalgoods/userdb"
	"github.com/julienschmidt/httprouter"
	_ "github.com/mattn/go-sqlite3"
)

var database *db.DB
var custSessions *scs.SessionManager
var staffSessions *scs.SessionManager
var store btcpay.Store
var users userdb.Authenticator

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

	if *test {
		store = btcpay.NewDummyStore()
		log.Println("\033[33m" + "warning: using btcpay dummy store" + "\033[0m")
	} else {
		store, err = btcpay.Load("data/btcpay.json")
		if err != nil {
			log.Printf("error loading btcpay store: %v", err)
			return
		}

		log.Println("don't forget to set up the webhook for your store")
		log.Println(`  URL: /rpc`)
		log.Println(`  Event: "An invoice is processing"`)
		log.Println(`  Event: "An invoice has expired"`)
		log.Println(`  Event: "An invoice has been settled"`)
	}

	users, err = userdb.Open("data/users.json")
	if err != nil {
		log.Printf("error opening userdb: %v", err)
		return
	}

	var stop = make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// customer http server

	custSessionsDB, err := sql.Open("sqlite3", "data/customer-sessions.sqlite3")
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
	custRtr.ServeFiles("/static/*filepath", http.FS(static.Files))
	custRtr.HandlerFunc(http.MethodGet, "/", wrapTmpl(custOrderGet))
	custRtr.HandlerFunc(http.MethodPost, "/", wrapTmpl(custOrderPost))
	custRtr.HandlerFunc(http.MethodGet, "/i/:purchaseid", wrapTmpl(custPurchaseGetBTCPay))
	custRtr.HandlerFunc(http.MethodPost, "/i/:purchaseid", wrapTmpl(custPurchasePostBTCPay))
	custRtr.HandlerFunc(http.MethodGet, "/i/:purchaseid/cash", wrapTmpl(custPurchaseGetCash))
	custRtr.HandlerFunc(http.MethodGet, "/i/:purchaseid/sepa", wrapTmpl(custPurchaseGetSEPA))
	custRtr.HandlerFunc(http.MethodGet, "/by-cookie", byCookie)
	custRtr.HandlerFunc(http.MethodGet, "/health", health)
	custRtr.HandlerFunc(http.MethodPost, "/rpc", rpc)
	custRtr.Handler("GET", "/captcha/:fn", captcha.Server(captcha.StdWidth, captcha.StdHeight))

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
	staffRtr.HandlerFunc(http.MethodGet, "/mark-paid/:payid", auth(wrapTmpl(staffMarkPaidGet)))
	staffRtr.HandlerFunc(http.MethodPost, "/mark-paid/:payid", auth(wrapTmpl(staffMarkPaidPost)))
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

func custOrderGet(w http.ResponseWriter, r *http.Request) error {
	lang := html.GetLanguage(r)
	return html.CustOrder.Execute(w, &html.CustOrderData{
		ArticlesByCategory: database.GetArticlesByCategory,
		Categories:         database.GetCategories,
		EUCountryCodes:     digitalgoods.EUCountryCodes[:],

		CaptchaID:     captcha.NewLen(6),
		CountryAnswer: lang.Translate("default-eu-country"),
		Language:      lang,
	})
}

func custOrderPost(w http.ResponseWriter, r *http.Request) error {

	// read user input

	co := &html.CustOrderData{
		ArticlesByCategory: database.GetArticlesByCategory,
		Categories:         database.GetCategories,
		EUCountryCodes:     digitalgoods.EUCountryCodes[:],

		CaptchaAnswer: r.PostFormValue("captcha-answer"),
		CaptchaID:     r.PostFormValue("captcha-id"),
		Cart:          make(map[string]int),
		OtherCountry:  make(map[string]string),
		CountryAnswer: r.PostFormValue("country"),
		Language:      html.GetLanguage(r),
	}

	articles, err := database.GetArticles()
	if err != nil {
		return err
	}

	order := digitalgoods.Order{} // in case of no errors

	// same logic as in order template
	for _, a := range articles {
		if !a.Portfolio() {
			continue
		}
		// featured countries
		for _, countryID := range a.FeaturedCountryIDs() {
			val := r.PostFormValue(a.ID + "-" + countryID)
			if val == "" {
				continue
			}
			amount, _ := strconv.Atoi(val)
			if max := a.Max(countryID); amount > max {
				amount = max // client must check their order before payment
			}
			if amount > 0 {
				co.Cart[a.ID+"-"+countryID] = amount
				order = append(order, digitalgoods.OrderRow{
					Amount:    amount,
					ArticleID: a.ID,
					CountryID: countryID,
					ItemPrice: a.Price,
				})
			}
		}
		// other country
		if amount, _ := strconv.Atoi(r.PostFormValue(a.ID + "-other-amount")); amount > 0 {
			countryID := r.PostFormValue(a.ID + "-other-country")
			if countryID == "" || !digitalgoods.IsISOCountryCode(countryID) {
				continue
			}
			if max := a.Max(countryID); amount > max {
				amount = max // client must check their order before payment
			}
			if amount > 0 {
				co.Cart[a.ID+"-other-amount"] = amount
				co.OtherCountry[a.ID] = countryID
				order = append(order, digitalgoods.OrderRow{
					Amount:    amount,
					ArticleID: a.ID,
					CountryID: countryID,
					ItemPrice: a.Price,
				})
			}
		}
	}

	// validate user input

	if len(order) == 0 {
		co.OrderErr = true
		return html.CustOrder.Execute(w, co)
	}

	if !digitalgoods.IsEUCountryCode(co.CountryAnswer) {
		co.CountryAnswer = ""
		co.CountryErr = true
		return html.CustOrder.Execute(w, co)
	}

	// VerifyString probably invalidates the captcha, so we check this last
	if !captcha.VerifyString(co.CaptchaID, co.CaptchaAnswer) {
		co.CaptchaAnswer = ""
		co.CaptchaID = captcha.NewLen(6)
		co.CaptchaErr = true
		return html.CustOrder.Execute(w, co)
	}

	id, err := database.AddPurchase(order, time.Now().AddDate(0, 0, 31).Format(digitalgoods.DateFmt), co.CountryAnswer)
	if err != nil {
		return err
	}

	http.Redirect(w, r, fmt.Sprintf("/i/%s%s", id, LangQuery(r)), http.StatusSeeOther)
	return nil
}

func custPurchaseGetBTCPay(w http.ResponseWriter, r *http.Request) error {
	return custPurchaseGet("btcpay", w, r)
}

func custPurchaseGetCash(w http.ResponseWriter, r *http.Request) error {
	return custPurchaseGet("cash", w, r)
}

func custPurchaseGetSEPA(w http.ResponseWriter, r *http.Request) error {
	return custPurchaseGet("sepa", w, r)
}

func custPurchaseGet(activeTab string, w http.ResponseWriter, r *http.Request) error {

	purchaseID := httprouter.ParamsFromContext(r.Context()).ByName("purchaseid")
	purchase, err := database.GetPurchaseByID(purchaseID)
	if err != nil {
		return err
	}

	var paysrvErr error

	// Query payserver in case the webhook has been missed. Load is reduced by querying only if the purchase status is StatusBTCPayInvoiceCreated.
	if purchase.Status == digitalgoods.StatusBTCPayInvoiceCreated {
		if invoice, err := store.GetInvoice(purchase.BTCPayInvoiceID); err == nil {
			// same as in webhook
			switch invoice.Status {
			case btcpay.InvoiceExpired:
				if err := database.SetBTCPayInvoiceExpired(purchase); err != nil {
					return err
				}
				// update purchase
				purchase.Status = digitalgoods.StatusBTCPayInvoiceExpired
			case btcpay.InvoiceProcessing:
				if err := database.SetBTCPayInvoiceProcessing(purchase); err != nil {
					return err
				}
				// re-read purchase
				if purchase, err = database.GetPurchaseByID(purchase.ID); err != nil {
					return err
				}
			case btcpay.InvoiceSettled:
				if err := database.SetSettled(purchase); err != nil {
					return err
				}
				// re-read purchase
				if purchase, err = database.GetPurchaseByID(purchase.ID); err != nil {
					return err
				}
			}
		} else {
			paysrvErr = err
		}
	}

	return html.CustPurchase.Execute(w, &html.CustPurchaseData{
		GroupedOrder: database.GroupedOrder, // returns empty orderGroups too

		Purchase:    purchase,
		URL:         fmt.Sprintf("%s/i/%s%s", AbsHost(r), purchase.ID, LangQuery(r)),
		PaysrvErr:   paysrvErr,
		PreferOnion: strings.HasSuffix(r.Host, ".onion") || strings.Contains(r.Host, ".onion:"),
		Language:    html.GetLanguage(r),
		ActiveTab:   activeTab,
		TabBTCPay:   fmt.Sprintf("/i/%s%s", purchase.ID, LangQuery(r)),
		TabCash:     fmt.Sprintf("/i/%s/cash%s", purchase.ID, LangQuery(r)),
		TabSepa:     fmt.Sprintf("/i/%s/sepa%s", purchase.ID, LangQuery(r)),
	})
}

func custPurchasePostBTCPay(w http.ResponseWriter, r *http.Request) error {

	purchaseID := httprouter.ParamsFromContext(r.Context()).ByName("purchaseid")
	purchase, err := database.GetPurchaseByID(purchaseID)
	if err != nil {
		return err
	}

	if purchase.BTCPayInvoiceID == "" {
		invoiceRequest := &btcpay.InvoiceRequest{
			Amount:   float64(purchase.Ordered.Sum()) / 100.0,
			Currency: "EUR",
		}
		invoiceRequest.DefaultLanguage = html.GetLanguage(r).Translate("btcpay-defaultlanguage")
		invoiceRequest.ExpirationMinutes = 60
		invoiceRequest.OrderID = fmt.Sprintf("digitalgoods %s", purchase.PayID) // PayID only
		invoiceRequest.RedirectURL = fmt.Sprintf("%s/by-cookie", AbsHost(r))
		btcInvoice, err := store.CreateInvoice(invoiceRequest)
		if err != nil {
			return err
		}

		if err := database.SetBTCPayInvoiceID(purchase, btcInvoice.ID); err != nil {
			return err
		}
		purchase.BTCPayInvoiceID = btcInvoice.ID

		// set cookie
		custSessions.Put(r.Context(), "purchase-id", purchase.ID)
	}

	link := store.InvoiceCheckoutLink(purchase.BTCPayInvoiceID)
	if strings.HasSuffix(r.Host, ".onion") || strings.Contains(r.Host, ".onion:") {
		link = store.InvoiceCheckoutLinkPreferOnion(purchase.BTCPayInvoiceID)
	}

	http.Redirect(w, r, link, http.StatusSeeOther)
	return nil
}

func byCookie(w http.ResponseWriter, r *http.Request) {
	if purchaseID := custSessions.GetString(r.Context(), "purchase-id"); purchaseID != "" {
		http.Redirect(w, r, fmt.Sprintf("/i/%s", purchaseID), http.StatusSeeOther)
	} else {
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func rpc(w http.ResponseWriter, r *http.Request) {

	var event, err = store.ProcessWebhook(r)
	if err != nil {
		log.Printf("rpc: error processing webhook: %v", err)
		return
	}

	purchase, err := database.GetPurchaseByBTCPayInvoiceID(event.InvoiceID)
	if err != nil {
		log.Printf("rpc: purchase not found for invoice id: %s", event.InvoiceID)
		return
	}

	switch event.Type {
	case btcpay.EventInvoiceExpired:
		if err := database.SetBTCPayInvoiceExpired(purchase); err != nil {
			log.Printf("rpc: error setting expired %s: %v", purchase.ID, err)
		}
	case btcpay.EventInvoiceSettled:
		if err := database.SetSettled(purchase); err != nil {
			log.Printf("rpc: error fulfilling order %s: %v", purchase.ID, err)
		}
	default:
		log.Printf("rpc: unknown event type: %s", event.Type)
	}
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
	payID := strings.ToUpper(strings.TrimSpace(r.PostFormValue("pay-id")))
	purchase, err := database.GetPurchaseByPayID(payID)
	if err != nil {
		return err
	}
	http.Redirect(w, r, fmt.Sprintf("/mark-paid/%s", purchase.PayID), http.StatusSeeOther)
	return nil
}

func staffMarkPaidGet(w http.ResponseWriter, r *http.Request) error {
	payID := strings.ToUpper(strings.TrimSpace(httprouter.ParamsFromContext(r.Context()).ByName("payid")))
	purchase, err := database.GetPurchaseByPayID(payID)
	if err != nil {
		return err
	}
	return html.StaffMarkPaid.Execute(w, struct {
		*digitalgoods.Purchase
		DB *db.DB
		html.Language
	}{
		purchase,
		database,
		html.GetLanguage(r),
	})
}

func staffMarkPaidPost(w http.ResponseWriter, r *http.Request) error {
	if r.PostFormValue("confirm") == "" {
		return errors.New("You did not confirm.")
	}
	payID := r.PostFormValue("pay-id")
	purchase, err := database.GetPurchaseByPayID(payID)
	if err != nil {
		return err
	}
	if err := database.SetSettled(purchase); err != nil {
		return err
	}
	http.Redirect(w, r, fmt.Sprintf("/mark-paid/%s", purchase.PayID), http.StatusSeeOther)
	return nil
}

type staffSelect struct {
	Articles       []digitalgoods.Article
	Underdelivered map[string]int // key: articleID-countryID
	html.Language
}

func (s *staffSelect) ISOCountryCodes() []string {
	return digitalgoods.ISOCountryCodes[:]
}

func (s *staffSelect) FeaturedCountryIDs(article digitalgoods.Article) []string {
	if !article.HasCountry {
		return []string{"all"}
	}
	ids := []string{}
	for _, countryID := range digitalgoods.ISOCountryCodes {
		if stock := article.Stock[countryID]; stock > 0 || s.Underdelivered[article.ID+"-"+countryID] > 0 {
			ids = append(ids, countryID)
		}
	}
	return ids
}

func (s *staffSelect) OtherCountryIDs(article digitalgoods.Article) []string {
	if !article.HasCountry {
		return nil
	}
	ids := []string{}
	for _, countryID := range digitalgoods.ISOCountryCodes {
		if stock := article.Stock[countryID]; stock == 0 && s.Underdelivered[article.ID+"-"+countryID] == 0 {
			ids = append(ids, countryID)
		}
	}
	return ids
}

func staffSelectGet(w http.ResponseWriter, r *http.Request) error {
	articles, err := database.GetArticles()
	if err != nil {
		return err
	}
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
			underdelivered[uf.ArticleID+"-"+uf.CountryID] += uf.Amount
		}
	}
	return html.StaffSelect.Execute(w, &staffSelect{
		articles,
		underdelivered,
		html.GetLanguage(r),
	})
}

func staffUploadGet(w http.ResponseWriter, r *http.Request) error {
	// redirect to image upload
	http.Redirect(w, r, fmt.Sprintf("/upload/%s/%s/image", httprouter.ParamsFromContext(r.Context()).ByName("articleid"), httprouter.ParamsFromContext(r.Context()).ByName("country")), http.StatusSeeOther)
	return nil
}

func staffUploadImageGet(w http.ResponseWriter, r *http.Request) error {
	article, err := database.GetArticle(httprouter.ParamsFromContext(r.Context()).ByName("articleid"))
	if err != nil {
		return err
	}
	countryID := httprouter.ParamsFromContext(r.Context()).ByName("country")
	return html.StaffUploadImage.Execute(w, struct {
		digitalgoods.Article
		Country string
		html.Language
	}{
		article,
		countryID,
		html.GetLanguage(r),
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
	article, err := database.GetArticle(httprouter.ParamsFromContext(r.Context()).ByName("articleid"))
	if err != nil {
		return err
	}
	countryID := httprouter.ParamsFromContext(r.Context()).ByName("country")
	return html.StaffUploadText.Execute(w, struct {
		digitalgoods.Article
		Country string
		html.Language
	}{
		article,
		countryID,
		html.GetLanguage(r),
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
