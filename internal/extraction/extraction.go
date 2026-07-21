// Package extraction reads label artwork with a vision model and returns the
// structured fields the verification engine compares (DECISIONS.md D4). The
// Extractor interface isolates the external API so a self-hosted backend could
// be substituted without touching the engine.
package extraction

import "context"

// Image is one label piece uploaded with an application.
type Image struct {
	Data      []byte
	MediaType string // e.g. "image/jpeg"
}

// Field is one extracted label value with provenance for the review UI
// (SPEC §4.5: agents can see where on the label a value was found).
type Field struct {
	Found      bool   `json:"found"`
	Value      string `json:"value"`
	Confidence string `json:"confidence"` // high | medium | low
	ImageIndex int    `json:"image_index"`
	Location   string `json:"location"`
}

// WarningFormat captures the formatting attributes 27 CFR part 16 regulates
// beyond wording (SPEC §5.4). These are visual judgments by the model.
type WarningFormat struct {
	HeaderAllCaps         bool `json:"header_all_caps"`
	HeaderBold            bool `json:"header_bold"`
	Continuous            bool `json:"continuous"`
	SeparateFromOtherText bool `json:"separate_from_other_text"`
}

// Result is everything extracted from one application's label pieces.
type Result struct {
	BrandName          Field         `json:"brand_name"`
	FancifulName       Field         `json:"fanciful_name"`
	ClassType          Field         `json:"class_type"`
	AlcoholContent     Field         `json:"alcohol_content"`
	NetContents        Field         `json:"net_contents"`
	NameAddress        Field         `json:"name_address"`
	CountryOfOrigin    Field         `json:"country_of_origin"`
	GovernmentWarning  Field         `json:"government_warning"`
	WarningFormat      WarningFormat `json:"warning_format"`
	Appellation        Field         `json:"appellation"`
	Varietal           Field         `json:"varietal"`
	VintageDate        Field         `json:"vintage_date"`
	SulfiteDeclaration Field         `json:"sulfite_declaration"`
	Unreadable         bool          `json:"unreadable"`
	Notes              string        `json:"notes"`
}

type Extractor interface {
	Extract(ctx context.Context, images []Image) (*Result, error)
}
