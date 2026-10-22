package mailer

import (
	"backend/internal/core/domain"
	"backend/internal/core/dto"
	"backend/internal/core/port/output"
	"bytes"
	"fmt"
	"html/template"
)

// systemMailer sends the system's own emails (welcome, invites, password resets, bookings).
// html/template escapes every value, so company, branch and user names cannot inject HTML.
type systemMailer struct {
	repo    MailerRepository
	appName string
}

// NewSystemMailer wraps the SMTP mailer with the system's email templates
func NewSystemMailer(config Config, appName string) output.Mailer {
	return &systemMailer{repo: NewMailerRepository(config), appName: appName}
}

var emailLayout = template.Must(template.New("email").Parse(`<!DOCTYPE html>
<html lang="en"><head><meta charset="UTF-8"><title>{{.Title}}</title></head>
<body style="font-family:Arial,sans-serif;background:#f4f4f4;padding:20px;">
  <div style="max-width:560px;margin:auto;background:#fff;padding:28px;border-radius:8px;">
    <h2 style="color:#222;">{{.Title}}</h2>
    <p style="font-size:16px;color:#333;">{{.Message}}</p>
    <p style="text-align:center;margin:32px 0;">
      <a href="{{.URL}}" style="background:#0b6bcb;color:#fff;text-decoration:none;padding:12px 24px;border-radius:5px;font-weight:bold;">{{.Button}}</a>
    </p>
    <p style="font-size:13px;color:#666;">{{.Note}}</p>
    <p style="font-size:13px;color:#666;word-break:break-all;">If the button does not work, open this link: <a href="{{.URL}}">{{.URL}}</a></p>
    <hr style="border:none;border-top:1px solid #ddd;margin:24px 0;">
    <p style="font-size:12px;color:#999;">{{.AppName}}</p>
  </div>
</body></html>`))

type emailData struct {
	Title, Message, Button, URL, Note, AppName string
}

func (m *systemMailer) send(to, subject string, d emailData) error {
	if domain.IsGeneratedEmail(to) {
		return nil // a stand-in address of an account without a real email: nothing can be delivered
	}
	d.AppName = m.appName
	var html bytes.Buffer
	if err := emailLayout.Execute(&html, d); err != nil {
		return err
	}
	plain := fmt.Sprintf("%s\n\n%s\n\n%s: %s\n\n%s\n\n-- %s", d.Title, d.Message, d.Button, d.URL, d.Note, m.appName)
	return m.repo.SendMail(dto.MailDTO{To: to, Subject: subject, Body: plain, BodyHTML: html.String()})
}

func (m *systemMailer) SendInvitation(to, companyName, inviterName, role, where, acceptURL string) error {
	return m.send(to, fmt.Sprintf("You're invited to join %s", companyName), emailData{
		Title:   fmt.Sprintf("Join %s", companyName),
		Message: fmt.Sprintf("%s invited you to join the %s team as %s (%s).", inviterName, companyName, role, where),
		Button:  "Accept invitation",
		URL:     acceptURL,
		Note:    "This invitation expires in 7 days. If you were not expecting it, you can ignore this email.",
	})
}

func (m *systemMailer) SendWelcome(to, name, companyName, setPasswordURL string) error {
	return m.send(to, fmt.Sprintf("Your %s booking system is ready", companyName), emailData{
		Title:   fmt.Sprintf("Welcome, %s", name),
		Message: fmt.Sprintf("The booking system of %s is installed and you are its owner. Choose your password to log in and set up your company and branches.", companyName),
		Button:  "Choose my password",
		URL:     setPasswordURL,
		Note:    "This link works once and expires in 7 days. If it expired, use \"Forgot password\" on the login page.",
	})
}

func (m *systemMailer) SendPasswordReset(to, name, resetURL string) error {
	return m.send(to, "Reset your password", emailData{
		Title:   "Reset your password",
		Message: fmt.Sprintf("Hi %s, we received a request to reset your %s password.", name, m.appName),
		Button:  "Set a new password",
		URL:     resetURL,
		Note:    "This link expires in 1 hour and works once. If you did not ask for it, ignore this email: your password stays the same.",
	})
}

func (m *systemMailer) SendBookingLink(to, branchName, code, link string) error {
	return m.send(to, fmt.Sprintf("Your booking %s at %s", code, branchName), emailData{
		Title:   fmt.Sprintf("Booking %s", code),
		Message: fmt.Sprintf("Thank you for booking at %s. Keep this link to see or cancel your booking.", branchName),
		Button:  "View my booking",
		URL:     link,
		Note:    "Anyone with this link can see and cancel the booking, so do not share it.",
	})
}

func (m *systemMailer) SendBookingUpdate(to, name, branchName, code, status, reason, link string) error {
	title := fmt.Sprintf("Booking %s confirmed", code)
	message := fmt.Sprintf("Hi %s, %s confirmed your booking %s. See you there!", name, branchName, code)
	note := "If you can no longer come, please cancel or contact the branch."
	if status == "cancelled" {
		title = fmt.Sprintf("Booking %s cancelled", code)
		message = fmt.Sprintf("Hi %s, %s cancelled your booking %s.", name, branchName, code)
		note = "Sorry for the change. You can book another time on the branch page."
		if reason != "" {
			note = "Reason: " + reason + ". " + note
		}
	}
	return m.send(to, title, emailData{Title: title, Message: message, Button: "Open " + branchName, URL: link, Note: note})
}
