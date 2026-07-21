# SPEC — TTB Label Verification Prototype

A standalone proof-of-concept tool that helps TTB compliance agents verify that alcohol beverage label artwork matches the data in a Certificate of Label Approval (COLA) application and complies with mandatory labeling rules. It informs future procurement decisions; it does not integrate with the existing COLA system.

Sources: `docs/INSTRUCTIONS.md`, `docs/REQUIREMENTS.md`, `docs/INTERVIEW_NOTES.md`, TTB checklists in `docs/checklists/`, sample labels in `docs/labels/`, and `docs/RESOURCES.md`.

---

## 1. Users & Context

- Primary users: TTB compliance agents reviewing label applications (~150,000/year agency-wide).
- Wide range of tech comfort — from near-retirement agents who print emails to recent graduates; half the team is over 50.
- Current process is fully manual: agent visually compares label artwork against application fields using a printed checklist, ~5–10 minutes per simple application.
- The tool assists the agent's decision; the agent remains the decision-maker (nuanced cases require human judgment).
- A prior vendor pilot failed on two fronts the design must not repeat: slow results (30–40 s/label) and reliance on external endpoints blocked by the agency firewall.

## 2. Inputs

- **Application data**: the structured fields declared by the applicant that the label must match. At minimum:
  - Beverage type (wine / malt beverage / distilled spirits) — determines which rule set applies.
  - Brand name.
  - Fanciful name (optional, where applicable).
  - Class/type designation (or statement of composition).
  - Alcohol content.
  - Net contents.
  - Name and address of bottler/producer/importer.
  - Domestic vs. imported (drives country-of-origin and importer rules).
  - Wine-specific where applicable: appellation of origin, grape varietal(s), vintage date.
  - Formula number where a formula is required.
- **Label artwork**: one or more images per application.
  - Labels may be multi-piece (brand label + back/strip/neck labels); mandatory information can be spread across pieces.
  - Container formats vary: bottles, cans, kegs (collars), growlers/crowlers.
  - Image quality varies: ideally flat artwork, but photos may arrive at odd angles, with bad lighting or glare (stretch goal to tolerate; see §8).
- **Batch input**: many applications submitted at once (see §6).

## 3. Outputs

- Per-application verification report:
  - Per-field result: found on label / not found; matches application / mismatch; with the extracted label value shown next to the application value.
  - Compliance-rule results (presence and correctness checks that don't depend on application data, e.g., government warning).
  - Overall status suitable for triage (e.g., pass / needs review / fail) — final wording and thresholds are a design decision.
  - Flagged items requiring human judgment, distinguished from hard failures.
- Batch-level summary that lets an agent triage a large set quickly (which labels are clean, which need attention).

## 4. Functional Requirements

### 4.1 Ingestion
- Upload label image(s) and enter/provide the corresponding application data.
- Support multiple images per application (multi-piece labels).
- Support single-application mode and batch mode.

### 4.2 Extraction
- Read the text content of the label image(s), including stylized fonts, curved/circular layouts (keg collars), and dense small print.
- Capture formatting attributes needed by the rules: capitalization, bold type, separation from surrounding text (see §5.4 — required for the warning statement checks).

### 4.3 Verification
- Compare extracted values against application data field-by-field.
- Apply the beverage-type-specific rule set (§5) including conditional/only-if rules.
- Evaluate rules across all label pieces together (mandatory info may sit on any piece, subject to placement rules like the distilled-spirits same-field-of-vision requirement).

### 4.4 Matching semantics
- Three tiers of matching, applied per rule:
  - **Exact**: government warning wording, punctuation, and casing — no tolerance.
  - **Normalized/equivalent**: case and trivial formatting differences are the same value ("STONE'S THROW" = "Stone's Throw"); acceptable abbreviation variants ("Alc./Vol.", "45% Alc./Vol. (90 Proof)" vs "Alcohol 45% by volume"); unit equivalences (e.g., 1 quart = 32 fl. oz.).
  - **Judgment**: near-matches or ambiguous cases are flagged for the agent rather than silently passed or failed.
- The system must never present a confident "pass" on something it could not actually read or verify — uncertainty is surfaced.

### 4.5 Review UX
- Show results in a way an agent can act on faster than their current manual pass: what was checked, what was found, what needs their eyes.
- Make it visually easy to compare application value vs. label value for each field.
- Allow the agent to see where on the label a value was found (supports trust and quick manual confirmation).

## 5. Verification Rule Catalog

This is the domain logic the system must encode. Citations refer to 27 CFR unless noted.

### 5.1 Common to all beverage types
- **Brand name**: present; matches the "Brand Name" field on the application (5.64 / 4.33 / 7.64).
- **Class/type designation** (or statement of composition / fanciful name + composition): present; consistent with a class/type recognized in the regulations; spelled correctly; separate and apart from other information; no conflicting designations on the same container (e.g., "vodka with natural flavors" vs "vodka"); if a formula is required, statement of composition consistent with the approved formula and formula number selected on the application (5.165/5.141, 4.21/4.34, 7.63).
- **Net contents**: present on the label or blown/embossed/molded into the container; acceptable format and abbreviations; meets an approved standard of fill where applicable (5.70/5.203, 4.37, 7.70).
- **Name and address** of bottler/producer (or importer, for imports): present; matches the name/DBA and city/state on the permit and application; immediately follows a phrase such as "Bottled By" / "Imported By" with no intervening text (5.66–5.68, 4.35, 7.66–7.68).
- **Country of origin** (imports only): statement present and compliant with CBP regulations (19 CFR 134.11).
- **Health warning statement**: see §5.4.
- **Conditional disclosures** (mandatory only if applicable): sulfite declaration at ≥10 ppm total sulfur dioxide; FD&C Yellow #5; cochineal extract/carmine; coloring materials.

### 5.2 Beverage-type-specific — Distilled Spirits (27 CFR part 5)
- **Same field of vision rule**: brand name, class/type designation, and alcohol content must all appear in the same field of vision — one side of the container viewable without turning it (5.63).
- **Alcohol content is mandatory** (5.65); acceptable abbreviations: "Alc.", "Alc", "Vol.", "Vol", "%". Proof may also be stated but must be distinguished from the mandatory percent-by-volume statement and appear in the same field of vision.
- Additional conditional statements: commodity/neutral-spirits statements (5.71), state of distillation for certain whiskies (5.66(f)), treatment with wood (5.73), statement of age in approved formats — mandatory for e.g. whisky aged under 4 years (5.74).

### 5.3 Beverage-type-specific — Wine (27 CFR part 4) and Malt Beverages (27 CFR part 7)
- **Wine — alcohol content is conditional**: "Table Wine" designation suffices for 7–14% ABV with no numeric statement required; a numeric ABV statement is required above 14% (4.36(a)).
- **Wine — appellation of origin**: mandatory if the label has a grape varietal designation, vintage date, semi-generic type designation, or is estate bottled; must appear in direct conjunction with the class/type designation; must match the application's appellation field (4.25/4.34).
- **Wine — varietals**: a varietal on the brand label is treated as the class/type designation and must match the application; two or more varietals require percentages totaling 100% (4.23).
- **Wine — sulfites**: nearly universal in practice; absence of the declaration requires lab evidence of <10 ppm.
- **Malt — alcohol content is conditional**: mandatory only if alcohol derives from added flavors/non-beverage ingredients or a state requires it (7.65); "ABV" is not an acceptable abbreviation; alcohol-by-weight only together with ABV.
- **Malt — net contents in U.S. customary units** (fl. oz., pints, quarts, gallons) with correct unit for the volume — e.g., 32 fl. oz. must be stated as 1 quart (7.70).
- **Malt — additional conditionals**: aspartame declaration in capital letters, separate and apart (7.63(b)(4)); geographically significant style names must be qualified (e.g., "Irish-Style"); non-alcoholic (<0.5% ABV) products have restricted class designations ("malt beverage" / "cereal beverage" / "near beer", no "ale"/"beer" etc.), require "contains less than 0.5% alcohol by volume" adjacent to "non-alcoholic", require the §5051 I.R.C. non-taxable statement if domestic, and do **not** require the government warning.
- **Malt — keg collars**: information may be handwritten except the health warning statement.

### 5.4 Government Health Warning Statement (27 CFR part 16)
- Present on every alcohol beverage label (exception: malt beverages under 0.5% ABV).
- Wording and punctuation must match the prescribed statement **exactly**, word-for-word.
- "GOVERNMENT WARNING" must be in capital letters **and bold type**.
- The "S" in Surgeon and "G" in General must be capitalized.
- Must appear as one continuous statement.
- Must be separate and apart from all other information on the label.
- Type-size/legibility constraints exist (e.g., for 750 mL containers: mandatory text ≥2 mm, warning ≤25 characters per inch) — applicants commonly attempt undersized fonts, altered wording, or wrong casing; these are rejections.

## 6. Batch Processing

- Accept bulk submissions of label applications (peak-season importers submit 200–300 at once; currently processed one at a time).
- Batch results must be triageable: agents work the problem labels first rather than paging through every result.
- Batch throughput matters, but each individual result must still meet the per-label responsiveness expectation when an agent drills in.

## 7. Non-Functional Requirements

### 7.1 Performance
- **Per-label verification results in ~5 seconds or less.** This is adoption-critical: the failed pilot took 30–40 s and agents abandoned it because manual review was faster.

### 7.2 Usability
- Benchmark: "something my 73-year-old mother could figure out." Clean, obvious, no hunting for buttons.
- No training should be required to run a basic check.
- Clear error states (unreadable image, missing fields, processing failure) with plain-language guidance.
- Must not add steps or friction compared to the manual process it assists.

### 7.3 Environment & deployment
- Deployed, publicly accessible working prototype (reviewers must be able to test it).
- Government network context: outbound traffic to many domains is blocked. Dependence on external endpoints is a known deployment risk for any production path — the architecture should acknowledge this constraint even if the prototype itself runs outside the agency network.
- Azure is the agency's cloud platform; FedRAMP would govern any production deployment. Neither is a prototype requirement — context only.

### 7.4 Security & data handling
- Prototype scope: no PII, no sensitive data storage, no federal retention/compliance obligations. "Just don't do anything crazy."

### 7.5 Quality of engineering (evaluation criteria)
- Correctness and completeness of core requirements over ambitious-but-incomplete features.
- Clean, organized code; appropriate technical choices for the scope; documented assumptions and trade-offs.

## 8. Workflows (for workflow diagrams)

- **Single application review**: agent enters application data and uploads label image(s) → system extracts and verifies → agent reviews per-field results → agent flags/overrides judgment calls → agent records outcome (approve / reject / request better image).
- **Batch review**: agent uploads a batch → system processes all applications → agent gets a triage summary → agent drills into flagged applications using the single-review flow.
- **Unreadable/poor image handling**: baseline — system reports the label (or specific fields) as unreadable so the agent can request a better image; stretch — tolerate imperfect photos (angles, lighting, glare) and still extract.
- **Human-judgment loop**: near-matches and ambiguous findings are routed to the agent as flags, never auto-decided.

## 9. Deliverables

- Source code repository with README (setup and run instructions) and brief documentation of approach, tools used, and assumptions.
- Deployed application URL (working, testable prototype).

## 10. Out of Scope

- Integration with the COLA system (separate authorization landscape; explicitly excluded).
- Production federal compliance: FedRAMP, PII policies, document retention.
- Full coverage of every conditional rule in 27 CFR — the checklists are the review baseline; comprehensiveness is explicitly secondary to a correct, complete core.

## 11. Open Considerations (design-time decisions)

- **Application data entry**: manual form vs. structured file upload vs. both. (Batch association of label images to application records is settled — see DECISIONS.md D6a: zip archive + `manifest.json` with explicit per-application image references.)
- **Rule coverage cut line**: which conditional rules (§5.1–5.3) are in the prototype vs. documented as future work. Core fields + warning statement are the minimum; conditionals are candidates for phased scope.
- **Match-confidence model**: how the three matching tiers (§4.4) translate into statuses and thresholds, and whether agents can tune or override them.
- **Formatting verification depth**: verifying bold/caps of the warning is required in spirit; verifying millimeter type sizes and characters-per-inch from an image of unknown scale may be infeasible — decide what is checked vs. flagged as "verify manually."
- **Same-field-of-vision check**: verifying this for distilled spirits requires knowing which images represent which container sides — decide how the prototype captures or approximates this.
- **Batch architecture**: synchronous vs. queued processing, progress feedback, and partial-failure behavior for a 300-label batch.
- **Extraction dependency**: self-hosted vs. external extraction/AI services, weighed against the firewall constraint (§7.3) and the 5-second budget (§7.1).
- **Result persistence**: whether verification reports are stored (and for how long) or ephemeral per session, given the no-sensitive-data posture.
- **Test data**: generate/source additional labels beyond the TTB samples (AI image generation suggested), including deliberate failure cases (wrong warning casing, undersized warning, mismatched ABV, missing net contents).
