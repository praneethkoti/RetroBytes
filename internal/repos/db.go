package repos

import (
	"log"

	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

func OpenDB(dsn string) (*sqlx.DB, error) {
	db, err := sqlx.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if err = db.Ping(); err != nil {
		return nil, err
	}

	if err := ensureSchema(db); err != nil {
		return nil, err
	}
	// Seed baseline data if DB is empty (categories/products/inventory)
	if err := seedIfEmpty(db); err != nil {
		return nil, err
	}
	// Add two more products (idempotent; safe to run every start)
	if err := seedDefaultData(db); err != nil {
		return nil, err
	}
	// Ensure users exist (idempotent; safe to run every start)
	if err := seedUsers(db); err != nil {
		return nil, err
	}

	return db, nil
}

func ensureSchema(db *sqlx.DB) error {
	schema := `
PRAGMA foreign_keys = ON;

-- Categories
CREATE TABLE IF NOT EXISTS categories(
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  created_at TEXT DEFAULT CURRENT_TIMESTAMP,
  updated_at TEXT
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_categories_name_nocase ON categories(LOWER(name));

-- Products
CREATE TABLE IF NOT EXISTS products(
  id TEXT PRIMARY KEY,
  category_id TEXT NOT NULL REFERENCES categories(id) ON DELETE RESTRICT,
  title TEXT NOT NULL,
  description TEXT,
  condition TEXT NOT NULL CHECK (condition IN ('FIRST_HAND','SECOND_HAND')),
  price NUMERIC NOT NULL CHECK (price >= 0),
  images_json TEXT,
  active INTEGER NOT NULL DEFAULT 1,
  created_at TEXT DEFAULT CURRENT_TIMESTAMP,
  updated_at TEXT
);
CREATE INDEX IF NOT EXISTS idx_products_category   ON products(category_id);
CREATE INDEX IF NOT EXISTS idx_products_title      ON products(LOWER(title));
CREATE INDEX IF NOT EXISTS idx_products_condition  ON products(condition);
CREATE INDEX IF NOT EXISTS idx_products_created_at ON products(created_at);

-- Inventory
CREATE TABLE IF NOT EXISTS inventory(
  product_id TEXT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
  region_code TEXT NOT NULL,
  qty INTEGER NOT NULL DEFAULT 0 CHECK (qty >= 0),
  updated_at TEXT,
  PRIMARY KEY(product_id, region_code)
);
CREATE INDEX IF NOT EXISTS idx_inventory_product ON inventory(product_id);
CREATE INDEX IF NOT EXISTS idx_inventory_region  ON inventory(region_code);

-- Carts
CREATE TABLE IF NOT EXISTS carts(
  id TEXT PRIMARY KEY,
  session_id TEXT UNIQUE NOT NULL,
  updated_at TEXT
);

CREATE TABLE IF NOT EXISTS cart_items(
  cart_id    TEXT NOT NULL REFERENCES carts(id) ON DELETE CASCADE,
  product_id TEXT NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
  qty INTEGER NOT NULL CHECK (qty >= 1),
  price_at_add NUMERIC NOT NULL,
  created_at TEXT,
  updated_at TEXT,
  PRIMARY KEY (cart_id, product_id)
);

-- Orders
CREATE TABLE IF NOT EXISTS orders(
  id TEXT PRIMARY KEY,
  session_id TEXT,
  region_code TEXT,
  fulfillment TEXT,              -- delivery|pickup
  customer_name TEXT,
  customer_email TEXT,
  total NUMERIC NOT NULL,
  status TEXT NOT NULL DEFAULT 'PLACED',
  created_at TEXT DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_orders_created_at ON orders(created_at);

CREATE TABLE IF NOT EXISTS order_items(
  order_id  TEXT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
  product_id TEXT NOT NULL REFERENCES products(id),
  qty INTEGER NOT NULL,
  price NUMERIC NOT NULL,
  condition TEXT NOT NULL,
  PRIMARY KEY (order_id, product_id)
);

-- Wishlists (for UC-5)
CREATE TABLE IF NOT EXISTS wishlists(
  id TEXT PRIMARY KEY,
  session_id TEXT UNIQUE NOT NULL,
  updated_at TEXT
);

CREATE TABLE IF NOT EXISTS wishlist_items(
  wishlist_id TEXT NOT NULL REFERENCES wishlists(id) ON DELETE CASCADE,
  product_id  TEXT NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
  created_at  TEXT,
  PRIMARY KEY (wishlist_id, product_id)
);

-- Users & Sessions
CREATE TABLE IF NOT EXISTS users(
  id TEXT PRIMARY KEY,
  email TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  password_hash TEXT NOT NULL,
  role TEXT NOT NULL CHECK (role IN ('USER','ADMIN')),
  created_at TEXT DEFAULT CURRENT_TIMESTAMP,
  updated_at TEXT
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users(LOWER(email));

CREATE TABLE IF NOT EXISTS sessions(
  id TEXT PRIMARY KEY,               -- same value as your 'sid' cookie
  user_id TEXT NULL REFERENCES users(id) ON DELETE SET NULL,
  created_at TEXT DEFAULT CURRENT_TIMESTAMP,
  last_seen  TEXT
);
CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);
`
	_, err := db.Exec(schema)
	return err
}

func seedIfEmpty(db *sqlx.DB) error {
	var n int
	if err := db.Get(&n, `SELECT COUNT(*) FROM categories`); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}

	log.Println("[seed] inserting demo categories/products/inventory")

	tx := db.MustBegin()
	tx.MustExec(`INSERT INTO categories(id,name) VALUES
	  ('retro-consoles','Retro Gaming Consoles'),
	  ('vintage-radios','Vintage Radios'),
	  ('retro-shoes','Retro Shoes'),
	  ('retro-electronics','Retro Electronics')`)

	tx.MustExec(`INSERT INTO products(id,category_id,title,description,condition,price,images_json) VALUES
	  ('gbc-001','retro-consoles','Game Boy Color','Handheld console','SECOND_HAND',129.99,'["products/gbc-001/main.jpg"]'),
	  ('nes-001','retro-consoles','NES Console','Classic 8-bit console','FIRST_HAND',199.00,'["products/nes-001/main.jpg"]'),
	  ('radio-001','vintage-radios','Philco 1939','Vintage vacuum tube radio','SECOND_HAND',349.50,'["products/radio-001/main.jpg"]')`)

	tx.MustExec(`INSERT INTO inventory(product_id,region_code,qty) VALUES
	  ('gbc-001','20742',8),
	  ('gbc-001','10001',1),
	  ('nes-001','20742',0),
	  ('nes-001','10001',5),
	  ('radio-001','20742',2)`)

	return tx.Commit()
}

// seedDefaultData inserts two extra products if they don't already exist.
// Safe to run on every startup (idempotent).
func seedDefaultData(db *sqlx.DB) error {
	tx := db.MustBegin()
	defer func() { _ = tx.Rollback() }()

	// Ensure base categories exist (no-op if already present)
	_, _ = tx.Exec(`
		INSERT INTO categories(id, name, created_at)
		SELECT 'retro-consoles', 'Retro Consoles', CURRENT_TIMESTAMP
		WHERE NOT EXISTS (SELECT 1 FROM categories WHERE id='retro-consoles')
	`)
	_, _ = tx.Exec(`
		INSERT INTO categories(id, name, created_at)
		SELECT 'vintage-radios', 'Vintage Radios', CURRENT_TIMESTAMP
		WHERE NOT EXISTS (SELECT 1 FROM categories WHERE id='vintage-radios')
	`)

	// Product #1: Super Nintendo (SNES) Console
	_, _ = tx.Exec(`
		INSERT INTO products(
			id, category_id, title, description, condition, price, images_json, active, created_at, updated_at
		)
		SELECT
			'snes-001', 'retro-consoles',
			'Super Nintendo (SNES) Console',
			'Classic 16-bit SNES console with controller. Tested and cleaned.',
			'SECOND_HAND', 199.00, '["products/snes-001/main.jpg"]', 1, CURRENT_TIMESTAMP, NULL
		WHERE NOT EXISTS (SELECT 1 FROM products WHERE id='snes-001')
	`)

	// Product #2: Zenith Royal 500 Transistor Radio
	_, _ = tx.Exec(`
		INSERT INTO products(
			id, category_id, title, description, condition, price, images_json, active, created_at, updated_at
		)
		SELECT
			'radio-zenith-500', 'vintage-radios',
			'Zenith Royal 500 (1960s) Transistor Radio',
			'Iconic vintage pocket radio. Cosmetic wear; works with 9V battery.',
			'SECOND_HAND', 89.00, '["products/radio-zenith-500/main.jpg"]', 1, CURRENT_TIMESTAMP, NULL
		WHERE NOT EXISTS (SELECT 1 FROM products WHERE id='radio-zenith-500')
	`)

	// Inventory (idempotent upsert)
	_, _ = tx.Exec(`
		INSERT INTO inventory(product_id, region_code, qty)
		VALUES
		  ('snes-001', '20742', 7),
		  ('snes-001', '10001', 3),
		  ('radio-zenith-500', '20742', 5),
		  ('radio-zenith-500', '10001', 0)
		ON CONFLICT(product_id, region_code) DO UPDATE SET qty = excluded.qty
	`)

	return tx.Commit()
}

// seedUsers ensures two USERs and one ADMIN exist (idempotent).
func seedUsers(db *sqlx.DB) error {
	type u struct {
		ID, Email, Name, Role, Hash string
	}
	mk := func(id, email, name, role, raw string) u {
		h, _ := bcrypt.GenerateFromPassword([]byte(raw), 12)
		return u{ID: id, Email: email, Name: name, Role: role, Hash: string(h)}
	}

	users := []u{
		mk("u-alice", "alice@retrobytes.test", "Alice", "USER", "Passw0rd!"),
		mk("u-bob", "bob@retrobytes.test", "Bob", "USER", "Passw0rd!"),
		mk("u-luke", "luke@retrobytes.test", "Luke", "USER", "Passw0rd!"),
		mk("u-yoda", "yoda@retrobytes.test", "Yoda", "USER", "Passw0rd!"),
		mk("u-admin", "admin@retrobytes.test", "Admin", "ADMIN", "Passw0rd!"),
	}

	tx := db.MustBegin()
	defer func() { _ = tx.Rollback() }()

	for _, x := range users {
		if _, err := tx.Exec(`
			INSERT INTO users(id,email,name,password_hash,role)
			VALUES(?,?,?,?,?)
			ON CONFLICT(email) DO NOTHING
		`, x.ID, x.Email, x.Name, x.Hash, x.Role); err != nil {
			return err
		}
	}

	return tx.Commit()
}
