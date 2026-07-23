package services

import (
	"retrobytes/internal/domain"
	"retrobytes/internal/repos"
)

type CatalogService struct {
	Cats  *repos.CategoryRepo
	Prods *repos.ProductRepo
}

func NewCatalogService(cats *repos.CategoryRepo, prods *repos.ProductRepo) *CatalogService {
	return &CatalogService{Cats: cats, Prods: prods}
}

func (s *CatalogService) ListCategories() ([]domain.Category, error) {
	return s.Cats.List()
}

func (s *CatalogService) ListProductsByCategory(catID string, page, pageSize int) ([]domain.Product, error) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 12
	}
	offset := (page - 1) * pageSize
	return s.Prods.ListByCategory(catID, pageSize, offset)
}

func (s *CatalogService) GetProduct(id string) (domain.Product, error) {
	return s.Prods.Get(id)
}

func (s *CatalogService) Search(q, category, condition string, page, pageSize int) ([]domain.Product, error) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 12
	}
	offset := (page - 1) * pageSize
	return s.Prods.Search(q, category, condition, pageSize, offset)
}
