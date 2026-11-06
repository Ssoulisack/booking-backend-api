// Package botapi talks to Telegram's Bot API (https://core.telegram.org/bots/api). The token is part of every
// request address, so no error from here ever carries an address: a failure says what Telegram answered, or only
// that Telegram could not be reached.
package botapi

import (
	"backend/internal/modules/telegram/port"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// DefaultBase is Telegram's address
const DefaultBase = "https://api.telegram.org"

// Client is the Bot API
type Client struct {
	base string
	http *http.Client
}

// New makes a client for Telegram (or, in tests, a stand-in at base)
func New(base string) *Client {
	if strings.TrimSpace(base) == "" {
		base = DefaultBase
	}
	return &Client{base: strings.TrimRight(base, "/"), http: &http.Client{Timeout: 12 * time.Second}}
}

type envelope struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result"`
	ErrorCode   int             `json:"error_code"`
	Description string          `json:"description"`
}

// call posts one method. The token is in the address; a failure is returned without it.
func (c *Client) call(ctx context.Context, token, method string, body map[string]any, into any) error {
	raw, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/bot"+url.PathEscape(token)+"/"+method, bytes.NewReader(raw))
	if err != nil {
		return errors.New("telegram: the bot token is not usable")
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return errors.New("telegram: cannot be reached from this server") // the error text would hold the address, so it is dropped
	}
	defer res.Body.Close()
	var env envelope
	if err := json.NewDecoder(res.Body).Decode(&env); err != nil {
		return &port.APIError{Code: res.StatusCode, Description: "unreadable answer"}
	}
	if !env.OK {
		return &port.APIError{Code: env.ErrorCode, Description: env.Description}
	}
	if into != nil {
		return json.Unmarshal(env.Result, into)
	}
	return nil
}

func (c *Client) Me(ctx context.Context, token string) (string, error) {
	var me struct {
		Username string `json:"username"`
	}
	if err := c.call(ctx, token, "getMe", map[string]any{}, &me); err != nil {
		return "", err
	}
	return "@" + me.Username, nil
}

func (c *Client) Chat(ctx context.Context, token, chatID string) (string, error) {
	var chat struct {
		Title     string `json:"title"`
		FirstName string `json:"first_name"`
	}
	if err := c.call(ctx, token, "getChat", map[string]any{"chat_id": chatID}, &chat); err != nil {
		return "", err
	}
	if chat.Title != "" {
		return chat.Title, nil
	}
	return chat.FirstName, nil
}

// FindGroups lists the groups and channels in the bot's recent updates (Telegram keeps about 24 hours, up to 100),
// the one with the latest activity first, each once. ErrNoGroup when there is none.
func (c *Client) FindGroups(ctx context.Context, token string) ([]port.Group, error) {
	var updates []struct {
		Message       *struct{ Chat chatRef } `json:"message"`
		MyChatMember  *struct{ Chat chatRef } `json:"my_chat_member"`
		ChannelPost   *struct{ Chat chatRef } `json:"channel_post"`
		EditedMessage *struct{ Chat chatRef } `json:"edited_message"`
	}
	err := c.call(ctx, token, "getUpdates", map[string]any{
		"limit": 100, "timeout": 0, "allowed_updates": []string{"message", "my_chat_member", "channel_post", "edited_message"},
	}, &updates)
	if err != nil {
		return nil, err
	}
	var out []port.Group
	seen := map[int64]bool{}
	for i := len(updates) - 1; i >= 0; i-- {
		for _, ref := range []*struct{ Chat chatRef }{updates[i].MyChatMember, updates[i].Message, updates[i].ChannelPost, updates[i].EditedMessage} {
			if ref == nil || seen[ref.Chat.ID] || !(ref.Chat.Type == "group" || ref.Chat.Type == "supergroup" || ref.Chat.Type == "channel") {
				continue
			}
			seen[ref.Chat.ID] = true
			out = append(out, port.Group{ID: strconv.FormatInt(ref.Chat.ID, 10), Title: ref.Chat.Title})
		}
	}
	if len(out) == 0 {
		return nil, port.ErrNoGroup
	}
	return out, nil
}

type chatRef struct {
	ID    int64  `json:"id"`
	Type  string `json:"type"`
	Title string `json:"title"`
}

func (c *Client) Send(ctx context.Context, token, chatID, text string) error {
	return c.call(ctx, token, "sendMessage", map[string]any{"chat_id": chatID, "text": text, "disable_web_page_preview": true}, nil)
}

// SendPhoto uploads a picture (multipart): the customer's payment slip, with a caption for the team
func (c *Client) SendPhoto(ctx context.Context, token, chatID, caption string, photo []byte, filename string) error {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("chat_id", chatID)
	_ = w.WriteField("caption", caption)
	part, err := w.CreateFormFile("photo", filename)
	if err != nil {
		return err
	}
	if _, err := part.Write(photo); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/bot"+url.PathEscape(token)+"/sendPhoto", &buf)
	if err != nil {
		return errors.New("telegram: the bot token is not usable")
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	res, err := c.http.Do(req)
	if err != nil {
		return errors.New("telegram: cannot be reached from this server")
	}
	defer res.Body.Close()
	var env envelope
	if err := json.NewDecoder(res.Body).Decode(&env); err != nil {
		return &port.APIError{Code: res.StatusCode, Description: "unreadable answer"}
	}
	if !env.OK {
		return &port.APIError{Code: env.ErrorCode, Description: env.Description}
	}
	return nil
}
