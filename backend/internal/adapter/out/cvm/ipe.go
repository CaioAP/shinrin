package cvm

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"golang.org/x/text/encoding/charmap"

	"github.com/CaioAP/shinrin/backend/internal/domain"
)

// Categories of IPE filings kept as news: the announcements that move
// prices. Others (minutes, calendars, presentations) are routine.
var newsCategories = map[string]bool{
	"Fato Relevante":        true,
	"Comunicado ao Mercado": true,
}

// News implements port.NewsSource with CVM's IPE dataset: every material
// fact (fato relevante) and market announcement filed by a listed company,
// tagged with the company's tickers.
func (c *Client) News(ctx context.Context, since time.Time) ([]domain.NewsItem, error) {
	tickers, err := c.tickerMap(ctx)
	if err != nil {
		return nil, err
	}
	byCNPJ := map[string][]domain.AssetKey{}
	for sym, cnpj := range tickers {
		byCNPJ[cnpj] = append(byCNPJ[cnpj], domain.AssetKey{Market: domain.MarketB3, Symbol: sym})
	}

	var out []domain.NewsItem
	for year := since.Year(); year <= c.now().Year(); year++ {
		body, err := c.download(ctx, fmt.Sprintf("%s/IPE/DADOS/ipe_cia_aberta_%d.zip", c.baseURL, year))
		if errors.Is(err, domain.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("IPE %d: %w", year, err)
		}
		items, err := parseIPE(body, since, byCNPJ)
		if err != nil {
			return nil, fmt.Errorf("IPE %d: %w", year, err)
		}
		out = append(out, items...)
	}
	return out, nil
}

// parseIPE reads ipe_cia_aberta_YYYY.csv:
// CNPJ_Companhia;Nome_Companhia;Codigo_CVM;Data_Referencia;Categoria;Tipo;
// Especie;Assunto;Data_Entrega;Tipo_Apresentacao;Protocolo_Entrega;Versao;
// Link_Download
func parseIPE(body []byte, since time.Time, byCNPJ map[string][]domain.AssetKey) ([]domain.NewsItem, error) {
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}
	var out []domain.NewsItem
	for _, zf := range zr.File {
		if !strings.HasSuffix(strings.ToLower(zf.Name), ".csv") {
			continue
		}
		rc, err := zf.Open()
		if err != nil {
			return nil, err
		}
		// Unlike the statements, this text is shown to people, so decode
		// Latin-1 to UTF-8.
		r := csv.NewReader(charmap.ISO8859_1.NewDecoder().Reader(rc))
		r.Comma, r.LazyQuotes, r.FieldsPerRecord = ';', true, -1
		header, err := r.Read()
		if err != nil {
			rc.Close()
			return nil, fmt.Errorf("header: %w", err)
		}
		col := map[string]int{}
		for i, h := range header {
			col[strings.TrimPrefix(strings.TrimSpace(h), "\u00ef\u00bb\u00bf")] = i
		}
		for {
			rec, err := r.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				rc.Close()
				return nil, err
			}
			get := func(name string) string {
				if i, ok := col[name]; ok && i < len(rec) {
					return strings.TrimSpace(rec[i])
				}
				return ""
			}
			category := get("Categoria")
			if !newsCategories[category] {
				continue
			}
			assets := byCNPJ[get("CNPJ_Companhia")]
			link := get("Link_Download")
			if len(assets) == 0 || link == "" {
				continue // not a listed share Shinrin can tag
			}
			delivered, err := time.Parse(time.DateOnly, get("Data_Entrega"))
			if err != nil || delivered.Before(since) {
				continue
			}
			title := category
			if subject := get("Assunto"); subject != "" {
				title += ": " + subject
			}
			out = append(out, domain.NewsItem{
				URL: link, Title: title, Summary: get("Nome_Companhia"), Lang: "pt",
				Source: sourceName + ":ipe", PublishedAt: delivered, Assets: assets,
			})
		}
		rc.Close()
	}
	return out, nil
}
