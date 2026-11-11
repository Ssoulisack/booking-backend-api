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

// Every email says the same thing twice in one message: the Lao part first, then the English part, with one
// link. (Dates, prices and names that come from the booking are written once, as they are in the app.)
var emailLayout = template.Must(template.New("email").Parse(`<!DOCTYPE html>
<html lang="lo"><head><meta charset="UTF-8"><title>{{.Lao.Title}} / {{.En.Title}}</title></head>
<body style="font-family:'Noto Sans Lao','Lao UI','Lao Sangam MN',Arial,sans-serif;background:#f4f4f4;padding:20px;">
  <div style="max-width:560px;margin:auto;background:#fff;padding:28px;border-radius:8px;">
    <div lang="lo">
      <h2 style="color:#222;">{{.Lao.Title}}</h2>
      <p style="font-size:16px;color:#333;">{{.Lao.Message}}</p>
      {{if $.Code}}<p style="text-align:center;font-size:34px;font-weight:bold;letter-spacing:10px;margin:24px 0;font-family:Arial,sans-serif;">{{$.Code}}</p>{{end}}
      <p style="text-align:center;margin:28px 0;">
        <a href="{{.URL}}" style="background:#0b6bcb;color:#fff;text-decoration:none;padding:12px 24px;border-radius:5px;font-weight:bold;">{{.Lao.Button}}</a>
      </p>
      <p style="font-size:13px;color:#666;">{{.Lao.Note}}</p>
    </div>
    <hr style="border:none;border-top:1px solid #ddd;margin:24px 0;">
    <div lang="en" style="font-family:Arial,sans-serif;">
      <h2 style="color:#222;">{{.En.Title}}</h2>
      <p style="font-size:16px;color:#333;">{{.En.Message}}</p>
      {{if $.Code}}<p style="text-align:center;font-size:34px;font-weight:bold;letter-spacing:10px;margin:24px 0;font-family:Arial,sans-serif;">{{$.Code}}</p>{{end}}
      <p style="text-align:center;margin:28px 0;">
        <a href="{{.URL}}" style="background:#0b6bcb;color:#fff;text-decoration:none;padding:12px 24px;border-radius:5px;font-weight:bold;">{{.En.Button}}</a>
      </p>
      <p style="font-size:13px;color:#666;">{{.En.Note}}</p>
    </div>
    <p style="font-size:13px;color:#666;word-break:break-all;">ຖ້າປຸ່ມບໍ່ເຮັດວຽກ ໃຫ້ເປີດລິ້ງນີ້ / If the button does not work, open this link: <a href="{{.URL}}">{{.URL}}</a></p>
    <hr style="border:none;border-top:1px solid #ddd;margin:24px 0;">
    <p style="font-size:12px;color:#999;">{{.AppName}}</p>
  </div>
</body></html>`))

// emailText is one language's part of an email
type emailText struct{ Title, Message, Button, Note string }

// emailData is one email: the same content in Lao and in English, and the one link
type emailData struct {
	Lao, En emailText
	URL     string
	AppName string
	// Code: a code shown large in both parts (email verification)
	Code string
}

// subjectOf puts both languages in the subject, Lao first
func subjectOf(lao, en string) string { return lao + " / " + en }

func (m *systemMailer) send(to, subject string, d emailData) error {
	return m.sendMany([]string{to}, subject, d)
}

// sendMany sends ONE email to several people: the first is in To, the others in Bcc (they do not see each other)
func (m *systemMailer) sendMany(to []string, subject string, d emailData) error {
	var real []string
	for _, a := range to {
		if a != "" && !domain.IsGeneratedEmail(a) { // a stand-in address has no mailbox: nothing can be delivered
			real = append(real, a)
		}
	}
	if len(real) == 0 {
		return nil
	}
	d.AppName = m.appName
	var html bytes.Buffer
	if err := emailLayout.Execute(&html, d); err != nil {
		return err
	}
	part := func(t emailText) string {
		return fmt.Sprintf("%s\n\n%s\n\n%s: %s\n\n%s", t.Title, t.Message, t.Button, d.URL, t.Note)
	}
	plain := part(d.Lao) + "\n\n----------\n\n" + part(d.En) + "\n\n-- " + m.appName
	return m.repo.SendMail(dto.MailDTO{To: real[0], Bcc: real[1:], Subject: subject, Body: plain, BodyHTML: html.String()})
}

// roleLao and whereLao write the two words the invitation takes from the system in Lao
func roleLao(role string) string {
	switch role {
	case domain.CompanyOwner:
		return "ເຈົ້າຂອງ"
	case domain.CompanyAdmin:
		return "ຜູ້ດູແລ"
	case domain.MemberManager:
		return "ຜູ້ຈັດການ"
	case domain.MemberStaff:
		return "ພະນັກງານ"
	}
	return role
}

func whereLao(where string) string {
	if where == "every branch" {
		return "ທຸກສາຂາ"
	}
	return where // branch names are written once
}

func (m *systemMailer) SendInvitation(to, companyName, inviterName, role, where, acceptURL string) error {
	return m.send(to, subjectOf(fmt.Sprintf("ທ່ານໄດ້ຮັບເຊີນເຂົ້າຮ່ວມ %s", companyName), fmt.Sprintf("You're invited to join %s", companyName)), emailData{
		URL: acceptURL,
		Lao: emailText{
			Title:   fmt.Sprintf("ເຂົ້າຮ່ວມ %s", companyName),
			Message: fmt.Sprintf("%s ເຊີນທ່ານເຂົ້າຮ່ວມທີມງານ %s ໃນຖານະ %s (%s).", inviterName, companyName, roleLao(role), whereLao(where)),
			Button:  "ຮັບຄຳເຊີນ",
			Note:    "ຄຳເຊີນນີ້ໝົດອາຍຸໃນ 7 ວັນ. ຖ້າທ່ານບໍ່ໄດ້ຄາດຫວັງ, ບໍ່ຕ້ອງສົນໃຈອີເມວນີ້.",
		},
		En: emailText{
			Title:   fmt.Sprintf("Join %s", companyName),
			Message: fmt.Sprintf("%s invited you to join the %s team as %s (%s).", inviterName, companyName, role, where),
			Button:  "Accept invitation",
			Note:    "This invitation expires in 7 days. If you were not expecting it, you can ignore this email.",
		},
	})
}

func (m *systemMailer) SendWelcome(to, name, companyName, setPasswordURL string) error {
	return m.send(to, subjectOf(fmt.Sprintf("ລະບົບຈອງຂອງ %s ພ້ອມແລ້ວ", companyName), fmt.Sprintf("Your %s booking system is ready", companyName)), emailData{
		URL: setPasswordURL,
		Lao: emailText{
			Title:   fmt.Sprintf("ຍິນດີຕ້ອນຮັບ, %s", name),
			Message: fmt.Sprintf("ລະບົບຈອງຂອງ %s ຖືກຕິດຕັ້ງແລ້ວ ແລະ ທ່ານເປັນເຈົ້າຂອງ. ເລືອກລະຫັດຜ່ານເພື່ອເຂົ້າສູ່ລະບົບ ແລ້ວຕັ້ງຄ່າບໍລິສັດ ແລະ ສາຂາຂອງທ່ານ.", companyName),
			Button:  "ເລືອກລະຫັດຜ່ານ",
			Note:    "ລິ້ງນີ້ໃຊ້ໄດ້ຄັ້ງດຽວ ແລະ ໝົດອາຍຸໃນ 7 ວັນ. ຖ້າໝົດອາຍຸ ໃຫ້ໃຊ້ “ລືມລະຫັດຜ່ານ” ໃນໜ້າເຂົ້າສູ່ລະບົບ.",
		},
		En: emailText{
			Title:   fmt.Sprintf("Welcome, %s", name),
			Message: fmt.Sprintf("The booking system of %s is installed and you are its owner. Choose your password to log in and set up your company and branches.", companyName),
			Button:  "Choose my password",
			Note:    "This link works once and expires in 7 days. If it expired, use \"Forgot password\" on the login page.",
		},
	})
}

func (m *systemMailer) SendPasswordReset(to, name, resetURL string) error {
	return m.send(to, subjectOf("ຕັ້ງລະຫັດຜ່ານໃໝ່", "Reset your password"), emailData{
		URL: resetURL,
		Lao: emailText{
			Title:   "ຕັ້ງລະຫັດຜ່ານໃໝ່",
			Message: fmt.Sprintf("ສະບາຍດີ %s, ພວກເຮົາໄດ້ຮັບຄຳຂໍຕັ້ງລະຫັດຜ່ານ %s ຂອງທ່ານໃໝ່.", name, m.appName),
			Button:  "ຕັ້ງລະຫັດຜ່ານໃໝ່",
			Note:    "ລິ້ງນີ້ໝົດອາຍຸໃນ 1 ຊົ່ວໂມງ ແລະ ໃຊ້ໄດ້ຄັ້ງດຽວ. ຖ້າທ່ານບໍ່ໄດ້ຂໍ ໃຫ້ບໍ່ສົນໃຈອີເມວນີ້: ລະຫັດຜ່ານຂອງທ່ານຈະບໍ່ປ່ຽນ.",
		},
		En: emailText{
			Title:   "Reset your password",
			Message: fmt.Sprintf("Hi %s, we received a request to reset your %s password.", name, m.appName),
			Button:  "Set a new password",
			Note:    "This link expires in 1 hour and works once. If you did not ask for it, ignore this email: your password stays the same.",
		},
	})
}

func (m *systemMailer) SendEmailCode(to, name, code, accountURL string) error {
	return m.send(to, subjectOf("ລະຫັດຢືນຢັນອີເມວ "+code, "Your email code "+code), emailData{
		URL:  accountURL,
		Code: code,
		Lao: emailText{
			Title:   "ຢືນຢັນອີເມວຂອງທ່ານ",
			Message: fmt.Sprintf("ສະບາຍດີ %s, ໃສ່ລະຫັດນີ້ໃນ %s ເພື່ອຢືນຢັນວ່າອີເມວນີ້ເປັນຂອງທ່ານ:", name, m.appName),
			Button:  "ເປີດບັນຊີຂອງຂ້ອຍ",
			Note:    "ລະຫັດໝົດອາຍຸໃນ 10 ນາທີ. ຖ້າທ່ານບໍ່ໄດ້ຂໍ ໃຫ້ບໍ່ສົນໃຈອີເມວນີ້.",
		},
		En: emailText{
			Title:   "Verify your email",
			Message: fmt.Sprintf("Hi %s, enter this code in %s to show this email is yours:", name, m.appName),
			Button:  "Open my account",
			Note:    "The code expires in 10 minutes. If you did not ask for it, ignore this email.",
		},
	})
}

func (m *systemMailer) SendBookingLink(to, branchName, code, link string) error {
	return m.send(to, subjectOf(fmt.Sprintf("ການຈອງ %s ທີ່ %s", code, branchName), fmt.Sprintf("Your booking %s at %s", code, branchName)), emailData{
		URL: link,
		Lao: emailText{
			Title:   fmt.Sprintf("ການຈອງ %s", code),
			Message: fmt.Sprintf("ຂອບໃຈທີ່ຈອງທີ່ %s. ເກັບລິ້ງນີ້ໄວ້ເພື່ອເບິ່ງ ຫຼື ຍົກເລີກການຈອງຂອງທ່ານ.", branchName),
			Button:  "ເບິ່ງການຈອງຂອງຂ້ອຍ",
			Note:    "ຜູ້ທີ່ມີລິ້ງນີ້ສາມາດເບິ່ງ ແລະ ຍົກເລີກການຈອງໄດ້, ດັ່ງນັ້ນຢ່າແບ່ງປັນລິ້ງນີ້.",
		},
		En: emailText{
			Title:   fmt.Sprintf("Booking %s", code),
			Message: fmt.Sprintf("Thank you for booking at %s. Keep this link to see or cancel your booking.", branchName),
			Button:  "View my booking",
			Note:    "Anyone with this link can see and cancel the booking, so do not share it.",
		},
	})
}

func (m *systemMailer) SendBookingUpdate(to, name, branchName, code, status, reason, link string) error {
	en := emailText{
		Title:   fmt.Sprintf("Booking %s confirmed", code),
		Message: fmt.Sprintf("Hi %s, %s confirmed your booking %s. See you there!", name, branchName, code),
		Button:  "Open " + branchName,
		Note:    "If you can no longer come, please cancel or contact the branch.",
	}
	lo := emailText{
		Title:   fmt.Sprintf("ການຈອງ %s ຢືນຢັນແລ້ວ", code),
		Message: fmt.Sprintf("ສະບາຍດີ %s, %s ໄດ້ຢືນຢັນການຈອງ %s ຂອງທ່ານແລ້ວ. ພົບກັນທີ່ນັ້ນ!", name, branchName, code),
		Button:  "ເປີດ " + branchName,
		Note:    "ຖ້າທ່ານບໍ່ສາມາດມາໄດ້ອີກ ກະລຸນາຍົກເລີກ ຫຼື ຕິດຕໍ່ສາຂາ.",
	}
	if status == "cancelled" {
		en.Title = fmt.Sprintf("Booking %s cancelled", code)
		en.Message = fmt.Sprintf("Hi %s, %s cancelled your booking %s.", name, branchName, code)
		en.Note = "Sorry for the change. You can book another time on the branch page."
		lo.Title = fmt.Sprintf("ການຈອງ %s ຖືກຍົກເລີກ", code)
		lo.Message = fmt.Sprintf("ສະບາຍດີ %s, %s ໄດ້ຍົກເລີກການຈອງ %s ຂອງທ່ານ.", name, branchName, code)
		lo.Note = "ຂໍອະໄພສຳລັບການປ່ຽນແປງ. ທ່ານສາມາດຈອງເວລາອື່ນໄດ້ທີ່ໜ້າສາຂາ."
		if reason != "" { // the reason is what the team typed: written once
			en.Note = "Reason: " + reason + ". " + en.Note
			lo.Note = "ເຫດຜົນ: " + reason + ". " + lo.Note
		}
	}
	return m.send(to, subjectOf(lo.Title, en.Title), emailData{URL: link, Lao: lo, En: en})
}

func (m *systemMailer) SendBookingReceived(to string, b output.BookingMail) error {
	en := emailText{
		Title:   fmt.Sprintf("Booking %s received", b.Code),
		Message: fmt.Sprintf("%s: %s, %s, %s.", b.BranchName, b.What, b.When, b.Price),
		Button:  "My bookings",
		Note:    "If you can no longer come, please cancel from your bookings or contact the branch.",
	}
	lo := emailText{
		Title:   fmt.Sprintf("ໄດ້ຮັບການຈອງ %s ແລ້ວ", b.Code),
		Message: en.Message,
		Button:  "ການຈອງຂອງຂ້ອຍ",
		Note:    "ຖ້າທ່ານບໍ່ສາມາດມາໄດ້ອີກ ກະລຸນາຍົກເລີກຈາກການຈອງຂອງທ່ານ ຫຼື ຕິດຕໍ່ສາຂາ.",
	}
	if b.Pending {
		en.Title = fmt.Sprintf("Booking %s is waiting for approval", b.Code)
		en.Note = "The branch has to approve it first. You will get another email when it is confirmed."
		lo.Title = fmt.Sprintf("ການຈອງ %s ກຳລັງລໍຖ້າການອະນຸມັດ", b.Code)
		lo.Note = "ສາຂາຕ້ອງອະນຸມັດກ່ອນ. ທ່ານຈະໄດ້ຮັບອີເມວອີກສະບັບເມື່ອຢືນຢັນແລ້ວ."
	}
	return m.send(to, subjectOf(lo.Title, en.Title)+" - "+b.BranchName, emailData{URL: b.Link, Lao: lo, En: en})
}

func (m *systemMailer) SendNewBooking(to []string, b output.BookingMail) error {
	en := emailText{
		Title:   fmt.Sprintf("New booking %s", b.Code),
		Message: fmt.Sprintf("%s booked %s, %s, %s.", b.Customer, b.What, b.When, b.Price),
		Button:  "Open bookings",
		Note:    "Sent once to the managers and staff of this branch.",
	}
	lo := emailText{
		Title:   fmt.Sprintf("ການຈອງໃໝ່ %s", b.Code),
		Message: fmt.Sprintf("%s ໄດ້ຈອງ %s, %s, %s.", b.Customer, b.What, b.When, b.Price),
		Button:  "ເປີດການຈອງ",
		Note:    "ສົ່ງຄັ້ງດຽວຫາຜູ້ຈັດການ ແລະ ພະນັກງານຂອງສາຂານີ້.",
	}
	if b.Pending {
		en.Title += " (needs approval)"
		en.Note = "It waits for approval. " + en.Note
		lo.Title += " (ຕ້ອງອະນຸມັດ)"
		lo.Note = "ກຳລັງລໍຖ້າການອະນຸມັດ. " + lo.Note
	}
	return m.sendMany(to, subjectOf(lo.Title, en.Title)+" - "+b.BranchName, emailData{URL: b.Link, Lao: lo, En: en})
}

func (m *systemMailer) SendBookingReleased(to, name, branchName, code, link string) error {
	lo := emailText{
		Title:   fmt.Sprintf("ການຈອງ %s ຖືກປ່ອຍແລ້ວ", code),
		Message: fmt.Sprintf("ສະບາຍດີ %s, ພວກເຮົາບໍ່ໄດ້ຮັບການຊຳລະເງິນສຳລັບການຈອງ %s ທີ່ %s ທັນເວລາ ໂຕະຈຶ່ງຖືກປ່ອຍໃຫ້ລູກຄ້າອື່ນ.", name, code, branchName),
		Button:  "ເປີດ " + branchName,
		Note:    "ຖ້າທ່ານຍັງຢາກມາ ກະລຸນາຈອງໃໝ່ທີ່ໜ້າສາຂາ. ຖ້າທ່ານຊຳລະແລ້ວ ກະລຸນາຕິດຕໍ່ສາຂາ: ພວກເຮົາຈະກວດສອບ ແລະ ຄືນເງິນໃຫ້ຖ້າຈຳເປັນ.",
	}
	en := emailText{
		Title:   fmt.Sprintf("Booking %s released", code),
		Message: fmt.Sprintf("Hi %s, we did not receive the payment for your booking %s at %s in time, so the tables were released for other customers.", name, code, branchName),
		Button:  "Open " + branchName,
		Note:    "If you still want to come, please book again on the branch page. If you already paid, please contact the branch: we will check it and give your money back if needed.",
	}
	return m.send(to, subjectOf(lo.Title, en.Title), emailData{URL: link, Lao: lo, En: en})
}

// SendRefund: the bank was asked to give the money back (done false), or confirmed it (done true)
func (m *systemMailer) SendRefund(to, name, branchName, code, amount, link string, done bool) error {
	lo := emailText{
		Title:   fmt.Sprintf("ຄືນເງິນສຳລັບການຈອງ %s", code),
		Message: fmt.Sprintf("ສະບາຍດີ %s, %s ໄດ້ຂໍໃຫ້ທະນາຄານຄືນເງິນ %s ສຳລັບການຈອງ %s.", name, branchName, amount, code),
		Button:  "ເປີດ " + branchName,
		Note:    "ທະນາຄານອາດໃຊ້ເວລາສອງສາມມື້ເພື່ອຄືນເງິນເຂົ້າບັນຊີທີ່ທ່ານຊຳລະ. ທ່ານຈະໄດ້ຮັບອີເມວອີກສະບັບເມື່ອສຳເລັດ.",
	}
	en := emailText{
		Title:   fmt.Sprintf("Refund for booking %s", code),
		Message: fmt.Sprintf("Hi %s, %s asked the bank to refund %s for booking %s.", name, branchName, amount, code),
		Button:  "Open " + branchName,
		Note:    "The bank may take a few days to return the money to the account you paid from. You will get another email when it is done.",
	}
	if done {
		lo.Title = fmt.Sprintf("ຄືນເງິນສຳເລັດ: ການຈອງ %s", code)
		lo.Message = fmt.Sprintf("ສະບາຍດີ %s, ທະນາຄານຢືນຢັນການຄືນເງິນ %s ສຳລັບການຈອງ %s ທີ່ %s ແລ້ວ.", name, amount, code, branchName)
		lo.Note = "ເງິນກຳລັງກັບຄືນເຂົ້າບັນຊີທີ່ທ່ານຊຳລະ."
		en.Title = fmt.Sprintf("Refund done: booking %s", code)
		en.Message = fmt.Sprintf("Hi %s, the bank confirmed the refund of %s for booking %s at %s.", name, amount, code, branchName)
		en.Note = "The money is on its way back to the account you paid from."
	}
	return m.send(to, subjectOf(lo.Title, en.Title), emailData{URL: link, Lao: lo, En: en})
}
