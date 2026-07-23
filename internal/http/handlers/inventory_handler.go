package handlers

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"retrobytes/internal/log"
	"retrobytes/internal/services"
	"retrobytes/internal/validate"
)

type InventoryHandler struct {
	Inv *services.InventoryService
}

func (h *InventoryHandler) Check(c *fiber.Ctx) error {
	// Validate productId
	productID := strings.TrimSpace(c.Query("productId"))
	if _, ok := validate.ID(productID); !ok {
		log.Security(c, "validation.fail", map[string]any{"field": "productId"})
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "missing or invalid productId"})
	}

	// Validate region/ZIP (allows simple ZIP/postal formats)
	region, ok := validate.Region(c.Query("region"))
	if !ok {
		log.Security(c, "validation.fail", map[string]any{"field": "region"})
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "enter a valid region/ZIP"})
	}

	// Business logic
	avail, err := h.Inv.CheckAvailability(productID, region)
	if err != nil {
		log.Error(c, "inventory.check.fail", err, map[string]any{"productId": productID, "region": region})
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "could not check availability"})
	}
	return c.JSON(avail)
}
