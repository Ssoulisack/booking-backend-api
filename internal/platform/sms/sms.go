// Package sms sends text messages through the Wenova SMS API (POST {baseUrl}/sms/package, the token in the body).
package sms

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Config is the sms: block of config.yaml (or APP_SMS_* variables)
type Config struct {
	// Enabled: false switches SMS codes off; registering, resetting and guest bookings then work without a code
	Enabled bool
	// BaseURL of the Wenova API, without /sms/package
	BaseURL string
	// Header is the registered sender name shown on the phone (e.g. WNV-OTP)
	Header string
	// Token from Dashboard → Key; or ScriptID instead (one of the two is needed)
	Token    string
	ScriptID int
	// UsePackage: true pays from the SMS package quota, false from the wallet (LAK per segment)
	UsePackage bool
}

// Sender sends one message to a Lao mobile number written 20xxxxxxxx
type Sender interface {
	Send(ctx context.Context, phone, message string) error
}

// Ready reports whether the settings are complete enough to send
func (c Config) Ready() error {
	switch {
	case strings.TrimSpace(c.BaseURL) == "":
		return errors.New("sms.baseUrl is empty")
	case strings.TrimSpace(c.Header) == "":
		return errors.New("sms.header is empty")
	case strings.TrimSpace(c.Token) == "" && c.ScriptID == 0:
		return errors.New("sms.token (or sms.scriptId) is empty")
	}
	return nil
}

type client struct {
	cfg  Config
	http *http.Client
}

// New builds the Wenova sender
func New(cfg Config) Sender {
	return &client{cfg: cfg, http: &http.Client{Timeout: 15 * time.Second}}
}

type request struct {
	Header      string `json:"header"`
	PhoneNumber string `json:"phoneNumber"`
	Message     string `json:"message"`
	Token       string `json:"token,omitempty"`
	ScriptID    int    `json:"scriptId,omitempty"`
	UsePackage  bool   `json:"usePackage"`
}

func (c *client) Send(ctx context.Context, phone, message string) error {
	body, err := json.Marshal(request{
		Header: c.cfg.Header, PhoneNumber: phone, Message: message,
		Token: c.cfg.Token, ScriptID: c.cfg.ScriptID, UsePackage: c.cfg.UsePackage,
	})
	if err != nil {
		return err
	}
	url := strings.TrimRight(c.cfg.BaseURL, "/") + "/sms/package"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	reply, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
	if res.StatusCode >= 300 {
		// e.g. 400 "Package not enough" / "Balance not enough": the operator's own words, for the log
		return fmt.Errorf("sms: %s: %s", res.Status, strings.TrimSpace(string(reply)))
	}
	return nil
}
