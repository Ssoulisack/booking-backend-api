package dto

// The ways a customer can send a payment slip
const (
	SlipWhatsapp = "whatsapp"
	SlipTelegram = "telegram"
)

// SlipUpload is "I paid, here is the slip": via WhatsApp (nothing to upload, the customer sent it there) or via
// Telegram (the picture, which goes to the branch's group and is not kept)
type SlipUpload struct {
	Via      string
	Data     []byte
	Filename string
}
