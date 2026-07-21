// Package verify is the deterministic rule engine (DECISIONS.md D5): the LLM
// extracts, this code decides. Every result carries a rule ID and CFR citation
// so an agent can ask "why did this fail?" and get an auditable answer.
package verify

import "ttb-label-verification/internal/extraction"

// Beverage types (SPEC §2) — they select which rule set applies.
const (
	Wine             = "wine"
	Malt             = "malt"
	DistilledSpirits = "distilled_spirits"
)

// Application is the COLA application data the label must match (SPEC §2).
type Application struct {
	BeverageType   string   `json:"beverage_type"` // wine | malt | distilled_spirits
	BrandName      string   `json:"brand_name"`
	FancifulName   string   `json:"fanciful_name,omitempty"`
	ClassType      string   `json:"class_type"`
	AlcoholContent float64  `json:"alcohol_content"` // percent ABV
	NetContents    string   `json:"net_contents"`
	NameAddress    string   `json:"name_address"`
	Imported       bool     `json:"imported"`
	Appellation    string   `json:"appellation,omitempty"`
	Varietals      []string `json:"varietals,omitempty"`
	VintageDate    string   `json:"vintage_date,omitempty"`
	FormulaNumber  string   `json:"formula_number,omitempty"`
}

// Statuses for individual rules and the overall report. StatusManual marks
// checks the prototype can never perform from an image (type size, field of
// vision): they are surfaced on every report but do not gate the overall
// verdict — needs_review is reserved for genuine judgment calls on this label.
const (
	StatusPass          = "pass"
	StatusFail          = "fail"
	StatusNeedsReview   = "needs_review"
	StatusNotApplicable = "not_applicable"
	StatusManual        = "manual"
)

// Matching tiers (SPEC §4.4) plus "manual" for checks the prototype cannot
// perform from an image (type size, field of vision).
const (
	TierExact      = "exact"
	TierNormalized = "normalized"
	TierJudgment   = "judgment"
	TierManual     = "manual"
)

// RuleResult is one row of the verification report (D6 report shape).
type RuleResult struct {
	RuleID           string `json:"rule_id"`
	Citation         string `json:"citation"`
	Field            string `json:"field"`
	ApplicationValue string `json:"application_value"`
	LabelValue       string `json:"label_value"`
	Status           string `json:"status"`
	Tier             string `json:"tier"`
	Note             string `json:"note,omitempty"`
}

// Report is the per-application verification output.
type Report struct {
	Status  string       `json:"status"`
	Results []RuleResult `json:"results"`
}

// Verify runs the rule set for the application's beverage type against the
// extracted label fields. Pure function; no I/O, no model calls.
func Verify(app Application, ext *extraction.Result) Report {
	var results []RuleResult

	results = append(results,
		checkBrandName(app, ext),
		checkClassType(app, ext),
		checkNetContents(app, ext),
		checkNameAddress(app, ext),
		checkCountryOfOrigin(app, ext),
	)
	results = append(results, checkGovernmentWarning(app, ext)...)

	switch app.BeverageType {
	case DistilledSpirits:
		results = append(results,
			checkSpiritsAlcoholContent(app, ext),
			checkSameFieldOfVision(),
		)
	case Wine:
		results = append(results,
			checkWineAlcoholContent(app, ext),
			checkWineAppellation(app, ext),
			checkWineVarietal(app, ext),
			checkWineVintage(app, ext),
			checkWineSulfites(ext),
		)
	case Malt:
		results = append(results,
			checkMaltAlcoholContent(app, ext),
		)
	}

	results = append(results, checkTypeSize())

	return Report{Status: overallStatus(results), Results: results}
}

// overallStatus: any fail → fail; else any needs_review → needs_review; the
// tool never presents a confident pass on something it flagged (SPEC §4.4).
// StatusManual rows do not gate the verdict — they appear on every report
// regardless of label content, so demoting for them would make "pass"
// unreachable and the triage statuses meaningless.
func overallStatus(results []RuleResult) string {
	status := StatusPass
	for _, r := range results {
		switch r.Status {
		case StatusFail:
			return StatusFail
		case StatusNeedsReview:
			status = StatusNeedsReview
		}
	}
	return status
}
