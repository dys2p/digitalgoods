package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/dys2p/digitalgoods"
	"github.com/dys2p/eco/id"
)

type DB struct {
	sqlDB *sql.DB

	// purchases
	insertPurchase               *sql.Stmt
	cleanupPurchases             *sql.Stmt
	getIDByPattern               *sql.Stmt
	getPurchaseByID              *sql.Stmt
	getPurchaseByIDAndAccessKey  *sql.Stmt
	getPurchaseByIDAndPaymentKey *sql.Stmt
	getPurchasesByStatus         *sql.Stmt
	updatePurchase               *sql.Stmt
	updatePurchaseCountry        *sql.Stmt
	updatePurchaseNotify         *sql.Stmt
	updatePurchaseStatus         *sql.Stmt

	// stock
	addToStock      *sql.Stmt
	deleteFromStock *sql.Stmt
	getFromStock    *sql.Stmt // might return less than n rows
	getStock        *sql.Stmt
	getStockAll     *sql.Stmt

	// VAT
	logVAT *sql.Stmt
}

func IsNotFound(err error) bool {
	return err == sql.ErrNoRows
}

func OpenDB() (*DB, error) {

	var sqlDB, err = sql.Open("sqlite3", filepath.Join(os.Getenv("STATE_DIRECTORY"), "digitalgoods.sqlite3?_busy_timeout=10000&_journal=WAL&_sync=NORMAL&cache=shared"))
	if err != nil {
		return nil, err
	}

	_, err = sqlDB.Exec(`
		create table if not exists purchase (
			id          text not null primary key,
			access_key  text not null,
			payment_key text not null,
			status      text not null,
			notifyproto text not null,
			notifyaddr  text not null,
			ordered     text not null, -- json
			delivered   text not null, -- json (codes removed from stock)
			create_date text not null, -- yyyy-mm-dd
			deletedate  text not null, -- yyyy-mm-dd
			countrycode text not null,
			unique(access_key),
			unique(payment_key)
		);
		create table if not exists stock (
			variant text not null,
			itemid  text not null primary key,
			addtime int  not null -- yyyy-mm-dd, sell oldest first
		);
		create table if not exists vat_log (
			purchase       text not null, -- six-digit id
			deliverydate   text not null, -- yyyy-mm-dd
			variant        text not null,
			amount         int  not null,
			itemprice      int  not null, -- euro cents
			countrycode    text not null
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
	db.insertPurchase = mustPrepare("insert into purchase (id, access_key, payment_key, status, notifyproto, notifyaddr, ordered, delivered, create_date, deletedate, countrycode) values (?, ?, ?, ?, ?, ?, ?, '[]', ?, ?, ?)")
	db.cleanupPurchases = mustPrepare("delete from purchase where status = ? and deletedate != '' and deletedate < ?")
	db.getIDByPattern = mustPrepare("select id from purchase where id like ? limit 10")
	db.getPurchaseByID = mustPrepare("             select id, access_key, payment_key, status, notifyproto, notifyaddr, ordered, delivered, create_date, deletedate, countrycode from purchase where id = ? limit 1")
	db.getPurchaseByIDAndAccessKey = mustPrepare(" select id, access_key, payment_key, status, notifyproto, notifyaddr, ordered, delivered, create_date, deletedate, countrycode from purchase where id = ? and access_key = ? limit 1")
	db.getPurchaseByIDAndPaymentKey = mustPrepare("select id, access_key, payment_key, status, notifyproto, notifyaddr, ordered, delivered, create_date, deletedate, countrycode from purchase where id = ? and payment_key = ? limit 1")
	db.getPurchasesByStatus = mustPrepare("select id from purchase where status = ?")
	db.updatePurchase = mustPrepare("update purchase set status = ?, delivered = ?, deletedate = ? where id = ?")
	db.updatePurchaseCountry = mustPrepare("update purchase set countrycode = ?                 where id = ?")
	db.updatePurchaseNotify = mustPrepare(" update purchase set notifyproto = ?, notifyaddr = ? where id = ?")
	db.updatePurchaseStatus = mustPrepare(" update purchase set status = ?, deletedate = ?      where id = ?")

	// stock
	db.addToStock = mustPrepare(`
		insert into stock (variant, itemid, addtime)
		values (?, ?, ?)
	`)
	db.deleteFromStock = mustPrepare(`
		delete
		from stock
		where itemid = ?
	`) // itemid is primary key
	db.getFromStock = mustPrepare(`
		select itemid
		from stock
		where variant = ?
		order by addtime asc
		limit ?
	`)
	db.getStock = mustPrepare(`
		select count(1)
		from stock
		where variant = ?
	`)
	db.getStockAll = mustPrepare(`
		select variant, count(1)
		from stock
		group by variant
	`)

	// VAT
	db.logVAT = mustPrepare("insert into vat_log (purchase, deliverydate, variant, amount, itemprice, countrycode) values (?, ?, ?, ?, ?, ?)")

	return db, nil
}

func (db *DB) InsertPurchase(purchase *digitalgoods.Purchase) error {
	orderJson, err := json.Marshal(purchase.Ordered)
	if err != nil {
		return err
	}
	for i := 0; i < 5; i++ { // try five times if pay id already exists, see id.New
		purchase.ID = id.New(6, id.AlphanumCaseInsensitiveDigits)
		if _, err = db.insertPurchase.Exec(purchase.ID, purchase.AccessKey, purchase.PaymentKey, purchase.Status, purchase.NotifyProto, purchase.NotifyAddr, orderJson, purchase.CreateDate, purchase.DeleteDate, purchase.CountryCode); err == nil {
			return nil
		}
	}
	log.Printf("database ran out of IDs, or other error: %v", err)
	return errors.New("database ran out of IDs")
}

func (db *DB) AddToStock(variantID, itemID string) error {
	_, err := db.addToStock.Exec(variantID, itemID, time.Now().Format(digitalgoods.DateFmt))
	return err
}

func (db *DB) GetStock() (digitalgoods.Stock, error) {
	rows, err := db.getStockAll.Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stock = make(digitalgoods.Stock)
	for rows.Next() {
		var variant string
		var count int
		if err := rows.Scan(&variant, &count); err != nil {
			return nil, err
		}
		stock[variant] = stock[variant] + count
	}
	return stock, nil
}

func (db *DB) Cleanup() error {
	// new
	result, err := db.cleanupPurchases.Exec(digitalgoods.StatusNew, time.Now().Format(digitalgoods.DateFmt))
	if err != nil {
		return err
	}
	if ra, _ := result.RowsAffected(); ra > 0 {
		log.Printf("deleted %d new purchases", ra)
	}

	// finalized
	result, err = db.cleanupPurchases.Exec(digitalgoods.StatusFinalized, time.Now().Format(digitalgoods.DateFmt))
	if err != nil {
		return err
	}
	if ra, _ := result.RowsAffected(); ra > 0 {
		log.Printf("deleted %d finalized purchases", ra)
	}
	return nil
}

func (db *DB) GetIDsByPattern(pattern string) ([]string, error) {
	rows, err := db.getIDByPattern.Query(pattern)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		err := rows.Scan(&id)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
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

func (db *DB) GetPurchaseByID(id string) (*digitalgoods.Purchase, error) {
	return db.getPurchaseWithStmt(db.getPurchaseByID, id)
}

func (db *DB) GetPurchaseByIDAndAccessKey(id, accessKey string) (*digitalgoods.Purchase, error) {
	return db.getPurchaseWithStmt(db.getPurchaseByIDAndAccessKey, id, accessKey)
}

func (db *DB) GetPurchaseByIDAndPaymentKey(id, paymentKey string) (*digitalgoods.Purchase, error) {
	return db.getPurchaseWithStmt(db.getPurchaseByIDAndPaymentKey, id, paymentKey)
}

// can be used within or without a transaction
func (db *DB) getPurchaseWithStmt(stmt *sql.Stmt, args ...any) (*digitalgoods.Purchase, error) {
	var purchase = &digitalgoods.Purchase{}
	var ordered string
	var delivered string
	if err := stmt.QueryRow(args...).Scan(&purchase.ID, &purchase.AccessKey, &purchase.PaymentKey, &purchase.Status, &purchase.NotifyProto, &purchase.NotifyAddr, &ordered, &delivered, &purchase.CreateDate, &purchase.DeleteDate, &purchase.CountryCode); err != nil {
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
			switch purchase.Delivered[i].VariantID {
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
func (db *DB) GetPurchases(status digitalgoods.Status) ([]string, error) {
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

func (db *DB) SetProcessing(purchase *digitalgoods.Purchase) error {
	_, err := db.updatePurchaseStatus.Exec(digitalgoods.StatusPaymentProcessing, time.Now().AddDate(0, 0, 31).Format(digitalgoods.DateFmt), purchase.ID)
	return err
}

func (db *DB) SetCountry(purchase *digitalgoods.Purchase, countryCode string) error {
	_, err := db.updatePurchaseCountry.Exec(countryCode, purchase.ID)
	return err
}

func (db *DB) SetNotify(purchase *digitalgoods.Purchase) error {
	_, err := db.updatePurchaseNotify.Exec(purchase.NotifyProto, purchase.NotifyAddr, purchase.ID)
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

		rows, err := tx.Stmt(db.getFromStock).Query(u.VariantID, u.Quantity)
		if err != nil {
			return err
		}
		defer rows.Close()

		var gotQuantity = 0

		for rows.Next() {
			var itemID string
			if err := rows.Scan(&itemID); err != nil {
				return err
			}
			if _, err := tx.Stmt(db.deleteFromStock).Exec(itemID); err != nil {
				return err
			}
			log.Printf("[%s] delivering %s: %s", purchase.ID, u.VariantID, digitalgoods.Mask(itemID))
			purchase.Delivered = append(purchase.Delivered, digitalgoods.DeliveredItem{
				VariantID:    u.VariantID,
				ID:           itemID,
				DeliveryDate: time.Now().Format(digitalgoods.DateFmt),
			})
			gotQuantity++
		}

		// log VAT

		if gotQuantity > 0 {
			if _, err := tx.Stmt(db.logVAT).Exec(purchase.ID, time.Now().Format(digitalgoods.DateFmt), u.VariantID, gotQuantity, u.ItemPrice, purchase.CountryCode); err != nil {
				return err
			}
		}
	}

	deliveredBytes, err := json.Marshal(purchase.Delivered)
	if err != nil {
		return err
	}

	unfulfilled, err = purchase.GetUnfulfilled()
	if err != nil {
		return err
	}
	if unfulfilled.Empty() {
		purchase.DeleteDate = time.Now().AddDate(0, 0, 31).Format(digitalgoods.DateFmt)
		purchase.Status = digitalgoods.StatusFinalized
	} else {
		purchase.DeleteDate = "" // don't delete
		purchase.Status = digitalgoods.StatusUnderdelivered
	}

	if _, err := tx.Stmt(db.updatePurchase).Exec(purchase.Status, string(deliveredBytes), purchase.DeleteDate, purchase.ID); err != nil {
		return err
	}

	return tx.Commit()
}
