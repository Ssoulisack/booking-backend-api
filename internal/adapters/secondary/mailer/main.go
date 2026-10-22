package mailer

import (
	"backend/internal/core/dto"
	"log"

	"gopkg.in/gomail.v2"
)

type Config struct {
	Host     string
	Username string
	Password string
	Port     int
	From     string
}

type MailerRepository interface {
	SendMail(mailer dto.MailDTO) error
}

type mailerRepository struct {
	mailer *gomail.Dialer
	config Config
}

// SendMail handles sending via SMTP
func (m *mailerRepository) SendMail(mailer dto.MailDTO) error {
	message := gomail.NewMessage()

	// Set sender and recipient
	message.SetHeader("From", m.config.From)
	message.SetHeader("To", mailer.To)
	message.SetHeader("Subject", mailer.Subject)

	// Add headers to help spam filters
	message.SetHeader("X-Mailer", "GoMail")
	message.SetHeader("List-Unsubscribe", "<mailto:unsubscribe@example.com>")
	message.SetHeader("Precedence", "bulk") // ถ้าเป็นอีเมลแบบอัตโนมัติ
	message.SetHeader("MIME-Version", "1.0")

	// Add plain and HTML body
	plainBody := mailer.Body
	if plainBody == "" {
		plainBody = "To view this invitation, please use an HTML-compatible email client."
	}
	message.SetBody("text/plain", plainBody)

	if mailer.BodyHTML != "" {
		message.AddAlternative("text/html", mailer.BodyHTML)
	}

	// Send email
	if err := m.mailer.DialAndSend(message); err != nil {
		log.Printf("❌ Failed to send email: %v", err)
		return err
	}

	log.Printf("✅ Email sent to %s with subject: %s", mailer.To, mailer.Subject)
	return nil
}

func NewMailerRepository(config Config) MailerRepository {
	port := config.Port
	if port == 0 {
		port = 587
	}
	mailer := gomail.NewDialer(config.Host, port, config.Username, config.Password)
	return &mailerRepository{mailer: mailer, config: config}
}

// Ensure RedisRepository implements CacheRepository
// var _ output.MailerRepository = (*MailerRepository)(nil)
