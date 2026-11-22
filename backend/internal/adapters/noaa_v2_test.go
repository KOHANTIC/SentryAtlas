package adapters

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"
)

// nwsMessage builds one alert message as the NWS API sends it.
func nwsMessage(id, msgType, sent, vtec string, zones []string, refs ...string) string {
	var r []string
	for _, ref := range refs {
		parts := strings.SplitN(ref, "@", 2)
		r = append(r, `{"identifier":"`+parts[0]+`","sent":"`+parts[1]+`"}`)
	}
	var z []string
	for _, zone := range zones {
		z = append(z, `"https://api.weather.gov/zones/forecast/`+zone+`"`)
	}
	params := `{}`
	if vtec != "" {
		params = `{"VTEC":["` + vtec + `"]}`
	}
	return `{"geometry":null,"properties":{"id":"` + id + `","event":"Small Craft Advisory","headline":"Small Craft Advisory","severity":"Minor","urgency":"Expected","certainty":"Likely","messageType":"` + msgType + `","sent":"` + sent + `","onset":"` + sent + `","expires":"2026-11-23T04:15:00-08:00","areaDesc":"` + strings.Join(zones, " and ") + `","senderName":"NWS Seattle WA","affectedZones":[` + strings.Join(z, ",") + `],"references":[` + strings.Join(r, ",") + `],"parameters":` + params + `}}`
}

// The pieces of one alert, its updates, and a cancelled alert, as the API
// lists them at one moment.
func TestNWSMergesTheMessagesOfOneAlert(t *testing.T) {
	t.Parallel()
	const vtec = "/O.EXT.KSEW.SC.Y.0152.000000T0000Z-261123T1500Z/"
	body := `{"features":[` + strings.Join([]string{
		// Two updates of one alert, issued together for different zones,
		// both pointing back at the alert's two first messages.
		nwsMessage("urn:upd.002", "Update", "2026-11-22T09:18:00-08:00", vtec, []string{"PZZ131", "PZZ132"}, "urn:first.002@2026-11-22T03:00:00-08:00"),
		nwsMessage("urn:upd.001", "Update", "2026-11-22T09:18:00-08:00", vtec, []string{"PZZ130"}, "urn:first.001@2026-11-22T03:00:00-08:00"),
		// A new alert of its own.
		nwsMessage("urn:other.001", "Alert", "2026-11-22T10:00:00-08:00", "/O.NEW.KSEW.GL.W.0020.261122T1800Z-261123T1800Z/", []string{"PZZ150"}),
		// A cancelled alert.
		nwsMessage("urn:gone.001", "Cancel", "2026-11-22T10:05:00-08:00", "/O.CAN.KSEW.SC.Y.0151.000000T0000Z-261122T1800Z/", []string{"PZZ160"}, "urn:gone.first@2026-11-22T02:00:00-08:00"),
	}, ",") + `]}`
	a := newTestNOAA(t, serveRaw(t, http.StatusOK, body))

	events, err := a.FetchV2(context.Background())
	if err != nil {
		t.Fatalf("FetchV2: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("got %d events, want 2: one merged alert and one new one, the cancelled one left out", len(events))
	}

	merged := events[0]
	if merged.ID != "noaa-urn:first.001" {
		t.Errorf("merged id = %q, want the alert's first message, noaa-urn:first.001, so updates keep it", merged.ID)
	}
	if want := []string{"PZZ130", "PZZ131", "PZZ132"}; merged.Location == nil || !slices.Equal(merged.Location.Zones, want) {
		t.Errorf("merged zones = %+v, want %v from both pieces", merged.Location, want)
	}
	if merged.Location.Center != nil {
		t.Errorf("a zone-only alert has center %v; zone shapes arrive with #25", merged.Location.Center)
	}
	if want := time.Date(2026, 11, 22, 17, 18, 0, 0, time.UTC); !merged.Time.Started.Equal(want) || merged.Time.Started.Location() != time.UTC {
		t.Errorf("started = %v, want %v in UTC", merged.Time.Started, want)
	}
	if merged.Time.Expires == nil || merged.Time.Expires.Location() != time.UTC {
		t.Errorf("expires = %v, want a UTC time", merged.Time.Expires)
	}
	if d := merged.Details.NOAA; d.MessageType != "Update" || d.Event != "Small Craft Advisory" {
		t.Errorf("details = %+v", d)
	}
	if merged.Severity == nil || merged.Severity.Level != "minor" || merged.Severity.SourceLevel != "Minor" {
		t.Errorf("severity = %+v, want minor from Minor", merged.Severity)
	}
	if events[1].ID != "noaa-urn:other.001" {
		t.Errorf("second id = %q, want the new alert's own id", events[1].ID)
	}
	for _, e := range events {
		if err := e.Validate(); err != nil {
			t.Errorf("%s: %v", e.ID, err)
		}
	}
}

func TestVTECEventKey(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"/O.CON.KDLH.SC.Y.0140.261122T2200Z-261123T0400Z/": "KDLH.SC.Y.0140",
		"/O.EXT.PAJK.SC.Y.0386.261123T0000Z-261123T2100Z/": "PAJK.SC.Y.0386",
		"not vtec":   "",
		"":           "",
		"/O.CON.KX/": "",
	} {
		if got := vtecEventKey(in); got != want {
			t.Errorf("vtecEventKey(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestNWSV2FromTheRealAlerts maps the recorded alerts onto v2.
func TestNWSV2FromTheRealAlerts(t *testing.T) {
	t.Parallel()
	a := newTestNOAA(t, serveFixture(t, "live/noaa.json", nil))
	events, err := a.FetchV2(context.Background())
	if err != nil {
		t.Fatalf("FetchV2: %v", err)
	}
	var polygons, zoneOnly int
	for _, e := range events {
		if err := e.Validate(); err != nil {
			t.Errorf("%s: %v", e.ID, err)
		}
		if e.Location == nil || len(e.Location.Zones) == 0 {
			t.Errorf("%s has no zones; every recorded alert names some", e.ID)
			continue
		}
		if e.Location.Geometry != nil {
			polygons++
		} else {
			zoneOnly++
		}
		if e.Time.Expires == nil {
			t.Errorf("%s has no expiry; every recorded alert carries one", e.ID)
		}
	}
	if polygons == 0 || zoneOnly == 0 {
		t.Errorf("polygons %d, zone-only %d: the recording holds both", polygons, zoneOnly)
	}
}
