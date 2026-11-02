package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/vanclief/ez"
)

const (
	// DefaultWhatsAppAPI is the Cloud API base, no trailing slash.
	DefaultWhatsAppAPI = "https://graph.facebook.com"
	// DefaultWhatsAppVersion is the Graph API version used when the
	// config names none.
	DefaultWhatsAppVersion = "v21.0"

	// maxWhatsAppText is the Cloud API limit for a text body.
	maxWhatsAppText = 4096
	maxErrorBytes   = 2000
)

// WhatsApp sends a Notice as a text message through the WhatsApp Cloud
// API. Numbers and the phone number id come from agc's config, the token
// from the WHATSAPP_TOKEN secret, read on each call so a new token needs
// no restart.
type WhatsApp struct {
	// BaseURL defaults to DefaultWhatsAppAPI.
	BaseURL string
	// Version defaults to DefaultWhatsAppVersion.
	Version string
	// PhoneNumberID is the sending number's id at Meta, not the number.
	PhoneNumberID string
	// Numbers maps a human's name to their WhatsApp number, digits only
	// with the country code (the Cloud API "to" field).
	Numbers map[string]string
	// Token returns the access token.
	Token func() (string, error)
	// HTTP defaults to a client with a 10 second timeout.
	HTTP *http.Client
}

var _ Notifier = (*WhatsApp)(nil)

type whatsappText struct {
	Body string `json:"body"`
}

type whatsappMessage struct {
	MessagingProduct string       `json:"messaging_product"`
	To               string       `json:"to"`
	Type             string       `json:"type"`
	Text             whatsappText `json:"text"`
}

// Notify sends the notice to the human's number. A human with no number
// configured is not an error: nothing is sent and nil is returned.
func (w *WhatsApp) Notify(ctx context.Context, notice Notice) error {
	number := w.Numbers[notice.Human]
	if number == "" {
		return nil
	}

	token, err := w.Token()
	if err != nil {
		return ez.Wrap(err)
	}

	payload := whatsappMessage{
		MessagingProduct: "whatsapp",
		To:               strings.TrimPrefix(number, "+"),
		Type:             "text",
		Text:             whatsappText{Body: Format(notice)},
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return ez.Wrap(err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, w.url(), bytes.NewReader(encoded))
	if err != nil {
		return ez.Wrap(err)
	}

	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")

	client := w.HTTP
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	response, err := client.Do(request)
	if err != nil {
		return ez.New(ez.EINTERNAL, "whatsapp request failed", err)
	}
	defer response.Body.Close()

	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return nil
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxErrorBytes))
	if err != nil {
		return ez.Wrap(err)
	}

	msg := fmt.Sprintf("whatsapp answered %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	return ez.New(ez.EINTERNAL, msg, nil)
}

func (w *WhatsApp) url() string {
	base := strings.TrimRight(w.BaseURL, "/")
	if base == "" {
		base = DefaultWhatsAppAPI
	}

	version := w.Version
	if version == "" {
		version = DefaultWhatsAppVersion
	}

	return base + "/" + version + "/" + w.PhoneNumberID + "/messages"
}

// Format is the text a human reads: who, which task, then the message.
func Format(notice Notice) string {
	text := notice.Text
	limit := maxWhatsAppText - 200
	runes := []rune(text)
	if len(runes) > limit {
		text = string(runes[:limit]) + "..."
	}

	return fmt.Sprintf("%s in %s:\n%s", notice.Sender, notice.Task, text)
}
