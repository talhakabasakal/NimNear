package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/application/nimiqtx/verification"
	"github.com/masterfabric-go/masterfabric/internal/domain/paymentrequest/model"
)

type reconciliationClassification string

const (
	classificationPaid             reconciliationClassification = "paid"
	classificationInvalid          reconciliationClassification = "invalid"
	classificationNotFinal         reconciliationClassification = "not_final"
	classificationNotFound         reconciliationClassification = "not_found_unresolved"
	classificationTransient        reconciliationClassification = "rpc_transient"
	classificationStaleExpired     reconciliationClassification = "stale_unresolved_expired"
	classificationStateUpdateError reconciliationClassification = "state_update_error"
	classificationClaimUnavailable reconciliationClassification = "claim_unavailable"
)

// ReconciliationResult is an operationally safe summary for structured logs.
type ReconciliationResult struct {
	RequestID uuid.UUID
	PublicID  uuid.UUID
	Before    model.Status
	After     model.Status
	Class     string
}

// ReconciliationReport describes one bounded reconciliation pass.
type ReconciliationReport struct {
	Selected int
	Results  []ReconciliationResult
}

func (uc *RequestUseCase) verifyAndPersist(ctx context.Context, request *model.Request) *model.Request {
	updated, _ := uc.reconcileRequest(ctx, request)
	return updated
}

func (uc *RequestUseCase) reconcileRequest(ctx context.Context, request *model.Request) (*model.Request, ReconciliationResult) {
	result := ReconciliationResult{}
	if request == nil {
		return request, result
	}
	result.RequestID = request.ID
	result.PublicID = request.PublicID
	result.Before = request.Status
	result.After = request.Status
	if uc.repo == nil || uc.verifier == nil || request.TransactionHash == nil ||
		(request.Status != model.StatusSubmitted && request.Status != model.StatusVerifying) {
		return request, result
	}
	if request.PayerUserID == nil || *request.PayerUserID == uuid.Nil {
		return uc.persistVerificationState(ctx, request, model.StatusFailed, result, classificationInvalid)
	}

	now := uc.now().UTC()
	stale := uc.reconciliationDeadlineElapsed(request, now)
	claimed, err := uc.repo.ClaimVerification(ctx, request.ID, now, uc.reconciliation.Interval, uc.reconciliation.Interval, stale)
	if err != nil {
		result.Class = string(classificationStateUpdateError)
		return request, result
	}
	if claimed == nil {
		result.Class = string(classificationClaimUnavailable)
		return request, result
	}

	outcome, verifyErr := uc.verifier.VerifyTransfer(ctx, verification.Transfer{
		TransactionHash: *claimed.TransactionHash,
		Recipient:       claimed.RecipientAddress,
		AmountLunas:     claimed.AmountLunas,
		PayerUserID:     *claimed.PayerUserID,
	})
	if verifyErr != nil {
		if stale {
			return uc.expireStale(ctx, claimed, result, classificationStaleExpired)
		}
		result.Class = string(classificationTransient)
		return request, result
	}

	switch outcome {
	case verification.OutcomeConfirmed:
		updated, result := uc.persistVerificationState(ctx, claimed, model.StatusPaid, result, classificationPaid)
		if updated != nil && updated.Status == model.StatusPaid {
			uc.log("payment_request_paid", "public_id", updated.PublicID, "status", updated.Status)
		}
		return updated, result
	case verification.OutcomeInvalid:
		return uc.persistVerificationState(ctx, claimed, model.StatusFailed, result, classificationInvalid)
	case verification.OutcomeNotFinal:
		if stale {
			return uc.expireStale(ctx, claimed, result, classificationStaleExpired)
		}
		return uc.persistVerificationState(ctx, claimed, model.StatusVerifying, result, classificationNotFinal)
	case verification.OutcomeNotFound:
		if stale {
			return uc.expireStale(ctx, claimed, result, classificationStaleExpired)
		}
		result.Class = string(classificationNotFound)
		return request, result
	default:
		if stale {
			return uc.expireStale(ctx, claimed, result, classificationStaleExpired)
		}
		result.Class = string(classificationTransient)
		return request, result
	}
}

func (uc *RequestUseCase) persistVerificationState(ctx context.Context, request *model.Request, status model.Status, result ReconciliationResult, class reconciliationClassification) (*model.Request, ReconciliationResult) {
	before := request.Status
	updated, err := uc.repo.SetVerificationState(ctx, request.ID, status, uc.now().UTC())
	if err != nil || updated == nil {
		result.Class = string(classificationStateUpdateError)
		return request, result
	}
	result.After = updated.Status
	result.Class = string(class)
	if updated.Status != before {
		uc.publishStatus(ctx, updated)
	}
	return updated, result
}

func (uc *RequestUseCase) expireStale(ctx context.Context, request *model.Request, result ReconciliationResult, class reconciliationClassification) (*model.Request, ReconciliationResult) {
	before := request.Status
	updated, err := uc.repo.ExpireUnresolved(ctx, request.ID, uc.now().UTC(), uc.reconciliation.Deadline)
	if err != nil || updated == nil {
		result.Class = string(classificationStateUpdateError)
		return request, result
	}
	result.After = updated.Status
	result.Class = string(class)
	if updated.Status != before {
		uc.publishStatus(ctx, updated)
	}
	return updated, result
}

func (uc *RequestUseCase) reconciliationDeadlineElapsed(request *model.Request, now time.Time) bool {
	deadline := request.UpdatedAt.Add(uc.reconciliation.Deadline)
	if request.ReconciliationDeadlineAt != nil {
		deadline = *request.ReconciliationDeadlineAt
	}
	return !now.Before(deadline)
}

// Reconcile processes at most the configured batch size and continues after an individual failure.
func (uc *RequestUseCase) Reconcile(ctx context.Context) (ReconciliationReport, error) {
	now := uc.now().UTC()
	candidates, err := uc.repo.ListReconciliationCandidates(ctx, now, uc.reconciliation.Interval, uc.reconciliation.Deadline, uc.reconciliation.BatchSize)
	if err != nil {
		return ReconciliationReport{}, err
	}
	report := ReconciliationReport{Selected: len(candidates), Results: make([]ReconciliationResult, 0, len(candidates))}
	for _, candidate := range candidates {
		if candidate == nil {
			continue
		}
		_, result := uc.reconcileRequest(ctx, candidate)
		report.Results = append(report.Results, result)
	}
	return report, nil
}
