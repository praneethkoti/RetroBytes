package services

import "retrobytes/internal/repos"

type WishlistService struct {
	Repo *repos.WishlistRepo
}

func NewWishlistService(r *repos.WishlistRepo) *WishlistService { return &WishlistService{Repo: r} }

func (s *WishlistService) Save(sessionID, productID string) error {
	id, err := s.Repo.Ensure(sessionID)
	if err != nil {
		return err
	}
	return s.Repo.Add(id, productID)
}

func (s *WishlistService) Unsave(sessionID, productID string) error {
	id, err := s.Repo.Ensure(sessionID)
	if err != nil {
		return err
	}
	return s.Repo.Remove(id, productID)
}

func (s *WishlistService) List(sessionID string) ([]repos.WishlistRow, error) {
	id, err := s.Repo.Ensure(sessionID)
	if err != nil {
		return nil, err
	}
	return s.Repo.List(id)
}
