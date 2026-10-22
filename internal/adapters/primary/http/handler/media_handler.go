package handler

import (
	"backend/internal/adapters/primary/http/middleware"
	"backend/internal/core/port/input"
	"backend/pkg/response"

	"github.com/gofiber/fiber/v2"
)

type (
	MediaHandler interface {
		Upload(c *fiber.Ctx) error
		Delete(c *fiber.Ctx) error
	}
	mediaHandler struct {
		svc input.MediaService
	}
)

func NewMediaHandler(svc input.MediaService) MediaHandler {
	return &mediaHandler{svc: svc}
}

// Upload expects multipart/form-data with a "file" part and ?folder=sports|coaches|classes|branding
func (h *mediaHandler) Upload(c *fiber.Ctx) error {
	header, err := c.FormFile("file")
	if err != nil {
		return fail(c, response.ErrorBadRequest("FILE_IS_REQUIRED"))
	}
	file, err := header.Open()
	if err != nil {
		return fail(c, response.ErrorBadRequest("INVALID_FILE"))
	}
	defer file.Close()

	res, err := h.svc.Upload(c.UserContext(), middleware.BranchID(c), c.Query("folder"), file)
	return created(c, res, err)
}

// Delete expects ?key= of a file this branch uploaded
func (h *mediaHandler) Delete(c *fiber.Ctx) error {
	return done(c, "DELETED", h.svc.Delete(c.UserContext(), middleware.BranchID(c), c.Query("key")))
}
