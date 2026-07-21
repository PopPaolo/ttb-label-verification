// Mirrors the Go API types (internal/verify, internal/extraction, internal/batch).

export type BeverageType = 'wine' | 'malt' | 'distilled_spirits'

export interface Application {
  beverage_type: BeverageType
  brand_name: string
  fanciful_name?: string
  class_type: string
  alcohol_content: number
  net_contents: string
  name_address: string
  imported: boolean
  appellation?: string
  varietals?: string[]
  vintage_date?: string
  formula_number?: string
}

export type RuleStatus = 'pass' | 'fail' | 'needs_review' | 'not_applicable' | 'manual'

export interface RuleResult {
  rule_id: string
  citation: string
  field: string
  application_value: string
  label_value: string
  status: RuleStatus
  tier: 'exact' | 'normalized' | 'judgment' | 'manual'
  note?: string
}

export interface ExtractedField {
  found: boolean
  value: string
  confidence: 'high' | 'medium' | 'low'
  image_index: number
  location: string
}

export interface ExtractionResult {
  brand_name: ExtractedField
  fanciful_name: ExtractedField
  class_type: ExtractedField
  alcohol_content: ExtractedField
  net_contents: ExtractedField
  name_address: ExtractedField
  country_of_origin: ExtractedField
  government_warning: ExtractedField
  warning_format: {
    header_all_caps: boolean
    header_bold: boolean
    continuous: boolean
    separate_from_other_text: boolean
  }
  appellation: ExtractedField
  varietal: ExtractedField
  vintage_date: ExtractedField
  sulfite_declaration: ExtractedField
  unreadable: boolean
  notes: string
}

export interface VerifyResponse {
  status: RuleStatus
  results: RuleResult[]
  extraction: ExtractionResult | null
  timings_ms: Record<string, number>
}

export type ItemStatus = 'queued' | 'processing' | 'done' | 'failed'

export interface BatchItemSummary {
  id: string
  status: ItemStatus
  report_status?: RuleStatus
  error?: string
}

export interface BatchSummary {
  id: string
  created_at: string
  total: number
  pending: number
  counts: Record<string, number>
  items: BatchItemSummary[]
}

export interface BatchItem {
  id: string
  status: ItemStatus
  report_status?: RuleStatus
  error?: string
  report?: { status: RuleStatus; results: RuleResult[] }
  extraction?: ExtractionResult
}
