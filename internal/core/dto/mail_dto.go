package dto

type MailDTO struct {
	To            string `json:"to"`
	Subject       string `json:"subject"`
	Body          string `json:"body"`
	BodyHTML      string `json:"body_html"`
	UseHTMLLayout bool   `json:"use_html_layout"`
}
