package services

import (
	"retrobytes/internal/domain"
	"retrobytes/internal/repos"
)

type CatalogService struct {
	Cats  *repos.CategoryRepo
	Prods *repos.ProductRepo
}

// maxPageSize caps how many rows any catalog/search query may return.
// It bounds the work per request so an oversized page size (from any caller,
// current or future) cannot exhaust server/database resources. See CWE-770.
const maxPageSize = 50

// clampPageSize normalizes page/pageSize into safe bounds: page >= 1, and
// pageSize within [1, maxPageSize] (defaulting to 12 when non-positive).
func clampPageSize(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 12
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	return page, pageSize
}

func NewCatalogService(cats *repos.CategoryRepo, prods *repos.ProductRepo) *CatalogService {
	return &CatalogService{Cats: cats, Prods: prods}
}

func (s *CatalogService) ListCategories() ([]domain.Category, error) {
	return s.Cats.List()
}

func (s *CatalogService) ListProductsByCategory(catID string, page, pageSize int) ([]domain.Product, error) {
	page, pageSize = clampPageSize(page, pageSize)
	offset := (page - 1) * pageSize
	return s.Prods.ListByCategory(catID, pageSize, offset)
}

func (s *CatalogService) GetProduct(id string) (domain.Product, error) {
	return s.Prods.Get(id)
}

func (s *CatalogService) Search(q, category, condition string, page, pageSize int) ([]domain.Product, error) {
	page, pageSize = clampPageSize(page, pageSize)
	offset := (page - 1) * pageSize
	return s.Prods.Search(q, category, condition, pageSize, offset)
}
