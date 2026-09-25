package dto

type CreateCaseRequest struct {
	RouteID            uint    `json:"route_id" validate:"required,gt=0"`
	BaselineTraceID    uint    `json:"baseline_trace_id" validate:"required,gt=0"`
	CurrentTraceID     uint    `json:"current_trace_id" validate:"required,gt=0,nefield=BaselineTraceID"`
	DistanceToleranceM float64 `json:"distance_tolerance_m" validate:"omitempty,gt=0,lte=1000"`
	LossIncreaseDB     float64 `json:"loss_increase_db" validate:"omitempty,gt=0,lte=20"`
}

type AnalyzeCaseRequest struct {
	DistanceToleranceM float64 `json:"distance_tolerance_m" validate:"omitempty,gt=0,lte=1000"`
	LossIncreaseDB     float64 `json:"loss_increase_db" validate:"omitempty,gt=0,lte=20"`
}

type BatchAnalyzeCasesRequest struct {
	CaseIDs            []uint  `json:"case_ids" validate:"required,min=1,max=100,dive,gt=0"`
	DistanceToleranceM float64 `json:"distance_tolerance_m" validate:"gt=0,lte=1000"`
	LossIncreaseDB     float64 `json:"loss_increase_db" validate:"gt=0,lte=20"`
}

type BatchCaseConflict struct {
	CaseID uint   `json:"case_id"`
	Reason string `json:"reason"`
	Status string `json:"status,omitempty"`
}

type BatchCaseResult struct {
	CaseID             uint     `json:"case_id"`
	RouteID            uint     `json:"route_id"`
	Outcome            string   `json:"outcome"`
	CaseStatus         string   `json:"case_status"`
	Version            uint     `json:"version"`
	EstimatedDistanceM *float64 `json:"estimated_distance_m,omitempty"`
	UncertaintyM       *float64 `json:"uncertainty_m,omitempty"`
	DifferenceCount    int      `json:"difference_count"`
	ErrorCode          string   `json:"error_code,omitempty"`
	ErrorMessage       string   `json:"error_message,omitempty"`
}

type BatchAnalyzeCasesResponse struct {
	BatchID   string            `json:"batch_id"`
	Total     int               `json:"total"`
	Succeeded int               `json:"succeeded"`
	Failed    int               `json:"failed"`
	Results   []BatchCaseResult `json:"results"`
}

const (
	BatchOutcomeSuccess = "succeeded"
	BatchOutcomeFailed  = "failed"

	BatchConflictNotFound  = "NOT_FOUND"
	BatchConflictNotDraft  = "NOT_DRAFT"
	BatchConflictDuplicate = "DUPLICATE_ID"
)

type ConfirmCaseRequest struct {
	Conclusion         string  `json:"conclusion" validate:"required,min=10,max=2000"`
	EstimatedDistanceM float64 `json:"estimated_distance_m" validate:"gte=0"`
	UncertaintyM       float64 `json:"uncertainty_m" validate:"gte=0,lte=5000"`
	Version            uint    `json:"version" validate:"required,gt=0"`
}

type CloseCaseRequest struct {
	Version uint `json:"version" validate:"required,gt=0"`
}

type CaseQuery struct {
	RouteID  *uint
	Status   string
	Page     int
	PageSize int
}

type CaseParameters struct {
	DistanceToleranceM float64 `json:"distance_tolerance_m"`
	LossIncreaseDB     float64 `json:"loss_increase_db"`
}
