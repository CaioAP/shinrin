// Package scoring is Shinrin's deterministic analysis engine (docs/design.md,
// section 8). It turns indicator snapshots into what an analyst would
// conclude, without an LLM:
//
//   - factor scores (valuation, quality, growth, momentum, income, risk,
//     sentiment), each 0-100 and relative to sector peers, with the metrics
//     behind them;
//   - a composite weighted by the investor's risk profile;
//   - fair value ranges (Graham, Bazin, a simple DCF);
//   - a valuation view (cheap, fair, expensive) and a timing view
//     (accumulate, wait, avoid) with the signals that produced them;
//   - allocation bands per profile, tilted by macro conditions.
//
// Everything is a pure function of its inputs, so results are reproducible,
// testable with fixed numbers, and the optional LLM layer can only explain
// them, never invent them.
package scoring
