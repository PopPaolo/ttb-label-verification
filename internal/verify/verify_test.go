package verify

import (
	"strings"
	"testing"

	"ttb-label-verification/internal/extraction"
)

// found returns a high-confidence extracted field.
func found(value string) extraction.Field {
	return extraction.Field{Found: true, Value: value, Confidence: "high", ImageIndex: 0}
}

func notFound() extraction.Field {
	return extraction.Field{Found: false, ImageIndex: -1}
}

// compliantExtraction is a fully compliant spirits label matching spiritsApp.
func compliantExtraction() *extraction.Result {
	return &extraction.Result{
		BrandName:         found("Stone's Throw"),
		ClassType:         found("Kentucky Straight Bourbon Whiskey"),
		AlcoholContent:    found("45% Alc./Vol. (90 Proof)"),
		NetContents:       found("750 mL"),
		NameAddress:       found("Bottled by Stone's Throw Distillery, Louisville, KY"),
		CountryOfOrigin:   notFound(),
		GovernmentWarning: found(CanonicalWarning),
		WarningFormat: extraction.WarningFormat{
			HeaderAllCaps: true, HeaderBold: true, Continuous: true, SeparateFromOtherText: true,
		},
		FancifulName: notFound(), Appellation: notFound(), Varietal: notFound(),
		VintageDate: notFound(), SulfiteDeclaration: notFound(),
	}
}

func spiritsApp() Application {
	return Application{
		BeverageType:   DistilledSpirits,
		BrandName:      "STONE'S THROW",
		ClassType:      "Kentucky Straight Bourbon Whiskey",
		AlcoholContent: 45,
		NetContents:    "750 mL",
		NameAddress:    "Bottled by Stone's Throw Distillery, Louisville, KY",
	}
}

func ruleByID(t *testing.T, rep Report, id string) RuleResult {
	t.Helper()
	for _, r := range rep.Results {
		if r.RuleID == id {
			return r
		}
	}
	t.Fatalf("rule %q not in report", id)
	return RuleResult{}
}

func TestCompliantSpiritsLabel(t *testing.T) {
	rep := Verify(spiritsApp(), compliantExtraction())
	// Manual checks (field of vision, type size) are surfaced but do not gate
	// the verdict — a clean label stamps pass.
	if rep.Status != StatusPass {
		t.Errorf("overall = %s, want pass (manual checks must not gate)", rep.Status)
	}
	for _, id := range []string{"brand-name", "class-type", "net-contents", "spirits-abv", "warning-wording", "warning-caps", "warning-bold"} {
		if r := ruleByID(t, rep, id); r.Status != StatusPass {
			t.Errorf("%s = %s (%s), want pass", id, r.Status, r.Note)
		}
	}
}

func TestBrandNameMismatchAndNearMatch(t *testing.T) {
	ext := compliantExtraction()
	ext.BrandName = found("Completely Different")
	rep := Verify(spiritsApp(), ext)
	if r := ruleByID(t, rep, "brand-name"); r.Status != StatusFail {
		t.Errorf("mismatched brand = %s, want fail", r.Status)
	}
	if rep.Status != StatusFail {
		t.Errorf("overall = %s, want fail", rep.Status)
	}

	ext.BrandName = found("Stone Throw") // 1 edit after normalization
	rep = Verify(spiritsApp(), ext)
	if r := ruleByID(t, rep, "brand-name"); r.Status != StatusNeedsReview || r.Tier != TierJudgment {
		t.Errorf("near-match brand = %s/%s, want needs_review/judgment", r.Status, r.Tier)
	}
}

func TestLowConfidencePassIsDemoted(t *testing.T) {
	ext := compliantExtraction()
	ext.BrandName.Confidence = "low"
	rep := Verify(spiritsApp(), ext)
	if r := ruleByID(t, rep, "brand-name"); r.Status != StatusNeedsReview {
		t.Errorf("low-confidence match = %s, want needs_review", r.Status)
	}
}

func TestWarningVariants(t *testing.T) {
	app := spiritsApp()

	t.Run("missing", func(t *testing.T) {
		ext := compliantExtraction()
		ext.GovernmentWarning = notFound()
		rep := Verify(app, ext)
		if r := ruleByID(t, rep, "warning-present"); r.Status != StatusFail {
			t.Errorf("missing warning = %s, want fail", r.Status)
		}
	})

	t.Run("title-case header", func(t *testing.T) {
		ext := compliantExtraction()
		ext.GovernmentWarning = found(strings.Replace(CanonicalWarning, "GOVERNMENT WARNING", "Government Warning", 1))
		ext.WarningFormat.HeaderAllCaps = false
		rep := Verify(app, ext)
		if r := ruleByID(t, rep, "warning-caps"); r.Status != StatusFail {
			t.Errorf("title-case header = %s, want fail", r.Status)
		}
		// wording (case-insensitive) still matches
		if r := ruleByID(t, rep, "warning-wording"); r.Status != StatusPass {
			t.Errorf("wording = %s, want pass", r.Status)
		}
	})

	t.Run("altered wording", func(t *testing.T) {
		ext := compliantExtraction()
		ext.GovernmentWarning = found(strings.Replace(CanonicalWarning, "birth defects", "birth issues", 1))
		rep := Verify(app, ext)
		if r := ruleByID(t, rep, "warning-wording"); r.Status != StatusFail {
			t.Errorf("altered wording = %s, want fail", r.Status)
		}
	})

	t.Run("lowercase surgeon general", func(t *testing.T) {
		ext := compliantExtraction()
		ext.GovernmentWarning = found(strings.Replace(CanonicalWarning, "Surgeon General", "surgeon general", 1))
		rep := Verify(app, ext)
		if r := ruleByID(t, rep, "warning-surgeon-general"); r.Status != StatusFail {
			t.Errorf("lowercase surgeon general = %s, want fail", r.Status)
		}
	})

	t.Run("not bold", func(t *testing.T) {
		ext := compliantExtraction()
		ext.WarningFormat.HeaderBold = false
		rep := Verify(app, ext)
		if r := ruleByID(t, rep, "warning-bold"); r.Status != StatusFail {
			t.Errorf("non-bold header = %s, want fail", r.Status)
		}
	})

	t.Run("line-wrapped warning still exact", func(t *testing.T) {
		ext := compliantExtraction()
		wrapped := strings.Replace(CanonicalWarning, "According to the Surgeon General,", "According to the\nSurgeon General,", 1)
		ext.GovernmentWarning = found(wrapped)
		rep := Verify(app, ext)
		if r := ruleByID(t, rep, "warning-wording"); r.Status != StatusPass {
			t.Errorf("wrapped warning = %s, want pass", r.Status)
		}
	})

	t.Run("exempt for low-alcohol malt", func(t *testing.T) {
		ext := compliantExtraction()
		ext.GovernmentWarning = notFound()
		app := Application{BeverageType: Malt, BrandName: "Near Bear", ClassType: "malt beverage", AlcoholContent: 0.4, NetContents: "12 fl oz", NameAddress: "x"}
		rep := Verify(app, ext)
		if r := ruleByID(t, rep, "warning-present"); r.Status != StatusNotApplicable {
			t.Errorf("<0.5%% malt warning = %s, want not_applicable", r.Status)
		}
	})
}

func TestWineABVConditional(t *testing.T) {
	base := Application{BeverageType: Wine, BrandName: "Chateau Test", ClassType: "Table Wine", AlcoholContent: 12.5, NetContents: "750 mL", NameAddress: "Bottled by Chateau Test, Napa, CA"}

	ext := compliantExtraction()
	ext.ClassType = found("Table Wine")
	ext.AlcoholContent = notFound()

	rep := Verify(base, ext)
	if r := ruleByID(t, rep, "wine-abv"); r.Status != StatusPass {
		t.Errorf("table wine 12.5%% without numeric ABV = %s (%s), want pass", r.Status, r.Note)
	}

	over := base
	over.AlcoholContent = 15.5
	rep = Verify(over, ext)
	if r := ruleByID(t, rep, "wine-abv"); r.Status != StatusFail {
		t.Errorf(">14%% without numeric ABV = %s, want fail", r.Status)
	}
}

func TestWineAppellationRequiredWithVarietal(t *testing.T) {
	app := Application{BeverageType: Wine, BrandName: "Chateau Test", ClassType: "Cabernet Sauvignon", AlcoholContent: 14.1, NetContents: "750 mL", NameAddress: "x", Varietals: []string{"Cabernet Sauvignon"}}
	ext := compliantExtraction()
	ext.ClassType = found("Cabernet Sauvignon")
	ext.AlcoholContent = found("14.1% ALC. BY VOL.")
	ext.Varietal = found("Cabernet Sauvignon")
	ext.Appellation = notFound()

	rep := Verify(app, ext)
	if r := ruleByID(t, rep, "wine-appellation"); r.Status != StatusFail {
		t.Errorf("varietal without appellation = %s, want fail", r.Status)
	}
}

func TestWineSulfitesFlaggedWhenAbsent(t *testing.T) {
	app := Application{BeverageType: Wine, BrandName: "Chateau Test", ClassType: "Table Wine", AlcoholContent: 12, NetContents: "750 mL", NameAddress: "x"}
	ext := compliantExtraction()
	ext.ClassType = found("Table Wine")
	ext.SulfiteDeclaration = notFound()
	rep := Verify(app, ext)
	if r := ruleByID(t, rep, "wine-sulfites"); r.Status != StatusNeedsReview {
		t.Errorf("absent sulfites = %s, want needs_review", r.Status)
	}
}

func TestMaltNetContentsUnits(t *testing.T) {
	app := Application{BeverageType: Malt, BrandName: "Hop Test", ClassType: "India Pale Ale", AlcoholContent: 6.5, NameAddress: "x"}
	cases := []struct {
		appNC, labelNC, want string
	}{
		{"12 fl oz", "12 FL. OZ.", StatusPass},
		{"750 mL", "750 mL", StatusFail},   // metric not allowed for malt
		{"1 quart", "32 FL. OZ.", StatusFail}, // must say 1 quart
		{"1 quart", "1 Quart", StatusPass},
	}
	for _, c := range cases {
		a := app
		a.NetContents = c.appNC
		ext := compliantExtraction()
		ext.NetContents = found(c.labelNC)
		rep := Verify(a, ext)
		if r := ruleByID(t, rep, "net-contents"); r.Status != c.want {
			t.Errorf("malt net contents %q = %s (%s), want %s", c.labelNC, r.Status, r.Note, c.want)
		}
	}
}

func TestMaltABVAbbreviation(t *testing.T) {
	app := Application{BeverageType: Malt, BrandName: "Hop Test", ClassType: "India Pale Ale", AlcoholContent: 6.5, NetContents: "12 fl oz", NameAddress: "x"}
	ext := compliantExtraction()
	ext.NetContents = found("12 FL. OZ.")
	ext.AlcoholContent = found("6.5% ABV")
	rep := Verify(app, ext)
	if r := ruleByID(t, rep, "malt-abv"); r.Status != StatusFail {
		t.Errorf("bare ABV abbreviation = %s, want fail", r.Status)
	}
}

func TestSpiritsManualChecksPresent(t *testing.T) {
	rep := Verify(spiritsApp(), compliantExtraction())
	if r := ruleByID(t, rep, "spirits-field-of-vision"); r.Status != StatusManual || r.Tier != TierManual {
		t.Errorf("field of vision = %s/%s, want manual/manual", r.Status, r.Tier)
	}
	if r := ruleByID(t, rep, "type-size"); r.Status != StatusManual || r.Tier != TierManual {
		t.Errorf("type size = %s/%s, want manual/manual", r.Status, r.Tier)
	}
}

func TestNetContentsUnitEquivalence(t *testing.T) {
	app := spiritsApp()
	app.NetContents = "1 quart"
	ext := compliantExtraction()
	ext.NetContents = found("32 FL. OZ.")
	rep := Verify(app, ext)
	if r := ruleByID(t, rep, "net-contents"); r.Status != StatusPass {
		t.Errorf("spirits 1 quart vs 32 fl oz = %s (%s), want pass", r.Status, r.Note)
	}
}
