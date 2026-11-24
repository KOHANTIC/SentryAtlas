package event

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

var docsDir = filepath.Join("..", "..", "..", "docs", "api", "v2")

// TestExampleSnapshotFollowsTheModel holds the published example to the Go
// types: no field the types lack, and every event valid.
func TestExampleSnapshotFollowsTheModel(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(docsDir, "snapshot.example.json"))
	if err != nil {
		t.Fatal(err)
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	var s Snapshot
	if err := d.Decode(&s); err != nil {
		t.Fatalf("example does not decode into Snapshot: %v", err)
	}
	if len(s.Events) == 0 {
		t.Fatal("example has no events")
	}
	seen := map[string]bool{}
	for _, e := range s.Events {
		if err := e.Validate(); err != nil {
			t.Errorf("example event %s: %v", e.ID, err)
		}
		seen[e.Source] = true
	}
	for _, src := range []string{SourceUSGS, SourceEONET, SourceNOAA, SourceGDACS} {
		if !seen[src] {
			t.Errorf("example has no %s event", src)
		}
	}
}

// TestSchemaMatchesTheGoTypes keeps docs/api/v2/schema.json and the Go types
// in step: every object in the schema has exactly the fields of its Go type,
// and requires exactly the ones the Go type never leaves out.
func TestSchemaMatchesTheGoTypes(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(docsDir, "schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Defs map[string]struct {
			Properties map[string]json.RawMessage `json:"properties"`
			Required   []string                   `json:"required"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("schema is not JSON: %v", err)
	}
	types := map[string]any{
		"Snapshot": Snapshot{}, "SourceStatus": SourceStatus{}, "Event": Event{},
		"Severity": Severity{}, "Magnitude": Magnitude{}, "Time": Time{},
		"Location": Location{}, "Geometry": Geometry{}, "Details": Details{},
		"USGS": USGS{}, "EONET": EONET{}, "NWS": NWS{}, "GDACS": GDACS{}, "Link": Link{},
		"Tsunami": Tsunami{}, "Quake": Quake{},
	}
	for name, v := range types {
		def, ok := schema.Defs[name]
		if !ok {
			t.Errorf("schema has no $defs/%s", name)
			continue
		}
		all, required := jsonFields(reflect.TypeOf(v))
		var props []string
		for p := range def.Properties {
			props = append(props, p)
		}
		sort.Strings(props)
		req := append([]string(nil), def.Required...)
		sort.Strings(req)
		if !reflect.DeepEqual(props, all) {
			t.Errorf("%s: schema properties %v, Go fields %v", name, props, all)
		}
		if len(req) == 0 {
			req = nil
		}
		if !reflect.DeepEqual(req, required) {
			t.Errorf("%s: schema requires %v, Go always writes %v", name, req, required)
		}
	}
}

// jsonFields lists a struct's JSON names, sorted, and those without omitempty.
func jsonFields(typ reflect.Type) (all, required []string) {
	for i := 0; i < typ.NumField(); i++ {
		tag := typ.Field(i).Tag.Get("json")
		name, opts, _ := strings.Cut(tag, ",")
		if name == "" || name == "-" {
			continue
		}
		all = append(all, name)
		if !strings.Contains(opts, "omitempty") {
			required = append(required, name)
		}
	}
	sort.Strings(all)
	sort.Strings(required)
	return all, required
}

func TestValidateRejectsWhatBreaksTheRules(t *testing.T) {
	t.Parallel()
	utc := time.Date(2026, 11, 22, 20, 0, 0, 0, time.UTC)
	good := func() Event {
		center := Point{-147.79, 60.301}
		return Event{
			ID: "usgs-ak1", Source: SourceUSGS, Type: "earthquake", Title: "M 4.1",
			Time:     Time{Started: utc, Updated: utc},
			Location: &Location{Center: &center, Geometry: &Geometry{Type: "Point", Coordinates: json.RawMessage(`[-147.79, 60.301]`)}},
			Details:  Details{USGS: &USGS{Kind: "earthquake", Status: "reviewed"}},
		}
	}
	if err := func() *Event { e := good(); return &e }().Validate(); err != nil {
		t.Fatalf("the good event is invalid: %v", err)
	}

	local := time.FixedZone("AKST", -9*3600)
	tooMany := make([]Point, MaxVertices+1)
	for i := range tooMany {
		tooMany[i] = Point{float64(i%180) / 2, 10}
	}
	tooManyJSON, _ := json.Marshal(tooMany)

	cases := []struct {
		name   string
		break_ func(e *Event)
		want   string
	}{
		{"unknown source", func(e *Event) { e.Source = "nasa" }, "source"},
		{"id without the source prefix", func(e *Event) { e.ID = "ak1" }, "id"},
		{"id with nothing after the prefix", func(e *Event) { e.ID = "usgs-" }, "id"},
		{"unknown type", func(e *Event) { e.Type = "meteor" }, "type"},
		{"empty title", func(e *Event) { e.Title = "" }, "title"},
		{"severity off the scale", func(e *Event) { e.Severity = &Severity{Level: "red", SourceLevel: "red"} }, "severity"},
		{"magnitude without a unit", func(e *Event) { e.Magnitude = &Magnitude{Value: 4.1} }, "unit"},
		{"no start", func(e *Event) { e.Time.Started = time.Time{} }, "started"},
		{"a time that is not UTC", func(e *Event) { e.Time.Updated = utc.In(local) }, "UTC"},
		{"an expiry that is not UTC", func(e *Event) { x := utc.In(local); e.Time.Expires = &x }, "UTC"},
		{"geometry without a center", func(e *Event) { e.Location.Center = nil }, "center"},
		{"empty location", func(e *Event) { e.Location = &Location{} }, "empty"},
		{"center off the globe", func(e *Event) { e.Location.Center = &Point{200, 0} }, "longitude"},
		{"center with too many decimals", func(e *Event) { e.Location.Center = &Point{-147.790001, 60.301} }, "decimals"},
		{"unknown geometry type", func(e *Event) { e.Location.Geometry.Type = "Circle" }, "Circle"},
		{"geometry that does not parse", func(e *Event) { e.Location.Geometry.Coordinates = json.RawMessage(`"oops"`) }, "parse"},
		{"too many vertices", func(e *Event) {
			e.Location.Geometry = &Geometry{Type: "LineString", Coordinates: tooManyJSON}
		}, "points, more than"},
		{"details for another source", func(e *Event) { e.Details = Details{GDACS: &GDACS{AlertLevel: "Red"}} }, "details"},
		{"details for two sources", func(e *Event) { e.Details.EONET = &EONET{} }, "details"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			e := good()
			c.break_(&e)
			err := e.Validate()
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("Validate = %v, want an error mentioning %q", err, c.want)
			}
		})
	}
}

func TestAZoneOnlyLocationIsValid(t *testing.T) {
	t.Parallel()
	e := Event{
		ID: "noaa-urn:1", Source: SourceNOAA, Type: "weather", Title: "Small Craft Advisory",
		Time:     Time{Started: time.Date(2026, 11, 22, 20, 12, 0, 0, time.UTC)},
		Location: &Location{Zones: []string{"LMZ844"}},
		Details:  Details{NOAA: &NWS{Event: "Small Craft Advisory"}},
	}
	if err := e.Validate(); err != nil {
		t.Errorf("a zone-only alert is invalid: %v", err)
	}
}

func TestRoundCoordinate(t *testing.T) {
	t.Parallel()
	for in, want := range map[float64]float64{-147.7900049: -147.79, 60.301004: 60.30100, 1.234566: 1.23457, -0.000004: 0} {
		if got := RoundCoordinate(in); got != want {
			t.Errorf("RoundCoordinate(%v) = %v, want %v", in, got, want)
		}
	}
}
