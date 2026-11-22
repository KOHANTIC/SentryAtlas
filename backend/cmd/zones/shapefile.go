package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strings"
)

// readDBF reads a dBASE table's rows, field name to trimmed text. A row
// marked deleted is empty, so rows still line up with the shapes.
func readDBF(b []byte) ([]map[string]string, error) {
	if len(b) < 32 {
		return nil, errors.New("dbf: too short")
	}
	n := int(binary.LittleEndian.Uint32(b[4:8]))
	headerLen := int(binary.LittleEndian.Uint16(b[8:10]))
	recordLen := int(binary.LittleEndian.Uint16(b[10:12]))
	type field struct {
		name string
		len  int
	}
	var fields []field
	for off := 32; ; off += 32 {
		if off >= len(b) {
			return nil, errors.New("dbf: no end to the field list")
		}
		if b[off] == 0x0D {
			break
		}
		if off+32 > len(b) {
			return nil, errors.New("dbf: field list cut short")
		}
		name, _, _ := strings.Cut(string(b[off:off+11]), "\x00")
		fields = append(fields, field{name, int(b[off+16])})
	}
	if headerLen+n*recordLen > len(b) {
		return nil, fmt.Errorf("dbf: %d rows of %d bytes do not fit", n, recordLen)
	}
	rows := make([]map[string]string, 0, n)
	for i := range n {
		r := b[headerLen+i*recordLen : headerLen+(i+1)*recordLen]
		row := map[string]string{}
		if r[0] == '*' {
			rows = append(rows, row)
			continue
		}
		off := 1
		for _, f := range fields {
			if off+f.len > len(r) {
				return nil, fmt.Errorf("dbf: row %d cut short", i)
			}
			// Latin-1 bytes are their own code points.
			var sb strings.Builder
			for _, c := range r[off : off+f.len] {
				sb.WriteRune(rune(c))
			}
			row[f.name] = strings.TrimSpace(sb.String())
			off += f.len
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// readSHP reads a polygon shapefile: each record's rings, as [lon, lat]
// points. A null record has no rings.
func readSHP(b []byte) ([][][][2]float64, error) {
	if len(b) < 100 {
		return nil, errors.New("shp: too short")
	}
	var records [][][][2]float64
	for off := 100; off < len(b); {
		if off+8 > len(b) {
			return nil, errors.New("shp: record header cut short")
		}
		size := int(binary.BigEndian.Uint32(b[off+4:off+8])) * 2
		c := b[off+8:]
		if size > len(c) {
			return nil, errors.New("shp: record cut short")
		}
		c = c[:size]
		off += 8 + size
		if len(c) < 4 {
			return nil, errors.New("shp: record without a shape type")
		}
		switch t := binary.LittleEndian.Uint32(c[0:4]); t {
		case 0:
			records = append(records, nil)
			continue
		case 5:
		default:
			return nil, fmt.Errorf("shp: shape type %d is not a polygon", t)
		}
		if len(c) < 44 {
			return nil, errors.New("shp: polygon header cut short")
		}
		parts := int(binary.LittleEndian.Uint32(c[36:40]))
		points := int(binary.LittleEndian.Uint32(c[40:44]))
		if 44+4*parts+16*points > len(c) {
			return nil, errors.New("shp: polygon cut short")
		}
		starts := make([]int, parts+1)
		for i := range parts {
			starts[i] = int(binary.LittleEndian.Uint32(c[44+4*i:]))
		}
		starts[parts] = points
		base := 44 + 4*parts
		rings := make([][][2]float64, 0, parts)
		for i := range parts {
			if starts[i] > starts[i+1] || starts[i+1] > points {
				return nil, errors.New("shp: ring starts out of order")
			}
			ring := make([][2]float64, 0, starts[i+1]-starts[i])
			for p := starts[i]; p < starts[i+1]; p++ {
				x := math.Float64frombits(binary.LittleEndian.Uint64(c[base+16*p:]))
				y := math.Float64frombits(binary.LittleEndian.Uint64(c[base+16*p+8:]))
				ring = append(ring, [2]float64{x, y})
			}
			rings = append(rings, ring)
		}
		records = append(records, rings)
	}
	return records, nil
}
