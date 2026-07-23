package domain

type Category struct {
	ID        string `db:"id"`
	Name      string `db:"name"`
	CreatedAt string `db:"created_at"`
	UpdatedAt string `db:"updated_at"`
}

type Product struct {
	ID          string  `db:"id"`
	CategoryID  string  `db:"category_id"`
	Title       string  `db:"title"`
	Description string  `db:"description"`
	Condition   string  `db:"condition"` // FIRST_HAND | SECOND_HAND
	Price       float64 `db:"price"`
	ImagesJSON  string  `db:"images_json"`
	Active      bool    `db:"active"`
	CreatedAt   string  `db:"created_at"`
	UpdatedAt   string  `db:"updated_at"`
}

type Availability struct {
	Status string `json:"status"` // IN_STOCK | LOW_STOCK | OUT_OF_STOCK
	Qty    int    `json:"qty,omitempty"`
	ETA    string `json:"eta,omitempty"`
}
