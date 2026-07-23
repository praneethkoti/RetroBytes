package handlers

import (
	"strings"

	"retrobytes/internal/log"
	"retrobytes/internal/services"
	"retrobytes/internal/validate"

	"github.com/gofiber/fiber/v2"
)

type SearchHandler struct {
	Catalog *services.CatalogService
}

func (h *SearchHandler) Search(c *fiber.Ctx) error {
	rawQ := c.Query("q")
	if strings.TrimSpace(rawQ) == "" {
		// Initial page load: show empty search without errors
		return render(c, "search", fiber.Map{"Q": "", "Products": []any{}, "Count": 0})
	}
	q, ok := validate.Q(rawQ)
	if !ok {
		log.Security(c, "validation.fail", map[string]any{"field": "q", "value": rawQ})
		return c.Status(fiber.StatusBadRequest).Render("search", fiber.Map{
			"Q": "", "Products": []any{}, "Count": 0, "Err": "Enter a valid keyword (letters/numbers only)",
		})
	}
	q = strings.ToLower(q)
	category := strings.TrimSpace(c.Query("category"))
	if category != "" {
		if _, ok := validate.ID(category); !ok {
			log.Security(c, "validation.fail", map[string]any{"field": "category"})
			return c.Status(fiber.StatusBadRequest).Render("search", fiber.Map{
				"Q": q, "Products": []any{}, "Count": 0, "Err": "Invalid category",
			})
		}
	}
	condition := strings.TrimSpace(c.Query("condition")) // FIRST_HAND | SECOND_HAND
	if condition != "" {
		if _, ok := validate.Condition(condition); !ok {
			log.Security(c, "validation.fail", map[string]any{"field": "condition"})
			return c.Status(fiber.StatusBadRequest).Render("search", fiber.Map{
				"Q": q, "Products": []any{}, "Count": 0, "Err": "Invalid filter",
			})
		}
	}

	products, err := h.Catalog.Search(q, category, condition, 1, 20)
	if err != nil {
		log.Error(c, "search.error", err, nil)
		return c.Status(500).Render("notfound", fiber.Map{"Message": "Could not load results. Please retry."})
	}

	return render(c, "search", fiber.Map{
		"Q": q, "CategoryID": category, "Condition": condition,
		"Products": products, "Count": len(products),
	})
}
