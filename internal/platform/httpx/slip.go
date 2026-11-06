package httpx

import (
	"backend/internal/core/dto"
	"backend/pkg/response"
	"io"

	"github.com/gofiber/fiber/v2"
)

// SlipUpload reads a "send slip" request: multipart with "via" (whatsapp | telegram) and, for telegram, the picture
// in "slip". No body at all means whatsapp.
func SlipUpload(c *fiber.Ctx) (dto.SlipUpload, error) {
	slip := dto.SlipUpload{Via: c.FormValue("via")}
	header, err := c.FormFile("slip")
	if err != nil {
		return slip, nil // no picture: the service decides whether that is fine
	}
	if header.Size > 4<<20 {
		return slip, response.ErrorBadRequest("SLIP_IMAGE_TOO_BIG")
	}
	f, err := header.Open()
	if err != nil {
		return slip, response.ErrorBadRequest("INVALID_SLIP_IMAGE")
	}
	defer f.Close()
	if slip.Data, err = io.ReadAll(io.LimitReader(f, 4<<20+1)); err != nil {
		return slip, response.ErrorBadRequest("INVALID_SLIP_IMAGE")
	}
	slip.Filename = header.Filename
	return slip, nil
}
