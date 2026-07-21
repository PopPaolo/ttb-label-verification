package verify

import (
	"fmt"
	"math"
	"strings"

	"ttb-label-verification/internal/extraction"
)

// matchText compares an application value against an extracted field using
// the normalized tier, escalating near-misses to the judgment tier
// (needs_review) rather than silently passing or failing them (SPEC §4.4).
func matchText(ruleID, citation, field, appVal string, labelField extraction.Field, required bool) RuleResult {
	r := RuleResult{RuleID: ruleID, Citation: citation, Field: field, ApplicationValue: appVal, LabelValue: labelField.Value, Tier: TierNormalized}

	if appVal == "" {
		r.Status = StatusNotApplicable
		r.Note = "not declared on application"
		return r
	}
	if !labelField.Found {
		if required {
			r.Status = StatusFail
			r.Note = "not found on label"
		} else {
			r.Status = StatusNeedsReview
			r.Tier = TierJudgment
			r.Note = "declared on application but not found on label"
		}
		return r
	}

	normApp, normLabel := Normalize(appVal), Normalize(labelField.Value)
	switch {
	case normApp == normLabel:
		r.Status = StatusPass
	case Levenshtein(normApp, normLabel) <= 2 || strings.Contains(normLabel, normApp) || strings.Contains(normApp, normLabel):
		r.Status = StatusNeedsReview
		r.Tier = TierJudgment
		r.Note = "near match — confirm equivalence"
	default:
		r.Status = StatusFail
		r.Note = "label value does not match application"
	}
	return demoteLowConfidence(r, labelField)
}

// demoteLowConfidence: a pass built on a low-confidence read is not a pass —
// uncertainty is surfaced to the agent (SPEC §4.4).
func demoteLowConfidence(r RuleResult, f extraction.Field) RuleResult {
	if r.Status == StatusPass && f.Found && f.Confidence == "low" {
		r.Status = StatusNeedsReview
		r.Tier = TierJudgment
		r.Note = strings.TrimSpace(r.Note + " low-confidence read — confirm against image")
	}
	return r
}

func checkBrandName(app Application, ext *extraction.Result) RuleResult {
	return matchText("brand-name", "27 CFR 5.64 / 4.33 / 7.64", "Brand name", app.BrandName, ext.BrandName, true)
}

func checkClassType(app Application, ext *extraction.Result) RuleResult {
	return matchText("class-type", "27 CFR 5.141 / 4.34 / 7.63", "Class/type designation", app.ClassType, ext.ClassType, true)
}

func checkNameAddress(app Application, ext *extraction.Result) RuleResult {
	r := matchText("name-address", "27 CFR 5.66 / 4.35 / 7.66", "Name and address", app.NameAddress, ext.NameAddress, true)
	// Address formatting varies legitimately (abbreviations, line breaks); a
	// non-identical but present statement is agent judgment, not a hard fail.
	if r.Status == StatusFail && ext.NameAddress.Found {
		r.Status = StatusNeedsReview
		r.Tier = TierJudgment
		r.Note = "differs from application — confirm name/DBA and city/state match the permit"
	}
	return r
}

func checkCountryOfOrigin(app Application, ext *extraction.Result) RuleResult {
	r := RuleResult{RuleID: "country-of-origin", Citation: "19 CFR 134.11", Field: "Country of origin", LabelValue: ext.CountryOfOrigin.Value, Tier: TierNormalized}
	if !app.Imported {
		r.Status = StatusNotApplicable
		r.Note = "domestic product"
		return r
	}
	r.ApplicationValue = "imported"
	if !ext.CountryOfOrigin.Found {
		r.Status = StatusFail
		r.Note = "imported product but no country-of-origin statement found"
		return r
	}
	r.Status = StatusPass
	return demoteLowConfidence(r, ext.CountryOfOrigin)
}

func checkNetContents(app Application, ext *extraction.Result) RuleResult {
	r := RuleResult{RuleID: "net-contents", Citation: "27 CFR 5.70 / 4.37 / 7.70", Field: "Net contents", ApplicationValue: app.NetContents, LabelValue: ext.NetContents.Value, Tier: TierNormalized}
	if !ext.NetContents.Found {
		// Net contents may be blown/embossed into the container rather than
		// printed on the label — flag for the agent, not a hard fail.
		r.Status = StatusNeedsReview
		r.Tier = TierJudgment
		r.Note = "not found on label — verify it is blown, embossed, or molded into the container"
		return r
	}
	appNC, appOK := ParseNetContents(app.NetContents)
	labelNC, labelOK := ParseNetContents(ext.NetContents.Value)
	if !appOK || !labelOK {
		r.Status = StatusNeedsReview
		r.Tier = TierJudgment
		r.Note = "could not parse quantity for comparison — confirm manually"
		return r
	}
	if math.Abs(appNC.Milliliters-labelNC.Milliliters)/appNC.Milliliters > 0.005 {
		r.Status = StatusFail
		r.Note = fmt.Sprintf("quantities differ (application ≈ %.0f mL, label ≈ %.0f mL)", appNC.Milliliters, labelNC.Milliliters)
		return r
	}
	// Malt beverages: U.S. customary units only, with the correct unit for the
	// volume — 32 fl. oz. must be stated as 1 quart (27 CFR 7.70).
	if app.BeverageType == Malt {
		if labelNC.Metric {
			r.Status = StatusFail
			r.Note = "malt beverage net contents must use U.S. customary units (fl. oz., pints, quarts, gallons)"
			return r
		}
		if labelNC.Unit == "fl oz" && labelNC.FlOz >= 16 {
			r.Status = StatusFail
			r.Note = "volumes of 1 pint or more must be stated in pints/quarts/gallons (e.g. 32 fl. oz. → 1 quart)"
			return r
		}
	}
	r.Status = StatusPass
	if appNC.Unit != labelNC.Unit {
		r.Note = "equivalent quantity in different units"
	}
	return demoteLowConfidence(r, ext.NetContents)
}

// compareABV compares a declared ABV against the label statement.
func compareABV(ruleID, citation string, appABV float64, labelField extraction.Field) RuleResult {
	r := RuleResult{RuleID: ruleID, Citation: citation, Field: "Alcohol content", ApplicationValue: fmt.Sprintf("%g%% ABV", appABV), LabelValue: labelField.Value, Tier: TierNormalized}
	labelABV, ok := ParseABV(labelField.Value)
	if !ok {
		r.Status = StatusNeedsReview
		r.Tier = TierJudgment
		r.Note = "could not parse an ABV from the label statement — confirm manually"
		return r
	}
	switch diff := math.Abs(labelABV - appABV); {
	case diff < 0.001:
		r.Status = StatusPass
	case diff <= 1.0:
		// Regulatory tolerances vary by product/proof; small deltas go to the agent.
		r.Status = StatusNeedsReview
		r.Tier = TierJudgment
		r.Note = fmt.Sprintf("label states %g%% — within possible tolerance, confirm", labelABV)
	default:
		r.Status = StatusFail
		r.Note = fmt.Sprintf("label states %g%%, application declares %g%%", labelABV, appABV)
	}
	return demoteLowConfidence(r, labelField)
}

func checkSpiritsAlcoholContent(app Application, ext *extraction.Result) RuleResult {
	if !ext.AlcoholContent.Found {
		return RuleResult{RuleID: "spirits-abv", Citation: "27 CFR 5.65", Field: "Alcohol content", ApplicationValue: fmt.Sprintf("%g%% ABV", app.AlcoholContent), Status: StatusFail, Tier: TierNormalized, Note: "alcohol content is mandatory for distilled spirits but was not found"}
	}
	return compareABV("spirits-abv", "27 CFR 5.65", app.AlcoholContent, ext.AlcoholContent)
}

func checkWineAlcoholContent(app Application, ext *extraction.Result) RuleResult {
	if !ext.AlcoholContent.Found {
		r := RuleResult{RuleID: "wine-abv", Citation: "27 CFR 4.36", Field: "Alcohol content", ApplicationValue: fmt.Sprintf("%g%% ABV", app.AlcoholContent), Tier: TierNormalized}
		if app.AlcoholContent > 14 {
			r.Status = StatusFail
			r.Note = "numeric alcohol content is mandatory above 14% ABV"
		} else if strings.Contains(Normalize(ext.ClassType.Value), "table wine") && app.AlcoholContent >= 7 {
			r.Status = StatusPass
			r.Note = `"Table Wine" designation suffices for 7–14% ABV (4.36(a))`
		} else {
			r.Status = StatusNeedsReview
			r.Tier = TierJudgment
			r.Note = "no alcohol statement found — acceptable for 7–14% ABV only with a Table Wine (or equivalent) designation"
		}
		return r
	}
	return compareABV("wine-abv", "27 CFR 4.36", app.AlcoholContent, ext.AlcoholContent)
}

func checkMaltAlcoholContent(app Application, ext *extraction.Result) RuleResult {
	if !ext.AlcoholContent.Found {
		// Conditional for malt (7.65): absence alone is not a violation.
		return RuleResult{RuleID: "malt-abv", Citation: "27 CFR 7.65", Field: "Alcohol content", ApplicationValue: fmt.Sprintf("%g%% ABV", app.AlcoholContent), Status: StatusNeedsReview, Tier: TierJudgment, Note: "no alcohol statement found — mandatory only if alcohol derives from added flavors or a state requires it"}
	}
	r := compareABV("malt-abv", "27 CFR 7.65", app.AlcoholContent, ext.AlcoholContent)
	// "ABV" alone is not an acceptable abbreviation for malt beverages (7.65).
	if bare := strings.Contains(strings.ToLower(ext.AlcoholContent.Value), "abv"); bare && r.Status == StatusPass {
		r.Status = StatusFail
		r.Note = `"ABV" is not an acceptable abbreviation for malt beverages`
	}
	return r
}

func checkWineAppellation(app Application, ext *extraction.Result) RuleResult {
	r := RuleResult{RuleID: "wine-appellation", Citation: "27 CFR 4.25 / 4.34", Field: "Appellation of origin", ApplicationValue: app.Appellation, LabelValue: ext.Appellation.Value, Tier: TierNormalized}
	triggersAppellation := ext.Varietal.Found || ext.VintageDate.Found
	if app.Appellation == "" && !triggersAppellation {
		r.Status = StatusNotApplicable
		return r
	}
	if !ext.Appellation.Found {
		if triggersAppellation {
			r.Status = StatusFail
			r.Note = "appellation is mandatory when the label carries a varietal designation or vintage date"
		} else {
			r.Status = StatusNeedsReview
			r.Tier = TierJudgment
			r.Note = "declared on application but not found on label"
		}
		return r
	}
	return matchText("wine-appellation", "27 CFR 4.25 / 4.34", "Appellation of origin", app.Appellation, ext.Appellation, false)
}

func checkWineVarietal(app Application, ext *extraction.Result) RuleResult {
	appVal := strings.Join(app.Varietals, ", ")
	r := RuleResult{RuleID: "wine-varietal", Citation: "27 CFR 4.23", Field: "Grape varietal(s)", ApplicationValue: appVal, LabelValue: ext.Varietal.Value, Tier: TierNormalized}
	if appVal == "" && !ext.Varietal.Found {
		r.Status = StatusNotApplicable
		return r
	}
	if ext.Varietal.Found && appVal == "" {
		r.Status = StatusFail
		r.Note = "label carries a varietal designation not declared on the application"
		return r
	}
	return matchText("wine-varietal", "27 CFR 4.23", "Grape varietal(s)", appVal, ext.Varietal, false)
}

func checkWineVintage(app Application, ext *extraction.Result) RuleResult {
	r := matchText("wine-vintage", "27 CFR 4.27", "Vintage date", app.VintageDate, ext.VintageDate, false)
	if app.VintageDate == "" && ext.VintageDate.Found {
		r.Status = StatusFail
		r.LabelValue = ext.VintageDate.Value
		r.Note = "label carries a vintage date not declared on the application"
	}
	return r
}

func checkWineSulfites(ext *extraction.Result) RuleResult {
	r := RuleResult{RuleID: "wine-sulfites", Citation: "27 CFR 4.32(e)", Field: "Sulfite declaration", LabelValue: ext.SulfiteDeclaration.Value, Tier: TierJudgment}
	if ext.SulfiteDeclaration.Found {
		r.Status = StatusPass
		r.Tier = TierNormalized
		return demoteLowConfidence(r, ext.SulfiteDeclaration)
	}
	r.Status = StatusNeedsReview
	r.Note = "no sulfite declaration — acceptable only with lab evidence of <10 ppm total sulfur dioxide"
	return r
}

// checkSameFieldOfVision: the prototype cannot know which images share a
// container side, so this is reported for manual verification (DECISIONS.md,
// Deferred).
func checkSameFieldOfVision() RuleResult {
	return RuleResult{RuleID: "spirits-field-of-vision", Citation: "27 CFR 5.63", Field: "Same field of vision", Status: StatusManual, Tier: TierManual, Note: "brand name, class/type, and alcohol content must share one field of vision — verify manually against the physical container sides"}
}

// checkTypeSize: millimeter type sizes are not verifiable from an image of
// unknown physical scale (DECISIONS.md, Deferred).
func checkTypeSize() RuleResult {
	return RuleResult{RuleID: "type-size", Citation: "27 CFR 16.22 / 5.4 / 4.38 / 7.52", Field: "Type size and legibility", Status: StatusManual, Tier: TierManual, Note: "minimum type sizes cannot be measured from an image of unknown scale — verify on the physical label"}
}
