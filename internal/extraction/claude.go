package extraction

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

const systemPrompt = `You extract label information from alcohol beverage label artwork for TTB compliance review.
Rules:
- Transcribe text exactly as printed. Never correct spelling, casing, or punctuation — differences from the expected text are exactly what the reviewer needs to see.
- The government health warning statement must be transcribed verbatim, character for character, including casing and punctuation.
- A label may span multiple images (front, back, neck, strip). A field found on any image counts as found; report the 0-based index of the image it appears on.
- If a field is not present, set found=false, value="", image_index=-1. If text is present but hard to read, still transcribe your best reading and set confidence to "low" — never silently guess with high confidence.
- For each found field, set location to a brief description of where on that label piece it appears (e.g. "top center", "lower third, inside bordered box").
- Formatting attributes (bold, capitalization, separation from other text) are visual judgments; report what the image shows.`

const userPrompt = `Extract the following from the attached label image(s):
- brand_name: the brand name (the most prominent product identity).
- fanciful_name: a fanciful/product name distinct from the brand name, if any.
- class_type: the class/type designation or statement of composition (e.g. "Cabernet Sauvignon", "Kentucky Straight Bourbon Whiskey", "India Pale Ale", "Vodka with Natural Flavors").
- alcohol_content: the alcohol content statement exactly as printed (e.g. "45% Alc./Vol. (90 Proof)", "ALC. 13.5% BY VOL.").
- net_contents: the net contents statement exactly as printed (e.g. "750 mL", "12 FL. OZ.").
- name_address: the name-and-address statement including its introductory phrase (e.g. "Bottled by Acme Winery, Napa, CA").
- country_of_origin: a country-of-origin statement (e.g. "Product of France"), if any.
- government_warning: the full government health warning statement, verbatim.
- warning_format: formatting of the warning statement (header all-caps, header bold, one continuous statement, separate and apart from other text).
- appellation: wine appellation of origin, if any.
- varietal: grape varietal designation(s), if any.
- vintage_date: wine vintage date, if any.
- sulfite_declaration: a sulfite declaration (e.g. "CONTAINS SULFITES"), if any.
- unreadable: true only if the image quality makes the label substantially unreadable.
- notes: anything a compliance reviewer should know (glare, cropped edges, conflicting statements, handwriting).`

// ClaudeExtractor implements Extractor against the Anthropic Messages API with
// a structured-output schema, so responses are guaranteed parseable (D4).
type ClaudeExtractor struct {
	client  anthropic.Client
	model   anthropic.Model
	effort  anthropic.OutputConfigEffort
	timeout time.Duration
}

func NewClaudeExtractor(apiKey, model, effort string, timeout time.Duration) *ClaudeExtractor {
	return &ClaudeExtractor{
		client:  anthropic.NewClient(option.WithAPIKey(apiKey)),
		model:   anthropic.Model(model),
		effort:  anthropic.OutputConfigEffort(effort),
		timeout: timeout,
	}
}

func (c *ClaudeExtractor) Extract(ctx context.Context, images []Image) (*Result, error) {
	if len(images) == 0 {
		return nil, fmt.Errorf("no images provided")
	}
	if c.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}

	blocks := make([]anthropic.ContentBlockParamUnion, 0, len(images)+1)
	for _, img := range images {
		blocks = append(blocks, anthropic.NewImageBlockBase64(img.MediaType, base64.StdEncoding.EncodeToString(img.Data)))
	}
	blocks = append(blocks, anthropic.NewTextBlock(userPrompt))

	adaptive := anthropic.ThinkingConfigAdaptiveParam{}
	resp, err := c.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     c.model,
		MaxTokens: 16000,
		System:    []anthropic.TextBlockParam{{Text: systemPrompt}},
		Thinking:  anthropic.ThinkingConfigParamUnion{OfAdaptive: &adaptive},
		OutputConfig: anthropic.OutputConfigParam{
			Effort: c.effort,
			Format: anthropic.JSONOutputFormatParam{Schema: resultSchema()},
		},
		Messages: []anthropic.MessageParam{anthropic.NewUserMessage(blocks...)},
	})
	if err != nil {
		return nil, fmt.Errorf("extraction request: %w", err)
	}

	switch resp.StopReason {
	case anthropic.StopReasonRefusal:
		return nil, fmt.Errorf("extraction refused by model safety system")
	case anthropic.StopReasonMaxTokens:
		return nil, fmt.Errorf("extraction output truncated (max_tokens)")
	}

	for _, block := range resp.Content {
		if text, ok := block.AsAny().(anthropic.TextBlock); ok {
			var result Result
			if err := json.Unmarshal([]byte(text.Text), &result); err != nil {
				return nil, fmt.Errorf("parsing extraction output: %w", err)
			}
			return &result, nil
		}
	}
	return nil, fmt.Errorf("extraction response contained no text block")
}

// resultSchema is the structured-output JSON schema mirroring Result. The
// repeated per-field object lives in $defs and is referenced — inlining it
// twelve times made the compiled constrained-output grammar exceed the API's
// size limit ("compiled grammar is too large"). Field meanings are described
// in the user prompt, not the schema, for the same reason.
func resultSchema() map[string]any {
	ref := map[string]any{"$ref": "#/$defs/label_field"}
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"$defs": map[string]any{
			"label_field": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"found", "value", "confidence", "image_index", "location"},
				"properties": map[string]any{
					"found":       map[string]any{"type": "boolean"},
					"value":       map[string]any{"type": "string"},
					"confidence":  map[string]any{"type": "string", "enum": []string{"high", "medium", "low"}},
					"image_index": map[string]any{"type": "integer"},
					"location":    map[string]any{"type": "string"},
				},
			},
		},
		"required": []string{
			"brand_name", "fanciful_name", "class_type", "alcohol_content", "net_contents",
			"name_address", "country_of_origin", "government_warning", "warning_format",
			"appellation", "varietal", "vintage_date", "sulfite_declaration", "unreadable", "notes",
		},
		"properties": map[string]any{
			"brand_name":         ref,
			"fanciful_name":      ref,
			"class_type":         ref,
			"alcohol_content":    ref,
			"net_contents":       ref,
			"name_address":       ref,
			"country_of_origin":  ref,
			"government_warning": ref,
			"warning_format": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"header_all_caps", "header_bold", "continuous", "separate_from_other_text"},
				"properties": map[string]any{
					"header_all_caps":          map[string]any{"type": "boolean"},
					"header_bold":              map[string]any{"type": "boolean"},
					"continuous":               map[string]any{"type": "boolean"},
					"separate_from_other_text": map[string]any{"type": "boolean"},
				},
			},
			"appellation":         ref,
			"varietal":            ref,
			"vintage_date":        ref,
			"sulfite_declaration": ref,
			"unreadable":          map[string]any{"type": "boolean"},
			"notes":               map[string]any{"type": "string"},
		},
	}
}
