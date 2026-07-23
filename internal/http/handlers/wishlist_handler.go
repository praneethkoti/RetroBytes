package handlers

import (
	"retrobytes/internal/domain"
	applog "retrobytes/internal/log"
	"retrobytes/internal/services"
	"retrobytes/internal/validate"

	"github.com/gofiber/fiber/v2"
)

type WishlistHandler struct {
	Wish *services.WishlistService
}

// ownerID returns the authenticated user's id. The wishlist routes are gated by
// RequireUser, which sets c.Locals("user"), so this is normally always present.
// If it is somehow missing, the caller treats it as unauthenticated.
func ownerID(c *fiber.Ctx) (string, bool) {
	u, ok := c.Locals("user").(*domain.User)
	if !ok || u == nil || u.ID == "" {
		return "", false
	}
	return u.ID, true
}

func (h *WishlistHandler) List(c *fiber.Ctx) error {
	uid, ok := ownerID(c)
	if !ok {
		return c.Redirect("/login")
	}
	items, err := h.Wish.List(uid)
	if err != nil {
		applog.Error(c, "wishlist.list.fail", err, nil)
		return c.Status(500).Render("notfound", fiber.Map{"Message": "Could not load wishlist"})
	}
	return render(c, "wishlist", fiber.Map{"Items": items})
}

func (h *WishlistHandler) Save(c *fiber.Ctx) error {
	uid, ok := ownerID(c)
	if !ok {
		return c.Redirect("/login")
	}
	pid := c.FormValue("productId")
	if _, ok := validate.ID(pid); !ok {
		return c.Status(400).SendString("missing productId")
	}
	// Wishlist is scoped to the authenticated user's id, so a user can only ever
	// modify their own wishlist regardless of session.
	if err := h.Wish.Save(uid, pid); err != nil {
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
	uid, ok := ownerID(c)
	if !ok {
		return c.Redirect("/login")
	}
	pid := c.FormValue("productId")
	if _, ok := validate.ID(pid); !ok {
		return c.Status(400).SendString("missing productId")
	}
	if err := h.Wish.Unsave(uid, pid); err != nil {
		applog.Error(c, "wishlist.unsave.fail", err, map[string]any{"product": pid})
		return c.Status(500).SendString("Could not unsave item")
	}
	applog.Audit(c, "wishlist.unsave", map[string]any{"product": pid})
	return c.Redirect("/wishlist")
}
