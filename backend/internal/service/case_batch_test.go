package service

import (
	"strings"
	"testing"

	"fiber-otdr-fault-localization/backend/internal/constants"
	"fiber-otdr-fault-localization/backend/internal/dto"
	"fiber-otdr-fault-localization/backend/internal/model"
	"fiber-otdr-fault-localization/backend/internal/repository"
	"gorm.io/datatypes"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newBatchTestService(t *testing.T) *CaseService {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:batch-analyze?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.LocalizationCase{}, &model.EventMarker{}, &model.AuditLog{}); err != nil {
		t.Fatal(err)
	}
	return NewCaseService(repository.NewStore(db))
}

func seedCase(t *testing.T, s *CaseService, status constants.CaseStatus, currentTraceID uint) model.LocalizationCase {
	t.Helper()
	item := model.LocalizationCase{RouteID: 1, BaselineTraceID: 1, CurrentTraceID: currentTraceID, CaseStatus: status, ParametersJSON: datatypes.JSON([]byte(`{}`)), DifferencesJSON: datatypes.JSON([]byte(`[]`)), Version: 1, CreatedBy: 1}
	if err := s.store.DB.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	return item
}

func seedEvent(t *testing.T, s *CaseService, traceID uint, distance float64) {
	t.Helper()
	event := model.EventMarker{TraceID: traceID, DistanceM: distance, EventType: constants.EventSplice, InsertionLossDB: 0.2, Confidence: 0.9, AlgorithmEventType: constants.EventSplice, AlgorithmDistanceM: distance, AlgorithmInsertionLossDB: 0.2}
	if err := s.store.DB.Create(&event).Error; err != nil {
		t.Fatal(err)
	}
}

func TestBatchAnalyzeReportsPerCaseOutcomes(t *testing.T) {
	svc := newBatchTestService(t)
	actor := Actor{ID: 1, Username: "analyst", Role: constants.RoleAnalyst, RequestID: "req-batch"}

	succeeding := seedCase(t, svc, constants.CaseDraft, 2)
	failing := seedCase(t, svc, constants.CaseDraft, 3) // current trace 3 has no events
	confirmed := seedCase(t, svc, constants.CaseConfirmed, 2)
	seedEvent(t, svc, 1, 1200)
	seedEvent(t, svc, 2, 1210)

	response, err := svc.BatchAnalyze(dto.BatchAnalyzeCasesRequest{
		CaseIDs:            []uint{succeeding.ID, failing.ID, confirmed.ID, 9999},
		DistanceToleranceM: 30,
		LossIncreaseDB:     0.5,
	}, actor)
	if err != nil {
		t.Fatal(err)
	}
	if response.BatchID == "" || response.Succeeded != 1 || response.Failed != 1 || response.Skipped != 2 {
		t.Fatalf("unexpected batch summary: %+v", response)
	}
	outcomes := map[uint]dto.BatchAnalyzeItemResult{}
	for _, result := range response.Results {
		outcomes[result.CaseID] = result
	}
	if got := outcomes[succeeding.ID]; got.Outcome != dto.BatchSucceeded || got.Case == nil || got.Case.CaseStatus != constants.CasePendingReview {
		t.Fatalf("expected successful recompute to pending_review: %+v", got)
	}
	if got := outcomes[failing.ID]; got.Outcome != dto.BatchFailed || !strings.Contains(got.Reason, "require detected events") {
		t.Fatalf("expected failure with reason: %+v", got)
	}
	if got := outcomes[confirmed.ID]; got.Outcome != dto.BatchSkipped || !strings.Contains(got.Reason, "confirmed") {
		t.Fatalf("expected confirmed case to be skipped with reason: %+v", got)
	}
	if got := outcomes[9999]; got.Outcome != dto.BatchSkipped || !strings.Contains(got.Reason, "does not exist") {
		t.Fatalf("expected missing case to be skipped: %+v", got)
	}

	reloaded, err := svc.store.Cases.Get(failing.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.CaseStatus != constants.CaseDraft || reloaded.AnalysisError == "" {
		t.Fatalf("failed case must roll back to draft with an error: status=%s error=%q", reloaded.CaseStatus, reloaded.AnalysisError)
	}
	if reloaded, err = svc.store.Cases.Get(confirmed.ID); err != nil || reloaded.CaseStatus != constants.CaseConfirmed {
		t.Fatalf("confirmed case must stay untouched: status=%s err=%v", reloaded.CaseStatus, err)
	}

	var audits []model.AuditLog
	if err := svc.store.DB.Where("action LIKE ?", "case.analysis_%").Order("id").Find(&audits).Error; err != nil {
		t.Fatal(err)
	}
	if len(audits) != 4 { // started+completed for the success, started+failed for the rollback
		t.Fatalf("expected analysis audit entries, got %d", len(audits))
	}
	for _, entry := range audits {
		if !strings.Contains(entry.After, response.BatchID) {
			t.Fatalf("audit entry %s missing batch id: %s", entry.Action, entry.After)
		}
	}
}
