// Package mail sends the API's plain-text email and keeps the waiting list's
// contacts at the provider. Senders never log or return an address, a message
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
	"net/url"
	"sort"
	"sync"
)

// Message is one plain-text email to one address.
type Message struct {
	To      string
	Subject string
	Text    string
}

// Contact is one confirmed person on the waiting list. Topics maps a topic
// at the provider to whether the person opted in to it.
type Contact struct {
	Email   string
	Area    string
	Hazards string
	Topics  map[string]bool
}

// Mailer sends messages and saves contacts.
type Mailer interface {
	Send(ctx context.Context, m Message) error
	SaveContact(ctx context.Context, c Contact) error
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

// ResendAPI is the root of Resend's API.
const ResendAPI = "https://api.resend.com"

// Resend sends through Resend's API. Open and click tracking are settings of
// the sending domain at Resend, and are off there; the messages are plain
// text, with no HTML part a tracker could rewrite.
type Resend struct {
	Key, From, ReplyTo string
	Client             *http.Client
	// Base overrides ResendAPI, for tests.
	Base string
}

type resendRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	Text    string   `json:"text"`
	ReplyTo string   `json:"reply_to"`
}

// Send posts one message.
func (r *Resend) Send(ctx context.Context, m Message) error {
	_, err := r.call(ctx, http.MethodPost, "/emails", resendRequest{From: r.From, To: []string{m.To}, Subject: m.Subject, Text: m.Text, ReplyTo: r.ReplyTo})
	return err
}

type resendTopic struct {
	ID           string `json:"id"`
	Subscription string `json:"subscription"`
}

// SaveContact creates the contact, or updates it when creating fails because
// it may already exist (Resend's docs do not say how a duplicate fails), then
// sets each topic. Saving the same contact twice leaves it as saved once.
func (r *Resend) SaveContact(ctx context.Context, c Contact) error {
	props := map[string]string{"area": c.Area, "hazards": c.Hazards}
	status, err := r.call(ctx, http.MethodPost, "/contacts", map[string]any{"email": c.Email, "unsubscribed": false, "properties": props})
	if err != nil {
		// Only a refusal that could mean "already there" is worth an update.
		if status != http.StatusBadRequest && status != http.StatusConflict && status != http.StatusUnprocessableEntity {
			return err
		}
		if _, uerr := r.call(ctx, http.MethodPatch, "/contacts/"+url.PathEscape(c.Email), map[string]any{"unsubscribed": false, "properties": props}); uerr != nil {
			return errors.Join(err, uerr)
		}
	}
	ids := make([]string, 0, len(c.Topics))
	for id := range c.Topics {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	topics := make([]resendTopic, 0, len(ids))
	for _, id := range ids {
		sub := "opt_out"
		if c.Topics[id] {
			sub = "opt_in"
		}
		topics = append(topics, resendTopic{ID: id, Subscription: sub})
	}
	_, err = r.call(ctx, http.MethodPatch, "/contacts/"+url.PathEscape(c.Email)+"/topics", map[string]any{"topics": topics})
	return err
}

// call sends one JSON request and returns the status. Its error carries
// Resend's status and error name, never Resend's message, which can quote the
// address, and never the path, which can hold it.
func (r *Resend) call(ctx context.Context, method, path string, payload any) (int, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("resend: %w", err)
	}
	base := r.Base
	if base == "" {
		base = ResendAPI
	}
	req, err := http.NewRequestWithContext(ctx, method, base+path, bytes.NewReader(body))
	if err != nil {
		return 0, errors.New("resend: bad request")
	}
	req.Header.Set("Authorization", "Bearer "+r.Key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := r.Client.Do(req)
	if err != nil {
		// A url.Error quotes the URL, whose path can hold the address.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return 0, fmt.Errorf("resend: %w", ctxErr)
		}
		return 0, errors.New("resend: request failed")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 == 2 {
		return resp.StatusCode, nil
	}
	var problem struct {
		Name string `json:"name"`
	}
	_ = json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&problem)
	if problem.Name == "" {
		problem.Name = "no error name"
	}
	return resp.StatusCode, fmt.Errorf("resend: status %d (%s)", resp.StatusCode, problem.Name)
}

// Log keeps messages instead of sending them, and prints each one, address and
// link included: for tests and local runs only (MAIL_SENDER=log), never production.
type Log struct {
	Logger   *slog.Logger
	mu       sync.Mutex
	sent     []Message
	contacts []Contact
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

// SaveContact records and prints the contact.
func (l *Log) SaveContact(_ context.Context, c Contact) error {
	l.mu.Lock()
	l.contacts = append(l.contacts, c)
	l.mu.Unlock()
	if l.Logger != nil {
		l.Logger.Info("contact not saved: log sender", "email", c.Email, "area", c.Area, "hazards", c.Hazards, "topics", c.Topics)
	}
	return nil
}

// Contacts returns the contacts saved so far, oldest first.
func (l *Log) Contacts() []Contact {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]Contact(nil), l.contacts...)
}

// Sent returns the messages so far, oldest first.
func (l *Log) Sent() []Message {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]Message(nil), l.sent...)
}
