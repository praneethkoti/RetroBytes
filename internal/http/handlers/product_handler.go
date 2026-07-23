package handlers

import (
	"retrobytes/internal/log"
	"retrobytes/internal/services"
	"retrobytes/internal/validate"

	"github.com/gofiber/fiber/v2"
)

type ProductHandler struct {
	Catalog *services.CatalogService
}

func (h *ProductHandler) Detail(c *fiber.Ctx) error {
	id, ok := validate.ID(c.Params("id"))
	if !ok {
		log.Security(c, "validation.fail", map[string]any{"field": "product"})
		return c.Status(404).Render("notfound", fiber.Map{"Message": "This item is no longer available"})
	}
	p, err := h.Catalog.GetProduct(id)
	if err != nil || p.ID == "" {
		return c.Status(404).Render("notfound", fiber.Map{"Message": "This item is no longer available"})
	}
	return render(c, "product", fiber.Map{"P": p})
}
