package dto

import "fiber-otdr-fault-localization/backend/internal/model"

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

// Batch analysis outcomes for a single case inside one batch run.
const (
	BatchSucceeded = "succeeded"
	BatchFailed    = "failed"
	BatchSkipped   = "skipped"
)

type BatchAnalyzeCasesRequest struct {
	CaseIDs            []uint  `json:"case_ids" validate:"required,min=1,max=50,unique,dive,gt=0"`
	DistanceToleranceM float64 `json:"distance_tolerance_m" validate:"required,gt=0,lte=1000"`
	LossIncreaseDB     float64 `json:"loss_increase_db" validate:"required,gt=0,lte=20"`
}

type BatchAnalyzeItemResult struct {
	CaseID  uint                    `json:"case_id"`
	Outcome string                  `json:"outcome"`
	Reason  string                  `json:"reason,omitempty"`
	Case    *model.LocalizationCase `json:"case,omitempty"`
}

type BatchAnalyzeResponse struct {
	BatchID   string                   `json:"batch_id"`
	Results   []BatchAnalyzeItemResult `json:"results"`
	Succeeded int                      `json:"succeeded"`
	Failed    int                      `json:"failed"`
	Skipped   int                      `json:"skipped"`
}
