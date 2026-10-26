package dto

type MailDTO struct {
	To            string   `json:"to"`
	Bcc           []string `json:"bcc,omitempty"` // more recipients who do not see each other
	Subject       string   `json:"subject"`
	Body          string   `json:"body"`
	BodyHTML      string   `json:"body_html"`
	UseHTMLLayout bool     `json:"use_html_layout"`
}
