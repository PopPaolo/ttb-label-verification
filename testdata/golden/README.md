# Golden label set (DECISIONS.md D12)

Six synthetic label applications with known ground truth, covering all three
beverage types and each verdict the engine can reach. Labels are authored as
SVG (so the ground truth — wording, casing, bold, deliberate defects — is
exact and reviewable in a diff) and rendered to PNG for upload. The set doubles
as the demo dataset: upload any case through the UI, or `golden-batch.zip`
through batch mode.

Each case directory contains:

- `*.svg` — label artwork source (edit these, then rerun `./regen.sh`)
- `*.png` — rendered artwork, what actually gets uploaded
- `application.json` — the COLA application data, ready to paste or batch
- `expected.json` — expected overall verdict and per-rule statuses

## Cases

| Case | Beverage | Deliberate condition | Expected overall |
| --- | --- | --- | --- |
| `wine-clean` | wine | none — fully compliant, two label pieces (front + back; warning, address, sulfites on the back) | `pass` |
| `wine-near-match` | wine | brand typo: label says SLIVER Creek, application says Silver Creek (edit distance 2 → judgment tier) | `needs_review` |
| `malt-clean` | malt | none — fully compliant single piece | `pass` |
| `malt-brand-mismatch` | malt | label brand IRON MEADOW ≠ application Golden Meadow | `fail` |
| `spirits-warning-fail` | distilled spirits | health warning drops "a car" — exact-tier wording, no tolerance | `fail` |
| `spirits-missing-net` | distilled spirits | imported vodka with no net contents anywhere; also exercises country-of-origin + Imported By | `fail` |

The type-size check (and same-field-of-vision for spirits) cannot be verified
from an image of unknown scale; those rows carry the non-gating `manual`
status ("Verify manually" in the UI) on every report and never demote the
overall verdict.

## Caveats

- `expected.json` assumes the extractor reads the (deliberately crisp) labels
  correctly at medium-or-better confidence. A low-confidence read demotes a
  pass to `needs_review` by design, so treat per-rule expectations as the
  target for a well-behaved extraction, not a hard API contract.
- Regenerating requires macOS (`qlmanage`) and Python 3, both stock.
