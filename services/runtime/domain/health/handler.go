package health

import "github.com/gofiber/fiber/v3"

type Handler struct{}

func NewHandler() *Handler {
	return &Handler{}
}

func (h *Handler) RegisterRoutes(r fiber.Router) {
	r.Get("/health", h.health)
}

func (h *Handler) health(c fiber.Ctx) error {
	return c.Status(fiber.StatusOK).SendString("OK")
}
