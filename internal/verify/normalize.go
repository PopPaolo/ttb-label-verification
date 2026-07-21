package verify

import (
	"regexp"
	"strconv"
	"strings"
)

// Normalize implements the "normalized/equivalent" matching tier (SPEC §4.4):
// case folding, typographic-quote/dash unification, punctuation stripping, and
// whitespace collapsing, so "STONE'S THROW" == "Stone's Throw".
func Normalize(s string) string {
	replacer := strings.NewReplacer(
		"‘", "'", "’", "'", "“", `"`, "”", `"`,
		"–", "-", "—", "-", " ", " ",
	)
	s = strings.ToLower(replacer.Replace(s))
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '.', ',', ';', ':', '!', '"', '\'', '(', ')':
			// dropped: trivial formatting per SPEC §4.4
		default:
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// Levenshtein powers the judgment tier: small edit distances after
// normalization flag a near-match for the agent instead of a hard fail.
func Levenshtein(a, b string) int {
	ar, br := []rune(a), []rune(b)
	prev := make([]int, len(br)+1)
	curr := make([]int, len(br)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ar); i++ {
		curr[0] = i
		for j := 1; j <= len(br); j++ {
			cost := 1
			if ar[i-1] == br[j-1] {
				cost = 0
			}
			curr[j] = min(prev[j]+1, min(curr[j-1]+1, prev[j-1]+cost))
		}
		prev, curr = curr, prev
	}
	return prev[len(br)]
}

var (
	percentRe = regexp.MustCompile(`(\d+(?:\.\d+)?)\s*%`)
	proofRe   = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*proof`)
)

// ParseABV extracts alcohol-by-volume from a label statement. Accepts the
// abbreviation variants in SPEC §4.4 ("45% Alc./Vol.", "Alcohol 45% by
// volume") and falls back to a proof statement (proof = 2 × ABV).
func ParseABV(s string) (float64, bool) {
	if m := percentRe.FindStringSubmatch(s); m != nil {
		v, err := strconv.ParseFloat(m[1], 64)
		return v, err == nil
	}
	if m := proofRe.FindStringSubmatch(s); m != nil {
		if v, err := strconv.ParseFloat(m[1], 64); err == nil {
			return v / 2, true
		}
	}
	return 0, false
}

type netUnit struct {
	pattern *regexp.Regexp
	ml      float64
	metric  bool
}

// Ordered so multi-word units match before their substrings ("fl oz" before "oz"/"l").
var netUnits = []netUnit{
	{regexp.MustCompile(`(?i)^fl\.?\s*oz|^fluid\s+ounces?|^floz`), 29.5735, false},
	{regexp.MustCompile(`(?i)^ounces?|^oz`), 29.5735, false},
	{regexp.MustCompile(`(?i)^milliliters?|^millilitres?|^ml`), 1, true},
	{regexp.MustCompile(`(?i)^liters?|^litres?|^l\b`), 1000, true},
	{regexp.MustCompile(`(?i)^pints?|^pt`), 473.176, false},
	{regexp.MustCompile(`(?i)^quarts?|^qt`), 946.353, false},
	{regexp.MustCompile(`(?i)^gallons?|^gal`), 3785.41, false},
}

var netQtyRe = regexp.MustCompile(`(\d+(?:\.\d+)?)\s*([a-zA-Z][a-zA-Z. ]*)`)

// NetContents is a parsed net-contents statement in a common unit (mL), which
// implements the unit-equivalence tier (1 quart = 32 fl. oz., SPEC §4.4).
type NetContents struct {
	Milliliters float64
	Unit        string // canonical: "fl oz", "ml", "l", "pint", "quart", "gallon"
	Metric      bool
	FlOz        float64 // value expressed in fluid ounces (for unit-correctness rules)
}

var canonicalUnits = []string{"fl oz", "fl oz", "ml", "l", "pint", "quart", "gallon"}

// ParseNetContents parses statements like "750 mL", "1 Quart", "12 FL. OZ.".
// Multi-part statements ("1 Pint 8 fl oz") sum their parts.
func ParseNetContents(s string) (NetContents, bool) {
	matches := netQtyRe.FindAllStringSubmatch(s, -1)
	if len(matches) == 0 {
		return NetContents{}, false
	}
	var out NetContents
	parsedAny := false
	for _, m := range matches {
		qty, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			continue
		}
		unitText := strings.TrimSpace(m[2])
		for i, u := range netUnits {
			if u.pattern.MatchString(unitText) {
				out.Milliliters += qty * u.ml
				if !parsedAny {
					out.Unit = canonicalUnits[i]
				}
				if u.metric {
					out.Metric = true
				}
				parsedAny = true
				break
			}
		}
	}
	if !parsedAny {
		return NetContents{}, false
	}
	out.FlOz = out.Milliliters / 29.5735
	return out, parsedAny
}
