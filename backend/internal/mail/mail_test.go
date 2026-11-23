package mail

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const (
	key = "re_test_secret_key"
	to  = "person@example.org"
)

func env(vars map[string]string) func(string) string {
	return func(k string) string { return vars[k] }
}

func TestFromEnv(t *testing.T) {
	ok := map[string]string{"RESEND_API_KEY": key, "MAIL_FROM": "SentryAtlas <hello@mail.example.org>", "MAIL_REPLY_TO": "contact@example.org"}
	with := func(k, v string) map[string]string {
		m := map[string]string{}
		for a, b := range ok {
			m[a] = b
		}
		m[k] = v
		return m
	}
	tests := []struct {
		name     string
		vars     map[string]string
		wantName string
		wantErr  string
	}{
		{"no key is off", map[string]string{"MAIL_FROM": "a@example.org"}, "off", ""},
		{"key and addresses", ok, "resend", ""},
		{"resend named", with("MAIL_SENDER", "resend"), "resend", ""},
		{"log sender", map[string]string{"MAIL_SENDER": "log"}, "log", ""},
		{"unknown sender", with("MAIL_SENDER", "smtp"), "off", "MAIL_SENDER"},
		{"no from", with("MAIL_FROM", ""), "off", "MAIL_FROM is required"},
		{"bad from", with("MAIL_FROM", "not an address"), "off", "MAIL_FROM is not"},
		{"no reply-to", with("MAIL_REPLY_TO", ""), "off", "MAIL_REPLY_TO is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, name, err := FromEnv(env(tt.vars), http.DefaultClient, nil)
			if name != tt.wantName {
				t.Errorf("name %q, want %q", name, tt.wantName)
			}
			if (m == nil) != (tt.wantName == "off") {
				t.Errorf("mailer %v with name %q", m, name)
			}
			if tt.wantErr == "" && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error %v, want one containing %q", err, tt.wantErr)
				}
				if strings.Contains(err.Error(), key) {
					t.Errorf("error carries the key: %v", err)
				}
			}
		})
	}
}

func TestResendSends(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method %s", r.Method)
		}
		if a := r.Header.Get("Authorization"); a != "Bearer "+key {
			t.Errorf("Authorization %q", a)
		}
		if c := r.Header.Get("Content-Type"); c != "application/json" {
			t.Errorf("Content-Type %q", c)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatal(err)
		}
		_, _ = io.WriteString(w, `{"id":"49a3999c-0ce1-4ea6-ab68-afcd6dc2e794"}`)
	}))
	defer srv.Close()

	r := &Resend{Key: key, From: "SentryAtlas <hello@mail.example.org>", ReplyTo: "contact@example.org", Client: srv.Client(), Base: srv.URL}
	if err := r.Send(context.Background(), Message{To: to, Subject: "Confirm", Text: "Open this link"}); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"from": "SentryAtlas <hello@mail.example.org>", "to": []any{to}, "subject": "Confirm",
		"text": "Open this link", "reply_to": "contact@example.org",
	}
	if len(got) != len(want) {
		t.Errorf("body has %d fields, want %d (plain text only, no html): %v", len(got), len(want), got)
	}
	for k, v := range want {
		gj, _ := json.Marshal(got[k])
		wj, _ := json.Marshal(v)
		if string(gj) != string(wj) {
			t.Errorf("%s = %s, want %s", k, gj, wj)
		}
	}
}

func TestResendErrorKeepsAddressAndKeyOut(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = io.WriteString(w, `{"statusCode":422,"name":"validation_error","message":"Invalid `+"`to`"+` field: `+to+`"}`)
	}))
	defer srv.Close()

	r := &Resend{Key: key, From: "a@example.org", ReplyTo: "b@example.org", Client: srv.Client(), Base: srv.URL}
	err := r.Send(context.Background(), Message{To: to, Subject: "s", Text: "t"})
	if err == nil {
		t.Fatal("no error for a 422")
	}
	if !strings.Contains(err.Error(), "422") || !strings.Contains(err.Error(), "validation_error") {
		t.Errorf("error %q lacks the status or the error name", err)
	}
	if strings.Contains(err.Error(), to) || strings.Contains(err.Error(), key) {
		t.Errorf("error %q carries the address or the key", err)
	}
}

func TestResendStopsWithContext(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { <-release }))
	defer srv.Close()
	defer close(release)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	r := &Resend{Key: key, From: "a@example.org", ReplyTo: "b@example.org", Client: srv.Client(), Base: srv.URL}
	if err := r.Send(ctx, Message{To: to}); err == nil || strings.Contains(err.Error(), to) {
		t.Fatalf("error %v, want a timeout without the address", err)
	}
}

func TestLogKeepsMessages(t *testing.T) {
	var out strings.Builder
	l := &Log{Logger: slog.New(slog.NewTextHandler(&out, nil))}
	for _, s := range []string{"one", "two"} {
		if err := l.Send(context.Background(), Message{To: to, Subject: s, Text: "body " + s}); err != nil {
			t.Fatal(err)
		}
	}
	sent := l.Sent()
	if len(sent) != 2 || sent[0].Subject != "one" || sent[1].Text != "body two" {
		t.Errorf("sent %+v", sent)
	}
	if !strings.Contains(out.String(), "body two") {
		t.Errorf("log sender did not print the message: %s", out.String())
	}
}

// contactServer answers create with createStatus and records every call.
func contactServer(t *testing.T, createStatus int) (*httptest.Server, *[]string, *[]any) {
	t.Helper()
	var calls []string
	var bodies []any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.EscapedPath())
		var b any
		if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
			t.Errorf("%s %s: %v", r.Method, r.URL.Path, err)
		}
		bodies = append(bodies, b)
		if r.Method == http.MethodPost && r.URL.Path == "/contacts" && createStatus != http.StatusCreated {
			w.WriteHeader(createStatus)
			_, _ = io.WriteString(w, `{"name":"validation_error","message":"Contact `+to+` already exists"}`)
			return
		}
		_, _ = io.WriteString(w, `{"object":"contact","id":"c1"}`)
	}))
	t.Cleanup(srv.Close)
	return srv, &calls, &bodies
}

func TestSaveContactCreatesThenSetsTopics(t *testing.T) {
	srv, calls, bodies := contactServer(t, http.StatusCreated)
	r := &Resend{Key: key, Client: srv.Client(), Base: srv.URL}
	c := Contact{Email: to, Area: "Sacramento", Hazards: "flood,wildfire", Topics: map[string]bool{"t-news": false, "t-alerts": true}}
	if err := r.SaveContact(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	want := []string{"POST /contacts", "PATCH /contacts/person@example.org/topics"}
	if strings.Join(*calls, "|") != strings.Join(want, "|") {
		t.Fatalf("calls %v, want %v", *calls, want)
	}
	created, _ := json.Marshal((*bodies)[0])
	if string(created) != `{"email":"person@example.org","properties":{"area":"Sacramento","hazards":"flood,wildfire"},"unsubscribed":false}` {
		t.Errorf("create body %s", created)
	}
	topics, _ := json.Marshal((*bodies)[1])
	if string(topics) != `[{"id":"t-alerts","subscription":"opt_in"},{"id":"t-news","subscription":"opt_out"}]` {
		t.Errorf("topics body %s", topics)
	}
}

func TestSaveContactUpdatesAnExistingOne(t *testing.T) {
	srv, calls, _ := contactServer(t, http.StatusUnprocessableEntity)
	r := &Resend{Key: key, Client: srv.Client(), Base: srv.URL}
	if err := r.SaveContact(context.Background(), Contact{Email: to, Topics: map[string]bool{"t": true}}); err != nil {
		t.Fatal(err)
	}
	want := []string{"POST /contacts", "PATCH /contacts/person@example.org", "PATCH /contacts/person@example.org/topics"}
	if strings.Join(*calls, "|") != strings.Join(want, "|") {
		t.Fatalf("calls %v, want %v", *calls, want)
	}
}

func TestSaveContactStopsOnOtherFailures(t *testing.T) {
	for _, status := range []int{http.StatusTooManyRequests, http.StatusUnauthorized, http.StatusInternalServerError} {
		srv, calls, _ := contactServer(t, status)
		r := &Resend{Key: key, Client: srv.Client(), Base: srv.URL}
		err := r.SaveContact(context.Background(), Contact{Email: to, Topics: map[string]bool{"t": true}})
		if err == nil || len(*calls) != 1 {
			t.Errorf("status %d: error %v after calls %v, want one call and an error", status, err, *calls)
		}
		if err != nil && (strings.Contains(err.Error(), to) || strings.Contains(err.Error(), key)) {
			t.Errorf("status %d: error %q carries the address or the key", status, err)
		}
	}
}

func TestResendUnreachableKeepsAddressOut(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	base := srv.URL
	srv.Close()
	r := &Resend{Key: key, Client: http.DefaultClient, Base: base}
	err := r.SaveContact(context.Background(), Contact{Email: to})
	if err == nil || strings.Contains(err.Error(), to) {
		t.Fatalf("error %v, want one without the address", err)
	}
}
