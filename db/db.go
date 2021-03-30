package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

const DateFmt = "2006-01-02"

type DB struct {
	sqlDB                *sql.DB
	addPurchase          *sql.Stmt
	addToStock           *sql.Stmt
	cleanupPurchases     *sql.Stmt
	deleteFromStock      *sql.Stmt
	getArticle           *sql.Stmt
	getArticles          *sql.Stmt
	getAvailableArticles *sql.Stmt
	getFromStock         *sql.Stmt
	getPurchase          *sql.Stmt
	getPurchases         *sql.Stmt
	updatePurchase       *sql.Stmt
	updateStatus         *sql.Stmt
}

func IsNotFound(err error) bool {
	return err == sql.ErrNoRows
}

func OpenDB() (*DB, error) {

	var sqlDB, err = sql.Open("sqlite3", "data/digitalgoods.sqlite3?_busy_timeout=10000&_journal=WAL&_sync=NORMAL&cache=shared")
	if err != nil {
		return nil, err
	}

	var db = &DB{
		sqlDB: sqlDB,
	}

	_, err = sqlDB.Exec(`
		create table if not exists article (
			id    TEXT    NOT NULL PRIMARY KEY, -- article id, like "foobar6"
			name  TEXT    NOT NULL,
			price INTEGER NOT NULL  -- euro cents
		);
		create table if not exists purchase (
			invoiceid  TEXT NOT NULL PRIMARY KEY,
			status     TEXT NOT NULL,
			ordered    TEXT NOT NULL, -- json
			delivered  TEXT NOT NULL, -- json (codes removed from stock)
			deletedate TEXT NOT NULL  -- yyyy-mm-dd
		);
		create table if not exists stock (
			article TEXT    NOT NULL, -- article id
			itemid  TEXT    NOT NULL PRIMARY KEY,
			image   BLOB,
			addtime INTEGER NOT NULL -- yyyy-mm-dd, sell oldest first
		);
	`)
	if err != nil {
		return nil, err
	}

	db.addPurchase, err = db.sqlDB.Prepare("insert into purchase (invoiceid, status, ordered, delivered, deletedate) values (?, ?, ?, '[]', '')")
	if err != nil {
		return nil, err
	}

	db.addToStock, err = db.sqlDB.Prepare("insert into stock (article, itemid, image, addtime) values (?, ?, ?, ?)")
	if err != nil {
		return nil, err
	}

	db.cleanupPurchases, err = db.sqlDB.Prepare("delete from purchase where status = ? and deletedate != '' and deletedate < ?")
	if err != nil {
		return nil, err
	}

	db.deleteFromStock, err = db.sqlDB.Prepare("delete from stock where itemid = ?")
	if err != nil {
		return nil, err
	}

	db.getArticle, err = db.sqlDB.Prepare("select id, name, price from article where id = ?")
	if err != nil {
		return nil, err
	}

	// left join
	db.getArticles, err = db.sqlDB.Prepare("select a.id, a.name, a.price, count(s.article) from article a left join stock s on a.id = s.article group by a.id order by a.name")
	if err != nil {
		return nil, err
	}

	// inner join
	db.getAvailableArticles, err = db.sqlDB.Prepare("select a.id, a.name, a.price, count(s.article) from article a inner join stock s on a.id = s.article group by a.id order by a.name")
	if err != nil {
		return nil, err
	}

	db.getFromStock, err = db.sqlDB.Prepare("select itemid, image from stock where article = ? order by addtime asc limit ?") // might return less than n rows
	if err != nil {
		return nil, err
	}

	db.getPurchase, err = db.sqlDB.Prepare("select status, ordered, delivered, deletedate from purchase where invoiceid = ? limit 1")
	if err != nil {
		return nil, err
	}

	db.getPurchases, err = db.sqlDB.Prepare("select invoiceid from purchase where status = ?")
	if err != nil {
		return nil, err
	}

	db.updatePurchase, err = db.sqlDB.Prepare("update purchase set status = ?, delivered = ?, deletedate = ? where invoiceid = ?")
	if err != nil {
		return nil, err
	}

	db.updateStatus, err = db.sqlDB.Prepare("update purchase set status = ?, deletedate = ? where invoiceid = ?")
	if err != nil {
		return nil, err
	}

	return db, nil
}

func (db *DB) AddPurchase(invoiceID string, order Order) error {
	orderJson, err := json.Marshal(order)
	if err != nil {
		return err
	}
	_, err = db.addPurchase.Exec(invoiceID, StatusNew, orderJson)
	return err
}

func (db *DB) AddToStock(articleID, itemID string, image []byte) error {
	_, err := db.addToStock.Exec(articleID, itemID, image, time.Now().Format(DateFmt))
	return err
}

func (db *DB) Cleanup() error {
	result, err := db.cleanupPurchases.Exec(StatusFinalized, time.Now().Format(DateFmt))
	if err != nil {
		return err
	}
	if ra, _ := result.RowsAffected(); ra > 0 {
		log.Printf("deleted %d purchases", ra)
	}
	return nil
}

// FulfilUnderdelivered can be called at any time.
func (db *DB) FulfilUnderdelivered() error {
	// no transaction required because SetSettled is idempotent
	rows, err := db.getPurchases.Query(StatusUnderdelivered)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var invoiceID string
		if err := rows.Scan(&invoiceID); err != nil {
			return err
		}
		if err := db.SetSettled(invoiceID); err != nil {
			return err
		}
	}
	return nil
}

func (db *DB) GetArticle(id string) (Article, error) {
	var article = Article{}
	return article, db.getArticle.QueryRow(id).Scan(&article.ID, &article.Name, &article.Price)
}

func (db *DB) GetArticles() ([]Article, error) {
	return db.getArticlesWithStmt(db.getArticles)
}

func (db *DB) GetAvailableArticles() ([]Article, error) {
	return db.getArticlesWithStmt(db.getAvailableArticles)
}

func (db *DB) getArticlesWithStmt(stmt *sql.Stmt) ([]Article, error) {
	rows, err := stmt.Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var articles = []Article{}
	for rows.Next() {
		var article = Article{}
		if err := rows.Scan(&article.ID, &article.Name, &article.Price, &article.Stock); err != nil {
			return nil, err
		}
		articles = append(articles, article)
	}
	return articles, nil
}

func (db *DB) GetPurchase(id string) (*Purchase, error) {
	return db.getPurchaseWithStmt(id, db.getPurchase)
}

// can be used within or without a transaction
func (db *DB) getPurchaseWithStmt(invoiceID string, stmt *sql.Stmt) (*Purchase, error) {

	var status string
	var ordered string
	var delivered string
	var deleteDate string
	if err := stmt.QueryRow(invoiceID).Scan(&status, &ordered, &delivered, &deleteDate); err != nil {
		return nil, err
	}

	var purchase = &Purchase{
		InvoiceID:  invoiceID,
		Status:     status,
		DeleteDate: deleteDate,
	}
	if err := json.Unmarshal([]byte(ordered), &purchase.Ordered); err != nil {
		return nil, fmt.Errorf("unmarshaling ordered: %w", err)
	}
	if err := json.Unmarshal([]byte(delivered), &purchase.Delivered); err != nil {
		return nil, fmt.Errorf("unmarshaling delivered: %w", err)
	}
	return purchase, nil
}

// GetPurchases returns the IDs of all purchases with the given status.
func (db *DB) GetPurchases(status string) ([]string, error) {
	rows, err := db.getPurchases.Query(status)
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

func (db *DB) SetExpired(id string) error {
	_, err := db.updateStatus.Exec(StatusExpired, time.Now().AddDate(0, 0, 31).Format(DateFmt), id)
	return err
}

// idempotent, must be called only if the invoice has been paid
func (db *DB) SetSettled(id string) error {

	tx, err := db.sqlDB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() // no effect if tx has been committed

	purchase, err := db.getPurchaseWithStmt(id, tx.Stmt(db.getPurchase))
	if err != nil {
		return err
	}

	unfulfilled := purchase.GetUnfulfilled()
	if len(unfulfilled) == 0 {
		return nil
	}

	for articleID, amount := range unfulfilled {

		rows, err := tx.Stmt(db.getFromStock).Query(articleID, amount)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var itemID string
			var image []byte
			if err := rows.Scan(&itemID, &image); err != nil {
				return err
			}
			if _, err := tx.Stmt(db.deleteFromStock).Exec(itemID); err != nil {
				return err
			}
			log.Printf("delivering %s: %s", id, Mask(itemID, 4))
			purchase.Delivered = append(purchase.Delivered, DeliveredItem{ArticleID: articleID, ID: itemID, Image: image, DeliveryDate: time.Now().Format(DateFmt)})
		}
	}

	deliveredBytes, err := json.Marshal(purchase.Delivered)
	if err != nil {
		return err
	}

	var newDeleteDate string
	var newStatus string
	if len(purchase.GetUnfulfilled()) == 0 {
		newDeleteDate = time.Now().AddDate(0, 0, 31).Format(DateFmt)
		newStatus = StatusFinalized
	} else {
		newStatus = StatusUnderdelivered
	}

	if _, err := tx.Stmt(db.updatePurchase).Exec(newStatus, string(deliveredBytes), newDeleteDate, id); err != nil {
		return err
	}

	return tx.Commit()
}
