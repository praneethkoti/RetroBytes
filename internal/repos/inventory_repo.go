package repos

import (
	"fmt"

	"github.com/jmoiron/sqlx"
)

type InventoryRepo struct{ db *sqlx.DB }

func NewInventoryRepo(db *sqlx.DB) *InventoryRepo { return &InventoryRepo{db: db} }

// Row used by admin inventory pages
type InventoryRow struct {
	ProductID  string `db:"product_id"`
	Title      string `db:"title"`
	RegionCode string `db:"region_code"`
	Qty        int    `db:"qty"`
}

// Back-compat alias if other code used InvRow before
type InvRow = InventoryRow

// ListAll returns all inventory rows with product titles (for /admin/inventory)
func (r *InventoryRepo) ListAll() ([]InventoryRow, error) {
	var rows []InventoryRow
	err := r.db.Select(&rows, `
		SELECT i.product_id, p.title, i.region_code, i.qty
		FROM inventory i
		JOIN products p ON p.id = i.product_id
		ORDER BY p.title, i.region_code
	`)
	return rows, err
}

// Optional alias if older code calls All()
func (r *InventoryRepo) All() ([]InventoryRow, error) { return r.ListAll() }

// Qty returns current stock for a product in a region.
// If no row exists, it returns sql.ErrNoRows from sqlx.Get (as tests expect).
func (r *InventoryRepo) Qty(productID, region string) (int, error) {
	var qty int
	err := r.db.Get(&qty, `
		SELECT qty FROM inventory
		WHERE product_id = ? AND region_code = ?
	`, productID, region)
	if err != nil {
		return 0, err
	}
	return qty, nil
}

// Decrement atomically subtracts "by" units if enough stock exists.
// Returns an error if there isn't sufficient stock.
func (r *InventoryRepo) Decrement(productID, region string, by int) error {
	res, err := r.db.Exec(`
		UPDATE inventory
		SET qty = qty - ?
		WHERE product_id = ? AND region_code = ? AND qty >= ?
	`, by, productID, region, by)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("insufficient stock for %s in %s", productID, region)
	}
	return nil
}

// UpsertQty sets qty for (productID, region) creating the row if needed.
func (r *InventoryRepo) UpsertQty(productID, region string, qty int) error {
	_, err := r.db.Exec(`
		INSERT INTO inventory(product_id, region_code, qty)
		VALUES (?, ?, ?)
		ON CONFLICT(product_id, region_code) DO UPDATE SET qty = excluded.qty
	`, productID, region, qty)
	return err
}
