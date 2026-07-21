package verify

import (
	"math"
	"testing"
)

func TestNormalize(t *testing.T) {
	cases := []struct{ a, b string }{
		{"STONE'S THROW", "Stone's Throw"},
		{"Stone’s  Throw", "stones throw"}, // curly quote, double space
		{"Alc./Vol.", "alc/vol"},
		{"OLD  TOM — GIN", "old tom - gin"},
	}
	for _, c := range cases {
		if Normalize(c.a) != Normalize(c.b) {
			t.Errorf("Normalize(%q)=%q != Normalize(%q)=%q", c.a, Normalize(c.a), c.b, Normalize(c.b))
		}
	}
	if Normalize("Vodka") == Normalize("Gin") {
		t.Error("distinct values must not normalize equal")
	}
}

func TestParseABV(t *testing.T) {
	cases := []struct {
		in   string
		want float64
		ok   bool
	}{
		{"45% Alc./Vol. (90 Proof)", 45, true},
		{"ALC. 13.5% BY VOL.", 13.5, true},
		{"Alcohol 45% by volume", 45, true},
		{"90 Proof", 45, true},
		{"5.0% ALC/VOL", 5, true},
		{"Table Wine", 0, false},
	}
	for _, c := range cases {
		got, ok := ParseABV(c.in)
		if ok != c.ok || (ok && math.Abs(got-c.want) > 0.001) {
			t.Errorf("ParseABV(%q) = %v,%v; want %v,%v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestParseNetContents(t *testing.T) {
	quart, ok1 := ParseNetContents("1 Quart")
	floz, ok2 := ParseNetContents("32 FL. OZ.")
	if !ok1 || !ok2 {
		t.Fatal("expected both statements to parse")
	}
	if math.Abs(quart.Milliliters-floz.Milliliters) > 1 {
		t.Errorf("1 quart (%.1f mL) should equal 32 fl oz (%.1f mL)", quart.Milliliters, floz.Milliliters)
	}

	ml, ok := ParseNetContents("750 mL")
	if !ok || !ml.Metric || ml.Milliliters != 750 {
		t.Errorf("750 mL parsed as %+v, ok=%v", ml, ok)
	}

	combo, ok := ParseNetContents("1 Pint 8 FL. OZ.")
	if !ok || math.Abs(combo.Milliliters-(473.176+8*29.5735)) > 1 {
		t.Errorf("1 pint 8 fl oz parsed as %+v, ok=%v", combo, ok)
	}

	if _, ok := ParseNetContents("a bottle"); ok {
		t.Error("unparseable statement must not report ok")
	}
}

func TestLevenshtein(t *testing.T) {
	if d := Levenshtein("kitten", "sitting"); d != 3 {
		t.Errorf("kitten/sitting = %d, want 3", d)
	}
	if d := Levenshtein("same", "same"); d != 0 {
		t.Errorf("identical = %d, want 0", d)
	}
}
