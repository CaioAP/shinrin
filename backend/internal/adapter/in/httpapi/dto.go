package httpapi

import (
	"time"

	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

// DTOs decouple the JSON contract from domain structs, so renaming a domain
// field never silently breaks the web app. Keep them in sync with
// web/shared/types/api.ts.

type metaDTO struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	Disclaimer string `json:"disclaimer"`
}

type healthDTO struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks,omitempty"`
}

type assetDTO struct {
	Market      string `json:"market"`
	Symbol      string `json:"symbol"`
	Class       string `json:"class"`
	Name        string `json:"name"`
	Sector      string `json:"sector,omitempty"`
	Currency    string `json:"currency"`
	IndexMember bool   `json:"indexMember"`
	Active      bool   `json:"active"`
}

type listDTO[T any] struct {
	Items []T `json:"items"`
}

func toAssetDTO(a domain.Asset) assetDTO {
	return assetDTO{
		Market:      string(a.Key.Market),
		Symbol:      string(a.Key.Symbol),
		Class:       string(a.Class),
		Name:        a.Name,
		Sector:      a.Sector,
		Currency:    a.Currency(),
		IndexMember: a.IndexMember,
		Active:      a.Active,
	}
}

// Analysis DTOs. Every analysis response carries the disclaimer, so no
// client can show a score without it.

type scoreInputDTO struct {
	Metric string  `json:"metric"`
	Value  float64 `json:"value"`
	Points float64 `json:"points"`
	Weight float64 `json:"weight"`
	Peers  int     `json:"peers,omitempty"`
}

type factorDTO struct {
	Factor    string          `json:"factor"`
	Value     float64         `json:"value"`
	PeerGroup string          `json:"peerGroup"`
	Inputs    []scoreInputDTO `json:"inputs"`
}

type signalDTO struct {
	Code    string             `json:"code"`
	Message string             `json:"message"`
	Weight  float64            `json:"weight"`
	Data    map[string]float64 `json:"data,omitempty"`
}

type fairValueDTO struct {
	Method      string             `json:"method"`
	Low         float64            `json:"low"`
	High        float64            `json:"high"`
	Assumptions map[string]float64 `json:"assumptions"`
}

type viewDTO struct {
	View    string      `json:"view"`
	Signals []signalDTO `json:"signals"`
}

type analysisDTO struct {
	Asset      assetDTO           `json:"asset"`
	AsOf       string             `json:"asOf"`
	Profile    string             `json:"profile"`
	Price      float64            `json:"price"`
	Composite  float64            `json:"composite"`
	Coverage   float64            `json:"coverage"`
	Factors    []factorDTO        `json:"factors"`
	Indicators map[string]float64 `json:"indicators"`
	FairValues []fairValueDTO     `json:"fairValues"`
	Valuation  viewDTO            `json:"valuation"`
	Timing     viewDTO            `json:"timing"`
	Notes      []string           `json:"notes"`
	Disclaimer string             `json:"disclaimer"`
}

type rankedDTO struct {
	Asset     assetDTO           `json:"asset"`
	AsOf      string             `json:"asOf"`
	Composite float64            `json:"composite"`
	Coverage  float64            `json:"coverage"`
	Factors   map[string]float64 `json:"factors"`
}

func toRankedDTO(r port.RankedAsset) rankedDTO {
	scores := make(map[string]float64, len(r.Factors))
	for _, f := range r.Factors {
		scores[string(f.Factor)] = f.Value
	}
	return rankedDTO{Asset: toAssetDTO(r.Asset), AsOf: dateString(r.AsOf), Composite: r.Composite, Coverage: r.Coverage, Factors: scores}
}

type rankingDTO struct {
	Profile    string      `json:"profile"`
	Items      []rankedDTO `json:"items"`
	Disclaimer string      `json:"disclaimer"`
}

type bandDTO struct {
	Class string  `json:"class"`
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
	Lean  string  `json:"lean"`
}

type outlookDTO struct {
	Profile    string      `json:"profile"`
	AsOf       string      `json:"asOf,omitempty"`
	Bands      []bandDTO   `json:"bands"`
	Signals    []signalDTO `json:"signals"`
	Notes      []string    `json:"notes"`
	Disclaimer string      `json:"disclaimer"`
}

func toAnalysisDTO(a domain.Analysis) analysisDTO {
	out := analysisDTO{
		Asset: toAssetDTO(a.Asset), AsOf: dateString(a.AsOf), Profile: string(a.Profile), Price: a.Price,
		Composite: a.Composite, Coverage: a.Coverage, Factors: make([]factorDTO, len(a.Scorecard.Factors)),
		Indicators: a.Indicators.Values, FairValues: make([]fairValueDTO, len(a.FairValues)),
		Valuation: viewDTO{View: string(a.Valuation), Signals: toSignalDTOs(a.ValuationSignals)},
		Timing:    viewDTO{View: string(a.Timing.View), Signals: toSignalDTOs(a.Timing.Signals)},
		Notes:     nonNil(a.Notes), Disclaimer: domain.Disclaimer,
	}
	if out.Indicators == nil {
		out.Indicators = map[string]float64{}
	}
	for i, f := range a.Scorecard.Factors {
		in := make([]scoreInputDTO, len(f.Inputs))
		for j, x := range f.Inputs {
			in[j] = scoreInputDTO{x.Metric, x.Value, x.Points, x.Weight, x.Peers}
		}
		out.Factors[i] = factorDTO{Factor: string(f.Factor), Value: f.Value, PeerGroup: f.PeerGroup, Inputs: in}
	}
	for i, fv := range a.FairValues {
		out.FairValues[i] = fairValueDTO{fv.Method, fv.Low, fv.High, fv.Assumptions}
	}
	return out
}

func toSignalDTOs(s []domain.Signal) []signalDTO {
	out := make([]signalDTO, len(s))
	for i, x := range s {
		out[i] = signalDTO{x.Code, x.Message, x.Weight, x.Data}
	}
	return out
}

func dateString(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.DateOnly)
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// Market data DTOs.

type priceBarDTO struct {
	Date     string  `json:"date"`
	Open     float64 `json:"open"`
	High     float64 `json:"high"`
	Low      float64 `json:"low"`
	Close    float64 `json:"close"`
	AdjClose float64 `json:"adjClose"`
	Volume   int64   `json:"volume"`
}

type pricesDTO struct {
	Range  string        `json:"range"`
	Source string        `json:"source,omitempty"`
	Items  []priceBarDTO `json:"items"`
}

type dividendDTO struct {
	ExDate string  `json:"exDate"`
	Type   string  `json:"type"`
	Value  float64 `json:"value"`
	Source string  `json:"source"`
}

type newsDTO struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	Summary     string `json:"summary,omitempty"`
	Lang        string `json:"lang,omitempty"`
	Source      string `json:"source"`
	PublishedAt string `json:"publishedAt"`
}

type macroIndicatorDTO struct {
	Code   string  `json:"code"`
	Value  float64 `json:"value"`
	Unit   string  `json:"unit"`
	AsOf   string  `json:"asOf"`
	Source string  `json:"source"`
}

type macroDTO struct {
	Items   []macroIndicatorDTO `json:"items"`
	Missing []string            `json:"missing"`
}

// Account DTOs.

const timeFormat = time.RFC3339

type userDTO struct {
	ID               int64             `json:"id"`
	Email            string            `json:"email"`
	CreatedAt        string            `json:"createdAt"`
	Profile          string            `json:"profile,omitempty"`
	ProfileAnswers   map[string]string `json:"profileAnswers,omitempty"`
	ProfileUpdatedAt string            `json:"profileUpdatedAt,omitempty"`
}

type sessionDTO struct {
	User      userDTO `json:"user"`
	Token     string  `json:"token"`
	ExpiresAt string  `json:"expiresAt"`
}

type questionDTO struct {
	ID      string   `json:"id"`
	Answers []string `json:"answers"`
}

func toUserDTO(u domain.User) userDTO {
	out := userDTO{ID: int64(u.ID), Email: u.Email, CreatedAt: u.CreatedAt.UTC().Format(timeFormat), Profile: string(u.Profile), ProfileAnswers: u.ProfileAnswers}
	if !u.ProfileAt.IsZero() {
		out.ProfileUpdatedAt = u.ProfileAt.UTC().Format(timeFormat)
	}
	return out
}

type assetKeyDTO struct {
	Market string `json:"market"`
	Symbol string `json:"symbol"`
}

type watchlistDTO struct {
	ID     int64         `json:"id"`
	Name   string        `json:"name"`
	Assets []assetKeyDTO `json:"assets"`
}

func toWatchlistDTO(w domain.Watchlist) watchlistDTO {
	out := watchlistDTO{ID: int64(w.ID), Name: w.Name, Assets: make([]assetKeyDTO, len(w.Assets))}
	for i, k := range w.Assets {
		out.Assets[i] = assetKeyDTO{Market: string(k.Market), Symbol: string(k.Symbol)}
	}
	return out
}

type watchlistEntryDTO struct {
	Asset  assetDTO   `json:"asset"`
	Scores *rankedDTO `json:"scores"` // null when not scored yet
}

type watchlistEntriesDTO struct {
	ID         int64               `json:"id"`
	Name       string              `json:"name"`
	Profile    string              `json:"profile"`
	Items      []watchlistEntryDTO `json:"items"`
	Disclaimer string              `json:"disclaimer"`
}

// AI DTOs. The API key is write-only: no DTO has a field for it.

type llmUsageDTO struct {
	Used  int    `json:"used"`
	Cap   int    `json:"cap"`
	Since string `json:"since"`
}

type llmSettingsDTO struct {
	// Available is false when the server has no master key, so keys cannot
	// be saved at all.
	Available  bool        `json:"available"`
	Configured bool        `json:"configured"`
	Provider   string      `json:"provider,omitempty"`
	Model      string      `json:"model,omitempty"`
	BaseURL    string      `json:"baseUrl,omitempty"`
	KeyHint    string      `json:"keyHint,omitempty"`
	MonthlyCap int         `json:"monthlyCap,omitempty"`
	UpdatedAt  string      `json:"updatedAt,omitempty"`
	Usage      llmUsageDTO `json:"usage"`
}

type allocationDTO struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
}

type reportOutputDTO struct {
	Summary       string        `json:"summary"`
	BullCase      []string      `json:"bullCase"`
	BearCase      []string      `json:"bearCase"`
	ValuationView string        `json:"valuationView"`
	TimingView    string        `json:"timingView"`
	FitForProfile string        `json:"fitForProfile"`
	Allocation    allocationDTO `json:"suggestedAllocationPct"`
	KeyRisks      []string      `json:"keyRisks"`
	Confidence    string        `json:"confidence"`
}

// citedDTO is one piece of data the report cites, resolved from its
// snapshot: a number, a label or a headline.
type citedDTO struct {
	Key   string   `json:"key"`
	Value *float64 `json:"value,omitempty"`
	Text  string   `json:"text,omitempty"`
}

type reportDTO struct {
	ID         int64           `json:"id"`
	Asset      assetKeyDTO     `json:"asset"`
	Profile    string          `json:"profile"`
	AsOf       string          `json:"asOf"`
	CreatedAt  string          `json:"createdAt"`
	Provider   string          `json:"provider"`
	Model      string          `json:"model"`
	TokensIn   int             `json:"tokensIn"`
	TokensOut  int             `json:"tokensOut"`
	Output     reportOutputDTO `json:"output"`
	Cited      []citedDTO      `json:"cited"`
	Omitted    []string        `json:"omitted"`
	Disclaimer string          `json:"disclaimer"`
}

func toReportDTO(r domain.Report) reportDTO {
	o := r.Output
	out := reportDTO{
		ID: r.ID, Asset: assetKeyDTO{Market: string(r.Asset.Market), Symbol: string(r.Asset.Symbol)}, Profile: string(r.Profile),
		AsOf: dateString(r.AsOf), CreatedAt: r.CreatedAt.UTC().Format(timeFormat), Provider: r.Provider, Model: r.Model,
		TokensIn: r.TokensIn, TokensOut: r.TokensOut, Omitted: nonNil(r.Omitted), Disclaimer: domain.Disclaimer,
		Output: reportOutputDTO{Summary: o.Summary, BullCase: nonNil(o.BullCase), BearCase: nonNil(o.BearCase), ValuationView: o.ValuationView,
			TimingView: o.TimingView, FitForProfile: o.FitForProfile, Allocation: allocationDTO{o.AllocationMinPct, o.AllocationMaxPct},
			KeyRisks: nonNil(o.KeyRisks), Confidence: o.Confidence},
		Cited: []citedDTO{},
	}
	for _, k := range o.CitedData {
		c := citedDTO{Key: k}
		if v, ok := r.Snapshot.Facts[k]; ok {
			c.Value = &v
		} else if l, ok := r.Snapshot.Labels[k]; ok {
			c.Text = l
		} else {
			for _, h := range r.Snapshot.Headlines {
				if h.ID == k {
					c.Text = h.Title
				}
			}
		}
		out.Cited = append(out.Cited, c)
	}
	return out
}
