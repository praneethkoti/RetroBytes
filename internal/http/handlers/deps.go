package handlers

import (
	"retrobytes/internal/config"
	"retrobytes/internal/repos"
	"retrobytes/internal/services"

	"github.com/jmoiron/sqlx"
)

type Deps struct {
	CategoryHandler  *CategoryHandler
	ProductHandler   *ProductHandler
	InventoryHandler *InventoryHandler
	SearchHandler    *SearchHandler
	CartHandler      *CartHandler
	OrderHandler     *OrderHandler
	WishlistHandler  *WishlistHandler
}

func NewDeps(db *sqlx.DB, cfg config.Config, auth *services.AuthService) *Deps {
	catRepo := repos.NewCategoryRepo(db)
	prodRepo := repos.NewProductRepo(db)
	invRepo := repos.NewInventoryRepo(db)
	cartRepo := repos.NewCartRepo(db)
	orderRepo := repos.NewOrderRepo(db)
	wishRepo := repos.NewWishlistRepo(db)

	catalogSvc := services.NewCatalogService(catRepo, prodRepo)
	invSvc := services.NewInventoryService(invRepo)
	cartSvc := services.NewCartService(cartRepo, prodRepo)
	orderSvc := services.NewOrderService(cartRepo, invRepo, orderRepo, prodRepo)
	wishSvc := services.NewWishlistService(wishRepo)

	return &Deps{
		CategoryHandler:  &CategoryHandler{Catalog: catalogSvc},
		ProductHandler:   &ProductHandler{Catalog: catalogSvc},
		InventoryHandler: &InventoryHandler{Inv: invSvc},
		SearchHandler:    &SearchHandler{Catalog: catalogSvc},
		CartHandler:      &CartHandler{Cart: cartSvc},
		OrderHandler:     &OrderHandler{Cart: cartSvc, Order: orderSvc, Repo: orderRepo, Auth: auth},
		WishlistHandler:  &WishlistHandler{Wish: wishSvc},
	}
}
