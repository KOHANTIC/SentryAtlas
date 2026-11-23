// Package waitlist runs the waiting list for location alerts: a signup sends a
// signed confirmation link, and confirming saves the contact at the mail
// provider. The service itself stores no one: between the two steps the link
// carries the request, and limits are counted in memory.
package waitlist

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	stdmail "net/mail"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/KOHANTIC/SentryAtlas/backend/internal/mail"
	"github.com/KOHANTIC/SentryAtlas/backend/internal/models"
)

const (
	linkLife = 48 * time.Hour
	// Resend's free plan sends 100 a day; the rest is headroom for retries.
	dailySends = 90
	// Sends to one address in a day, so the list cannot be used to flood a mailbox.
	perAddress = 3
	maxArea    = 100
	maxBody    = 4 << 10
)

// Config is what the list needs; New reports it off when any part is missing.
type Config struct {
	Mailer mail.Mailer
	// Secrets sign links: the first signs, all verify.
	Secrets [][]byte
	// AlertsTopic and NewsTopic are the provider's topic ids for the two consents.
	AlertsTopic, NewsTopic string
	// ConfirmURL is the page the link opens; the token follows its "#".
	ConfirmURL string
	Logger     *slog.Logger
	// Now and Resolve are replaced in tests.
	Now     func() time.Time
	Resolve func(ctx context.Context, domain string) error
}

// List serves the two routes.
type List struct {
	cfg Config
	on  bool

	mu        sync.Mutex
	day       string
	sentToday int
	byAddress map[[32]byte][]time.Time
}

// New returns the list, which is off unless the mailer, a secret, both topics
// and the confirmation page are all set.
func New(cfg Config) *List {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Resolve == nil {
		cfg.Resolve = canReceiveMail
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	on := cfg.Mailer != nil && len(cfg.Secrets) > 0 && cfg.AlertsTopic != "" && cfg.NewsTopic != "" && cfg.ConfirmURL != ""
	return &List{cfg: cfg, on: on, byAddress: map[[32]byte][]time.Time{}}
}

// On reports whether the list takes signups.
func (l *List) On() bool { return l.on }

type signupRequest struct {
	Email   string   `json:"email"`
	Area    string   `json:"area"`
	Hazards []string `json:"hazards"`
	News    bool     `json:"news"`
}

// Signup takes an address and the person's choices, and emails a confirmation
// link. It answers the same way whether or not the address was seen before.
func (l *List) Signup(w http.ResponseWriter, r *http.Request) {
	if !l.on {
		reply(w, http.StatusServiceUnavailable, "error", "the waiting list is not open")
		return
	}
	var req signupRequest
	if err := decode(w, r, &req); err != nil {
		reply(w, http.StatusBadRequest, "error", "the request is not valid JSON with email, area, hazards and news")
		return
	}
	s, problem := l.check(req)
	if problem != "" {
		reply(w, http.StatusBadRequest, "error", problem)
		return
	}
	domain := s.Email[strings.LastIndexByte(s.Email, '@')+1:]
	if err := l.cfg.Resolve(r.Context(), domain); err != nil {
		reply(w, http.StatusBadRequest, "error", "that address's domain does not receive email")
		return
	}

	switch l.take(s.Email) {
	case overAddress:
		// Said to be sent, so the answer does not reveal the address was seen.
		reply(w, http.StatusAccepted, "status", "sent")
		return
	case overBudget:
		reply(w, http.StatusServiceUnavailable, "error", "the waiting list has taken all the signups it can today; try again tomorrow")
		return
	}

	s.Expires = l.cfg.Now().Add(linkLife).Unix()
	token, err := sign(s, l.cfg.Secrets)
	if err != nil {
		reply(w, http.StatusInternalServerError, "error", "could not make the link")
		return
	}
	if err := l.cfg.Mailer.Send(r.Context(), confirmation(s, l.cfg.ConfirmURL+"#"+token)); err != nil {
		l.cfg.Logger.Error("waitlist: confirmation not sent", "error", err)
		// 503, not 502: the platform's edge replaces a 502 with its own page.
		reply(w, http.StatusServiceUnavailable, "error", "the email could not be sent; try again in a few minutes")
		return
	}
	reply(w, http.StatusAccepted, "status", "sent")
}

type confirmRequest struct {
	Token string `json:"token"`
}

// Confirm checks a link's token and saves the contact. Confirming again is harmless.
func (l *List) Confirm(w http.ResponseWriter, r *http.Request) {
	if !l.on {
		reply(w, http.StatusServiceUnavailable, "error", "the waiting list is not open")
		return
	}
	var req confirmRequest
	if err := decode(w, r, &req); err != nil {
		reply(w, http.StatusBadRequest, "status", "invalid")
		return
	}
	s, err := verify(req.Token, l.cfg.Secrets, l.cfg.Now())
	switch {
	case errors.Is(err, errExpired):
		reply(w, http.StatusGone, "status", "expired")
		return
	case err != nil:
		reply(w, http.StatusBadRequest, "status", "invalid")
		return
	}
	c := mail.Contact{
		Email:   s.Email,
		Area:    s.Area,
		Hazards: strings.Join(s.Hazards, ","),
		Topics:  map[string]bool{l.cfg.AlertsTopic: true, l.cfg.NewsTopic: s.News},
	}
	if err := l.cfg.Mailer.SaveContact(r.Context(), c); err != nil {
		l.cfg.Logger.Error("waitlist: contact not saved", "error", err)
		reply(w, http.StatusServiceUnavailable, "status", "failed")
		return
	}
	reply(w, http.StatusOK, "status", "confirmed")
}

// check normalizes a request into what the link will carry, or says what is wrong.
func (l *List) check(req signupRequest) (signup, string) {
	addr, err := stdmail.ParseAddress(strings.TrimSpace(req.Email))
	if err != nil || addr.Name != "" || len(addr.Address) > 254 || !strings.Contains(addr.Address, "@") {
		return signup{}, "email is not an email address"
	}
	area := strings.TrimSpace(req.Area)
	if len([]rune(area)) > maxArea {
		return signup{}, fmt.Sprintf("area is longer than %d characters", maxArea)
	}
	if strings.IndexFunc(area, unicode.IsControl) >= 0 {
		return signup{}, "area holds control characters"
	}
	seen := map[string]bool{}
	var hazards []string
	for _, h := range req.Hazards {
		if !models.IsValidEventType(h) {
			return signup{}, fmt.Sprintf("hazards holds an unknown type: %q", h)
		}
		if !seen[h] {
			seen[h] = true
			hazards = append(hazards, h)
		}
	}
	return signup{Email: strings.ToLower(addr.Address), Area: area, Hazards: hazards, News: req.News}, ""
}

type verdict int

const (
	allowed verdict = iota
	overAddress
	overBudget
)

// take counts one send against the address and the day, unless either is used up.
func (l *List) take(email string) verdict {
	now := l.cfg.Now().UTC()
	key := sha256.Sum256([]byte(email))
	l.mu.Lock()
	defer l.mu.Unlock()
	if day := now.Format(time.DateOnly); day != l.day {
		l.day, l.sentToday = day, 0
	}
	recent := l.byAddress[key][:0]
	for _, t := range l.byAddress[key] {
		if now.Sub(t) < 24*time.Hour {
			recent = append(recent, t)
		}
	}
	// Forget addresses with nothing recent, so the map holds a day of sends at most.
	for k, ts := range l.byAddress {
		if k != key && len(ts) > 0 && now.Sub(ts[len(ts)-1]) >= 24*time.Hour {
			delete(l.byAddress, k)
		}
	}
	if len(recent) >= perAddress {
		l.byAddress[key] = recent
		return overAddress
	}
	if l.sentToday >= dailySends {
		l.byAddress[key] = recent
		return overBudget
	}
	l.sentToday++
	l.byAddress[key] = append(recent, now)
	return allowed
}

// confirmation is the email, in the wording approved on 2026-11-23.
func confirmation(s signup, link string) mail.Message {
	area := s.Area
	if area == "" {
		area = "Anywhere"
	}
	hazards := "All types"
	if len(s.Hazards) > 0 {
		labels := make([]string, len(s.Hazards))
		for i, h := range s.Hazards {
			labels[i] = label(h)
		}
		hazards = strings.Join(labels, ", ")
	}
	news := "No"
	if s.News {
		news = "Yes"
	}
	text := strings.Join([]string{
		"Someone asked to join the SentryAtlas waiting list for location alerts with this address. If it was you, confirm here within 48 hours:",
		"",
		link,
		"",
		"You asked for:",
		"Area: " + area,
		"Hazards: " + hazards,
		"Product news: " + news,
		"",
		"If it was not you, ignore this email. Without confirmation the address is not added to anything.",
		"",
		"SentryAtlas is not an official warning service. For warnings where you are, follow your local authorities.",
		"",
		"Questions: reply to this email.",
		"Privacy: https://sentryatlas.com/privacy/",
	}, "\n")
	return mail.Message{To: s.Email, Subject: "Confirm your place on the SentryAtlas waiting list", Text: text}
}

// label turns an event type into words: winter_storm is "Winter storm".
func label(t string) string {
	t = strings.ReplaceAll(t, "_", " ")
	return strings.ToUpper(t[:1]) + t[1:]
}

// canReceiveMail accepts a domain with mail servers, or with none listed but
// an address (mail then goes to the domain itself), and refuses a null MX.
func canReceiveMail(ctx context.Context, domain string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	mxs, err := net.DefaultResolver.LookupMX(ctx, domain)
	if err == nil {
		if len(mxs) == 1 && mxs[0].Host == "." {
			return errors.New("null MX")
		}
		return nil
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) && dnsErr.IsNotFound {
		if addrs, herr := net.DefaultResolver.LookupHost(ctx, domain); herr == nil && len(addrs) > 0 {
			return nil
		}
	}
	return err
}

func decode(w http.ResponseWriter, r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if dec.More() {
		return errors.New("more than one JSON value")
	}
	return nil
}

func reply(w http.ResponseWriter, status int, key, value string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{key: value})
}
