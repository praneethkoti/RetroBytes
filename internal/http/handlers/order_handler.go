package handlers

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"retrobytes/internal/domain"
	applog "retrobytes/internal/log"
	"retrobytes/internal/repos"
	"retrobytes/internal/services"
	"retrobytes/internal/validate"
)

type OrderHandler struct {
	Cart  *services.CartService
	Order *services.OrderService
	Repo  *repos.OrderRepo
	Auth  *services.AuthService
}

type OrderDeps struct {
	Cart *services.CartService
	Ord  *services.OrderService
}

func (h *OrderHandler) ensureSID(c *fiber.Ctx) string {
	sid := c.Cookies("sid")
	if sid == "" {
		sid = uuid.NewString()
		c.Cookie(&fiber.Cookie{
			Name:     "sid",
			Value:    sid,
			Path:     "/",
			HTTPOnly: true,
			SameSite: fiber.CookieSameSiteLaxMode,
			Secure:   false, // enable true behind TLS
		})
	}
	return sid
}

func (h *OrderHandler) Checkout(c *fiber.Ctx) error {
	cv, err := h.Cart.View(h.ensureSID(c))
	if err != nil {
		applog.Error(c, "checkout.load", err, nil)
		return c.Status(fiber.StatusInternalServerError).Render("notfound", fiber.Map{"Message": "Could not load your cart"})
	}
	return render(c, "checkout", fiber.Map{"Cart": cv})
}

func (h *OrderHandler) Place(c *fiber.Ctx) error {
	sid := h.ensureSID(c)

	// Validate region/ZIP
	region, ok := validate.Region(c.FormValue("region"))
	if !ok {
		applog.Security(c, "validation.fail", map[string]any{"field": "region"})
		return c.Status(fiber.StatusBadRequest).SendString("invalid region/ZIP")
	}

	// Validate email and name
	email, ok := validate.Email(c.FormValue("email"))
	if !ok {
		applog.Security(c, "validation.fail", map[string]any{"field": "email"})
		return c.Status(fiber.StatusBadRequest).SendString("invalid email")
	}
	name, ok := validate.Name(c.FormValue("name"))
	if !ok {
		applog.Security(c, "validation.fail", map[string]any{"field": "name"})
		return c.Status(fiber.StatusBadRequest).SendString("name must be 1-20 characters")
	}

	// Normalize fulfillment
	fulfillment := strings.ToLower(strings.TrimSpace(c.FormValue("fulfillment")))
	if fulfillment != "delivery" && fulfillment != "pickup" {
		fulfillment = "delivery"
	}

	contact := services.Contact{Name: name, Email: email}

	orderID, serverTotal, clientTotal, err := h.Order.Place(sid, region, fulfillment, contact)
	if err != nil {
		// business rule errors (e.g., insufficient stock) surface as 400
		applog.Security(c, "order.place.fail", map[string]any{"sid": sid, "error": err.Error()})
		return c.Status(fiber.StatusBadRequest).SendString("Could not place order. Please review quantities and try again.")
	}
	applog.Audit(c, "order.place", map[string]any{
		"order_id":     orderID,
		"server_total": serverTotal,
		"client_total": clientTotal,
		"mismatch":     serverTotal != clientTotal,
	})

	// Show detailed confirmation page
	return c.Redirect("/order/" + orderID)
}

func (h *OrderHandler) View(c *fiber.Ctx) error {
	oid := c.Params("id")
	if oid == "" {
		return c.Status(fiber.StatusNotFound).Render("notfound", fiber.Map{"Message": "Order not found"})
	}

	o, items, err := h.Repo.Get(oid)
	if err != nil {
		return c.Status(fiber.StatusNotFound).Render("notfound", fiber.Map{"Message": "Order not found"})
	}

	// Ownership check: session owner or same user via sessions.user_id; admins allowed
	sid := c.Cookies("sid")
	var uID string
	var uRole string
	if h.Auth != nil && sid != "" {
		if u, err := h.Auth.CurrentUser(sid); err == nil && u != nil {
			uID = u.ID
			uRole = u.Role
		}
	}
	if !(sid != "" && sid == o.SessionID) && !(uID != "" && uID == o.UserID) {
		if uRole == "ADMIN" {
			return render(c, "order", fiber.Map{"Order": o, "Items": items})
		}
		applog.Security(c, "access.denied.order", map[string]any{"order_id": oid})
		return c.Status(fiber.StatusNotFound).Render("notfound", fiber.Map{"Message": "Order not found"})
	}

	return render(c, "order", fiber.Map{"Order": o, "Items": items})
}

// History lists orders for the current logged-in user.
func (h *OrderHandler) History(c *fiber.Ctx) error {
	u, _ := c.Locals("user").(*domain.User)
	// If RequireUser is used, user is guaranteed; fallback to 404
	if u == nil {
		return c.Status(fiber.StatusNotFound).Render("notfound", fiber.Map{"Message": "Orders not available"})
	}
	orders, err := h.Repo.ListByUser(u.ID)
	if err != nil {
		applog.Error(c, "orders.history.fail", err, nil)
		return c.Status(fiber.StatusInternalServerError).Render("notfound", fiber.Map{"Message": "Could not load orders"})
	}
	// Fallback: show session orders if none linked to user (e.g., pre-login)
	if len(orders) == 0 {
		if sid := c.Cookies("sid"); sid != "" {
			if sessOrders, err := h.Repo.ListBySession(sid); err == nil && len(sessOrders) > 0 {
				orders = sessOrders
			}
		}
	}
	return render(c, "order_history", fiber.Map{"Orders": orders})
}
