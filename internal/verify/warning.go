package verify

import (
	"strings"

	"ttb-label-verification/internal/extraction"
)

// CanonicalWarning is the health warning statement prescribed by 27 CFR 16.21.
// Wording and punctuation must match exactly; no tolerance (SPEC §4.4/§5.4).
const CanonicalWarning = "GOVERNMENT WARNING: (1) According to the Surgeon General, women should not drink alcoholic beverages during pregnancy because of the risk of birth defects. (2) Consumption of alcoholic beverages impairs your ability to drive a car or operate machinery, and may cause health problems."

const warningCitation = "27 CFR 16.21"

// collapseSpace joins wrapped lines: line breaks on a label are not wording
// differences, but every character otherwise counts.
func collapseSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// sliceAsPrinted returns the phrase as it actually appears in the label text,
// matched case-insensitively — so a casing row can show "Government Warning"
// (the defect) instead of reprinting the entire statement.
func sliceAsPrinted(label, phrase string) string {
	idx := strings.Index(strings.ToLower(label), phrase)
	if idx < 0 {
		return ""
	}
	return label[idx : idx+len(phrase)]
}

// checkGovernmentWarning returns the part-16 rule rows: presence, exact
// wording, casing, and the formatting attributes the model observed.
func checkGovernmentWarning(app Application, ext *extraction.Result) []RuleResult {
	warning := ext.GovernmentWarning

	// Malt beverages under 0.5% ABV do not require the warning (SPEC §5.3).
	if app.BeverageType == Malt && app.AlcoholContent < 0.5 {
		return []RuleResult{{RuleID: "warning-present", Citation: warningCitation, Field: "Government warning", Status: StatusNotApplicable, Tier: TierExact, Note: "not required for malt beverages under 0.5% ABV"}}
	}

	if !warning.Found {
		return []RuleResult{{RuleID: "warning-present", Citation: warningCitation, Field: "Government warning", Status: StatusFail, Tier: TierExact, Note: "health warning statement not found on any label piece"}}
	}

	// The full transcription appears once, on the wording row (where it sits
	// next to the canonical text for comparison); every other row shows only
	// the slice of the warning it actually checks.
	results := []RuleResult{{RuleID: "warning-present", Citation: warningCitation, Field: "Government warning", Status: StatusPass, Tier: TierExact}}

	label := collapseSpace(warning.Value)

	// Exact wording and punctuation, compared case-insensitively; casing has
	// its own rules below so a casing error is reported precisely.
	wording := RuleResult{RuleID: "warning-wording", Citation: warningCitation, Field: "Warning wording", ApplicationValue: CanonicalWarning, LabelValue: warning.Value, Tier: TierExact}
	if strings.EqualFold(label, CanonicalWarning) {
		wording.Status = StatusPass
	} else {
		wording.Status = StatusFail
		wording.Note = "wording or punctuation differs from the prescribed statement — no tolerance"
	}
	if wording.Status == StatusPass && warning.Confidence == "low" {
		wording.Status = StatusNeedsReview
		wording.Tier = TierJudgment
		wording.Note = "low-confidence read — confirm verbatim text against image"
	}
	results = append(results, wording)

	// "GOVERNMENT WARNING" in capital letters (checked from the transcription)
	// and bold type (visual observation).
	caps := RuleResult{RuleID: "warning-caps", Citation: "27 CFR 16.22(a)(1)", Field: `"GOVERNMENT WARNING" casing`, LabelValue: sliceAsPrinted(label, "government warning"), Tier: TierExact}
	if strings.Contains(label, "GOVERNMENT WARNING") && ext.WarningFormat.HeaderAllCaps {
		caps.Status = StatusPass
	} else {
		caps.Status = StatusFail
		caps.Note = `"GOVERNMENT WARNING" must appear entirely in capital letters`
	}
	results = append(results, caps)

	bold := RuleResult{RuleID: "warning-bold", Citation: "27 CFR 16.22(a)(1)", Field: `"GOVERNMENT WARNING" bold type`, Tier: TierJudgment}
	if ext.WarningFormat.HeaderBold {
		bold.Status = StatusPass
		bold.Note = "appears bold in the image — confirm on the physical label if borderline"
	} else {
		bold.Status = StatusFail
		bold.Note = `"GOVERNMENT WARNING" does not appear in bold type`
	}
	results = append(results, bold)

	// Surgeon General capitalization (part of the exact-wording requirement,
	// reported separately so the specific defect is visible).
	sg := RuleResult{RuleID: "warning-surgeon-general", Citation: warningCitation, Field: `"Surgeon General" casing`, LabelValue: sliceAsPrinted(label, "surgeon general"), Tier: TierExact}
	if !strings.Contains(label, "Surgeon General") && strings.Contains(strings.ToLower(label), "surgeon general") {
		sg.Status = StatusFail
		sg.Note = `the "S" in Surgeon and "G" in General must be capitalized`
	} else if strings.Contains(label, "Surgeon General") {
		sg.Status = StatusPass
	} else {
		sg.Status = StatusFail
		sg.Note = `"Surgeon General" not found in the warning text`
	}
	results = append(results, sg)

	continuous := RuleResult{RuleID: "warning-continuous", Citation: "27 CFR 16.22(b)", Field: "Warning continuity", Tier: TierJudgment}
	if ext.WarningFormat.Continuous {
		continuous.Status = StatusPass
	} else {
		continuous.Status = StatusFail
		continuous.Note = "warning must appear as one continuous statement"
	}
	results = append(results, continuous)

	separate := RuleResult{RuleID: "warning-separate", Citation: "27 CFR 16.22(b)", Field: "Warning separation", Tier: TierJudgment}
	if ext.WarningFormat.SeparateFromOtherText {
		separate.Status = StatusPass
	} else {
		separate.Status = StatusNeedsReview
		separate.Note = "warning may not be separate and apart from other label text — confirm"
	}
	results = append(results, separate)

	return results
}
