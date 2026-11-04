// Package http: the media module's driving adapter: upload, list and delete of pictures for a branch
// or for the company.
package http

import (
	"backend/internal/modules/media/port"
	"backend/internal/platform/httpx"
	"backend/internal/platform/middleware"
	"backend/pkg/response"

	"github.com/gofiber/fiber/v2"
)

// Handler answers the media routes
type Handler struct{ svc port.MediaService }

// New builds the handler
func New(svc port.MediaService) *Handler { return &Handler{svc: svc} }

// BranchUpload expects multipart/form-data with a "file" part and ?folder=sports|coaches|classes|branding
func (h *Handler) BranchUpload(c *fiber.Ctx) error {
	header, err := c.FormFile("file")
	if err != nil {
		return httpx.Fail(c, response.ErrorBadRequest("FILE_IS_REQUIRED"))
	}
	file, err := header.Open()
	if err != nil {
		return httpx.Fail(c, response.ErrorBadRequest("INVALID_FILE"))
	}
	defer file.Close()

	res, err := h.svc.Upload(c.UserContext(), middleware.BranchID(c), c.Query("folder"), file)
	return httpx.Created(c, res, err)
}

// BranchList returns the branch's media library: every file with where it is used
func (h *Handler) BranchList(c *fiber.Ctx) error {
	res, err := h.svc.List(c.UserContext(), middleware.BranchID(c))
	return httpx.OK(c, res, err)
}

// Delete expects ?key= of a file this branch uploaded
func (h *Handler) BranchDelete(c *fiber.Ctx) error {
	return httpx.Done(c, "DELETED", h.svc.Delete(c.UserContext(), middleware.BranchID(c), c.Query("key")))
}

// Upload stores a company file (logo, cover): ?folder=branding
func (h *Handler) CompanyUpload(c *fiber.Ctx) error {
	file, err := c.FormFile("file")
	if err != nil {
		return httpx.Fail(c, response.ErrorBadRequest("FILE_IS_REQUIRED"))
	}
	f, err := file.Open()
	if err != nil {
		return httpx.Fail(c, response.ErrorBadRequest("INVALID_FILE"))
	}
	defer f.Close()
	res, err := h.svc.Upload(c.UserContext(), 0, c.Query("folder"), f)
	return httpx.Created(c, res, err)
}

// CompanyList is the company's media library (logo and cover files only, never a branch's)
func (h *Handler) CompanyList(c *fiber.Ctx) error {
	res, err := h.svc.List(c.UserContext(), 0)
	return httpx.OK(c, res, err)
}

func (h *Handler) CompanyDelete(c *fiber.Ctx) error {
	return httpx.Done(c, "DELETED", h.svc.Delete(c.UserContext(), 0, c.Query("key")))
}
