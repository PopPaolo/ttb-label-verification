import type { ExtractedField, ExtractionResult, RuleResult, RuleStatus } from './types'

const statusLabel: Record<RuleStatus, string> = {
  pass: 'Pass',
  fail: 'Fail',
  needs_review: 'Needs review',
  not_applicable: 'N/A',
  manual: 'Verify manually',
}

// Which extracted field backs each rule, so every row can say where on the
// label the value was read from (SPEC §4.5).
const ruleSource: Record<string, keyof ExtractionResult> = {
  'brand-name': 'brand_name',
  'class-type': 'class_type',
  'net-contents': 'net_contents',
  'name-address': 'name_address',
  'country-of-origin': 'country_of_origin',
  'warning-present': 'government_warning',
  'warning-wording': 'government_warning',
  'warning-separate': 'government_warning',
  'warning-continuous': 'government_warning',
  'spirits-abv': 'alcohol_content',
  'wine-abv': 'alcohol_content',
  'malt-abv': 'alcohol_content',
  'wine-appellation': 'appellation',
  'wine-varietal': 'varietal',
  'wine-sulfites': 'sulfite_declaration',
}

function whereFound(rule: RuleResult, extraction: ExtractionResult | null, imageNames: string[]) {
  const key = ruleSource[rule.rule_id]
  if (!extraction || !key) return null
  const field = extraction[key] as ExtractedField
  if (!field.found) return null
  const name = imageNames[field.image_index]
  const image = name ? `image ${field.image_index + 1} (${name})` : `image ${field.image_index + 1}`
  const location = field.location ? ` — ${field.location}` : ''
  return `Found on ${image}${location} · ${field.confidence} confidence`
}

interface Props {
  status: RuleStatus
  results: RuleResult[]
  extraction: ExtractionResult | null
  timingsMS?: Record<string, number>
  imageNames?: string[]
}

export default function ReportView({ status, results, extraction, timingsMS, imageNames = [] }: Props) {
  return (
    <section aria-label="Verification report">
      <div className="report-head">
        <span className={`stamp ${status}`} role="status">
          {statusLabel[status]}
        </span>
        {timingsMS && (
          <span className="report-meta">
            checked in {(timingsMS.total / 1000).toFixed(1)} s · label read in{' '}
            {(timingsMS.extraction / 1000).toFixed(1)} s
          </span>
        )}
      </div>

      <div className="rules">
        <div className="rules-head" aria-hidden="true">
          <span>Check</span>
          <span>Application says</span>
          <span>Label shows</span>
          <span>Result</span>
        </div>
        {results.map((r) => {
          const where = r.status === 'not_applicable' ? null : whereFound(r, extraction, imageNames)
          return (
            <div key={r.rule_id} className={`rule-row${r.status === 'not_applicable' ? ' na' : ''}`}>
              <div className="rule-name">
                {r.field}
                <span className="cite">{r.citation}</span>
              </div>
              <div className="value" data-col="Application says">
                {r.application_value || <span className="dash">—</span>}
              </div>
              <div className="value" data-col="Label shows">
                {r.label_value || (
                  <span className="dash">
                    {r.status === 'fail' || r.status === 'needs_review' ? 'not found' : '—'}
                  </span>
                )}
                {where && <span className="where">{where}</span>}
              </div>
              <span className={`chip ${r.status}`}>{statusLabel[r.status]}</span>
              {r.note && <p className="rule-note">{r.note}</p>}
            </div>
          )
        })}
      </div>

      {extraction?.notes && (
        <p className="extract-notes">Notes from reading the label: {extraction.notes}</p>
      )}
    </section>
  )
}
