package service

import (
	"encoding/json"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"

	"fiber-otdr-fault-localization/backend/internal/constants"
	"fiber-otdr-fault-localization/backend/internal/dto"
	"fiber-otdr-fault-localization/backend/internal/model"
	"fiber-otdr-fault-localization/backend/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var batchTestSeq uint64

func newBatchTestStore(t *testing.T) *repository.Store {
	t.Helper()
	seq := atomic.AddUint64(&batchTestSeq, 1)
	db, err := gorm.Open(sqlite.Open("file:batch-case-"+itoa(seq)+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.FiberRoute{}, &model.TraceCapture{}, &model.EventMarker{}, &model.LocalizationCase{}, &model.AuditLog{}); err != nil {
		t.Fatal(err)
	}
	return repository.NewStore(db)
}

func itoa(v uint64) string {
	if v == 0 {
		return "0"
	}
	digits := []byte{}
	for v > 0 {
		digits = append([]byte{byte('0' + v%10)}, digits...)
		v /= 10
	}
	return string(digits)
}

func batchActor() Actor {
	return Actor{ID: 1, Username: "analyst", Role: constants.RoleAnalyst, RequestID: "req-batch-case"}
}

func seedCase(t *testing.T, store *repository.Store, status constants.CaseStatus) model.LocalizationCase {
	t.Helper()
	traceBase := uint(atomic.AddUint64(&batchTestSeq, 1) * 100)
	baseTrace, currentTrace := traceBase+1, traceBase+2
	params, _ := json.Marshal(dto.CaseParameters{DistanceToleranceM: 25, LossIncreaseDB: 0.5})
	item := model.LocalizationCase{RouteID: 1, BaselineTraceID: baseTrace, CurrentTraceID: currentTrace, CaseStatus: status, ParametersJSON: params, DifferencesJSON: []byte("[]"), Version: 1, CreatedBy: 1}
	if err := store.Cases.Create(&item); err != nil {
		t.Fatal(err)
	}
	return item
}

func seedEvents(t *testing.T, store *repository.Store, traceID uint, distances []float64) {
	t.Helper()
	events := make([]model.EventMarker, 0, len(distances))
	for _, distance := range distances {
		events = append(events, model.EventMarker{TraceID: traceID, DistanceM: distance, EventType: constants.EventSplice, InsertionLossDB: 0.3, ReflectanceDB: -40, Confidence: 0.9, AlgorithmEventType: constants.EventSplice, AlgorithmDistanceM: distance, AlgorithmInsertionLossDB: 0.3})
	}
	if err := store.DB.Create(&events).Error; err != nil {
		t.Fatal(err)
	}
}

func TestBatchAnalyzeSucceedsForDraftCasesAndAppliesParameters(t *testing.T) {
	store := newBatchTestStore(t)
	svc := NewCaseService(store)
	first := seedCase(t, store, constants.CaseDraft)
	second := seedCase(t, store, constants.CaseDraft)
	seedEvents(t, store, first.BaselineTraceID, []float64{100, 500})
	seedEvents(t, store, first.CurrentTraceID, []float64{102, 505, 900})
	seedEvents(t, store, second.BaselineTraceID, []float64{200})
	seedEvents(t, store, second.CurrentTraceID, []float64{202})

	response, err := svc.BatchAnalyze(dto.BatchAnalyzeCasesRequest{CaseIDs: []uint{first.ID, second.ID}, DistanceToleranceM: 30, LossIncreaseDB: 1.2}, batchActor())
	if err != nil {
		t.Fatalf("batch analyze failed: %v", err)
	}
	if response.Total != 2 || response.Succeeded != 2 || response.Failed != 0 {
		t.Fatalf("unexpected batch summary: %+v", response)
	}
	for _, result := range response.Results {
		if result.Outcome != dto.BatchOutcomeSuccess || result.CaseStatus != string(constants.CasePendingReview) {
			t.Fatalf("unexpected per-case result: %+v", result)
		}
		reloaded, err := store.Cases.Get(result.CaseID)
		if err != nil {
			t.Fatal(err)
		}
		if reloaded.CaseStatus != constants.CasePendingReview {
			t.Fatalf("case %d not pending review: %s", result.CaseID, reloaded.CaseStatus)
		}
		var params dto.CaseParameters
		if err := json.Unmarshal(reloaded.ParametersJSON, &params); err != nil {
			t.Fatal(err)
		}
		if params.DistanceToleranceM != 30 || params.LossIncreaseDB != 1.2 {
			t.Fatalf("batch parameters not persisted: %+v", params)
		}
	}
	var audits []model.AuditLog
	if err := store.DB.Where("action IN ?", []string{"case.batch_analysis_started", "case.batch_analysis_completed", "case.analysis_started", "case.analysis_completed"}).Find(&audits).Error; err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, entry := range audits {
		counts[entry.Action]++
		if entry.RequestID != "req-batch-case" {
			t.Fatalf("audit missing request id: %+v", entry)
		}
	}
	if counts["case.batch_analysis_started"] != 1 || counts["case.batch_analysis_completed"] != 1 || counts["case.analysis_started"] != 2 || counts["case.analysis_completed"] != 2 {
		t.Fatalf("unexpected audit counts: %v", counts)
	}
}

func TestBatchAnalyzeReportsFailureAndReturnsCaseToDraft(t *testing.T) {
	store := newBatchTestStore(t)
	svc := NewCaseService(store)
	ready := seedCase(t, store, constants.CaseDraft)
	missingEvents := seedCase(t, store, constants.CaseDraft)
	seedEvents(t, store, ready.BaselineTraceID, []float64{100})
	seedEvents(t, store, ready.CurrentTraceID, []float64{101})

	response, err := svc.BatchAnalyze(dto.BatchAnalyzeCasesRequest{CaseIDs: []uint{ready.ID, missingEvents.ID}, DistanceToleranceM: 30, LossIncreaseDB: 1.2}, batchActor())
	if err != nil {
		t.Fatalf("batch should return per-case failures, got: %v", err)
	}
	if response.Succeeded != 1 || response.Failed != 1 {
		t.Fatalf("unexpected batch summary: %+v", response)
	}
	byID := map[uint]dto.BatchCaseResult{}
	for _, result := range response.Results {
		byID[result.CaseID] = result
	}
	failed := byID[missingEvents.ID]
	if failed.Outcome != dto.BatchOutcomeFailed || failed.ErrorCode != CodeAlgorithmInput || failed.CaseStatus != string(constants.CaseDraft) {
		t.Fatalf("unexpected failed result: %+v", failed)
	}
	reloaded, err := store.Cases.Get(missingEvents.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.CaseStatus != constants.CaseDraft || reloaded.AnalysisError == "" {
		t.Fatalf("failed case must return to draft with error: %+v", reloaded)
	}
	var failedAudits int64
	store.DB.Model(&model.AuditLog{}).Where("action = ? AND resource_id = ?", "case.analysis_failed", missingEvents.ID).Count(&failedAudits)
	if failedAudits != 1 {
		t.Fatalf("expected one failure audit entry, got %d", failedAudits)
	}
}

func TestBatchAnalyzeRejectsConflictingSelectionsBeforeTouchingCases(t *testing.T) {
	store := newBatchTestStore(t)
	svc := NewCaseService(store)
	draft := seedCase(t, store, constants.CaseDraft)
	confirmed := seedCase(t, store, constants.CaseConfirmed)
	closed := seedCase(t, store, constants.CaseClosed)
	seedEvents(t, store, draft.BaselineTraceID, []float64{100})
	seedEvents(t, store, draft.CurrentTraceID, []float64{101})

	_, err := svc.BatchAnalyze(dto.BatchAnalyzeCasesRequest{CaseIDs: []uint{draft.ID, confirmed.ID, closed.ID, 999999, draft.ID}, DistanceToleranceM: 30, LossIncreaseDB: 1.2}, batchActor())
	var appErr *AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError, got %v", err)
	}
	if appErr.Code != CodeConflict || appErr.Status != http.StatusConflict {
		t.Fatalf("unexpected error: %+v", appErr)
	}
	details, ok := appErr.Details.(map[string]any)
	if !ok {
		t.Fatalf("missing conflict details: %+v", appErr.Details)
	}
	conflicts, _ := details["conflicts"].([]dto.BatchCaseConflict)
	reasons := map[uint]string{}
	for _, conflict := range conflicts {
		reasons[conflict.CaseID] = conflict.Reason
	}
	if reasons[confirmed.ID] != dto.BatchConflictNotDraft || reasons[closed.ID] != dto.BatchConflictNotDraft || reasons[999999] != dto.BatchConflictNotFound || reasons[draft.ID] != dto.BatchConflictDuplicate {
		t.Fatalf("unexpected conflicts: %v", reasons)
	}
	confirmedConflict := dto.BatchCaseConflict{}
	for _, conflict := range conflicts {
		if conflict.CaseID == confirmed.ID {
			confirmedConflict = conflict
		}
	}
	if confirmedConflict.Status != string(constants.CaseConfirmed) {
		t.Fatalf("conflict for confirmed case must carry current status: %+v", confirmedConflict)
	}
	for _, id := range []uint{draft.ID, confirmed.ID, closed.ID} {
		reloaded, getErr := store.Cases.Get(id)
		if getErr != nil {
			t.Fatal(getErr)
		}
		if reloaded.Version != 1 {
			t.Fatalf("case %d must not change during rejected batch, version=%d", id, reloaded.Version)
		}
	}
	var auditCount int64
	store.DB.Model(&model.AuditLog{}).Where("action LIKE ?", "case.batch_%").Count(&auditCount)
	if auditCount != 0 {
		t.Fatalf("rejected batch must not write batch audit entries, got %d", auditCount)
	}
}
