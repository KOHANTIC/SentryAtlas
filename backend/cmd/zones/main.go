// Command zones builds the NWS zone tables from NWS's shapefiles: the
// centers the API places zone-only alerts by, and simplified outlines for
// the map. Run from backend/ (docs/data/nws-zones.md):
//
//	go run ./cmd/zones [zip URL or path ...]
package main

import (
	"archive/zip"
	"bytes"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// defaultSources are the files of 16 April 2026 from weather.gov/gis:
// public zones, marine zones, offshore zones and counties. A refresh puts
// the newer dated names here.
var defaultSources = []string{
	"https://www.weather.gov/source/gis/Shapefiles/WSOM/z_16ap26.zip",
	"https://www.weather.gov/source/gis/Shapefiles/WSOM/mz16ap26.zip",
	"https://www.weather.gov/source/gis/Shapefiles/WSOM/oz16ap26.zip",
	"https://www.weather.gov/source/gis/Shapefiles/County/c_16ap26.zip",
}

func main() {
	centersPath := flag.String("centers", "internal/zones/centers.json", "where to write the zone centers")
	shapesPath := flag.String("shapes", "../web/public/data/nws-zones.json", "where to write the zone outlines")
	flag.Parse()
	sources := flag.Args()
	if len(sources) == 0 {
		sources = defaultSources
	}
	if err := run(sources, *centersPath, *shapesPath); err != nil {
		fmt.Fprintln(os.Stderr, "zones:", err)
		os.Exit(1)
	}
}

func run(sources []string, centersPath, shapesPath string) error {
	all := map[string]*zone{}
	for _, src := range sources {
		data, err := load(src)
		if err != nil {
			return fmt.Errorf("%s: %w", src, err)
		}
		rows, shapes, err := readZip(data)
		if err != nil {
			return fmt.Errorf("%s: %w", src, err)
		}
		if err := add(all, rows, shapes); err != nil {
			return fmt.Errorf("%s: %w", src, err)
		}
		fmt.Printf("%s: %d records\n", src, len(rows))
	}
	centers := centersFile(all)
	shapes, err := shapesFile(all)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(shapesPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(centersPath, centers, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(shapesPath, shapes, 0o644); err != nil {
		return err
	}
	fmt.Printf("%d zones: %s %d bytes, %s %d bytes\n", len(all), centersPath, len(centers), shapesPath, len(shapes))
	return nil
}

// load reads a zip from a URL or a local path.
func load(src string) ([]byte, error) {
	if !strings.HasPrefix(src, "https://") && !strings.HasPrefix(src, "http://") {
		return os.ReadFile(src)
	}
	client := &http.Client{Timeout: 5 * time.Minute}
	req, err := http.NewRequest(http.MethodGet, src, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "SentryAtlas zones build (github.com/KOHANTIC/SentryAtlas)")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("answered HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 200<<20))
}

// readZip reads the one .dbf and .shp a zone archive holds.
func readZip(data []byte) ([]map[string]string, [][][][2]float64, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, nil, err
	}
	var dbf, shp []byte
	for _, f := range zr.File {
		name := strings.ToLower(f.Name)
		if !strings.HasSuffix(name, ".dbf") && !strings.HasSuffix(name, ".shp") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, nil, err
		}
		b, err := io.ReadAll(io.LimitReader(rc, 500<<20))
		if cerr := rc.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return nil, nil, err
		}
		if strings.HasSuffix(name, ".dbf") {
			dbf = b
		} else {
			shp = b
		}
	}
	if dbf == nil || shp == nil {
		return nil, nil, fmt.Errorf("no .dbf and .shp in the archive")
	}
	rows, err := readDBF(dbf)
	if err != nil {
		return nil, nil, err
	}
	shapes, err := readSHP(shp)
	if err != nil {
		return nil, nil, err
	}
	return rows, shapes, nil
}
