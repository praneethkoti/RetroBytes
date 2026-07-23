package repos

import (
	"time"

	"github.com/jmoiron/sqlx"
)

type WishlistRepo struct{ db *sqlx.DB }

func NewWishlistRepo(db *sqlx.DB) *WishlistRepo { return &WishlistRepo{db: db} }

func (r *WishlistRepo) Ensure(sessionID string) (string, error) {
	var id string
	if err := r.db.Get(&id, `SELECT id FROM wishlists WHERE session_id=?`, sessionID); err == nil {
		return id, nil
	}
	_, err := r.db.Exec(`INSERT INTO wishlists(id,session_id,updated_at) VALUES(?,?,?)`,
		sessionID, sessionID, time.Now().Format(time.RFC3339))
	if err != nil {
		return "", err
	}
	return sessionID, nil
}

func (r *WishlistRepo) Add(wishlistID, productID string) error {
	_, err := r.db.Exec(`
	  INSERT INTO wishlist_items(wishlist_id, product_id, created_at)
	  VALUES(?, ?, CURRENT_TIMESTAMP)
	  ON CONFLICT(wishlist_id, product_id) DO NOTHING
	`, wishlistID, productID)
	return err
}

func (r *WishlistRepo) Remove(wishlistID, productID string) error {
	_, err := r.db.Exec(`DELETE FROM wishlist_items WHERE wishlist_id=? AND product_id=?`, wishlistID, productID)
	return err
}

type WishlistRow struct {
	ProductID string  `db:"product_id"`
	Title     string  `db:"title"`
	Condition string  `db:"condition"`
	Price     float64 `db:"price"`
	Active    bool    `db:"active"`
}

func (r *WishlistRepo) List(wishlistID string) ([]WishlistRow, error) {
	var out []WishlistRow
	err := r.db.Select(&out, `
	  SELECT p.id AS product_id, p.title, p.condition, p.price, p.active
	  FROM wishlist_items wi
	  JOIN products p ON p.id = wi.product_id
	  WHERE wi.wishlist_id = ?
	  ORDER BY p.title
	`, wishlistID)
	return out, err
}
