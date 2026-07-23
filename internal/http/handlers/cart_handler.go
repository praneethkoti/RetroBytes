package handlers

import (
	applog "retrobytes/internal/log"
	"retrobytes/internal/services"
	"retrobytes/internal/validate"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type CartHandler struct {
	Cart *services.CartService
}

func (h *CartHandler) ensureSID(c *fiber.Ctx) string {
	sid := c.Cookies("sid")
	if sid == "" {
		sid = uuid.NewString()
		c.Cookie(&fiber.Cookie{
			Name:     "sid",
			Value:    sid,
			Path:     "/",
			HTTPOnly: true,
			SameSite: fiber.CookieSameSiteLaxMode,
			Secure:   false,
		})
	}
	return sid
}

func (h *CartHandler) Add(c *fiber.Ctx) error {
	sid := h.ensureSID(c)
	productID := c.FormValue("productId")
	qty := validate.Qty(c.FormValue("qty"))

	if qty <= 0 {
		qty = 1
	}
	if _, ok := validate.ID(productID); !ok {
		return c.Status(400).SendString("missing productId")
	}
	if err := h.Cart.Add(sid, productID, qty); err != nil {
		applog.Error(c, "cart.add.fail", err, map[string]any{"product": productID, "qty": qty})
		return c.Status(400).SendString("Cart limit reached (10 items). Please start a new order to add more items")
	}
	applog.Audit(c, "cart.add", map[string]any{"product": productID, "qty": qty})
	return c.Redirect("/cart")
}

func (h *CartHandler) View(c *fiber.Ctx) error {
	sid := h.ensureSID(c)
	cv, err := h.Cart.View(sid)
	if err != nil {
		applog.Error(c, "cart.view.fail", err, nil)
		return c.Status(500).Render("notfound", fiber.Map{"Message": "Could not load cart"})
	}
	return render(c, "cart", fiber.Map{"Cart": cv})

}
