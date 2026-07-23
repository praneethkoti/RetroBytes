package services_test

import (
	"testing"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite" // pure-Go SQLite driver

	"retrobytes/internal/repos"
	"retrobytes/internal/services"
)

func memdbAll(t *testing.T) *sqlx.DB {
	t.Helper()
	db, err := sqlx.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	schema := `
	CREATE TABLE categories(id TEXT PRIMARY KEY, name TEXT, created_at TEXT, updated_at TEXT);
	CREATE TABLE products(id TEXT PRIMARY KEY, category_id TEXT, title TEXT, description TEXT,
	  condition TEXT, price NUMERIC, images_json TEXT, active INTEGER, created_at TEXT, updated_at TEXT);
	CREATE TABLE inventory(product_id TEXT, region_code TEXT, qty INTEGER,
	  PRIMARY KEY(product_id, region_code));
	CREATE TABLE carts(id TEXT PRIMARY KEY, session_id TEXT UNIQUE NOT NULL, updated_at TEXT);
	CREATE TABLE cart_items(cart_id TEXT, product_id TEXT, qty INTEGER, price_at_add NUMERIC,
	  created_at TEXT, updated_at TEXT, PRIMARY KEY(cart_id, product_id));
	CREATE TABLE orders(id TEXT PRIMARY KEY, session_id TEXT, region_code TEXT, fulfillment TEXT,
	  customer_name TEXT, customer_email TEXT, total NUMERIC, status TEXT, created_at TEXT);
	CREATE TABLE order_items(order_id TEXT, product_id TEXT, qty INTEGER, price NUMERIC, condition TEXT,
	  PRIMARY KEY(order_id, product_id));

	INSERT INTO categories(id,name) VALUES ('retro-consoles','Retro Consoles');
	INSERT INTO products(id,category_id,title,description,condition,price,images_json,active,created_at)
	  VALUES ('gbc-001','retro-consoles','Game Boy Color','Handheld','SECOND_HAND',129.99,'[]',1,'now');
	INSERT INTO inventory(product_id,region_code,qty) VALUES ('gbc-001','20742',5);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestOrderFlow_AddCartCheckout(t *testing.T) {
	db := memdbAll(t)

	// repos
	cartRepo := repos.NewCartRepo(db)
	prodRepo := repos.NewProductRepo(db)
	invRepo := repos.NewInventoryRepo(db)
	orderRepo := repos.NewOrderRepo(db)

	// services
	cartSvc := services.NewCartService(cartRepo, prodRepo)
	orderSvc := services.NewOrderService(cartRepo, invRepo, orderRepo, prodRepo)

	sid := "test-session"
	if err := cartSvc.Add(sid, "gbc-001", 2); err != nil {
		t.Fatal(err)
	}

	cv, err := cartSvc.View(sid)
	if err != nil {
		t.Fatal(err)
	}
	if len(cv.Items) != 1 || cv.Total < 259.98 {
		t.Fatalf("bad cart view: %+v", cv)
	}

	oid, serverTotal, clientTotal, err := orderSvc.Place(sid, "20742", "delivery", services.Contact{Name: "Tester", Email: "t@e.com"})
	if err != nil {
		t.Fatal(err)
	}
	if oid == "" {
		t.Fatal("no order id")
	}
	if serverTotal == 0 || clientTotal == 0 {
		t.Fatalf("totals should be set, got server=%v client=%v", serverTotal, clientTotal)
	}

	// inventory decremented from 5 to 3
	qty, err := invRepo.Qty("gbc-001", "20742")
	if err != nil {
		t.Fatal(err)
	}
	if qty != 3 {
		t.Fatalf("want qty=3, got %d", qty)
	}
}
