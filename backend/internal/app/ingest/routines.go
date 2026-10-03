package ingest

import (
	"context"

	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

// routine adapts a sync call to port.Routine.
type routine struct {
	name string
	run  func(ctx context.Context) error
}

func (r routine) Name() string                  { return r.name }
func (r routine) Run(ctx context.Context) error { return r.run(ctx) }

// UniverseRoutine syncs an index's members.
func (s *Service) UniverseRoutine(name string, src port.UniverseSource, idx domain.Index) port.Routine {
	return routine{name, func(ctx context.Context) error { _, err := s.SyncUniverse(ctx, src, idx); return err }}
}

// MarketPricesRoutine syncs prices from a bulk market source.
func (s *Service) MarketPricesRoutine(name string, src port.MarketPriceSource) port.Routine {
	return routine{name, func(ctx context.Context) error { _, err := s.SyncMarketPrices(ctx, src); return err }}
}

// PricesRoutine syncs prices (and optionally corporate actions) per asset.
func (s *Service) PricesRoutine(name string, m domain.Market, prices port.PriceSource, actions port.CorporateActionSource) port.Routine {
	return routine{name, func(ctx context.Context) error { _, err := s.SyncPrices(ctx, m, prices, actions); return err }}
}

// CorporateActionsRoutine syncs corporate actions per asset.
func (s *Service) CorporateActionsRoutine(name string, m domain.Market, src port.CorporateActionSource) port.Routine {
	return routine{name, func(ctx context.Context) error { _, err := s.SyncCorporateActions(ctx, m, src); return err }}
}

// FundamentalsRoutine syncs fundamentals per asset.
func (s *Service) FundamentalsRoutine(name string, m domain.Market, src port.FundamentalsSource) port.Routine {
	return routine{name, func(ctx context.Context) error { _, err := s.SyncFundamentals(ctx, m, src); return err }}
}
