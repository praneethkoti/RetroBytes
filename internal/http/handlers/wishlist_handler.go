package handlers

import (
	applog "retrobytes/internal/log"
	"retrobytes/internal/services"
	"retrobytes/internal/validate"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type WishlistHandler struct {
	Wish *services.WishlistService
}

func (h *WishlistHandler) ensureSID(c *fiber.Ctx) string {
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

func (h *WishlistHandler) List(c *fiber.Ctx) error {
	sid := h.ensureSID(c)
	items, err := h.Wish.List(sid)
	if err != nil {
		applog.Error(c, "wishlist.list.fail", err, nil)
		return c.Status(500).Render("notfound", fiber.Map{"Message": "Could not load wishlist"})
	}
	return render(c, "wishlist", fiber.Map{"Items": items})
}

func (h *WishlistHandler) Save(c *fiber.Ctx) error {
	sid := h.ensureSID(c)
	pid := c.FormValue("productId")
	if _, ok := validate.ID(pid); !ok {
		return c.Status(400).SendString("missing productId")
	}
	if err := h.Wish.Save(sid, pid); err != nil {
		applog.Error(c, "wishlist.save.fail", err, map[string]any{"product": pid})
		return c.Status(500).SendString("Could not save item")
	}
	// redirect back to product or wishlist
	back := c.Get("Referer")
	if back == "" {
		back = "/wishlist"
	}
	applog.Audit(c, "wishlist.save", map[string]any{"product": pid})
	return c.Redirect(back)
}

func (h *WishlistHandler) Unsave(c *fiber.Ctx) error {
	sid := h.ensureSID(c)
	pid := c.FormValue("productId")
	if _, ok := validate.ID(pid); !ok {
		return c.Status(400).SendString("missing productId")
	}
	if err := h.Wish.Unsave(sid, pid); err != nil {
		applog.Error(c, "wishlist.unsave.fail", err, map[string]any{"product": pid})
		return c.Status(500).SendString("Could not unsave item")
	}
	applog.Audit(c, "wishlist.unsave", map[string]any{"product": pid})
	return c.Redirect("/wishlist")
}
