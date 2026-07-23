package services_test

import (
	"database/sql"
	"testing"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite" // pure-Go SQLite driver

	"retrobytes/internal/repos"
	"retrobytes/internal/services"
)

func memdb(t *testing.T) *sqlx.DB {
	t.Helper()
	db, err := sqlx.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	schema := `
	CREATE TABLE inventory(
	  product_id TEXT,
	  region_code TEXT,
	  qty INTEGER,
	  PRIMARY KEY(product_id, region_code)
	);
	INSERT INTO inventory(product_id, region_code, qty) VALUES
	  ('gbc-001','20742',6),
	  ('gbc-001','00000',0);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestInventoryService_CheckAvailability(t *testing.T) {
	db := memdb(t)
	invRepo := repos.NewInventoryRepo(db)
	svc := services.NewInventoryService(invRepo)

	// in stock
	a, err := svc.CheckAvailability("gbc-001", "20742")
	if err != nil {
		t.Fatal(err)
	}
	if a.Status != "IN_STOCK" || a.Qty != 6 {
		t.Fatalf("want IN_STOCK(6), got %+v", a)
	}

	// out of stock (no row)
	a, err = svc.CheckAvailability("gbc-001", "99999")
	if err != nil && err != sql.ErrNoRows {
		t.Fatal(err)
	}
	if a.Status != "OUT_OF_STOCK" {
		t.Fatalf("want OUT_OF_STOCK, got %+v", a)
	}
}
