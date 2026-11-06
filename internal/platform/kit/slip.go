package kit

import (
	"backend/pkg/response"
	"net/http"
)

// MaxSlipBytes is the biggest payment-slip picture taken (the server's request limit is 4 MB; the website shrinks
// phone photos first)
const MaxSlipBytes = 3_500_000

var slipTypes = map[string]string{"image/jpeg": "jpg", "image/png": "png", "image/webp": "webp"}

// CheckSlipImage makes sure an upload is a real picture (jpg, png or webp, judged by its bytes, not its name) of a
// sensible size, and gives the file extension to use
func CheckSlipImage(data []byte) (string, error) {
	if len(data) == 0 {
		return "", response.ErrorBadRequest("SLIP_IMAGE_REQUIRED")
	}
	if len(data) > MaxSlipBytes {
		return "", response.ErrorBadRequest("SLIP_IMAGE_TOO_BIG")
	}
	ext, ok := slipTypes[http.DetectContentType(data)]
	if !ok {
		return "", response.ErrorBadRequest("INVALID_SLIP_IMAGE")
	}
	return ext, nil
}
