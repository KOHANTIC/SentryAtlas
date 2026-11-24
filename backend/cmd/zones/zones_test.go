package main

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"
)

// le and be write v in little- or big-endian order to a buffer, which
// cannot fail.
func le(b *bytes.Buffer, v any) {
	if err := binary.Write(b, binary.LittleEndian, v); err != nil {
		panic(err)
	}
}

func be(b *bytes.Buffer, v any) {
	if err := binary.Write(b, binary.BigEndian, v); err != nil {
		panic(err)
	}
}

// dbf builds a dBASE table of text fields; a row starting "*" is deleted.
func dbf(fields []string, widths []int, rows [][]string) []byte {
	var b bytes.Buffer
	recLen := 1
	for _, w := range widths {
		recLen += w
	}
	header := make([]byte, 32)
	header[0] = 3
	binary.LittleEndian.PutUint32(header[4:], uint32(len(rows)))
	binary.LittleEndian.PutUint16(header[8:], uint16(32+32*len(fields)+1))
	binary.LittleEndian.PutUint16(header[10:], uint16(recLen))
	b.Write(header)
	for i, f := range fields {
		d := make([]byte, 32)
		copy(d, f)
		d[11] = 'C'
		d[16] = byte(widths[i])
		b.Write(d)
	}
	b.WriteByte(0x0D)
	for _, r := range rows {
		flag := byte(' ')
		if len(r) > 0 && r[0] == "*" {
			flag, r = '*', r[1:]
		}
		b.WriteByte(flag)
		for i, v := range r {
			cell := bytes.Repeat([]byte(" "), widths[i])
			copy(cell, v)
			b.Write(cell)
		}
	}
	return b.Bytes()
}

// shp builds a polygon shapefile; a nil record is a null shape.
func shp(records [][][][2]float64) []byte {
	var b bytes.Buffer
	b.Write(make([]byte, 100))
	for n, rings := range records {
		var c bytes.Buffer
		if rings == nil {
			le(&c, int32(0))
		} else {
			points := 0
			for _, r := range rings {
				points += len(r)
			}
			le(&c, int32(5))
			c.Write(make([]byte, 32))
			le(&c, int32(len(rings)))
			le(&c, int32(points))
			start := 0
			for _, r := range rings {
				le(&c, int32(start))
				start += len(r)
			}
			for _, r := range rings {
				for _, p := range r {
					le(&c, p)
				}
			}
		}
		be(&b, int32(n+1))
		be(&b, int32(c.Len()/2))
		b.Write(c.Bytes())
	}
	return b.Bytes()
}

// square is a clockwise ring, as shapefiles draw outer rings.
func square(x, y, side float64) [][2]float64 {
	return [][2]float64{{x, y}, {x, y + side}, {x + side, y + side}, {x + side, y}, {x, y}}
}

func TestReadDBFKeepsRowsInLine(t *testing.T) {
	t.Parallel()
	rows, err := readDBF(dbf([]string{"STATE", "ZONE"}, []int{2, 3}, [][]string{{"AL", "019"}, {"*", "XX", "000"}, {"WA", "001"}}))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 || rows[0]["STATE"] != "AL" || rows[0]["ZONE"] != "019" || len(rows[1]) != 0 || rows[2]["ZONE"] != "001" {
		t.Errorf("rows = %v, want AL 019, an empty deleted row, WA 001", rows)
	}
	if _, err := readDBF([]byte("short")); err == nil {
		t.Error("a truncated table was read")
	}
}

func TestReadSHP(t *testing.T) {
	t.Parallel()
	got, err := readSHP(shp([][][][2]float64{{square(0, 0, 1), square(5, 5, 1)}, nil}))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || len(got[0]) != 2 || got[0][1][2] != [2]float64{6, 6} || got[1] != nil {
		t.Errorf("records = %v", got)
	}
	if _, err := readSHP(shp([][][][2]float64{{square(0, 0, 1)}})[:120]); err == nil {
		t.Error("a truncated file was read")
	}
}

func TestZoneIDs(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		row  map[string]string
		want string
	}{
		{map[string]string{"STATE": "AL", "ZONE": "019", "STATE_ZONE": "AL019"}, "ALZ019"},
		{map[string]string{"STATE": "ME", "FIPS": "23029"}, "MEC029"},
		{map[string]string{"ID": "PHZ113"}, "PHZ113"},
		{map[string]string{"id": "", "NAME": "North Atlantic Ocean"}, ""},
		{map[string]string{}, ""},
	} {
		if got := zoneID(c.row); got != c.want {
			t.Errorf("zoneID(%v) = %q, want %q", c.row, got, c.want)
		}
	}
}

func TestASplitZoneIsCenteredByArea(t *testing.T) {
	t.Parallel()
	all := map[string]*zone{}
	rows := []map[string]string{{"ID": "PZZ001"}, {"ID": "PZZ001"}}
	if err := add(all, rows, [][][][2]float64{{square(0, 0, 3)}, {square(10, 0, 1)}}); err != nil {
		t.Fatal(err)
	}
	c, ok := all["PZZ001"].center()
	// Areas 9 and 1, centers 1.5 and 10.5: (9*1.5 + 10.5) / 10 = 2.4.
	if !ok || math.Abs(c[0]-2.4) > 1e-9 || math.Abs(c[1]-1.4) > 1e-9 {
		t.Errorf("center = %v, want [2.4 1.4], pulled toward the larger part", c)
	}
	if err := add(all, rows, [][][][2]float64{{square(0, 0, 1)}}); err == nil {
		t.Error("rows and shapes that do not line up were accepted")
	}
}

// Shapefiles split a zone at 180°; its center must stay near 180°, not 0°.
func TestACenterAcrossTheDateLine(t *testing.T) {
	t.Parallel()
	all := map[string]*zone{}
	east := [][2]float64{{179, 50}, {179, 51}, {180, 51}, {180, 50}, {179, 50}}
	west := [][2]float64{{-180, 50}, {-180, 51}, {-179, 51}, {-179, 50}, {-180, 50}}
	if err := add(all, []map[string]string{{"STATE": "AK", "FIPS": "02016"}}, [][][][2]float64{{east, west}}); err != nil {
		t.Fatal(err)
	}
	c, _ := all["AKC016"].center()
	if math.Abs(math.Abs(c[0])-180) > 1e-9 || math.Abs(c[1]-50.5) > 1e-9 {
		t.Errorf("center = %v, want [±180 50.5]", c)
	}
}

func TestATinyZoneKeepsAnOutline(t *testing.T) {
	t.Parallel()
	all := map[string]*zone{}
	if err := add(all, []map[string]string{{"ID": "GMZ001"}}, [][][][2]float64{{square(-80, 25, 0.004)}}); err != nil {
		t.Fatal(err)
	}
	if out := all["GMZ001"].outline(); len(out) != 1 || len(out[0]) < 4 {
		t.Errorf("outline = %v, want the ring as drawn: it is smaller than the tolerance", out)
	}
	if _, err := shapesFile(all); err != nil {
		t.Fatal(err)
	}
}
