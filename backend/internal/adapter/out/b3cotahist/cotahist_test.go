package b3cotahist_test

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/CaioAP/shinrin/backend/internal/adapter/out/b3cotahist"
	"github.com/CaioAP/shinrin/backend/internal/domain"
)

// record builds one 245-column COTAHIST line following B3's layout.
func record(date, bdi, sym, tpmerc string, open, high, low, last int64, qty int64, factor int) string {
	var b strings.Builder
	pad := func(s string, n int) { fmt.Fprintf(&b, "%-*s", n, s) }
	num := func(v int64, n int) { fmt.Fprintf(&b, "%0*d", n, v) }
	pad("01", 2)
	pad(date, 8)
	pad(bdi, 2)
	pad(sym, 12)
	pad(tpmerc, 3)
	pad("PETROBRAS", 12)
	pad("PN      N2", 10)
	pad("", 3)
	pad("R$", 4)
	num(open, 13)
	num(high, 13)
	num(low, 13)
	num((high+low)/2, 13) // PREMED
	num(last, 13)
	num(0, 13) // PREOFC
	num(0, 13) // PREOFV
	num(1234, 5)
	num(qty, 18)
	num(qty*last, 18) // VOLTOT
	num(0, 13)
	pad("0", 1)
	pad("99991231", 8)
	num(int64(factor), 7)
	num(0, 13)
	pad("BRPETRACNPR6", 12)
	pad("115", 3)
	return b.String()
}

func zipOf(t *testing.T, name string, lines ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	header := "00COTAHIST.2025BOVESPA 20250310" + strings.Repeat(" ", 214)
	fmt.Fprint(w, header+"\r\n"+strings.Join(lines, "\r\n")+"\r\n")
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestRecordBuilderMatchesLayoutWidth(t *testing.T) {
	if n := len(record("20250310", "02", "PETR4", "010", 1, 1, 1, 1, 1, 1)); n != 245 {
		t.Fatalf("record width = %d, want 245", n)
	}
}

func TestParse(t *testing.T) {
	lines := []string{
		record("20250310", "02", "PETR4", "010", 3712, 3780, 3690, 3750, 51234500, 1),
		record("20250310", "12", "MXRF11", "010", 1001, 1010, 999, 1005, 900000, 1),
		record("20250310", "02", "ABCD3", "010", 150000, 150000, 150000, 150000, 10, 1000), // quoted per 1000
		record("20250310", "96", "PETR4F", "020", 3712, 3780, 3690, 3750, 99, 1),           // fractional: dropped
		record("20250310", "78", "PETRC400", "070", 10, 20, 5, 15, 100, 1),                 // option: dropped
		record("20250101", "02", "PETR4", "010", 1, 1, 1, 1, 1, 1),                         // outside range
	}
	in := strings.Join(lines, "\r\n") + "\r\n99COTAHIST.2025BOVESPA" + strings.Repeat(" ", 223)
	from := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2025, 3, 31, 0, 0, 0, 0, time.UTC)

	bars, err := b3cotahist.Parse(strings.NewReader(in), from, to)
	if err != nil {
		t.Fatal(err)
	}
	if len(bars) != 3 {
		t.Fatalf("got %d bars, want 3: %+v", len(bars), bars)
	}
	p := bars[0]
	want := domain.PriceBar{
		Asset: domain.AssetKey{Market: domain.MarketB3, Symbol: "PETR4"},
		Date:  time.Date(2025, 3, 10, 0, 0, 0, 0, time.UTC),
		Open:  37.12, High: 37.80, Low: 36.90, Close: 37.50, Volume: 51234500, Source: "b3_cotahist",
	}
	if p != want {
		t.Errorf("PETR4 = %+v\nwant   %+v", p, want)
	}
	if bars[1].Asset.Symbol != "MXRF11" || bars[1].Close != 10.05 {
		t.Errorf("FII bar = %+v", bars[1])
	}
	if bars[2].Close != 1.5 {
		t.Errorf("FATCOT 1000 close = %v, want 1.5", bars[2].Close)
	}
}

func TestMarketPricesPlansFiles(t *testing.T) {
	var (
		mu  sync.Mutex
		got []string
	)
	files := map[string][]byte{
		"COTAHIST_A2024.ZIP":     zipOf(t, "COTAHIST_A2024.TXT", record("20241230", "02", "PETR4", "010", 1, 1, 1, 3600, 1, 1)),
		"COTAHIST_M012025.ZIP":   zipOf(t, "COTAHIST_M012025.TXT", record("20250131", "02", "PETR4", "010", 1, 1, 1, 3620, 1, 1)),
		"COTAHIST_M022025.ZIP":   zipOf(t, "COTAHIST_M022025.TXT", record("20250228", "02", "PETR4", "010", 1, 1, 1, 3650, 1, 1)),
		"COTAHIST_D03032025.ZIP": zipOf(t, "COTAHIST_D03032025.TXT", record("20250303", "02", "PETR4", "010", 1, 1, 1, 3700, 1, 1)),
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")
		mu.Lock()
		got = append(got, name)
		mu.Unlock()
		if strings.HasPrefix(name, "COTAHIST_D04") { // a holiday, served as an HTML page
			fmt.Fprint(w, "<html>not found</html>")
			return
		}
		b, ok := files[name]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write(b)
	}))
	defer srv.Close()

	today := time.Date(2025, 3, 5, 21, 0, 0, 0, time.UTC)
	src := b3cotahist.New(srv.Client(), srv.URL, func() time.Time { return today })
	bars, err := src.MarketPrices(context.Background(), time.Date(2024, 12, 30, 0, 0, 0, 0, time.UTC), today)
	if err != nil {
		t.Fatal(err)
	}
	wantFiles := []string{"COTAHIST_A2024.ZIP", "COTAHIST_M012025.ZIP", "COTAHIST_M022025.ZIP", "COTAHIST_D03032025.ZIP", "COTAHIST_D04032025.ZIP", "COTAHIST_D05032025.ZIP"}
	if strings.Join(got, ",") != strings.Join(wantFiles, ",") {
		t.Errorf("requested %v\nwant      %v", got, wantFiles)
	}
	// 4 and 5 March are skipped: one is an HTML page, the other a 404.
	if len(bars) != 4 || bars[3].Close != 37 {
		t.Errorf("bars = %+v", bars)
	}

	// A missing monthly or yearly file is a real error, not a holiday.
	delete(files, "COTAHIST_M012025.ZIP")
	if _, err := src.MarketPrices(context.Background(), time.Date(2025, 1, 10, 0, 0, 0, 0, time.UTC), today); err == nil {
		t.Error("missing monthly file should fail")
	}
}
