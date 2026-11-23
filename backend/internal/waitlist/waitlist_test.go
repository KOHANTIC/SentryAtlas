package waitlist

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/KOHANTIC/SentryAtlas/backend/internal/mail"
)

var (
	secret = []byte("test-secret-one")
	older  = []byte("test-secret-old")
	start  = time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
)

// clock is a settable time for the list under test.
type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func newList(t *testing.T) (*List, *mail.Log, *clock) {
	t.Helper()
	sender := &mail.Log{}
	c := &clock{start}
	l := New(Config{
		Mailer: sender, Secrets: [][]byte{secret}, AlertsTopic: "t-alerts", NewsTopic: "t-news",
		ConfirmURL: "https://sentryatlas.com/waitlist/confirm/", Now: c.now,
		Resolve: func(_ context.Context, domain string) error {
			if domain == "nomail.example" {
				return errors.New("no MX")
			}
			return nil
		},
	})
	return l, sender, c
}

func post(h http.HandlerFunc, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodPost, "/api/v2/waitlist", strings.NewReader(body)))
	return rec
}

// tokenFrom pulls the token out of the link in a confirmation email.
func tokenFrom(t *testing.T, m mail.Message) string {
	t.Helper()
	for _, line := range strings.Split(m.Text, "\n") {
		if strings.HasPrefix(line, "https://sentryatlas.com/waitlist/confirm/#") {
			return strings.TrimPrefix(line, "https://sentryatlas.com/waitlist/confirm/#")
		}
	}
	t.Fatalf("no link in %q", m.Text)
	return ""
}

func TestSignupSendsTheApprovedEmail(t *testing.T) {
	l, sender, _ := newList(t)
	rec := post(l.Signup, `{"email":" Person@Example.org ","area":"Sacramento, CA","hazards":["flood","winter_storm","flood"],"news":true}`)
	if rec.Code != http.StatusAccepted || !strings.Contains(rec.Body.String(), `"status":"sent"`) {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	sent := sender.Sent()
	if len(sent) != 1 {
		t.Fatalf("sent %d messages", len(sent))
	}
	m := sent[0]
	if m.To != "person@example.org" || m.Subject != "Confirm your place on the SentryAtlas waiting list" {
		t.Errorf("to %q subject %q", m.To, m.Subject)
	}
	for _, want := range []string{
		"Someone asked to join the SentryAtlas waiting list for location alerts with this address. If it was you, confirm here within 48 hours:",
		"Area: Sacramento, CA\nHazards: Flood, Winter storm\nProduct news: Yes",
		"If it was not you, ignore this email. Without confirmation the address is not added to anything.",
		"SentryAtlas is not an official warning service. For warnings where you are, follow your local authorities.",
		"Questions: reply to this email.\nPrivacy: https://sentryatlas.com/privacy/",
	} {
		if !strings.Contains(m.Text, want) {
			t.Errorf("email lacks %q:\n%s", want, m.Text)
		}
	}
	// The token sits after "#", so no request line or log ever carries it.
	if s, err := verify(tokenFrom(t, m), [][]byte{secret}, start); err != nil || s.Email != "person@example.org" || !s.News || len(s.Hazards) != 2 {
		t.Errorf("token holds %+v, %v", s, err)
	}
}

func TestSignupDefaultsInTheEmail(t *testing.T) {
	l, sender, _ := newList(t)
	if rec := post(l.Signup, `{"email":"a@example.org"}`); rec.Code != http.StatusAccepted {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if text := sender.Sent()[0].Text; !strings.Contains(text, "Area: Anywhere\nHazards: All types\nProduct news: No") {
		t.Errorf("defaults missing:\n%s", text)
	}
}

func TestSignupRefuses(t *testing.T) {
	tests := map[string]string{
		"not JSON":        `email=a@example.org`,
		"unknown field":   `{"email":"a@example.org","name":"A"}`,
		"two values":      `{"email":"a@example.org"}{"email":"b@example.org"}`,
		"no address":      `{"email":""}`,
		"bad address":     `{"email":"not an address"}`,
		"display name":    `{"email":"A <a@example.org>"}`,
		"unknown hazard":  `{"email":"a@example.org","hazards":["meteor"]}`,
		"long area":       `{"email":"a@example.org","area":"` + strings.Repeat("x", maxArea+1) + `"}`,
		"control in area": `{"email":"a@example.org","area":"a\u0007b"}`,
		"domain no mail":  `{"email":"a@nomail.example"}`,
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			l, sender, _ := newList(t)
			rec := post(l.Signup, body)
			if rec.Code != http.StatusBadRequest || len(sender.Sent()) != 0 {
				t.Errorf("%d %s, sent %d", rec.Code, rec.Body, len(sender.Sent()))
			}
		})
	}
}

func TestOffWithoutConfiguration(t *testing.T) {
	for name, cfg := range map[string]Config{
		"no mailer":   {Secrets: [][]byte{secret}, AlertsTopic: "a", NewsTopic: "n", ConfirmURL: "u"},
		"no secret":   {Mailer: &mail.Log{}, AlertsTopic: "a", NewsTopic: "n", ConfirmURL: "u"},
		"no topics":   {Mailer: &mail.Log{}, Secrets: [][]byte{secret}, ConfirmURL: "u"},
		"no page URL": {Mailer: &mail.Log{}, Secrets: [][]byte{secret}, AlertsTopic: "a", NewsTopic: "n"},
	} {
		l := New(cfg)
		if l.On() {
			t.Errorf("%s: on", name)
		}
		for _, h := range []http.HandlerFunc{l.Signup, l.Confirm} {
			if rec := post(h, `{"email":"a@example.org"}`); rec.Code != http.StatusServiceUnavailable {
				t.Errorf("%s: %d", name, rec.Code)
			}
		}
	}
}

func TestAddressLimitAnswersTheSame(t *testing.T) {
	l, sender, c := newList(t)
	for i := 0; i < perAddress+2; i++ {
		rec := post(l.Signup, `{"email":"a@example.org"}`)
		if rec.Code != http.StatusAccepted || rec.Body.String() != "{\"status\":\"sent\"}\n" {
			t.Fatalf("signup %d: %d %s", i, rec.Code, rec.Body)
		}
	}
	if n := len(sender.Sent()); n != perAddress {
		t.Errorf("sent %d, want %d", n, perAddress)
	}
	c.t = c.t.Add(24 * time.Hour)
	post(l.Signup, `{"email":"a@example.org"}`)
	if n := len(sender.Sent()); n != perAddress+1 {
		t.Errorf("a day later: sent %d, want %d", n, perAddress+1)
	}
}

func TestDailyBudget(t *testing.T) {
	l, sender, c := newList(t)
	for i := 0; i < dailySends; i++ {
		if rec := post(l.Signup, fmt.Sprintf(`{"email":"p%d@example.org"}`, i)); rec.Code != http.StatusAccepted {
			t.Fatalf("signup %d: %d", i, rec.Code)
		}
	}
	if rec := post(l.Signup, `{"email":"one-more@example.org"}`); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("over budget: %d %s", rec.Code, rec.Body)
	}
	// The budget is per UTC day.
	c.t = time.Date(2026, 11, 24, 0, 0, 1, 0, time.UTC)
	if rec := post(l.Signup, `{"email":"one-more@example.org"}`); rec.Code != http.StatusAccepted {
		t.Errorf("next day: %d", rec.Code)
	}
	if n := len(sender.Sent()); n != dailySends+1 {
		t.Errorf("sent %d", n)
	}
	if n := len(l.byAddress); n > dailySends+1 {
		t.Errorf("remembers %d addresses", n)
	}
}

func TestConfirmSavesBothConsentsApart(t *testing.T) {
	for _, news := range []bool{true, false} {
		l, sender, _ := newList(t)
		post(l.Signup, fmt.Sprintf(`{"email":"a@example.org","area":"Reno","hazards":["wildfire"],"news":%t}`, news))
		token := tokenFrom(t, sender.Sent()[0])
		for i := 0; i < 2; i++ { // confirming twice is harmless
			rec := post(l.Confirm, `{"token":"`+token+`"}`)
			if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "confirmed") {
				t.Fatalf("confirm %d: %d %s", i, rec.Code, rec.Body)
			}
		}
		cs := sender.Contacts()
		if len(cs) != 2 {
			t.Fatalf("saved %d", len(cs))
		}
		got, _ := json.Marshal(cs[0])
		want := fmt.Sprintf(`{"Email":"a@example.org","Area":"Reno","Hazards":"wildfire","Topics":{"t-alerts":true,"t-news":%t}}`, news)
		if string(got) != want {
			t.Errorf("contact %s, want %s", got, want)
		}
	}
}

func TestConfirmRefuses(t *testing.T) {
	l, sender, c := newList(t)
	post(l.Signup, `{"email":"a@example.org"}`)
	token := tokenFrom(t, sender.Sent()[0])
	payload, sig, _ := strings.Cut(token, ".")
	forged, _ := sign(signup{Email: "a@example.org", Expires: start.Add(time.Hour).Unix()}, [][]byte{[]byte("not the secret")})

	for name, body := range map[string]string{
		"tampered payload": `{"token":"` + payload + `x.` + sig + `"}`,
		"tampered sig":     `{"token":"` + payload + `.` + sig[:len(sig)-2] + `AA"}`,
		"another secret":   `{"token":"` + forged + `"}`,
		"no dot":           `{"token":"abc"}`,
		"empty":            `{"token":""}`,
		"not JSON":         `token=abc`,
	} {
		if rec := post(l.Confirm, body); rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "invalid") {
			t.Errorf("%s: %d %s", name, rec.Code, rec.Body)
		}
	}
	c.t = c.t.Add(linkLife)
	if rec := post(l.Confirm, `{"token":"`+token+`"}`); rec.Code != http.StatusGone || !strings.Contains(rec.Body.String(), "expired") {
		t.Errorf("expired: %d %s", rec.Code, rec.Body)
	}
	if n := len(sender.Contacts()); n != 0 {
		t.Errorf("saved %d contacts from bad tokens", n)
	}
}

func TestOldSecretStillVerifies(t *testing.T) {
	token, err := sign(signup{Email: "a@example.org", Expires: start.Add(time.Hour).Unix()}, [][]byte{older})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := verify(token, [][]byte{secret, older}, start); err != nil {
		t.Errorf("rotated: %v", err)
	}
	if _, err := verify(token, [][]byte{secret}, start); !errors.Is(err, errInvalid) {
		t.Errorf("old secret dropped: %v", err)
	}
}

// failing is a mailer whose provider is down.
type failing struct{ mail.Log }

func (*failing) Send(context.Context, mail.Message) error        { return errors.New("down") }
func (*failing) SaveContact(context.Context, mail.Contact) error { return errors.New("down") }

func TestProviderDown(t *testing.T) {
	l, sender, _ := newList(t)
	post(l.Signup, `{"email":"a@example.org"}`)
	token := tokenFrom(t, sender.Sent()[0])
	l.cfg.Mailer = &failing{}
	if rec := post(l.Signup, `{"email":"b@example.org"}`); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("send down: %d", rec.Code)
	}
	if rec := post(l.Confirm, `{"token":"`+token+`"}`); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("save down: %d", rec.Code)
	}
}

func TestCountsForTheReport(t *testing.T) {
	l, sender, _ := newList(t)
	post(l.Signup, `{"email":"a@example.org","news":true}`)
	post(l.Signup, `{"email":"b@example.org"}`)
	post(l.Signup, `{"email":"bad"}`)
	post(l.Confirm, `{"token":"`+tokenFrom(t, sender.Sent()[0])+`"}`)
	post(l.Confirm, `{"token":"`+tokenFrom(t, sender.Sent()[1])+`"}`)
	if sent, confirmed, news := l.takeCounts(); sent != 2 || confirmed != 2 || news != 1 {
		t.Errorf("counts %d sent, %d confirmed, %d news; want 2, 2, 1", sent, confirmed, news)
	}
	if sent, confirmed, news := l.takeCounts(); sent+confirmed+news != 0 {
		t.Errorf("counts not reset: %d %d %d", sent, confirmed, news)
	}
}
