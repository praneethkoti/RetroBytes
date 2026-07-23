package repos

import "github.com/jmoiron/sqlx"

type OrderRepo struct{ db *sqlx.DB }

func NewOrderRepo(db *sqlx.DB) *OrderRepo { return &OrderRepo{db: db} }

// ---------- Admin list summary ----------
type OrderSummary struct {
	ID            string  `db:"id"`
	SessionID     string  `db:"session_id"`
	CustomerName  string  `db:"customer_name"`
	CustomerEmail string  `db:"customer_email"`
	Total         float64 `db:"total"`
	Status        string  `db:"status"`
	CreatedAt     string  `db:"created_at"`
}

// ---------- Order detail (used by /order/:id) ----------
type OrderRow struct {
	ID          string  `db:"id"`
	SessionID   string  `db:"session_id"`
	UserID      string  `db:"user_id"`
	Region      string  `db:"region_code"`
	Fulfillment string  `db:"fulfillment"`
	Customer    string  `db:"customer_name"`
	Email       string  `db:"customer_email"`
	Total       float64 `db:"total"`
	Status      string  `db:"status"`
	CreatedAt   string  `db:"created_at"`
}

type OrderItemRow struct {
	Title     string  `db:"title"`
	Condition string  `db:"condition"`
	Qty       int     `db:"qty"`
	Price     float64 `db:"price"`
	Subtotal  float64 `db:"subtotal"`
}

// ---------- Methods your service needs ----------

// Create inserts a new order header.
func (r *OrderRepo) Create(orderID, sessionID, region, fulfillment, name, email string, total float64) error {
	_, err := r.db.Exec(`
	  INSERT INTO orders
	    (id, session_id, region_code, fulfillment, customer_name, customer_email, total, status, created_at)
	  VALUES
	    (?,  ?,         ?,           ?,           ?,             ?,              ?,     'PLACED', CURRENT_TIMESTAMP)
	`, orderID, sessionID, region, fulfillment, name, email, total)
	return err
}

// InsertItem inserts a single line item.
func (r *OrderRepo) InsertItem(orderID, productID string, qty int, price float64, condition string) error {
	_, err := r.db.Exec(`
	  INSERT INTO order_items(order_id, product_id, qty, price, condition)
	  VALUES(?, ?, ?, ?, ?)
	`, orderID, productID, qty, price, condition)
	return err
}

// ---------- Used by order page/admin ----------

func (r *OrderRepo) Get(orderID string) (OrderRow, []OrderItemRow, error) {
	var o OrderRow
	if err := r.db.Get(&o, `
		SELECT o.id, o.session_id, COALESCE(s.user_id,'') AS user_id, o.region_code, o.fulfillment, o.customer_name, o.customer_email, o.total, o.status, o.created_at
		FROM orders o
		LEFT JOIN sessions s ON s.id = o.session_id
		WHERE o.id = ?
	`, orderID); err != nil {
		return OrderRow{}, nil, err
	}

	var items []OrderItemRow
	if err := r.db.Select(&items, `
		SELECT p.title, oi.condition, oi.qty, oi.price, (oi.qty * oi.price) AS subtotal
		FROM order_items oi
		JOIN products p ON p.id = oi.product_id
		WHERE oi.order_id = ?
		ORDER BY p.title
	`, orderID); err != nil {
		return OrderRow{}, nil, err
	}

	return o, items, nil
}

func (r *OrderRepo) ListLatest(limit int) ([]OrderSummary, error) {
	if limit <= 0 {
		limit = 100
	}
	var out []OrderSummary
	err := r.db.Select(&out, `
		SELECT id, session_id, customer_name, customer_email, total, status, created_at
		FROM orders
		ORDER BY datetime(created_at) DESC
		LIMIT ?
	`, limit)
	return out, err
}

// ListByUser returns orders for a given user via session linkage.
func (r *OrderRepo) ListByUser(userID string) ([]OrderSummary, error) {
	var out []OrderSummary
	err := r.db.Select(&out, `
		SELECT o.id, o.session_id, o.customer_name, o.customer_email, o.total, o.status, o.created_at
		FROM orders o
		JOIN sessions s ON s.id = o.session_id
		WHERE s.user_id = ?
		ORDER BY datetime(o.created_at) DESC
	`, userID)
	return out, err
}

// ListBySession returns orders tied to a given session id (helps show anon or pre-login orders).
func (r *OrderRepo) ListBySession(sessionID string) ([]OrderSummary, error) {
	var out []OrderSummary
	err := r.db.Select(&out, `
		SELECT id, session_id, customer_name, customer_email, total, status, created_at
		FROM orders
		WHERE session_id = ?
		ORDER BY datetime(created_at) DESC
	`, sessionID)
	return out, err
}

func (r *OrderRepo) UpdateStatus(id, status string) error {
	_, err := r.db.Exec(`UPDATE orders SET status = ? WHERE id = ?`, status, id)
	return err
}
