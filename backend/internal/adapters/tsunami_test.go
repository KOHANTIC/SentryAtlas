package adapters

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/KOHANTIC/SentryAtlas/backend/internal/event"
)

// The fixtures are both centers' live feeds and CAP messages of 2026-11-23:
// information statements on an M6.5 near Vanuatu.
const (
	ptwcFeed = "https://www.tsunami.gov/events/xml/PHEBAtom.xml"
	ntwcFeed = "https://www.tsunami.gov/events/xml/PAAQAtom.xml"
	ptwcCAP  = "https://www.tsunami.gov/events/PHEB/2026/11/23/26327000/1/WEGM42/PHEBCAP.xml"
	ntwcCAP  = "https://www.tsunami.gov/events/PAAQ/2026/11/23/toxos9/1/WEAK53/PAAQCAP.xml"
)

// beforeExpiry is inside both fixtures' hour of validity.
var beforeExpiry = time.Date(2026, 11, 23, 3, 0, 0, 0, time.UTC)

func tsunamiFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "tsunami", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(data)
}

// asWarning turns an information statement into a warning, as a center
// would issue one, with a later bulletin number.
func asWarning(capXML, code string) string {
	r := strings.NewReplacer(
		"<event>Tsunami Information</event>", "<event>Tsunami Warning</event>",
		"<severity>Minor</severity>", "<severity>Extreme</severity>",
		"<identifier>"+code+"-1-", "<identifier>"+code+"-2-",
	)
	out := r.Replace(capXML)
	start, end := strings.Index(out, "<instruction>"), strings.Index(out, "</instruction>")
	return out[:start] + "<instruction>Move to high ground now.\n\nStay away from the shore.</instruction>" + out[end+len("</instruction>"):]
}

// tsunamiAt serves each URL from the given body, and 404 for anything else.
type tsunamiAt map[string]string

func (m tsunamiAt) RoundTrip(r *http.Request) (*http.Response, error) {
	body, ok := m[r.URL.String()]
	code := http.StatusOK
	if !ok {
		code = http.StatusNotFound
	}
	if body == "500" {
		code = http.StatusInternalServerError
	}
	return &http.Response{StatusCode: code, Body: io.NopCloser(bytes.NewBufferString(body)), Header: http.Header{}, Request: r}, nil
}

func tsunamiAdapter(responses tsunamiAt, now time.Time) *TsunamiAdapter {
	a := NewTsunamiAdapter(&http.Client{Transport: responses}, "test")
	a.now = func() time.Time { return now }
	return a
}

func TestTsunamiInformationIsNoEvent(t *testing.T) {
	a := tsunamiAdapter(tsunamiAt{
		ptwcFeed: tsunamiFixture(t, "PHEBAtom.xml"), ptwcCAP: tsunamiFixture(t, "PHEBCAP.xml"),
		ntwcFeed: tsunamiFixture(t, "PAAQAtom.xml"), ntwcCAP: tsunamiFixture(t, "PAAQCAP.xml"),
	}, beforeExpiry)
	events, err := a.FetchV2(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Errorf("got %d events, want none: an information statement says no tsunami is expected", len(events))
	}
}

func TestTsunamiWarning(t *testing.T) {
	a := tsunamiAdapter(tsunamiAt{
		ptwcFeed: tsunamiFixture(t, "PHEBAtom.xml"), ptwcCAP: asWarning(tsunamiFixture(t, "PHEBCAP.xml"), "PHEB"),
		ntwcFeed: tsunamiFixture(t, "PAAQAtom.xml"), ntwcCAP: tsunamiFixture(t, "PAAQCAP.xml"),
	}, beforeExpiry)
	events, err := a.FetchV2(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want the one warning", len(events))
	}
	e := events[0]
	if err := e.Validate(); err != nil {
		t.Fatalf("the event is not valid v2: %v", err)
	}
	// The bulletin number (2) is left out of the id, so later bulletins update one event.
	if e.ID != "tsunami-PHEB-26327000" || e.Source != event.SourceTsunami || e.Type != "tsunami" {
		t.Errorf("id, source, type = %q, %q, %q", e.ID, e.Source, e.Type)
	}
	if e.Title != "Tsunami Warning: VANUATU REGION" {
		t.Errorf("Title = %q", e.Title)
	}
	if e.Severity == nil || e.Severity.Level != event.SeverityExtreme || e.Severity.SourceLevel != "Tsunami Warning" {
		t.Errorf("Severity = %+v, want extreme from the bulletin's Extreme", e.Severity)
	}
	if e.Location == nil || e.Location.Center == nil || *e.Location.Center != (event.Point{172.279, -13.546}) {
		t.Errorf("Location = %+v, want the epicenter", e.Location)
	}
	if want := time.Date(2026, 11, 23, 2, 40, 57, 0, time.UTC); !e.Time.Started.Equal(want) {
		t.Errorf("Started = %v, want the quake's origin %v", e.Time.Started, want)
	}
	if want := time.Date(2026, 11, 23, 2, 49, 16, 0, time.UTC); !e.Time.Updated.Equal(want) {
		t.Errorf("Updated = %v, want the bulletin's sent time %v", e.Time.Updated, want)
	}
	if e.Time.Expires == nil || !e.Time.Expires.Equal(time.Date(2026, 11, 23, 3, 49, 16, 0, time.UTC)) {
		t.Errorf("Expires = %v", e.Time.Expires)
	}
	if !strings.HasPrefix(e.URL, "https://www.tsunami.gov/events/PHEB/") {
		t.Errorf("URL = %q, want the bulletin on tsunami.gov over https", e.URL)
	}
	d := e.Details.Tsunami
	if d == nil {
		t.Fatal("no tsunami details")
	}
	if d.Center != "PTWC" || d.Event != "Tsunami Warning" || d.MessageType != "Alert" || d.Area != "VANUATU REGION" {
		t.Errorf("details = %+v", d)
	}
	if d.Instruction != "Move to high ground now.\n\nStay away from the shore." {
		t.Errorf("Instruction = %q, want the bulletin's words exactly", d.Instruction)
	}
	q := d.Quake
	if q.Magnitude == nil || *q.Magnitude != 6.5 || q.MagnitudeType != "Mwp" || q.DepthKm == nil || *q.DepthKm != 614 || q.Origin == nil {
		t.Errorf("Quake = %+v", q)
	}
}

func TestTsunamiNTWCWarning(t *testing.T) {
	a := tsunamiAdapter(tsunamiAt{
		ptwcFeed: tsunamiFixture(t, "PHEBAtom.xml"), ptwcCAP: tsunamiFixture(t, "PHEBCAP.xml"),
		ntwcFeed: tsunamiFixture(t, "PAAQAtom.xml"), ntwcCAP: asWarning(tsunamiFixture(t, "PAAQCAP.xml"), "PAAQ"),
	}, beforeExpiry)
	events, err := a.FetchV2(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].ID != "tsunami-PAAQ-toxos9" || events[0].Details.Tsunami.Center != "NTWC" {
		t.Fatalf("events = %+v, want NTWC's warning", events)
	}
}

func TestTsunamiLeavesOutWhatIsNotALiveThreat(t *testing.T) {
	warning := asWarning(tsunamiFixture(t, "PHEBCAP.xml"), "PHEB")
	cases := map[string]struct {
		cap string
		now time.Time
	}{
		"a cancellation":   {strings.Replace(warning, "<msgType>Alert</msgType>", "<msgType>Cancel</msgType>", 1), beforeExpiry},
		"a test message":   {strings.Replace(warning, "<status>Actual</status>", "<status>Test</status>", 1), beforeExpiry},
		"an expired one":   {warning, time.Date(2026, 11, 23, 4, 0, 0, 0, time.UTC)},
		"a cancel product": {strings.Replace(warning, "<event>Tsunami Warning</event>", "<event>Tsunami Warning Cancellation</event>", 1), beforeExpiry},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			a := tsunamiAdapter(tsunamiAt{
				ptwcFeed: tsunamiFixture(t, "PHEBAtom.xml"), ptwcCAP: tc.cap,
				ntwcFeed: tsunamiFixture(t, "PAAQAtom.xml"), ntwcCAP: tsunamiFixture(t, "PAAQCAP.xml"),
			}, tc.now)
			events, err := a.FetchV2(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if len(events) != 0 {
				t.Errorf("got %+v, want no event", events)
			}
		})
	}
}

func TestTsunamiCenterFailingFailsTheFetch(t *testing.T) {
	a := tsunamiAdapter(tsunamiAt{
		ptwcFeed: "500",
		ntwcFeed: tsunamiFixture(t, "PAAQAtom.xml"), ntwcCAP: tsunamiFixture(t, "PAAQCAP.xml"),
	}, beforeExpiry)
	if _, err := a.FetchV2(context.Background()); err == nil {
		t.Error("want an error when a center does not answer, so the last good copy is kept")
	}
}

func TestTsunamiThreatProducts(t *testing.T) {
	for product, want := range map[string]bool{
		"Tsunami Warning": true, "Tsunami Advisory": true, "Tsunami Watch": true, "Tsunami Threat Message": true,
		"Tsunami Information": false, "Tsunami Information Statement": false, "Tsunami Warning Cancellation": false, "Earthquake Warning": false,
	} {
		if got := isTsunamiThreat(product); got != want {
			t.Errorf("isTsunamiThreat(%q) = %v, want %v", product, got, want)
		}
	}
}
