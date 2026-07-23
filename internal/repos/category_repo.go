package repos

import (
"retrobytes/internal/domain"
"github.com/jmoiron/sqlx"
)

type CategoryRepo struct{ db *sqlx.DB }
func NewCategoryRepo(db *sqlx.DB) *CategoryRepo { return &CategoryRepo{db: db} }

func (r *CategoryRepo) List() ([]domain.Category, error) {
var out []domain.Category
err := r.db.Select(&out, `
  SELECT
    id,
    name,
    created_at,
    COALESCE(updated_at,'') AS updated_at
  FROM categories
  ORDER BY name
`)
return out, err
}
