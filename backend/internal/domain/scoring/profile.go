package scoring

import "github.com/CaioAP/shinrin/backend/internal/domain"

// ProfileWeights says how much each factor counts for each risk profile.
// Conservative investors weight quality, income and low risk; aggressive
// ones weight growth and momentum. Each row sums to 1.
var ProfileWeights = map[domain.RiskProfile]map[domain.Factor]float64{
	domain.ProfileConservative: {
		domain.FactorValuation: 0.20, domain.FactorQuality: 0.25, domain.FactorGrowth: 0.05,
		domain.FactorMomentum: 0.05, domain.FactorIncome: 0.20, domain.FactorRisk: 0.20, domain.FactorSentiment: 0.05,
	},
	domain.ProfileModerate: {
		domain.FactorValuation: 0.20, domain.FactorQuality: 0.20, domain.FactorGrowth: 0.15,
		domain.FactorMomentum: 0.10, domain.FactorIncome: 0.10, domain.FactorRisk: 0.15, domain.FactorSentiment: 0.10,
	},
	domain.ProfileAggressive: {
		domain.FactorValuation: 0.15, domain.FactorQuality: 0.15, domain.FactorGrowth: 0.25,
		domain.FactorMomentum: 0.20, domain.FactorIncome: 0.05, domain.FactorRisk: 0.05, domain.FactorSentiment: 0.15,
	},
}

// Composite is the profile-weighted average of the factors present in sc,
// renormalised over the weights that had a score. Coverage is the share of
// the profile's total weight those factors represent.
func Composite(sc domain.Scorecard, p domain.RiskProfile) (composite, coverage float64) {
	w := ProfileWeights[p]
	var sum, used, total float64
	for _, f := range domain.Factors {
		total += w[f]
	}
	for _, fs := range sc.Factors {
		sum += fs.Value * w[fs.Factor]
		used += w[fs.Factor]
	}
	if used == 0 || total == 0 {
		return 0, 0
	}
	return round1(sum / used), round1(used/total*100) / 100
}
