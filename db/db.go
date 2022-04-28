package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/dys2p/digitalgoods"
	"golang.org/x/text/language"
)

type DB struct {
	sqlDB *sql.DB

	// purchases
	addPurchase                   *sql.Stmt
	cleanupPurchases              *sql.Stmt
	getPurchaseByID               *sql.Stmt
	getPurchaseByBTCPayInvoiceID  *sql.Stmt
	getPurchaseByPayID            *sql.Stmt
	getPurchasesByStatus          *sql.Stmt
	updatePurchase                *sql.Stmt
	updatePurchaseBTCPayInvoiceID *sql.Stmt
	updateStatus                  *sql.Stmt

	// stock
	addToStock      *sql.Stmt
	deleteFromStock *sql.Stmt
	getFromStock    *sql.Stmt // might return less than n rows
	getStock        *sql.Stmt

	// article
	getArticle            *sql.Stmt
	getArticles           *sql.Stmt
	getArticlesByCategory *sql.Stmt

	// categories
	getCategories           *sql.Stmt
	getCategoryDescriptions *sql.Stmt

	// VAT
	logVAT *sql.Stmt
}

func IsNotFound(err error) bool {
	return err == sql.ErrNoRows
}

func OpenDB() (*DB, error) {

	var sqlDB, err = sql.Open("sqlite3", "data/digitalgoods.sqlite3?_busy_timeout=10000&_journal=WAL&_sync=NORMAL&cache=shared")
	if err != nil {
		return nil, err
	}

	_, err = sqlDB.Exec(`
		pragma foreign_keys = on;
		create table if not exists category (
			id   text not null primary key,
			name text not null
		);
		create table if not exists category_description (
			category text not null,
			language text not null,
			htmltext text not null,
			foreign key (category) references category(id),
			primary key (category, language)
		);
		create table if not exists article (
			id          text    not null primary key,
			category    text    not null,
			name        text    not null,
			price       integer not null, -- euro cents
			ondemand    boolean not null,
			hide        boolean not null, -- article is no longer sold, but we don't delete it from the database because that would break purchases
			has_country boolean not null, -- taxed separately
			foreign key (category) references category(id)
		);
		create table if not exists purchase (
			id          text not null primary key,
			invoiceid   text not null, -- btcpay
			payid       text not null,
			status      text not null,
			ordered     text not null, -- json
			delivered   text not null, -- json (codes removed from stock)
			deletedate  text not null, -- yyyy-mm-dd
			countrycode text not null,
			unique(payid)
		);
		create table if not exists stock (
			article text not null,
			country text not null,
			itemid  text not null primary key,
			image   blob,
			addtime int  not null, -- yyyy-mm-dd, sell oldest first
			foreign key (article) references article(id)
		);
		create table if not exists vat_log (
			deliverydate   text not null, -- yyyy-mm-dd
			article        text not null,
			articlecountry text not null,
			amount         int  not null,
			itemprice      int  not null, -- euro cents
			countrycode    text not null,
			foreign key (article) references article(id)
		);
	`)
	if err != nil {
		return nil, err
	}

	var db = &DB{
		sqlDB: sqlDB,
	}

	mustPrepare := func(s string) *sql.Stmt {
		stmt, err := sqlDB.Prepare(s)
		if err != nil {
			panic(err)
		}
		return stmt
	}

	// purchase
	db.addPurchase = mustPrepare("insert into purchase (id, invoiceid, payid, status, ordered, delivered, deletedate, countrycode) values (?, '', ?, ?, ?, '[]', ?, ?)")
	db.cleanupPurchases = mustPrepare("delete from purchase where status = ? and deletedate != '' and deletedate < ?")
	db.getPurchaseByID = mustPrepare("select id, invoiceid, payid, status, ordered, delivered, deletedate, countrycode from purchase where id = ? limit 1")
	db.getPurchaseByBTCPayInvoiceID = mustPrepare("select id, invoiceid, payid, status, ordered, delivered, deletedate, countrycode from purchase where invoiceid = ? limit 1")
	db.getPurchaseByPayID = mustPrepare("select id, invoiceid, payid, status, ordered, delivered, deletedate, countrycode from purchase where payid = ? limit 1")
	db.getPurchasesByStatus = mustPrepare("select id from purchase where status = ?")
	db.updatePurchase = mustPrepare("update purchase set status = ?, delivered = ?, deletedate = ? where id = ?")
	db.updatePurchaseBTCPayInvoiceID = mustPrepare("update purchase set invoiceid = ?, status = ? where id = ?")
	db.updateStatus = mustPrepare("update purchase set status = ?, deletedate = ? where id = ?")

	// stock
	db.addToStock = mustPrepare("insert into stock (article, country, itemid, image, addtime) values (?, ?, ?, ?, ?)")
	db.deleteFromStock = mustPrepare("delete from stock where itemid = ?") // itemid is primary key
	db.getFromStock = mustPrepare("select itemid, image from stock where article = ? and country = ? order by addtime asc limit ?")
	db.getStock = mustPrepare("select country, count(1) from stock where article = ? group by country")

	// article
	db.getArticle = mustPrepare("select id, category, name, price, ondemand, hide, has_country from article where id = ?")
	db.getArticles = mustPrepare("select id, name, price, ondemand, hide, has_country from article group by id order by name")
	db.getArticlesByCategory = mustPrepare("select id, name, price, ondemand, hide, has_country from article where category = ? group by id order by price asc")

	// categories
	db.getCategories = mustPrepare("select id, name from category order by name")
	db.getCategoryDescriptions = mustPrepare("select category, language, htmltext from category_description")

	// VAT
	db.logVAT = mustPrepare("insert into vat_log (deliverydate, article, articlecountry, amount, itemprice, countrycode) values (?, ?, ?, ?, ?, ?)")

	return db, nil
}

func (db *DB) AddPurchase(order digitalgoods.Order, deleteDate, countryCode string) (string, error) {
	orderJson, err := json.Marshal(order)
	if err != nil {
		return "", err
	}
	for i := 0; i < 5; i++ { // try five times if pay id already exists, see NewPayID
		id := digitalgoods.NewPurchaseID()
		payID := digitalgoods.NewPayID()
		if _, err := db.addPurchase.Exec(id, payID, digitalgoods.StatusNew, orderJson, deleteDate, countryCode); err == nil {
			return id, nil
		}
	}
	return "", errors.New("database ran out of IDs")
}

func (db *DB) AddToStock(articleID, countryID, itemID string, image []byte) error {
	_, err := db.addToStock.Exec(articleID, countryID, itemID, image, time.Now().Format(digitalgoods.DateFmt))
	return err
}

func (db *DB) Cleanup() error {
	result, err := db.cleanupPurchases.Exec(digitalgoods.StatusFinalized, time.Now().Format(digitalgoods.DateFmt))
	if err != nil {
		return err
	}
	if ra, _ := result.RowsAffected(); ra > 0 {
		log.Printf("deleted %d purchases", ra)
	}
	return nil
}

// FulfilUnderdelivered calls SetSettled for all underdelivered purchases. It can be called at any time.
func (db *DB) FulfilUnderdelivered() error {
	// no transaction required because SetSettled is idempotent
	rows, err := db.getPurchasesByStatus.Query(digitalgoods.StatusUnderdelivered)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		purchase, err := db.GetPurchaseByID(id)
		if err != nil {
			return err
		}
		if err := db.SetSettled(purchase); err != nil {
			return err
		}
	}
	return nil
}

func (db *DB) GetArticle(id string) (digitalgoods.Article, error) {
	var article = digitalgoods.Article{}
	if err := db.getArticle.QueryRow(id).Scan(&article.ID, &article.CategoryID, &article.Name, &article.Price, &article.OnDemand, &article.Hide, &article.HasCountry); err != nil {
		return article, err
	}
	if err := db.readStock(&article); err != nil {
		return article, err
	}
	return article, nil
}

func (db *DB) GetArticles() ([]digitalgoods.Article, error) {
	return db.articles(db.getArticles)
}

func (db *DB) GetArticlesByCategory(category *digitalgoods.Category) ([]digitalgoods.Article, error) {
	return db.articles(db.getArticlesByCategory, category.ID)
}

func (db *DB) articles(stmt *sql.Stmt, args ...interface{}) ([]digitalgoods.Article, error) {
	rows, err := stmt.Query(args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var articles = []digitalgoods.Article{}
	for rows.Next() {
		var article = digitalgoods.Article{}
		if err := rows.Scan(&article.ID, &article.Name, &article.Price, &article.OnDemand, &article.Hide, &article.HasCountry); err != nil {
			return nil, err
		}
		if err := db.readStock(&article); err != nil {
			return nil, err
		}
		articles = append(articles, article)
	}
	return articles, nil
}

func (db *DB) readStock(article *digitalgoods.Article) error {
	rows, err := db.getStock.Query(article.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	article.Stock = make(map[string]int)
	for rows.Next() {
		var country string
		var count int
		if err := rows.Scan(&country, &count); err != nil {
			return err
		}
		if count > 0 {
			article.Stock[country] = count
		}
	}
	return nil
}

func (db *DB) GetCategories() ([]*digitalgoods.Category, error) {

	// table category
	rows, err := db.getCategories.Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var categories = []*digitalgoods.Category{}
	var catMap = map[string]*digitalgoods.Category{}
	for rows.Next() {
		var category = &digitalgoods.Category{}
		if err := rows.Scan(&category.ID, &category.Name); err != nil {
			return nil, err
		}
		categories = append(categories, category)
		catMap[category.ID] = category
	}

	// table category_description
	rows, err = db.getCategoryDescriptions.Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var categoryID string
		var lang string
		var htmltext string
		if err := rows.Scan(&categoryID, &lang, &htmltext); err != nil {
			return nil, err
		}
		if c, ok := catMap[categoryID]; ok {
			c.DescriptionLangs = append(c.DescriptionLangs, language.Make(lang))
			c.DescriptionTexts = append(c.DescriptionTexts, htmltext)
		}
	}

	return categories, nil
}

func (db *DB) GetPurchaseByID(id string) (*digitalgoods.Purchase, error) {
	return db.getPurchaseWithStmt(id, db.getPurchaseByID)
}

func (db *DB) GetPurchaseByBTCPayInvoiceID(btcpayInvoiceID string) (*digitalgoods.Purchase, error) {
	return db.getPurchaseWithStmt(btcpayInvoiceID, db.getPurchaseByBTCPayInvoiceID)
}
func (db *DB) GetPurchaseByPayID(btcpayInvoiceID string) (*digitalgoods.Purchase, error) {
	return db.getPurchaseWithStmt(btcpayInvoiceID, db.getPurchaseByPayID)
}

// can be used within or without a transaction
func (db *DB) getPurchaseWithStmt(whereArg string, stmt *sql.Stmt) (*digitalgoods.Purchase, error) {
	var purchase = &digitalgoods.Purchase{}
	var ordered string
	var delivered string
	if err := stmt.QueryRow(whereArg).Scan(&purchase.ID, &purchase.BTCPayInvoiceID, &purchase.PayID, &purchase.Status, &ordered, &delivered, &purchase.DeleteDate, &purchase.CountryCode); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(ordered), &purchase.Ordered); err != nil {
		return nil, fmt.Errorf("unmarshaling ordered: %w", err)
	}
	if err := json.Unmarshal([]byte(delivered), &purchase.Delivered); err != nil {
		return nil, fmt.Errorf("unmarshaling delivered: %w", err)
	}
	// backwards compatibility
	for i := range purchase.Delivered {
		if purchase.Delivered[i].CountryID == "" {
			switch purchase.Delivered[i].ArticleID {
			case "tutanota12":
				fallthrough
			case "tutanota24":
				fallthrough
			case "tutanota48":
				// backwards compatibility, can be removed in one month
				purchase.Delivered[i].CountryID = "DE"
			default:
				purchase.Delivered[i].CountryID = "all"
			}
		}
	}
	return purchase, nil
}

// GetPurchases returns the IDs of all purchases with the given status.
func (db *DB) GetPurchases(status string) ([]string, error) {
	rows, err := db.getPurchasesByStatus.Query(status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids = []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (db *DB) SetBTCPayInvoiceExpired(purchase *digitalgoods.Purchase) error {
	_, err := db.updateStatus.Exec(digitalgoods.StatusBTCPayInvoiceExpired, time.Now().AddDate(0, 0, 31).Format(digitalgoods.DateFmt), purchase.ID)
	return err
}

func (db *DB) SetBTCPayInvoiceProcessing(purchase *digitalgoods.Purchase) error {
	_, err := db.updateStatus.Exec(digitalgoods.StatusBTCPayInvoiceProcessing, time.Now().AddDate(0, 0, 31).Format(digitalgoods.DateFmt), purchase.ID)
	return err
}

// idempotent, must be called only if the invoice has been paid
func (db *DB) SetSettled(purchase *digitalgoods.Purchase) error {

	tx, err := db.sqlDB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() // no effect if tx has been committed

	unfulfilled, err := purchase.GetUnfulfilled()
	if err != nil {
		return err
	}
	if len(unfulfilled) == 0 {
		return nil
	}

	for _, u := range unfulfilled {

		// get from stock

		rows, err := tx.Stmt(db.getFromStock).Query(u.ArticleID, u.CountryID, u.Amount)
		if err != nil {
			return err
		}
		defer rows.Close()

		var gotAmount = 0

		for rows.Next() {
			var itemID string
			var image []byte
			if err := rows.Scan(&itemID, &image); err != nil {
				return err
			}
			if _, err := tx.Stmt(db.deleteFromStock).Exec(itemID); err != nil {
				return err
			}
			log.Printf("delivering %s: %s", digitalgoods.Mask(purchase.ID), digitalgoods.Mask(itemID))
			purchase.Delivered = append(purchase.Delivered, digitalgoods.DeliveredItem{
				ArticleID:    u.ArticleID,
				CountryID:    u.CountryID,
				ID:           itemID,
				Image:        image,
				DeliveryDate: time.Now().Format(digitalgoods.DateFmt),
			})
			gotAmount++
		}

		// log VAT

		if gotAmount > 0 {
			if _, err := tx.Stmt(db.logVAT).Exec(time.Now().Format(digitalgoods.DateFmt), u.ArticleID, u.CountryID, gotAmount, u.ItemPrice, purchase.CountryCode); err != nil {
				return err
			}
		}
	}

	deliveredBytes, err := json.Marshal(purchase.Delivered)
	if err != nil {
		return err
	}

	var newDeleteDate string
	var newStatus string
	unfulfilled, err = purchase.GetUnfulfilled()
	if err != nil {
		return err
	}
	if unfulfilled.Empty() {
		newDeleteDate = time.Now().AddDate(0, 0, 31).Format(digitalgoods.DateFmt)
		newStatus = digitalgoods.StatusFinalized
	} else {
		newDeleteDate = "" // don't delete
		newStatus = digitalgoods.StatusUnderdelivered
	}

	if _, err := tx.Stmt(db.updatePurchase).Exec(newStatus, string(deliveredBytes), newDeleteDate, purchase.ID); err != nil {
		return err
	}

	return tx.Commit()
}

// SetBTCPayInvoiceID sets the invoice ID to the given value and the status to StatusBTCPayInvoiceCreated.
func (db *DB) SetBTCPayInvoiceID(purchase *digitalgoods.Purchase, btcpayInvoiceID string) error {
	if purchase.BTCPayInvoiceID != "" {
		return errors.New("an invoice has already been created")
	}
	_, err := db.updatePurchaseBTCPayInvoiceID.Exec(btcpayInvoiceID, digitalgoods.StatusBTCPayInvoiceCreated, purchase.ID)
	return err
}

func (db *DB) GroupedOrder(order digitalgoods.Order) ([]digitalgoods.OrderGroup, error) {
	categories, err := db.GetCategories()
	if err != nil {
		return nil, err
	}
	result := make([]digitalgoods.OrderGroup, len(categories))
	for i := range categories {
		result[i].Category = categories[i]
	}
	for _, row := range order {
		article, err := db.GetArticle(row.ArticleID)
		if err != nil {
			return nil, err
		}
		// linear search, well...
		for i := range categories {
			if categories[i].ID == article.CategoryID {
				result[i].Rows = append(result[i].Rows, digitalgoods.OrderArticle{
					OrderRow: row,
					Article:  &article,
				})
			}
		}
		// don't check the unlikely case that no category is found because this is just the "ordered" section and not the "delivered goods" section
	}
	return result, nil
}
