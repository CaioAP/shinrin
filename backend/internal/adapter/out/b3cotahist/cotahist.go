// Package b3cotahist reads B3's official historical quotation files
// (COTAHIST, "Séries Históricas"): fixed-width text inside a zip, one line per
// ticker per trading day, free and without a key. It implements
// port.MarketPriceSource.
//
// Layout: https://www.b3.com.br/data/files/33/67/B9/50/D84057102C784E47AC094EA8/SeriesHistoricas_Layout.pdf
//
// Prices in these files are raw, not adjusted for dividends or splits.
package b3cotahist

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

// DefaultBaseURL is where B3 serves the files.
const DefaultBaseURL = "https://bvmf.bmfbovespa.com.br/InstDados/SerHist/"

const sourceName = "b3_cotahist"

// Source downloads and parses COTAHIST files.
type Source struct {
	client  *http.Client
	baseURL string
	now     func() time.Time
	// maxFileBytes guards against a runaway download; a full year is about
	// 90 MB zipped at most.
	maxFileBytes int64
}

var _ port.MarketPriceSource = (*Source)(nil)

// New builds a source. baseURL may be empty for DefaultBaseURL; now may be
// nil for time.Now.
func New(client *http.Client, baseURL string, now func() time.Time) *Source {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	if now == nil {
		now = time.Now
	}
	return &Source{client: client, baseURL: strings.TrimSuffix(baseURL, "/") + "/", now: now, maxFileBytes: 512 << 20}
}

// Name implements port.MarketPriceSource.
func (s *Source) Name() string { return sourceName }

// Market implements port.MarketPriceSource.
func (s *Source) Market() domain.Market { return domain.MarketB3 }

// MarketPrices implements port.MarketPriceSource. It picks the fewest files
// covering the range: a yearly file for past years, monthly files for past
// months of the current year, and daily files for the current month. A
// missing daily file means no trading that day (a holiday) and is skipped.
func (s *Source) MarketPrices(ctx context.Context, from, to time.Time) ([]domain.PriceBar, error) {
	from, to = dateOnly(from), dateOnly(to)
	var out []domain.PriceBar
	for _, f := range plan(from, to, dateOnly(s.now())) {
		bars, err := s.fetch(ctx, f, from, to)
		if errors.Is(err, errNoFile) && f.daily {
			continue
		}
		if err != nil {
			return nil, err
		}
		out = append(out, bars...)
	}
	return out, nil
}

// file is one COTAHIST download.
type file struct {
	name  string // e.g. COTAHIST_A2024.ZIP
	daily bool
}

// plan lists the files covering [from, to] as seen on date today.
func plan(from, to, today time.Time) []file {
	var files []file
	seen := map[string]bool{}
	add := func(f file) {
		if !seen[f.name] {
			seen[f.name] = true
			files = append(files, f)
		}
	}
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		switch {
		case d.Year() < today.Year():
			add(file{name: fmt.Sprintf("COTAHIST_A%d.ZIP", d.Year())})
		case d.Month() < today.Month():
			add(file{name: fmt.Sprintf("COTAHIST_M%02d%d.ZIP", d.Month(), d.Year())})
		case d.Weekday() != time.Saturday && d.Weekday() != time.Sunday:
			add(file{name: fmt.Sprintf("COTAHIST_D%02d%02d%d.ZIP", d.Day(), d.Month(), d.Year()), daily: true})
		}
	}
	return files
}

var errNoFile = errors.New("file not published")

func (s *Source) fetch(ctx context.Context, f file, from, to time.Time) ([]domain.PriceBar, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL+f.name, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get %s: %w", f.name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("%s: %w", f.name, errNoFile)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get %s: status %s", f.name, resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, s.maxFileBytes))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", f.name, err)
	}
	// B3 answers a missing daily file with a small HTML page and status 200
	// on some days, so check the zip signature rather than the status alone.
	if !bytes.HasPrefix(body, []byte("PK")) {
		return nil, fmt.Errorf("%s: %w (not a zip)", f.name, errNoFile)
	}
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", f.name, err)
	}
	var out []domain.PriceBar
	for _, zf := range zr.File {
		rc, err := zf.Open()
		if err != nil {
			return nil, fmt.Errorf("open %s/%s: %w", f.name, zf.Name, err)
		}
		bars, err := Parse(rc, from, to)
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("parse %s/%s: %w", f.name, zf.Name, err)
		}
		out = append(out, bars...)
	}
	return out, nil
}

// Market types and BDI codes kept. TPMERC 010 is the cash (spot) market;
// fractional lots (020) and derivatives are dropped. CODBDI 02 is the standard
// lot, 12 real estate funds (FII), 14 investment certificates and ETFs.
const spotMarket = "010"

var keptBDI = map[string]bool{"02": true, "12": true, "14": true}

// Parse reads COTAHIST lines and returns spot-market bars with
// from <= date <= to. It is exported for tests and for offline backfills of a
// downloaded file.
func Parse(r io.Reader, from, to time.Time) ([]domain.PriceBar, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 512), 4096)
	var out []domain.PriceBar
	line := 0
	for sc.Scan() {
		line++
		b, ok, err := parseLine(sc.Text())
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		if !ok || b.Date.Before(from) || b.Date.After(to) {
			continue
		}
		out = append(out, b)
	}
	return out, sc.Err()
}

// parseLine decodes one record. ok is false for header, trailer and dropped
// market types. Positions below are the layout's 1-based columns.
func parseLine(l string) (domain.PriceBar, bool, error) {
	l = strings.TrimRight(l, "\r")
	if len(l) < 245 || l[0:2] != "01" {
		return domain.PriceBar{}, false, nil
	}
	if field(l, 25, 27) != spotMarket || !keptBDI[field(l, 11, 12)] {
		return domain.PriceBar{}, false, nil
	}

	date, err := time.Parse("20060102", field(l, 3, 10))
	if err != nil {
		return domain.PriceBar{}, false, fmt.Errorf("date: %w", err)
	}
	sym, err := domain.NewSymbol(field(l, 13, 24))
	if err != nil {
		return domain.PriceBar{}, false, err
	}
	factor, err := strconv.ParseFloat(field(l, 211, 217), 64)
	if err != nil || factor <= 0 {
		factor = 1 // quotation factor: prices are per 1 share or per 1000
	}

	var nums [5]float64
	for i, pos := range [][2]int{{57, 69}, {70, 82}, {83, 95}, {109, 121}, {153, 170}} {
		v, err := strconv.ParseInt(field(l, pos[0], pos[1]), 10, 64)
		if err != nil {
			return domain.PriceBar{}, false, fmt.Errorf("column %d: %w", pos[0], err)
		}
		nums[i] = float64(v)
	}
	price := func(cents float64) float64 { return cents / 100 / factor }
	return domain.PriceBar{
		Asset:  domain.AssetKey{Market: domain.MarketB3, Symbol: sym},
		Date:   date,
		Open:   price(nums[0]),
		High:   price(nums[1]),
		Low:    price(nums[2]),
		Close:  price(nums[3]),
		Volume: int64(nums[4]), // QUATOT: shares traded
		Source: sourceName,
	}, true, nil
}

// field returns columns start..end (1-based, inclusive), trimmed.
func field(l string, start, end int) string { return strings.TrimSpace(l[start-1 : end]) }

func dateOnly(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
