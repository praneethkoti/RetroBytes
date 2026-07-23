package services

import (
	"database/sql"
	"errors"
	"fmt"

	"retrobytes/internal/repos"

	"github.com/google/uuid"
)

type Contact struct {
	Name  string
	Email string
}

type OrderService struct {
	Carts  *repos.CartRepo
	Inv    *repos.InventoryRepo
	Orders *repos.OrderRepo
	Prods  *repos.ProductRepo
}

func NewOrderService(carts *repos.CartRepo, inv *repos.InventoryRepo, orders *repos.OrderRepo, prods *repos.ProductRepo) *OrderService {
	return &OrderService{Carts: carts, Inv: inv, Orders: orders, Prods: prods}
}

func (s *OrderService) Place(sessionID, region, fulfillment string, contact Contact) (string, float64, float64, error) {
	if region == "" {
		return "", 0, 0, errors.New("missing region")
	}
	if fulfillment == "" {
		fulfillment = "delivery"
	}

	cartID, err := s.Carts.EnsureCart(sessionID)
	if err != nil {
		return "", 0, 0, err
	}

	items, err := s.Carts.Items(cartID)
	if err != nil {
		return "", 0, 0, err
	}
	if len(items) == 0 {
		return "", 0, 0, errors.New("cart empty")
	}

	// pre-check stock and recompute totals from trusted product data
	serverTotal := 0.0
	clientTotal := 0.0
	for i, it := range items {
		qty, err := s.Inv.Qty(it.ProductID, region)
		if err != nil && err != sql.ErrNoRows {
			return "", 0, 0, err
		}
		if qty < it.Qty {
			return "", 0, 0, fmt.Errorf("insufficient stock for %s (need %d, have %d)", it.ProductID, it.Qty, qty)
		}
		// overwrite price/condition with current catalog data
		p, err := s.Prods.Get(it.ProductID)
		if err != nil {
			return "", 0, 0, err
		}
		clientTotal += it.Price * float64(it.Qty)
		items[i].Price = p.Price
		items[i].Condition = p.Condition
		serverTotal += p.Price * float64(it.Qty)
	}

	// decrement
	for _, it := range items {
		if err := s.Inv.Decrement(it.ProductID, region, it.Qty); err != nil {
			return "", 0, 0, err
		}
	}

	// create order
	orderID := uuid.NewString()
	if err := s.Orders.Create(orderID, sessionID, region, fulfillment, contact.Name, contact.Email, serverTotal); err != nil {
		return "", 0, 0, err
	}
	for _, it := range items {
		if err := s.Orders.InsertItem(orderID, it.ProductID, it.Qty, it.Price, it.Condition); err != nil {
			return "", 0, 0, err
		}
	}
	_ = s.Carts.Clear(cartID)
	return orderID, serverTotal, clientTotal, nil

}
