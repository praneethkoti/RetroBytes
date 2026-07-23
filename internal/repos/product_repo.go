package repos

import (
"retrobytes/internal/domain"
"github.com/jmoiron/sqlx"
)

type ProductRepo struct{ db *sqlx.DB }
func NewProductRepo(db *sqlx.DB) *ProductRepo { return &ProductRepo{db: db} }

func (r *ProductRepo) ListByCategory(catID string, limit, offset int) ([]domain.Product, error) {
var out []domain.Product
err := r.db.Select(&out, `
  SELECT
    id, category_id, title, description, condition, price, images_json, active,
    created_at, COALESCE(updated_at,'') AS updated_at
  FROM products
  WHERE category_id = ? AND active = 1
  ORDER BY created_at DESC
  LIMIT ? OFFSET ?
`, catID, limit, offset)
return out, err
}

func (r *ProductRepo) Get(id string) (domain.Product, error) {
var p domain.Product
err := r.db.Get(&p, `
  SELECT
    id, category_id, title, description, condition, price, images_json, active,
    created_at, COALESCE(updated_at,'') AS updated_at
  FROM products
  WHERE id = ?
`, id)
return p, err
}

func (r *ProductRepo) Search(q, catID, cond string, limit, offset int) ([]domain.Product, error) {
where := `active = 1`
args := []any{}
if q != "" {
where += ` AND (LOWER(title) LIKE ? OR LOWER(description) LIKE ?)`
args = append(args, "%"+q+"%", "%"+q+"%")
}
if catID != "" { where += ` AND category_id = ?`; args = append(args, catID) }
if cond != "" { where += ` AND condition = ?`; args = append(args, cond) }

sql := `
  SELECT
    id, category_id, title, description, condition, price, images_json, active,
    created_at, COALESCE(updated_at,'') AS updated_at
  FROM products
  WHERE ` + where + `
  ORDER BY created_at DESC
  LIMIT ? OFFSET ?`
args = append(args, limit, offset)

var out []domain.Product
err := r.db.Select(&out, sql, args...)
return out, err
}
