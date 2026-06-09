package api

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/ivpn/dns/api/internal/announcements"
)

// @Summary Get announcements
// @Description Get the list of currently published announcements. Public endpoint (no authentication).
// @Tags Announcements
// @Produce json
// @Success 200 {array} announcements.Announcement
// @Router /api/v1/announcements [get]
func (s *APIServer) getAnnouncements() fiber.Handler {
	return func(c *fiber.Ctx) error {
		visible := announcements.Visible(s.Announcements.Get(), time.Now())
		return c.Status(fiber.StatusOK).JSON(visible)
	}
}
