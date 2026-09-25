export const CASE_STATUSES = ['draft', 'analyzing', 'pending_review', 'confirmed', 'closed'] as const
export type CaseStatus = (typeof CASE_STATUSES)[number]

export interface LocalizationCase {
  id: number
  route_id: number
  baseline_trace_id: number
  current_trace_id: number
  case_status: CaseStatus
  estimated_distance_m?: number
  uncertainty_m?: number
  conclusion: string
  reviewer_id?: number
  closed_at?: string
  analysis_error: string
  parameters_json: { distance_tolerance_m: number; loss_increase_db: number }
  differences_json: Difference[]
  version: number
  created_by: number
  created_at: string
  updated_at: string
}

export interface Difference {
  kind: 'new' | 'disappeared' | 'loss_increased'
  baseline_event_id?: number
  current_event_id?: number
  distance_m: number
  loss_delta_db: number
  confidence: number
}

export type BatchOutcome = 'succeeded' | 'failed'

export interface BatchCaseResult {
  case_id: number
  route_id?: number
  outcome: BatchOutcome
  case_status: CaseStatus
  version: number
  estimated_distance_m?: number
  uncertainty_m?: number
  difference_count: number
  error_code?: string
  error_message?: string
}

export interface BatchAnalyzeResponse {
  batch_id: string
  total: number
  succeeded: number
  failed: number
  results: BatchCaseResult[]
}

export interface BatchCaseConflict {
  case_id: number
  reason: 'NOT_FOUND' | 'NOT_DRAFT' | 'DUPLICATE_ID'
  status?: string
}

export const caseStatusLabel: Record<CaseStatus, string> = {
  draft: '草稿', analyzing: '分析中', pending_review: '待复核', confirmed: '已确认', closed: '已关闭',
}
