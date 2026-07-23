package services

import (
	"database/sql"

	"retrobytes/internal/domain"
	"retrobytes/internal/repos"
)

type InventoryService struct {
	Inv *repos.InventoryRepo
}

func NewInventoryService(inv *repos.InventoryRepo) *InventoryService {
	return &InventoryService{Inv: inv}
}

// CheckAvailability converts qty  IN_STOCK / LOW_STOCK / OUT_OF_STOCK.
func (s *InventoryService) CheckAvailability(productID, region string) (domain.Availability, error) {
	qty, err := s.Inv.Qty(productID, region)
	if err != nil {
		// If no inventory row exists, treat as 0.
		if err == sql.ErrNoRows {
			return domain.Availability{Status: "OUT_OF_STOCK", Qty: 0}, nil
		}
		return domain.Availability{}, err
	}

	status := "OUT_OF_STOCK"
	switch {
	case qty >= 5:
		status = "IN_STOCK"
	case qty > 0:
		status = "LOW_STOCK"
	}
	return domain.Availability{Status: status, Qty: qty, ETA: ""}, nil
}
