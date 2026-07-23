package services

import (
	"fmt"
	"retrobytes/internal/repos"
)

type CartService struct {
	Repo  *repos.CartRepo
	Carts *repos.CartRepo
	Prods *repos.ProductRepo
}

func NewCartService(carts *repos.CartRepo, prods *repos.ProductRepo) *CartService {
	return &CartService{Carts: carts, Prods: prods}
}

func (s *CartService) Add(sessionID, productID string, qty int) error {
	if qty < 1 {
		qty = 1
	}
	cartID, err := s.Carts.EnsureCart(sessionID)
	if err != nil {
		return err
	}
	// Enforce total cart quantity limit (max 10 items overall)
	items, err := s.Carts.Items(cartID)
	if err != nil {
		return err
	}
	total := 0
	existingQtyForProduct := 0
	for _, it := range items {
		total += it.Qty
		if it.ProductID == productID {
			existingQtyForProduct = it.Qty
		}
	}
	remaining := 10 - total
	if remaining <= 0 {
		return fmt.Errorf("cart limit reached (max 10 items)")
	}
	if qty > remaining {
		qty = remaining
	}
	p, err := s.Prods.Get(productID)
	if err != nil {
		return err
	}
	// Check stock for this product in default region? (Region not captured in cart; enforce per-product cap by stock if available)
	// Without a region, we can’t query exact stock. We’ll cap add to 10 total and let OrderService enforce stock by region at checkout.
	// But we can still prevent absurd per-product adds by limiting line qty to remaining cart capacity.
	finalQty := qty
	if existingQtyForProduct+qty > 10 {
		finalQty = 10 - existingQtyForProduct
		if finalQty < 1 {
			return fmt.Errorf("cannot add more of this item (cart limit)")
		}
	}
	return s.Carts.UpsertItem(cartID, productID, finalQty, p.Price)
}

type CartView struct {
	Items []repos.CartItemRow
	Total float64
}

func (s *CartService) View(sessionID string) (CartView, error) {
	cartID, err := s.Carts.EnsureCart(sessionID)
	if err != nil {
		return CartView{}, err
	}
	items, total, err := s.Carts.View(cartID)
	if err != nil {
		return CartView{}, err
	}
	return CartView{Items: items, Total: total}, nil
}

func (s *CartService) MergeOnLogin(userID, sid string) error {
	return s.Repo.MergeForLogin(userID, sid)
}
