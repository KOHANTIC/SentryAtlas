// Package mail sends the API's plain-text email: today, the waiting list's
// confirmation link (#13). Senders never log or return an address, a message
// body or the provider key, so production logs hold none of them.
package mail

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/mail"
	"sync"
)

// Message is one plain-text email to one address.
type Message struct {
	To      string
	Subject string
	Text    string
}

// Mailer sends one message.
type Mailer interface {
	Send(ctx context.Context, m Message) error
}

// FromEnv builds the mailer the environment asks for and names it for the
// startup line. With no RESEND_API_KEY mail is off: the mailer is nil and the
// name "off". A key without a valid MAIL_FROM and MAIL_REPLY_TO is an error,
// and mail stays off rather than the server refusing to start.
func FromEnv(getenv func(string) string, client *http.Client, logger *slog.Logger) (Mailer, string, error) {
	switch getenv("MAIL_SENDER") {
	case "log":
		return &Log{Logger: logger}, "log", nil
	case "", "resend":
	default:
		return nil, "off", errors.New("MAIL_SENDER must be empty, resend or log")
	}
	key := getenv("RESEND_API_KEY")
	if key == "" {
		return nil, "off", nil
	}
	from, err := address("MAIL_FROM", getenv("MAIL_FROM"))
	if err != nil {
		return nil, "off", err
	}
	replyTo, err := address("MAIL_REPLY_TO", getenv("MAIL_REPLY_TO"))
	if err != nil {
		return nil, "off", err
	}
	return &Resend{Key: key, From: from, ReplyTo: replyTo, Client: client}, "resend", nil
}

// address checks one configured address; the error names the variable only.
func address(name, v string) (string, error) {
	if v == "" {
		return "", fmt.Errorf("%s is required when RESEND_API_KEY is set", name)
	}
	if _, err := mail.ParseAddress(v); err != nil {
		return "", fmt.Errorf("%s is not an email address", name)
	}
	return v, nil
}

// ResendEndpoint is Resend's send-email API.
const ResendEndpoint = "https://api.resend.com/emails"

// Resend sends through Resend's API. Open and click tracking are settings of
// the sending domain at Resend, and are off there; the messages are plain
// text, with no HTML part a tracker could rewrite.
type Resend struct {
	Key, From, ReplyTo string
	Client             *http.Client
	// Endpoint overrides ResendEndpoint, for tests.
	Endpoint string
}

type resendRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	Text    string   `json:"text"`
	ReplyTo string   `json:"reply_to"`
}

// Send posts one message. Its error carries Resend's status and error name,
// never Resend's message, which can quote the address.
func (r *Resend) Send(ctx context.Context, m Message) error {
	body, err := json.Marshal(resendRequest{From: r.From, To: []string{m.To}, Subject: m.Subject, Text: m.Text, ReplyTo: r.ReplyTo})
	if err != nil {
		return fmt.Errorf("resend: %w", err)
	}
	endpoint := r.Endpoint
	if endpoint == "" {
		endpoint = ResendEndpoint
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("resend: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+r.Key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := r.Client.Do(req)
	if err != nil {
		// A url.Error names the endpoint, never the body; ctx errors pass through.
		return fmt.Errorf("resend: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 == 2 {
		return nil
	}
	var problem struct {
		Name string `json:"name"`
	}
	_ = json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&problem)
	if problem.Name == "" {
		problem.Name = "no error name"
	}
	return fmt.Errorf("resend: status %d (%s)", resp.StatusCode, problem.Name)
}

// Log keeps messages instead of sending them, and prints each one, address and
// link included: for tests and local runs only (MAIL_SENDER=log), never production.
type Log struct {
	Logger *slog.Logger
	mu     sync.Mutex
	sent   []Message
}

// Send records and prints the message.
func (l *Log) Send(_ context.Context, m Message) error {
	l.mu.Lock()
	l.sent = append(l.sent, m)
	l.mu.Unlock()
	if l.Logger != nil {
		l.Logger.Info("mail not sent: log sender", "to", m.To, "subject", m.Subject, "text", m.Text)
	}
	return nil
}

// Sent returns the messages so far, oldest first.
func (l *Log) Sent() []Message {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]Message(nil), l.sent...)
}
