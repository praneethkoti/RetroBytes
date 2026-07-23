package services

import "retrobytes/internal/repos"

type WishlistService struct {
	Repo *repos.WishlistRepo
}

func NewWishlistService(r *repos.WishlistRepo) *WishlistService { return &WishlistService{Repo: r} }

// The ownerID is the authenticated user's id. Each user's wishlist is keyed by
// that id, so a user can only ever read or modify their own wishlist.

func (s *WishlistService) Save(ownerID, productID string) error {
	id, err := s.Repo.Ensure(ownerID)
	if err != nil {
		return err
	}
	return s.Repo.Add(id, productID)
}

func (s *WishlistService) Unsave(ownerID, productID string) error {
	id, err := s.Repo.Ensure(ownerID)
	if err != nil {
		return err
	}
	return s.Repo.Remove(id, productID)
}

func (s *WishlistService) List(ownerID string) ([]repos.WishlistRow, error) {
	id, err := s.Repo.Ensure(ownerID)
	if err != nil {
		return nil, err
	}
	return s.Repo.List(id)
}
