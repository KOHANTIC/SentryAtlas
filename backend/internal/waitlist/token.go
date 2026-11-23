package waitlist

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// signup is what a confirmation link carries: the whole request, so the
// server keeps nothing between the signup and the confirmation.
type signup struct {
	Email   string   `json:"e"`
	Area    string   `json:"a,omitempty"`
	Hazards []string `json:"h,omitempty"`
	News    bool     `json:"n,omitempty"`
	Expires int64    `json:"x"`
}

var (
	errInvalid = errors.New("invalid token")
	errExpired = errors.New("expired token")
)

var b64 = base64.RawURLEncoding

// sign returns payload.signature, both base64url, signed with the first secret.
func sign(s signup, secrets [][]byte) (string, error) {
	payload, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	p := b64.EncodeToString(payload)
	return p + "." + b64.EncodeToString(mac(secrets[0], p)), nil
}

// verify accepts a token signed with any of the secrets, so a new secret can
// be put first while links signed with the old one still work until they expire.
func verify(token string, secrets [][]byte, now time.Time) (signup, error) {
	p, sig, ok := strings.Cut(token, ".")
	if !ok {
		return signup{}, errInvalid
	}
	got, err := b64.DecodeString(sig)
	if err != nil {
		return signup{}, errInvalid
	}
	signed := false
	for _, secret := range secrets {
		if hmac.Equal(got, mac(secret, p)) {
			signed = true
			break
		}
	}
	if !signed {
		return signup{}, errInvalid
	}
	payload, err := b64.DecodeString(p)
	if err != nil {
		return signup{}, errInvalid
	}
	var s signup
	if err := json.Unmarshal(payload, &s); err != nil || s.Email == "" {
		return signup{}, errInvalid
	}
	if now.Unix() >= s.Expires {
		return signup{}, errExpired
	}
	return s, nil
}

func mac(secret []byte, payload string) []byte {
	h := hmac.New(sha256.New, secret)
	_, _ = h.Write([]byte(payload)) // a hash never fails to write
	return h.Sum(nil)
}
