package main

import (
	"embed"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/alexedwards/scs/v2/memstore"
	"github.com/dchest/captcha"
	"github.com/dys2p/btcpay"
	"github.com/dys2p/digitalgoods/db"
	"github.com/dys2p/digitalgoods/html"
	"github.com/dys2p/digitalgoods/userdb"
	"github.com/julienschmidt/httprouter"
	_ "github.com/mattn/go-sqlite3"
)

var database *db.DB
var sessionManager *scs.SessionManager
var store btcpay.Store
var users userdb.Authenticator

//go:embed static
var static embed.FS

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
		log.Println(`  Event: "An invoice has expired"`)
		log.Println(`  Event: "An invoice has been settled"`)
	}

	users, err = userdb.Open()
	if err != nil {
		log.Printf("error opening userdb: %v", err)
		return
	}

	var stop = make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// assets for customer and staff routers

	var substatic, _ = fs.Sub(fs.FS(static), "static")

	// customer http server

	var custRtr = httprouter.New()
	custRtr.ServeFiles("/static/*filepath", http.FS(substatic))
	custRtr.HandlerFunc(http.MethodGet, "/", wrapTmpl(custOrderGet))
	custRtr.HandlerFunc(http.MethodPost, "/", wrapTmpl(custOrderPost))
	custRtr.HandlerFunc(http.MethodGet, "/i/:invoiceid", wrapTmpl(custPurchaseGet))
	custRtr.HandlerFunc(http.MethodGet, "/health", health)
	custRtr.HandlerFunc(http.MethodPost, "/rpc", rpc)
	custRtr.Handler("GET", "/captcha/:fn", captcha.Server(captcha.StdWidth, captcha.StdHeight))

	var custSrv = ListenAndServe("tcp", ":9002", custRtr, stop)
	defer custSrv.Shutdown()

	// staff http server with session management

	sessionManager = scs.New()
	sessionManager.Cookie.SameSite = http.SameSiteLaxMode // prevent CSRF
	sessionManager.Store = memstore.New()

	var staffRtr = httprouter.New()
	staffRtr.ServeFiles("/static/*filepath", http.FS(substatic))
	staffRtr.HandlerFunc(http.MethodGet, "/login", wrapTmpl(staffLoginGet))
	staffRtr.HandlerFunc(http.MethodPost, "/login", wrapTmpl(staffLoginPost))
	// with authentication:
	staffRtr.HandlerFunc(http.MethodGet, "/", auth(wrapTmpl(staffIndexGet)))
	staffRtr.HandlerFunc(http.MethodGet, "/logout", auth(wrapTmpl(staffLogoutGet)))
	staffRtr.HandlerFunc(http.MethodGet, "/mark-paid", auth(wrapTmpl(staffMarkPaidGet)))
	staffRtr.HandlerFunc(http.MethodPost, "/mark-paid", auth(wrapTmpl(staffMarkPaidPost)))
	staffRtr.HandlerFunc(http.MethodPost, "/mark-paid-confirm", auth(wrapTmpl(staffMarkPaidConfirmPost)))
	staffRtr.HandlerFunc(http.MethodGet, "/upload", auth(wrapTmpl(staffSelectGet)))
	staffRtr.HandlerFunc(http.MethodGet, "/upload/:articleid", auth(wrapTmpl(staffUploadGet)))
	staffRtr.HandlerFunc(http.MethodGet, "/upload/:articleid/image", auth(wrapTmpl(staffUploadImageGet)))
	staffRtr.HandlerFunc(http.MethodPost, "/upload/:articleid/image", auth(wrapAPI(staffUploadImagePost)))
	staffRtr.HandlerFunc(http.MethodGet, "/upload/:articleid/text", auth(wrapTmpl(staffUploadTextGet)))
	staffRtr.HandlerFunc(http.MethodPost, "/upload/:articleid/text", auth(wrapAPI(staffUploadTextPost)))

	var staffSrv = ListenAndServe("tcp", "127.0.0.1:9003", sessionManager.LoadAndSave(staffRtr), stop)
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

type custOrder struct {
	CaptchaAnswer string
	CaptchaErr    bool
	CaptchaID     string
	Cart          map[string]int
	CountryAnswer string
	CountryErr    bool
	OrderErr      bool
	html.Language
}

func (*custOrder) ArticlesByCategory(category db.Category) ([]db.Article, error) {
	return database.GetArticlesByCategory(category)
}

func (*custOrder) Categories() ([]*db.Category, error) {
	return database.GetCategories()
}

func custOrderGet(w http.ResponseWriter, r *http.Request) error {
	lang := html.GetLanguage(r)
	return html.CustOrder.Execute(w, &custOrder{
		CaptchaID:     captcha.NewLen(6),
		CountryAnswer: lang.Translate("default-county"),
		Language:      lang,
	})
}

func custOrderPost(w http.ResponseWriter, r *http.Request) error {

	// read user input

	co := &custOrder{
		CaptchaAnswer: r.PostFormValue("captcha-answer"),
		CaptchaID:     r.PostFormValue("captcha-id"),
		Cart:          make(map[string]int),
		CountryAnswer: r.PostFormValue("country"),
		Language:      html.GetLanguage(r),
	}

	articles, err := database.GetArticles()
	if err != nil {
		return err
	}

	order := db.Order{}

	for _, a := range articles {
		if a.Hide {
			continue
		}
		val := r.PostFormValue(a.ID)
		if val == "" {
			continue
		}
		amount, _ := strconv.Atoi(val)
		if amount > a.Stock {
			amount = a.Stock // client must check their order before payment
		}
		if amount <= 0 {
			continue
		}
		co.Cart[a.ID] = amount
		order = append(order, db.OrderRow{Amount: amount, ArticleID: a.ID, ItemPrice: a.Price})
	}

	// validate user input

	if len(order) == 0 {
		co.OrderErr = true
		return html.CustOrder.Execute(w, co)
	}

	if !html.IsCountryCode(co.CountryAnswer) {
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

	invoiceRequest := &btcpay.InvoiceRequest{
		Amount:   order.SumEUR(),
		Currency: "EUR",
	}
	invoiceRequest.ExpirationMinutes = 60
	invoiceRequest.DefaultLanguage = "de-DE"
	invoiceRequest.OrderID = "digitalgoods"
	invoiceRequest.RedirectURL = fmt.Sprintf("%s/i/{InvoiceId}%s", AbsHost(r), LangQuery(r)) // purchase ID is invoice ID
	btcInvoice, err := store.CreateInvoice(invoiceRequest)
	if err != nil {
		return err
	}

	if err := database.AddPurchase(btcInvoice.ID, order, co.CountryAnswer); err != nil {
		return err
	}

	http.Redirect(w, r, fmt.Sprintf("/i/%s%s", btcInvoice.ID, LangQuery(r)), http.StatusSeeOther)
	return nil
}

type custPurchase struct {
	Purchase         *db.Purchase
	URL              string
	PaysrvErr        error
	IsNew            bool
	IsUnderdelivered bool
	html.Language
}

func (cp *custPurchase) CheckoutLink() template.URL {
	return template.URL(store.InvoiceCheckoutLink(cp.Purchase.InvoiceID))
}

func (cp *custPurchase) GetArticleName(id string) string {
	article, err := database.GetArticle(id)
	if err != nil {
		return id
	}
	return article.Name
}

type orderGroup struct {
	Category *db.Category
	Rows     []db.OrderRow
}

// returns empty orderGroups too
func (cp *custPurchase) GroupedOrder() ([]orderGroup, error) {
	categories, err := database.GetCategories()
	if err != nil {
		return nil, err
	}
	result := make([]orderGroup, len(categories))
	for i := range categories {
		result[i].Category = categories[i]
		result[i].Rows = []db.OrderRow{}
	}
	for _, row := range cp.Purchase.Ordered {
		article, err := database.GetArticle(row.ArticleID)
		if err != nil {
			return nil, err
		}
		// linear search, well...
		for i := range categories {
			if categories[i].ID == article.CategoryID {
				result[i].Rows = append(result[i].Rows, row)
			}
		}
		// don't check the unlikely case that no category is found because this is just the "ordered" section and not the "delivered goods" section
	}
	return result, nil
}

func custPurchaseGet(w http.ResponseWriter, r *http.Request) error {

	invoiceID := httprouter.ParamsFromContext(r.Context()).ByName("invoiceid")

	purchase, err := database.GetPurchase(invoiceID)
	if err != nil {
		return err
	}

	var paysrvErr error

	// Query payserver in case the webhook has been missed. Load is reduced by querying only if the purchase status is new.
	if purchase.Status == db.StatusNew {
		if invoice, err := store.GetInvoice(invoiceID); err == nil {
			// same as in webhook
			switch invoice.Status {
			case btcpay.InvoiceExpired:
				if err := database.SetExpired(purchase.InvoiceID); err != nil {
					return err
				}
				// update purchase
				purchase.Status = db.StatusExpired
			case btcpay.InvoiceSettled:
				if err := database.SetSettled(purchase.InvoiceID); err != nil {
					return err
				}
				// reload purchase
				if purchase, err = database.GetPurchase(invoiceID); err != nil {
					return err
				}
			}
		} else {
			paysrvErr = err
		}
	}

	return html.CustPurchase.Execute(w, &custPurchase{
		purchase,
		AbsHost(r) + r.URL.String(),
		paysrvErr,
		purchase.Status == db.StatusNew,
		purchase.Status == db.StatusUnderdelivered,
		html.GetLanguage(r),
	})
}

func rpc(w http.ResponseWriter, r *http.Request) {

	var event, err = store.ProcessWebhook(r)
	if err != nil {
		log.Printf("rpc: error processing webhook: %v", err)
		return
	}

	switch event.Type {
	case btcpay.EventInvoiceExpired:
		if err := database.SetExpired(event.InvoiceID); err != nil {
			log.Printf("rpc: error setting expired %s: %v", event.InvoiceID, err)
		}
	case btcpay.EventInvoiceSettled:
		if err := database.SetSettled(event.InvoiceID); err != nil {
			log.Printf("rpc: error fulfilling order %s: %v", event.InvoiceID, err)
		}
	default:
		log.Printf("rpc: unknown event type: %s", event.Type)
	}
}

func staffIndexGet(w http.ResponseWriter, r *http.Request) error {
	unfulfilled, err := database.GetPurchases(db.StatusUnderdelivered)
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
	sessionManager.Put(r.Context(), "username", username)
	http.Redirect(w, r, "/", http.StatusSeeOther)
	return nil
}

func staffLogoutGet(w http.ResponseWriter, r *http.Request) error {
	sessionManager.Destroy(r.Context())
	http.Redirect(w, r, "/", http.StatusSeeOther)
	return nil
}

func staffMarkPaidGet(w http.ResponseWriter, r *http.Request) error {
	return html.StaffMarkPaid.Execute(w, nil)
}

func staffMarkPaidPost(w http.ResponseWriter, r *http.Request) error {
	id := r.PostFormValue("id")
	purchase, err := database.GetPurchase(id)
	if err != nil {
		return err
	}
	return html.StaffMarkPaidConfirm.Execute(w, purchase)
}

func staffMarkPaidConfirmPost(w http.ResponseWriter, r *http.Request) error {
	if r.PostFormValue("confirm") == "" {
		return errors.New("You did not confirm.")
	}
	if err := database.SetSettled(r.PostFormValue("id")); err != nil {
		return err
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
	return nil
}

func staffSelectGet(w http.ResponseWriter, r *http.Request) error {
	articles, err := database.GetArticles()
	if err != nil {
		return nil
	}
	return html.StaffSelect.Execute(w, articles)
}

func staffUploadGet(w http.ResponseWriter, r *http.Request) error {
	// redirect to image upload
	http.Redirect(w, r, fmt.Sprintf("/upload/%s/image", httprouter.ParamsFromContext(r.Context()).ByName("articleid")), http.StatusSeeOther)
	return nil
}

func staffUploadImageGet(w http.ResponseWriter, r *http.Request) error {
	article, err := database.GetArticle(httprouter.ParamsFromContext(r.Context()).ByName("articleid"))
	if err != nil {
		return err
	}
	return html.StaffUploadImage.Execute(w, article)
}

func staffUploadImagePost(w http.ResponseWriter, r *http.Request) error {

	var articleID = httprouter.ParamsFromContext(r.Context()).ByName("articleid")

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

	if err := database.AddToStock(articleID, header.Filename, data); err != nil {
		return err
	}

	log.Printf("added image to stock: %s %s", articleID, db.Mask(header.Filename, 4))

	return database.FulfilUnderdelivered()
}

func staffUploadTextGet(w http.ResponseWriter, r *http.Request) error {
	article, err := database.GetArticle(httprouter.ParamsFromContext(r.Context()).ByName("articleid"))
	if err != nil {
		return err
	}
	return html.StaffUploadText.Execute(w, article)
}

func staffUploadTextPost(w http.ResponseWriter, r *http.Request) error {

	var articleID = httprouter.ParamsFromContext(r.Context()).ByName("articleid")

	for _, code := range strings.Fields(r.PostFormValue("codes")) {
		if err := database.AddToStock(articleID, code, nil); err == nil {
			log.Printf("added code to stock: %s %s", articleID, db.Mask(code, 20))
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
