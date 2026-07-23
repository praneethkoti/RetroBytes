package handlers

import (
	"strconv"

	applog "retrobytes/internal/log"
	"retrobytes/internal/repos"
	"retrobytes/internal/validate"

	"github.com/gofiber/fiber/v2"
)

type AdminHandler struct {
	OrderRepo *repos.OrderRepo
	Inv       *repos.InventoryRepo
	Users     *repos.UserRepo
}

// GET /admin
func (h *AdminHandler) Dashboard(c *fiber.Ctx) error {
	return render(c, "admin_dashboard", fiber.Map{})
}

// GET /admin/orders
func (h *AdminHandler) OrdersPage(c *fiber.Ctx) error {
	ords, err := h.OrderRepo.ListLatest(100)
	if err != nil {
		applog.Error(c, "admin.orders.list.fail", err, nil)
		return c.Status(500).Render("notfound", fiber.Map{"Message": "Could not load orders"})
	}
	return render(c, "admin_orders", fiber.Map{"Orders": ords})
}

// POST /admin/orders/:id/status
func (h *AdminHandler) UpdateOrderStatus(c *fiber.Ctx) error {
	id := c.Params("id")
	status := c.FormValue("status")
	if id == "" || status == "" {
		return c.Status(400).SendString("missing id or status")
	}
	if err := h.OrderRepo.UpdateStatus(id, status); err != nil {
		applog.Error(c, "admin.orders.update.fail", err, map[string]any{"order_id": id})
		return c.Status(400).SendString("could not update status")
	}
	applog.Audit(c, "admin.orders.update", map[string]any{"order_id": id, "status": status})
	return c.Redirect("/admin/orders")
}

// GET /admin/inventory
func (h *AdminHandler) Inventory(c *fiber.Ctx) error {
	rows, err := h.Inv.ListAll()
	if err != nil {
		applog.Error(c, "admin.inventory.list.fail", err, nil)
		return c.Status(500).Render("notfound", fiber.Map{"Message": "Could not load inventory"})
	}
	ords, _ := h.OrderRepo.ListLatest(25)
	return render(c, "admin_inventory", fiber.Map{"Rows": rows, "Orders": ords})
}

// POST /admin/inventory
func (h *AdminHandler) UpdateInventory(c *fiber.Ctx) error {
	pid := c.FormValue("product_id")
	region := c.FormValue("region")
	qtyStr := c.FormValue("qty")

	qty, err := strconv.Atoi(qtyStr)
	region, ok := validate.Region(region)
	if _, okID := validate.ID(pid); !okID || !ok || err != nil || qty < 0 {
		return c.Status(400).SendString("invalid input")
	}
	if err := h.Inv.UpsertQty(pid, region, qty); err != nil {
		applog.Error(c, "admin.inventory.save.fail", err, map[string]any{"product": pid, "region": region, "qty": qty})
		return c.Status(400).SendString("could not save inventory")
	}
	applog.Audit(c, "admin.inventory.save", map[string]any{"product": pid, "region": region, "qty": qty})
	return c.Redirect("/admin/inventory")
}

// UsersPage lists users (excluding admin).
func (h *AdminHandler) UsersPage(c *fiber.Ctx) error {
	var users []struct {
		ID    string `db:"id"`
		Email string `db:"email"`
		Name  string `db:"name"`
		Role  string `db:"role"`
	}
	if err := h.Users.DB.Select(&users, `SELECT id,email,name,role FROM users WHERE role != 'ADMIN' ORDER BY email`); err != nil {
		applog.Error(c, "admin.users.list.fail", err, nil)
		return c.Status(500).Render("notfound", fiber.Map{"Message": "Could not load users"})
	}
	return render(c, "admin_users", fiber.Map{"Users": users})
}

// DeleteUser deletes a user and related data, cancels their orders.
func (h *AdminHandler) DeleteUser(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).SendString("missing id")
	}
	if err := h.Users.DeleteUserCascade(id); err != nil {
		applog.Error(c, "admin.users.delete.fail", err, map[string]any{"user_id": id})
		return c.Status(400).SendString("could not delete user")
	}
	applog.Audit(c, "admin.users.delete", map[string]any{"user_id": id})
	return c.Redirect("/admin/users")
}
