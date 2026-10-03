package domain

import (
	"fmt"
	"slices"
)

// The risk questionnaire follows the Brazilian suitability process (horizon,
// objective, loss tolerance, experience, liquidity needs, share of wealth).
// Questions and answers are stable codes; the web app owns the wording.

// SuitabilityQuestion is one question and its answers, from the most
// cautious (0 points) to the boldest (len-1 points).
type SuitabilityQuestion struct {
	ID      string
	Answers []string
}

// SuitabilityQuestions is the questionnaire, in display order.
var SuitabilityQuestions = []SuitabilityQuestion{
	{ID: "horizon", Answers: []string{"lt_1y", "1_3y", "3_5y", "gt_5y"}},
	{ID: "objective", Answers: []string{"preserve", "income", "balanced", "growth"}},
	{ID: "drawdown", Answers: []string{"sell_all", "sell_some", "hold", "buy_more"}},
	{ID: "experience", Answers: []string{"none", "fixed_income", "funds_stocks", "active"}},
	{ID: "liquidity", Answers: []string{"most", "some", "little", "none"}},
	{ID: "wealth_share", Answers: []string{"gt_75", "50_75", "25_50", "lt_25"}},
}

// SuitabilityAnswers maps question id to answer code.
type SuitabilityAnswers map[string]string

// ProfileFromAnswers scores the questionnaire: up to 3 points per question,
// 0-6 conservative, 7-13 moderate, 14-18 aggressive. Money needed within a
// year caps the profile at conservative whatever else was answered, as
// suitability rules do. Every question must be answered with a known code.
func ProfileFromAnswers(a SuitabilityAnswers) (RiskProfile, int, error) {
	if len(a) != len(SuitabilityQuestions) {
		return "", 0, fmt.Errorf("%w: answer all %d questions", ErrInvalid, len(SuitabilityQuestions))
	}
	total := 0
	for _, q := range SuitabilityQuestions {
		i := slices.Index(q.Answers, a[q.ID])
		if i < 0 {
			return "", 0, fmt.Errorf("%w: unknown answer %q to %s", ErrInvalid, a[q.ID], q.ID)
		}
		total += i
	}
	switch {
	case a["horizon"] == "lt_1y" || total <= 6:
		return ProfileConservative, total, nil
	case total <= 13:
		return ProfileModerate, total, nil
	}
	return ProfileAggressive, total, nil
}
