package handlers

import (
	"retrobytes/internal/log"
	"retrobytes/internal/services"
	"retrobytes/internal/validate"

	"github.com/gofiber/fiber/v2"
)

type CategoryHandler struct {
	Catalog *services.CatalogService
}

func (h *CategoryHandler) Home(c *fiber.Ctx) error {
	cats, err := h.Catalog.ListCategories()
	if err != nil {
		log.Error(c, "categories.list.fail", err, nil)
		return c.Status(500).Render("notfound", fiber.Map{"Message": "Could not load categories"})
	}
	return render(c, "home", fiber.Map{"Categories": cats})

}

func (h *CategoryHandler) List(c *fiber.Ctx) error {
	catID, ok := validate.ID(c.Params("id"))
	if !ok {
		log.Security(c, "validation.fail", map[string]any{"field": "category"})
		return c.Status(404).Render("notfound", fiber.Map{"Message": "Category not found"})
	}
	products, err := h.Catalog.ListProductsByCategory(catID, 1, 12)
	if err != nil {
		log.Error(c, "category.products.fail", err, map[string]any{"category": catID})
		return c.Status(500).Render("notfound", fiber.Map{"Message": "Could not load items"})
	}
	return render(c, "category", fiber.Map{"CategoryID": catID, "Products": products})

}
