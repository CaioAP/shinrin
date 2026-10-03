package scoring

import (
	"strings"
	"unicode"

	"github.com/CaioAP/shinrin/backend/internal/domain"
)

// Headline tone with a small bilingual (English and Portuguese) finance
// lexicon. It is deliberately simple: transparent, free and reproducible.
// An LLM can read the same headlines inside a user's AI report.
var (
	positiveWords = wordSet(`beat beats surge surges soar soars jump jumps rally rallies record growth grows gain gains
		upgrade upgraded upgrades outperform profit profits strong strength raise raises raised boost boosts
		dividend dividends buyback buybacks expansion approve approved approval wins win partnership acquisition
		lucro lucros alta altas cresce crescimento recorde supera superou dispara disparam avanca avança avancam
		sobe sobem ganho ganhos elevacao elevação eleva melhora aprovacao aprovação aprova dividendo dividendos
		proventos recompra expansao expansão parceria aquisicao aquisição positivo forte`)
	negativeWords = wordSet(`miss misses missed plunge plunges fall falls fell drop drops slump slumps loss losses
		downgrade downgraded downgrades underperform weak weakness cut cuts lawsuit probe investigation fraud
		default bankruptcy recall layoffs warning warns decline declines debt fine fined halt halted scandal
		prejuizo prejuízo perda perdas queda quedas cai caem recua recuam despenca rebaixa rebaixamento
		fraude investigacao investigação multa processo calote recuperacao recuperação judicial falencia falência
		demissoes demissões alerta piora negativo fraco endividamento suspensao suspensão escandalo escândalo`)
)

func wordSet(s string) map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields(s) {
		m[w] = true
	}
	return m
}

// Tone scores one text from -1 (all negative words) to 1 (all positive).
// ok is false when no lexicon word appears, so neutral text does not drag
// the average toward zero.
func Tone(text string) (tone float64, ok bool) {
	var pos, neg int
	for _, w := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool { return !unicode.IsLetter(r) }) {
		switch {
		case positiveWords[w]:
			pos++
		case negativeWords[w]:
			neg++
		}
	}
	if pos+neg == 0 {
		return 0, false
	}
	return float64(pos-neg) / float64(pos+neg), true
}

// NewsToneOf averages the tone of headlines (title and summary). It returns
// the mean tone of the items that carried any signal and how many items were
// read; ok is false when none carried signal.
func NewsToneOf(items []domain.NewsItem) (tone float64, count int, ok bool) {
	var sum float64
	var scored int
	for _, it := range items {
		if t, has := Tone(it.Title + " " + it.Summary); has {
			sum += t
			scored++
		}
	}
	if scored == 0 {
		return 0, len(items), false
	}
	return sum / float64(scored), len(items), true
}
